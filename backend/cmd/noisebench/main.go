// Command noisebench measures how well Chaukanna works on NOISY phone calls.
//
// For every scenario in benchmark/cases it builds a realistic call:
//
//	Timbre speaks each line (a male "caller" voice where available)
//	  -> background noise mixed in at each SNR level
//	  -> squeezed to 8 kHz μ-law (what a phone line delivers)
//	  -> streamed to Prisma in real time
//	  -> judged by the same detector as the live server (+ Evon if configured)
//
// and reports, per noise level: scams caught, false alarms, time-to-detect, and
// how close the transcript stays to the clean one.
//
//	cd backend
//	go run ./cmd/noisebench                                  # street noise, all cases, 4 levels
//	go run ./cmd/noisebench -noise babble -snr clean,5,0
//	go run ./cmd/noisebench -noise ~/market.wav              # your own recording
//	go run ./cmd/noisebench -only scam-01,legit-02 -save-audio ../benchmark/noisy
//
// Uses Gnani credits: Timbre once per line (cached on disk), Prisma once per call per level.
package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"chaukanna/internal/audio"
	"chaukanna/internal/cases"
	"chaukanna/internal/config"
	"chaukanna/internal/detector"
	"chaukanna/internal/evaluate"
	"chaukanna/internal/noise"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/scam"
	"chaukanna/internal/stt"
	"chaukanna/internal/tts"
)

var (
	levelsFlag = flag.String("snr", "clean,10,5,0", "noise levels: 'clean' or SNR in dB (lower = noisier)")
	noiseFlag  = flag.String("noise", "street", "street | babble | white | path/to/recording.wav")
	onlyFlag   = flag.String("only", "", "comma-separated case id prefixes to run (default: all)")
	parallel   = flag.Int("parallel", 3, "calls streamed to Prisma at the same time")
	outFlag    = flag.String("out", "../benchmark/results/noise/noise-results.md", "markdown report")
	saveAudio  = flag.String("save-audio", "", "folder to save every noisy call as WAV (to listen to)")
	jsonFlag   = flag.String("json", "", "also write every call's transcript and verdict as JSON (for debugging misses)")
)

// maleVoices: Timbre v2.5 male voices, so the "caller" sounds like a typical scammer.
var maleVoices = map[string]string{
	"hi": "Deepak", "en": "Pranav", "bn": "Dhruva", "mr": "Ishaan", "kn": "Kavin",
	"ml": "Riyaan", "gu": "Veera", "pa": "Zayan",
}

type job struct {
	c     cases.Case
	level string
	call  []int16
}

