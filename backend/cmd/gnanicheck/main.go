// Command gnanicheck verifies your Gnani setup end to end (uses ~30 s of credits):
//
//	For each language (Hindi, English, Tamil, Bengali, Marathi):
//	  Timbre: text -> speech
//	  Phone:  speech squeezed to 8 kHz μ-law (exactly what a phone line delivers)
//	  Prisma: that phone audio -> text, streamed in real time
//	Evon:   the Hindi transcript -> scam verdict (only if EVON_BASE_URL is set)
//
//	cd backend && go run ./cmd/gnanicheck              # all languages
//	cd backend && go run ./cmd/gnanicheck -langs hi    # just Hindi
//
// It also saves gnanicheck.wav (the Hindi scam sentence), used by cmd/smoketest.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"chaukanna/internal/audio"
	"chaukanna/internal/config"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/scam"
	"chaukanna/internal/stt"
	"chaukanna/internal/tts"
)

// sentences: a scam line in each language (romanised words like OTP are normal in real calls).
var sentences = map[string]string{
	"hi": "नमस्ते, मैं सीबीआई से बोल रहा हूँ। आपके नाम पर अरेस्ट वारंट है, अपना ओटीपी बताइए।",
	"en": "Hello madam, I am calling from the customs department. A parcel in your name has drugs. Please share the OTP now.",
	"ta": "வணக்கம், நான் வங்கியில் இருந்து பேசுகிறேன். உங்கள் கணக்கு முடக்கப்படும், உடனே OTP சொல்லுங்கள்.",
	"bn": "নমস্কার, আমি পুলিশ থেকে বলছি। আপনার নামে গ্রেফতারি পরোয়ানা আছে, এখনই OTP বলুন।",
	"mr": "नमस्कार, मी पोलीस स्टेशनमधून बोलतोय. तुमच्या नावावर अटक वॉरंट आहे, लगेच OTP सांगा.",
}

func main() {
	langs := flag.String("langs", "hi,en,ta,bn,mr", "comma-separated language codes to test")
	flag.Parse()

	cfg := config.Load()
	if cfg.GnaniAPIKey == "" {
		log.Fatal("GNANI_API_KEY is empty in backend/.env")
	}
	timbre := tts.New(cfg.TimbreURL, cfg.GnaniAPIKey, cfg.TimbreModel, cfg.TimbreVoice, cfg.TimbreSampleRate)
	prisma := stt.New(cfg.PrismaWSURL, cfg.GnaniAPIKey, cfg.PrismaModel, cfg.PrismaSampleRate)
	fmt.Printf("Timbre %s  ·  Prisma %s  ·  audio over a simulated 8 kHz μ-law phone line\n\n", cfg.TimbreModel, cfg.PrismaModel)

	var hindiTranscript []string
	passed, total := 0, 0
	for _, lang := range strings.Split(*langs, ",") {
		lang = strings.TrimSpace(lang)
		text, ok := sentences[lang]
		if !ok {
			fmt.Printf("[%s] no test sentence, skipping\n", lang)
			continue
		}
		total++
		fmt.Printf("── %s ──\n   said:  %s\n", scam.LanguageName(lang), text)

		// 1) Timbre
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		started := time.Now()
		samples, err := timbre.Synthesize(ctx, text, lang)
		cancel()
		if err != nil {
			fmt.Printf("   ❌ Timbre: %v\n\n", err)
			continue
		}
		ttsTime := time.Since(started)
		if lang == "hi" {
			_ = os.WriteFile("gnanicheck.wav", audio.EncodeWAV(samples, audio.PhoneRate), 0o644)
		}

		// 2) Phone line: μ-law round trip, like Twilio.
		phone := audio.MulawDecode(audio.MulawEncode(samples))

		// 3) Prisma
		started = time.Now()
		heard, err := transcribe(prisma, phone, lang)
		if err != nil {
			fmt.Printf("   ❌ Prisma: %v\n\n", err)
			continue
		}
		fmt.Printf("   heard: %s\n", strings.Join(heard, " "))
		fmt.Printf("   ✅ Timbre %.1f s audio in %s · Prisma round trip %s\n\n",
			audio.Duration(samples, audio.PhoneRate), ttsTime.Round(time.Millisecond), time.Since(started).Round(100*time.Millisecond))
		if len(heard) > 0 {
			passed++
		}
		if lang == "hi" {
			hindiTranscript = heard
		}
	}
	fmt.Printf("Timbre + Prisma: %d/%d languages OK\n\n", passed, total)

	checkEvon(cfg, hindiTranscript)
}

// transcribe streams phone audio to Prisma at real-time pace and returns the final transcripts.
func transcribe(prisma *stt.Client, samples []int16, lang string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	stream, err := prisma.Open(ctx, lang)
	if err != nil {
		return nil, err
	}
	done := make(chan []string)
	go func() {
		var texts []string
		for t := range stream.Results() {
			texts = append(texts, t.Text)
		}
		done <- texts
	}()

	// 20 ms frames like a phone call, then 1.5 s of silence so Prisma closes the segment.
	padded := append(samples, make([]int16, audio.PhoneRate*3/2)...)
	for i := 0; i < len(padded); i += 160 {
		if err := stream.Send(padded[i:min(i+160, len(padded))]); err != nil {
			return nil, err
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)
	stream.Close()
	return <-done, nil
}

// checkEvon sends the Hindi transcript to Evon, if Evon is configured.
func checkEvon(cfg config.Config, transcript []string) {
	evon := reasoner.New(cfg.EvonBaseURL, cfg.EvonModel, cfg.EvonAPIKey, cfg.EvonTimeout)
	if !evon.Enabled() {
		fmt.Println("Evon: skipped (set EVON_BASE_URL and EVON_MODEL in backend/.env)")
		return
	}
	if len(transcript) == 0 {
		transcript = []string{sentences["hi"]}
	}
	fmt.Printf("── Evon (%s) ──\n", cfg.EvonModel)
	var window []scam.Line
	for i, t := range transcript {
		window = append(window, scam.Line{T: float64(3 + i*5), Text: t})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	started := time.Now()
	a, err := evon.AnalyzeGuardian(ctx, reasoner.GuardianInput{Window: window, UserLanguage: "hi", ElapsedSec: 10})
	if err != nil {
		fmt.Printf("   ❌ Evon failed: %v\n", err)
		return
	}
	fmt.Printf("   risk=%.2f  stage=%s  (%s)\n", a.Risk, a.Stage, time.Since(started).Round(time.Millisecond))
	for _, f := range a.Flags {
		fmt.Printf("   🚩 %s: %q\n", f.Type, f.Quote)
	}
	fmt.Printf("   reason:  %s\n   warning: %s\n", a.Reason, a.WarningText)
	if a.Risk >= 0.6 {
		fmt.Println("   ✅ Evon works and flags the scam")
	} else {
		fmt.Println("   ⚠️  Evon answered but gave a low risk; check the prompt / model")
	}
}
