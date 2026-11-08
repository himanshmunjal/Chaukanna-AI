// Command benchmark measures how well Chaukanna detects scams.
//
//	cd backend
//	go run ./cmd/benchmark                       # text scripts, Evon if configured
//	go run ./cmd/benchmark -audio                # use recorded audio through Prisma when a case has one
//	go run ./cmd/benchmark -out ../benchmark/results/results.json
//	go run ./cmd/benchmark -out ../benchmark/results/results.json -resume   # continue a run stopped by a rate limit
//
// For each scenario in benchmark/cases it replays the conversation through the
// same detector the live server uses, on a simulated clock, and reports:
//   - recall / false-positive rate (was a scam detected?)
//   - time-to-detect (how many seconds into the call?)
//   - LLM latency and failed requests (if an LLM is configured)
//
// For noisy-audio robustness see cmd/noisebench.
// Only report numbers you actually measured with these tools in your submission.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"chaukanna/internal/audio"
	"chaukanna/internal/cases"
	"chaukanna/internal/config"
	"chaukanna/internal/detector"
	"chaukanna/internal/evaluate"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/scam"
	"chaukanna/internal/stt"
)

func main() {
	useAudio := flag.Bool("audio", false, "transcribe the case's audio file with Prisma instead of using its text lines")
	outPath := flag.String("out", "", "write detailed results as JSON to this file")
	totalsOnly := flag.Bool("totals-only", false, "hide per-call lines, so a held-out set isn't tuned on by accident")
	mode := flag.String("mode", detector.ModelAdds, "how the model and rules combine: adds (as on live calls), decides, or only (no rules)")
	resume := flag.Bool("resume", false, "keep the results already in -out and run only the remaining cases")
	flag.Parse()

	cfg := config.Load()
	evon := reasoner.New(cfg.EvonBaseURL, cfg.EvonModel, cfg.EvonAPIKey, cfg.EvonTimeout)
	evon.WaitOnRateLimit(5 * time.Minute) // a rate limit must not turn into a rules verdict
	prisma := stt.New(cfg.PrismaWSURL, cfg.GnaniAPIKey, cfg.PrismaModel, cfg.PrismaSampleRate)
	detCfg := detector.Config{
		RiskSuspicious: cfg.RiskSuspicious, RiskCaution: cfg.RiskCaution, RiskScam: cfg.RiskScam, RiskClear: cfg.RiskClear,
		Window: cfg.EvonWindow, Interval: cfg.EvonInterval, ModelMode: *mode,
	}
	if *mode != detector.ModelAdds && *mode != detector.ModelDecides && *mode != detector.ModelOnly {
		log.Fatalf("unknown -mode %q: use decides, adds or only", *mode)
	}
	if *mode == detector.ModelOnly && !evon.Enabled() {
		log.Fatal("-mode only needs a model: set EVON_BASE_URL and EVON_MODEL")
	}
	fmt.Printf("Model mode: %s\n", *mode)

	list, err := cases.LoadDir(cfg.BenchmarkDir)
	if err != nil {
		log.Fatalf("could not read scenarios in %s: %v", cfg.BenchmarkDir, err)
	}
	if len(list) == 0 {
		log.Fatalf("no scenario files (*.json) in %s\n"+
			"  - for the held-out set, add calls first:  go run ./cmd/exportcalls -scam 14,15 -legit 16\n"+
			"  - BENCHMARK_DIR in backend/.env should normally be ../benchmark/cases", cfg.BenchmarkDir)
	}
	engine := "rule-based pre-filter only (no LLM configured)"
	if evon.Enabled() {
		engine = "pre-filter + LLM (" + cfg.EvonModel + ")"
		if *mode == detector.ModelOnly {
			engine = "LLM only, no rules (" + cfg.EvonModel + ")"
		}
	}
	fmt.Printf("Running %d cases with %s\n\n", len(list), engine)

	var results []evaluate.Result
	finished := map[string]bool{}
	if *resume {
		data, err := os.ReadFile(*outPath)
		if err != nil {
			log.Fatalf("-resume needs an existing -out file: %v", err)
		}
		if err := json.Unmarshal(data, &results); err != nil {
			log.Fatalf("could not read %s: %v", *outPath, err)
		}
		for _, r := range results {
			finished[r.ID] = true
		}
		fmt.Printf("Resuming: %d cases already finished\n\n", len(results))
	}

	stopped := false
	for _, c := range list {
		if finished[c.ID] {
			continue
		}
		lines := c.Lines
		if *useAudio && c.Audio != "" {
			if l, err := transcribeFile(prisma, c.Audio, c.Language); err != nil {
				log.Printf("%s: audio transcription failed, using text lines: %v", c.ID, err)
			} else {
				lines = l
			}
		}
		r := evaluate.Run(c, lines, detCfg, evon)
		if r.RateLimited {
			// Rules would have to decide the remaining calls, which would not be an LLM score.
			fmt.Printf("\n🛑 Stopped at %s: %s\n", c.ID, r.EvonErrors[len(r.EvonErrors)-1])
			stopped = true
			break
		}
		results = append(results, r)
		mark := "✅"
		if !r.Correct() {
			mark = "❌"
		}
		if !*totalsOnly {
			fmt.Printf("%s %-38s label=%-5s detected=%-5v at=%5.1fs risk=%.2f llm_failures=%d\n", mark, c.ID, c.Label, r.Detected, r.DetectedAt, r.FinalRisk, r.EvonFailures)
		}
		for _, e := range r.EvonErrors {
			fmt.Printf("     llm error: %s\n", e)
		}
	}

	s := evaluate.Summarize(results)
	fmt.Println("\n================ Summary ================")
	fmt.Printf("Scam calls:          %d   Legit calls: %d\n", s.Scams, s.Legit)
	fmt.Printf("Recall (scams caught):   %s, %s\n", s.Recall(), evaluate.RangeText(s.TP, s.Scams))
	fmt.Printf("False-positive rate:     %s, %s\n", s.FPR(), evaluate.RangeText(s.FP, s.Legit))
	fmt.Printf("Precision:               %s\n", s.Precision())
	fmt.Printf("Scams warned or cautioned:  %s\n", evaluate.Pct(s.CautionedScams, s.Scams))
	fmt.Printf("Genuine calls cautioned:    %s\n", evaluate.Pct(s.CautionedLegit, s.Legit))
	if s.HasDetect {
		fmt.Printf("Median time-to-detect:   %.1f s\n", s.MedianDetect)
		fmt.Printf("p95 time-to-detect:      %.1f s\n", s.P95Detect)
	}
	if s.HasEvon {
		fmt.Printf("LLM median latency:      %.0f ms\n", s.EvonMedianMs)
	}
	if evon.Enabled() {
		fmt.Printf("LLM requests:            %d ok, %d failed\n", s.EvonCalls, s.EvonFailures)
		if s.EvonFailures > 0 {
			fmt.Println("⚠️  Failed requests used rules instead, so this is not a pure LLM score.")
		}
	}

	printByScamType(list, results)

	if *outPath != "" {
		data, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(*outPath, data, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\nDetailed results written to %s\n", *outPath)
	}
	if stopped {
		log.Fatalf("INCOMPLETE: %d of %d cases finished. Wait for the limit to reset, then rerun with -resume.", len(results), len(list))
	}
}

// printByScamType shows how each family of scam did, and how often Chaukanna named the
// family correctly. It appears only when the calls were labelled with their type.
func printByScamType(list []cases.Case, results []evaluate.Result) {
	truth := map[string]string{}
	for _, c := range list {
		if c.Category != "" {
			truth[c.ID] = c.Category
		}
	}
	type tally struct{ total, caught, named int }
	byType := map[string]*tally{}
	for _, r := range results {
		category, ok := truth[r.ID]
		if !ok || r.Label != "scam" {
			continue
		}
		t := byType[category]
		if t == nil {
			t = &tally{}
			byType[category] = t
		}
		t.total++
		if r.Detected {
			t.caught++
		}
		if r.ScamType == category {
			t.named++
		}
	}
	if len(byType) == 0 {
		return
	}
	names := make([]string, 0, len(byType))
	for name := range byType {
		names = append(names, name)
	}
	sort.Strings(names)
	var total, named int
	fmt.Println("\n============= By scam type =============")
	fmt.Printf("%-34s %-10s %s\n", "Type", "Caught", "Type named correctly")
	for _, name := range names {
		t := byType[name]
		label := scam.CategoryLabels[scam.Category(name)]
		if label == "" {
			label = name
		}
		fmt.Printf("%-34s %-10s %d/%d\n", label, fmt.Sprintf("%d/%d", t.caught, t.total), t.named, t.total)
		total += t.total
		named += t.named
	}
	fmt.Printf("\nType named correctly overall: %s\n", evaluate.Pct(named, total))
}

// transcribeFile streams a recorded WAV through Prisma like a phone call.
func transcribeFile(prisma *stt.Client, path, language string) ([]scam.Line, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	samples, rate, err := audio.ParseWAV(data)
	if err != nil {
		return nil, err
	}
	return evaluate.Transcribe(prisma, audio.Resample(samples, rate, audio.PhoneRate), language)
}