func main() {
	flag.Parse()
	cfg := config.Load()
	if cfg.GnaniAPIKey == "" {
		log.Fatal("GNANI_API_KEY is empty in backend/.env")
	}
	timbre := tts.New(cfg.TimbreURL, cfg.GnaniAPIKey, cfg.TimbreModel, cfg.TimbreVoice, cfg.TimbreSampleRate)
	prisma := stt.New(cfg.PrismaWSURL, cfg.GnaniAPIKey, cfg.PrismaModel, cfg.PrismaSampleRate)
	evon := reasoner.New(cfg.EvonBaseURL, cfg.EvonModel, cfg.EvonAPIKey, cfg.EvonTimeout)
	detCfg := detector.Config{
		RiskSuspicious: cfg.RiskSuspicious, RiskCaution: cfg.RiskCaution, RiskScam: cfg.RiskScam, RiskClear: cfg.RiskClear,
		Window: cfg.EvonWindow, Interval: cfg.EvonInterval,
	}

	list := selectCases(cfg.BenchmarkDir, *onlyFlag)
	levels := strings.Split(*levelsFlag, ",")

	// 1) Build the clean call audio for every case (Timbre, cached on disk).
	fmt.Printf("Building %d calls with Timbre (%s)...\n", len(list), cfg.TimbreModel)
	cacheDir := filepath.Join(filepath.Dir(cfg.BenchmarkDir), ".tts-cache")
	clean := map[string][]int16{}
	var allClips [][]int16
	totalSec := 0.0
	for _, c := range list {
		call, err := buildCall(timbre, cfg.TimbreModel, cacheDir, c)
		if err != nil {
			log.Fatalf("%s: %v", c.ID, err)
		}
		clean[c.ID] = call
		if !c.IsScam() {
			// The background crowd must not contain scam words, or we would be
			// testing whether Chaukanna reacts to OTHER people's scam talk.
			allClips = append(allClips, call)
		}
		totalSec += audio.Duration(call, audio.PhoneRate)
	}
	streams := len(list) * len(levels)
	fmt.Printf("Streaming %d calls (%.1f min of audio) to Prisma, %d at a time, ~%.0f min...\n\n",
		streams, totalSec*float64(len(levels))/60, *parallel, (totalSec+4*float64(len(list)))*float64(len(levels))/60/float64(*parallel))

	// 2) Mix noise and run every (case, level) through Prisma + detector.
	var jobs []job
	for _, c := range list {
		for _, lvl := range levels {
			jobs = append(jobs, job{c: c, level: strings.TrimSpace(lvl), call: noisy(clean[c.ID], c.ID, strings.TrimSpace(lvl), allClips)})
		}
	}

	results := map[string]map[string]evaluate.Result{} // level -> case -> result
	var mu sync.Mutex
	work := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < max(1, *parallel); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				if *saveAudio != "" {
					_ = os.MkdirAll(*saveAudio, 0o755)
					_ = os.WriteFile(filepath.Join(*saveAudio, j.c.ID+"_"+j.level+".wav"), audio.EncodeWAV(j.call, audio.PhoneRate), 0o644)
				}
				lines, err := evaluate.Transcribe(prisma, j.call, j.c.Language)
				if err != nil {
					log.Printf("%s @ %s: prisma: %v", j.c.ID, j.level, err)
					continue
				}
				r := evaluate.Run(j.c, lines, detCfg, evon)
				mu.Lock()
				if results[j.level] == nil {
					results[j.level] = map[string]evaluate.Result{}
				}
				results[j.level][j.c.ID] = r
				mu.Unlock()
				mark := "✅"
				if !r.Correct() {
					mark = "❌"
				}
				fmt.Printf("%s %-7s %-40s risk=%.2f  heard: %s\n", mark, j.level, j.c.ID, r.FinalRisk, shorten(r.Transcript, 70))
			}
		}()
	}
	for _, j := range jobs {
		work <- j
	}
	close(work)
	wg.Wait()

	// 3) Report.
	report := buildReport(list, levels, results, cfg, evon.Enabled())
	fmt.Println("\n" + report)
	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(report), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Report written to %s\n", *outFlag)
	}
	if *jsonFlag != "" {
		data, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(*jsonFlag, data, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Details written to %s\n", *jsonFlag)
	}
}

// selectCases loads benchmark cases, optionally filtered by id prefix.
func selectCases(dir, only string) []cases.Case {
	all, err := cases.LoadDir(dir)
	if err != nil || len(all) == 0 {
		log.Fatalf("no cases in %s: %v", dir, err)
	}
	if only == "" {
		return all
	}
	var out []cases.Case
	for _, c := range all {
		for _, p := range strings.Split(only, ",") {
			if p = strings.TrimSpace(p); p != "" && strings.HasPrefix(c.ID, p) {
				out = append(out, c)
				break
			}
		}
	}
	if len(out) == 0 {
		log.Fatalf("no case matches -only %q", only)
	}
	return out
}

// buildCall speaks every line of a case and joins them with short pauses.
func buildCall(timbre *tts.Client, model, cacheDir string, c cases.Case) ([]int16, error) {
	voice := ""
	if model == "timbre-v2.5" {
		voice = maleVoices[scam.BaseLanguage(c.Language)]
	}
	pause := make([]int16, audio.PhoneRate*8/10) // 0.8 s between sentences
	call := make([]int16, audio.PhoneRate/2)     // 0.5 s before the first one
	for _, line := range c.Lines {
		clip, err := speak(timbre, cacheDir, line.Text, c.Language, voice)
		if err != nil {
			return nil, err
		}
		call = append(append(call, clip...), pause...)
	}
	return call, nil
}

