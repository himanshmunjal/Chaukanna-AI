// Command warningcheck proves each spoken warning can be understood: Timbre speaks
// it over a phone-quality line, Prisma transcribes it back, and the two are compared.
//
//	cd backend && go run ./cmd/warningcheck
//
// A high match shows the audio is clear. It does NOT show the wording is natural,
// so a native speaker should still read the non-Hindi/English lines.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"chaukanna/internal/audio"
	"chaukanna/internal/config"
	"chaukanna/internal/scam"
	"chaukanna/internal/stt"
	"chaukanna/internal/tts"
)

func main() {
	cfg := config.Load()
	timbre := tts.New(cfg.TimbreURL, cfg.GnaniAPIKey, cfg.TimbreModel, cfg.TimbreVoice, cfg.TimbreSampleRate)
	prisma := stt.New(cfg.PrismaWSURL, cfg.GnaniAPIKey, cfg.PrismaModel, cfg.PrismaSampleRate)
	if !timbre.Enabled() || !prisma.Enabled() {
		log.Fatal("set GNANI_API_KEY in backend/.env")
	}

	fmt.Println("| Language | Warning match | Caution match | Prisma heard (warning) |\n|---|---|---|---|")
	for _, lang := range []string{"hi", "en", "ta", "te", "kn", "ml", "bn", "mr", "gu", "pa"} {
		w, heard, err := roundTrip(timbre, prisma, scam.DefaultWarning(lang), lang)
		if err != nil {
			log.Fatalf("%s: %v", lang, err)
		}
		c, _, err := roundTrip(timbre, prisma, scam.Caution(lang), lang)
		if err != nil {
			log.Fatalf("%s: %v", lang, err)
		}
		fmt.Printf("| %s | %.0f%% | %.0f%% | %s |\n", scam.LanguageName(lang), w*100, c*100, heard)
	}
	fmt.Println("\nPrisma writes English words in Latin script, which lowers the Hindi match even when the audio is perfect: read the transcript as well as the number.")
}

func roundTrip(timbre *tts.Client, prisma *stt.Client, text, lang string) (float64, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	samples, err := timbre.Synthesize(ctx, text, lang)
	if err != nil {
		return 0, "", err
	}
	heard, err := prisma.TranscribeFile(ctx, "warning.wav", audio.EncodeWAV(samples, audio.PhoneRate), lang)
	if err != nil {
		return 0, "", err
	}
	return similarity(scam.Spoken(text, lang), heard), heard, nil
}

// similarity is 1 - edit distance / length, over letters only (punctuation doesn't matter).
func similarity(a, b string) float64 {
	x, y := letters(a), letters(b)
	if len(x) == 0 || len(y) == 0 {
		return 0
	}
	prev := make([]int, len(y)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(x); i++ {
		cur := make([]int, len(y)+1)
		cur[0] = i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return 1 - float64(prev[len(y)])/float64(max(len(x), len(y)))
}

func letters(s string) []rune {
	var out []rune
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r) {
			out = append(out, r)
		}
	}
	return out
}