// speak returns Timbre audio for one line, cached on disk so re-runs cost nothing.
func speak(timbre *tts.Client, dir, text, lang, voice string) ([]int16, error) {
	sum := sha1.Sum([]byte(voice + "|" + lang + "|" + text))
	path := filepath.Join(dir, hex.EncodeToString(sum[:8])+".wav")
	if data, err := os.ReadFile(path); err == nil {
		if samples, _, err := audio.ParseWAV(data); err == nil {
			return samples, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	samples, err := timbre.SynthesizeWithVoice(ctx, text, lang, voice)
	if err != nil {
		return nil, err
	}
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(path, audio.EncodeWAV(samples, audio.PhoneRate), 0o644)
	return samples, nil
}

// noisy mixes the chosen noise at the level and passes it through a phone line (μ-law).
func noisy(call []int16, id, level string, allClips [][]int16) []int16 {
	out := call
	if level != "clean" {
		snr, err := strconv.ParseFloat(level, 64)
		if err != nil {
			log.Fatalf("bad -snr level %q (use 'clean' or a number like 5)", level)
		}
		h := fnv.New64a()
		h.Write([]byte(id))
		seed := int64(h.Sum64())

		var n []float64
		switch *noiseFlag {
		case "street":
			n = noise.Street(len(call), seed)
		case "white":
			n = noise.White(len(call), seed)
		case "babble":
			n = noise.Babble(len(call), allClips, 4, seed)
		default:
			n, err = noise.FromWAV(*noiseFlag, len(call))
			if err != nil {
				log.Fatalf("noise file: %v", err)
			}
		}
		out = noise.Mix(call, n, snr)
	}
	return audio.MulawDecode(audio.MulawEncode(out))
}

// buildReport makes a markdown table per noise level plus a per-case grid.
func buildReport(list []cases.Case, levels []string, results map[string]map[string]evaluate.Result, cfg config.Config, evonOn bool) string {
	var b strings.Builder
	engine := "rules only"
	if evonOn {
		engine = "rules + Evon (" + cfg.EvonModel + ")"
	}
	fmt.Fprintf(&b, "## Noisy-call benchmark\n\n")
	fmt.Fprintf(&b, "%d calls · noise: **%s** · Timbre %s → 8 kHz μ-law phone line → Prisma %s · %s · %s\n\n",
		len(list), *noiseFlag, cfg.TimbreModel, cfg.PrismaModel, engine, time.Now().Format("2006-01-02"))
	b.WriteString("| Noise level | Scams caught | False alarms | Median time-to-detect | Transcript match vs clean |\n|---|---|---|---|---|\n")

	cleanText := map[string]string{}
	for id, r := range results["clean"] {
		cleanText[id] = r.Transcript
	}
	for _, lvl := range levels {
		lvl = strings.TrimSpace(lvl)
		var rs []evaluate.Result
		var sims []float64
		for _, c := range list {
			if r, ok := results[lvl][c.ID]; ok {
				rs = append(rs, r)
				if ref, ok := cleanText[c.ID]; ok && lvl != "clean" {
					sims = append(sims, similarity(ref, r.Transcript))
				}
			}
		}
		s := evaluate.Summarize(rs)
		detect, sim := "–", "–"
		if s.HasDetect {
			detect = fmt.Sprintf("%.1f s", s.MedianDetect)
		}
		if lvl == "clean" {
			sim = "100% (reference)"
		} else if len(sims) > 0 {
			sim = fmt.Sprintf("%.0f%%", 100*mean(sims))
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", levelName(lvl), s.Recall(), s.FPR(), detect, sim)
	}

	b.WriteString("\n<details><summary>Per call</summary>\n\n| Call | Label |")
	for _, lvl := range levels {
		fmt.Fprintf(&b, " %s |", strings.TrimSpace(lvl))
	}
	b.WriteString("\n|---|---|" + strings.Repeat("---|", len(levels)) + "\n")
	ids := make([]cases.Case, len(list))
	copy(ids, list)
	sort.Slice(ids, func(i, j int) bool { return ids[i].ID < ids[j].ID })
	for _, c := range ids {
		fmt.Fprintf(&b, "| %s | %s |", c.ID, c.Label)
		for _, lvl := range levels {
			r, ok := results[strings.TrimSpace(lvl)][c.ID]
			switch {
			case !ok:
				b.WriteString(" – |")
			case r.Correct():
				fmt.Fprintf(&b, " ✅ %.0f%% |", 100*r.FinalRisk)
			default:
				fmt.Fprintf(&b, " ❌ %.0f%% |", 100*r.FinalRisk)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\n</details>\n")
	return b.String()
}

func levelName(l string) string {
	if l == "clean" {
		return "Clean phone line"
	}
	names := map[string]map[string]string{
		"babble": {"10": "crowded room", "5": "crowded market", "0": "another voice as loud as the caller"},
		"white":  {"10": "light hiss", "5": "strong hiss", "0": "hiss as loud as the voice"},
	}
	byLevel, ok := names[*noiseFlag]
	if !ok { // street noise and recorded files
		byLevel = map[string]string{"10": "busy street", "5": "loud traffic", "0": "noise as loud as the voice"}
	}
	if name, ok := byLevel[l]; ok {
		return l + " dB (" + name + ")"
	}
	return l + " dB"
}

// similarity is 1 - (character edit distance / length), ignoring case and punctuation.
func similarity(a, b string) float64 {
	ra, rb := []rune(simplify(a)), []rune(simplify(b))
	if len(ra) == 0 && len(rb) == 0 {
		return 1
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return 1 - float64(prev[len(rb)])/float64(max(len(ra), len(rb)))
}

func simplify(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r))
	}), " ")
}

func mean(v []float64) float64 {
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
