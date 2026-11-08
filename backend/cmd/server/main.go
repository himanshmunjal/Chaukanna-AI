// Command server is the Chaukanna backend.
//
//	cd backend && go run ./cmd/server
//
// It reads backend/.env, opens the database, wires every service together
// and starts the Gin HTTP server.
package main

import (
	"context"
	"log"
	"time"

	"chaukanna/internal/alerts"
	"chaukanna/internal/api"
	"chaukanna/internal/clips"
	"chaukanna/internal/config"
	"chaukanna/internal/database"
	"chaukanna/internal/detector"
	"chaukanna/internal/drill"
	"chaukanna/internal/gnanihttp"
	"chaukanna/internal/hub"
	"chaukanna/internal/models"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/scam"
	"chaukanna/internal/session"
	"chaukanna/internal/stt"
	"chaukanna/internal/telegram"
	"chaukanna/internal/telephony"
	"chaukanna/internal/tts"
	"chaukanna/internal/twilio"

	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	// --- Storage ---
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	database.SeedDefaultUser(db, cfg)
	// A call open across a restart can never finish now, so don't show it as running.
	if n := db.Model(&models.Call{}).Where("ended_at IS NULL").Update("ended_at", time.Now()).RowsAffected; n > 0 {
		log.Printf("startup: closed %d call(s) left open by the last run", n)
	}

	// --- External services (each one is optional; missing keys = feature disabled) ---
	prisma := stt.New(cfg.PrismaWSURL, cfg.GnaniAPIKey, cfg.PrismaModel, cfg.PrismaSampleRate)
	timbre := tts.New(cfg.TimbreURL, cfg.GnaniAPIKey, cfg.TimbreModel, cfg.TimbreVoice, cfg.TimbreSampleRate)
	evon := reasoner.New(cfg.EvonBaseURL, cfg.EvonModel, cfg.EvonAPIKey, cfg.EvonTimeout)
	if !cfg.LLMPlusRules {
		log.Println("startup: LLM_PLUS_RULES=false, so calls run on the rules only")
	} else if evon.Enabled() {
		log.Printf("startup: rules + LLM (%s)", cfg.EvonModel)
	}
	twilioClient := twilio.New(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioPhoneNumber)
	telegramClient := telegram.New(cfg.TelegramBotToken)
	notifier := alerts.New(db, alerts.WhatsAppConfig{
		Token:         cfg.WhatsAppToken,
		PhoneNumberID: cfg.WhatsAppPhoneNumberID,
		APIVersion:    cfg.WhatsAppAPIVersion,
	}, twilioClient, telegramClient)

	status := map[string]bool{
		"prisma":     prisma.Enabled(),
		"timbre":     timbre.Enabled(),
		"evon":       evon.Enabled(),
		"twilio":     twilioClient.Enabled(),
		"whatsapp":   cfg.WhatsAppToken != "" && cfg.WhatsAppPhoneNumberID != "",
		"telegram":   telegramClient.Enabled(),
		"public_url": cfg.PublicURL != "",
	}
	for name, ok := range status {
		if !ok {
			log.Printf("startup: %-10s not configured (see backend/.env)", name)
		}
	}

	// --- Instant warnings: synthesise the phrases we always need, in the background ---
	if timbre.Enabled() {
		go prewarmVoice(db, timbre, cfg.DemoMode)
	}

	// --- Live call engine ---
	h := hub.New()
	clipStore := clips.New()
	sessions := session.NewManager(
		session.Deps{
			DB:       db,
			Hub:      h,
			Evon:     evon,
			TTS:      timbre,
			Alerts:   notifier,
			Clips:    clipStore,
			Callback: twilioClient,
		},
		session.Settings{
			Detector: detector.Config{
				RiskSuspicious: cfg.RiskSuspicious,
				RiskCaution:    cfg.RiskCaution,
				RiskScam:       cfg.RiskScam,
				RiskClear:      cfg.RiskClear,
				Window:         cfg.EvonWindow,
				Interval:       cfg.EvonInterval,
			},
			WarningCooldown:  cfg.WarningCooldown,
			StoreTranscripts: cfg.StoreTranscripts,
			ScreenMaxTurns:   cfg.ScreenMaxTurns,
			PublicURL:        cfg.PublicURL,
			DashboardURL:     cfg.DashboardURL,
		},
	)
	tel := telephony.New(sessions, prisma, twilioClient, cfg.PublicURL, telephony.Options{
		TestMode:    cfg.TelephonyTestMode,
		ExotelToken: cfg.ExotelStreamToken,
		DemoTTS:     timbre,
	})
	warnIfExposed(cfg)

	// --- HTTP ---
	go telegramClient.Listen(context.Background(), api.TelegramLinker(db))
	go purgeOldTranscripts(db, cfg.RetentionDays)
	if cfg.DemoMode {
		go purgeDemoCalls(db)
	}

	router := api.NewRouter(cfg, db, h, sessions, tel, clipStore, status, api.Services{
		Evon: evon, Prisma: prisma, TTS: timbre, Telegram: telegramClient,
		Detector: detector.Config{
			RiskSuspicious: cfg.RiskSuspicious, RiskCaution: cfg.RiskCaution, RiskScam: cfg.RiskScam,
			RiskClear: cfg.RiskClear, Window: cfg.EvonWindow, Interval: cfg.EvonInterval,
		},
	})
	log.Printf("🛡️  Chaukanna backend listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

// purgeOldTranscripts deletes transcripts and quotes past the retention period, once
// at startup and then daily. Risk scores and flag types stay, so the stats still work.
func purgeOldTranscripts(db *gorm.DB, days int) {
	if days <= 0 {
		return
	}
	for {
		cutoff := time.Now().AddDate(0, 0, -days)
		lines := db.Where("created_at < ?", cutoff).Delete(&models.TranscriptLine{}).RowsAffected
		quotes := db.Model(&models.RedFlag{}).Where("created_at < ? AND quote <> ''", cutoff).Update("quote", "").RowsAffected
		if lines+quotes > 0 {
			log.Printf("retention: removed %d transcript lines and %d quotes older than %d days", lines, quotes, days)
		}
		time.Sleep(24 * time.Hour)
	}
}

// demoCallLife is how long a visitor's call stays around.
const demoCallLife = time.Hour

// purgeDemoCalls deletes visitors' calls, and everything said on them, after an hour.
func purgeDemoCalls(db *gorm.DB) {
	for {
		var ids []uint
		db.Model(&models.Call{}).Where("demo_owner <> '' AND started_at < ?", time.Now().Add(-demoCallLife)).Pluck("id", &ids)
		if len(ids) > 0 {
			db.Where("call_id IN ?", ids).Delete(&models.TranscriptLine{})
			db.Where("call_id IN ?", ids).Delete(&models.RedFlag{})
			db.Where("call_id IN ?", ids).Delete(&models.Alert{})
			db.Where("id IN ?", ids).Delete(&models.Call{})
			log.Printf("demo: removed %d old call(s)", len(ids))
		}
		time.Sleep(10 * time.Minute)
	}
}

// warnIfExposed says so at startup when a deployment looks unprotected.
func warnIfExposed(cfg config.Config) {
	public := cfg.PublicURL != "" || cfg.DemoMode
	if public && cfg.DashboardPassword == "" && cfg.DashboardToken == "" && !cfg.DemoMode {
		log.Println("⚠️  PUBLIC_URL is set but DASHBOARD_PASSWORD is not: anyone with the address can open the dashboard")
	}
	if cfg.DemoMode && cfg.DashboardPassword == "" {
		log.Println("demo mode: no DASHBOARD_PASSWORD, so nobody can log in as family on this deployment")
	}
	if cfg.EncryptionKey == "" {
		log.Println("startup: ENCRYPTION_KEY is not set: call transcripts are stored readable")
	}
	if cfg.TelephonyTestMode {
		log.Println("⚠️  TELEPHONY_TEST_MODE is on: telephony streams aren't verified. Never use this on a public server")
	}
}

// prewarmVoice generates the standard warning (and the Screen Mode greeting,
// follow-up and goodbye) for every protected user's language, so the first
// warning on a live call plays instantly instead of waiting ~1-2 s for Timbre.
func prewarmVoice(db *gorm.DB, timbre *tts.Client, demo bool) {
	started := time.Now()
	var users []models.ProtectedUser
	db.Find(&users)

	// Only the languages actually needed: every clip spends Timbre credits on each start.
	langs := map[string]bool{}
	if len(users) == 0 {
		langs["hi"] = true
	}
	if demo { // visitors without a key hear only what is prepared now
		langs["hi"], langs["en"] = true, true
	}
	var items [][2]string
	for _, u := range users {
		langs[scam.BaseLanguage(u.Language)] = true
		items = append(items, [2]string{scam.Greeting(u.Language, u.Name), u.Language})
	}
	for lang := range langs {
		for _, w := range scam.AllWarnings(lang) {
			items = append(items, [2]string{w, lang})
		}
		items = append(items,
			[2]string{scam.FollowUp(lang), lang},
			[2]string{scam.Closing(lang), lang},
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	ok := timbre.Prewarm(ctx, items)
	log.Printf("startup: %d/%d voice clips ready for instant playback (%s)", ok, len(items), time.Since(started).Round(time.Millisecond))

	if demo {
		// The demo's greeting and Scam Drill's caller lines.
		greetings := [][2]string{{scam.Greeting("hi", models.DemoUserName), "hi"}, {scam.Greeting("en", models.DemoUserName), "en"}}
		timbre.Prewarm(ctx, greetings)
		for _, sc := range drill.Scenarios {
			for _, step := range sc.Steps {
				if _, err := timbre.SynthesizeCaller(gnanihttp.Background(ctx), step.Line, sc.Lang); err != nil {
					log.Printf("startup: drill clip: %v", err)
				}
			}
		}
	}

	// Family voices last: they take longest, and the stock clips cover the meantime.
	for _, u := range users {
		if u.VoiceEmbedding != "" {
			clones, cancelClones := context.WithTimeout(context.Background(), 10*time.Minute)
			n := timbre.PrewarmCloned(clones, scam.AllWarnings(u.Language), u.Language, u.VoiceEmbedding)
			cancelClones()
			log.Printf("startup: %d warnings ready in %s's voice for %s", n, u.VoiceOwner, u.Name)
		}
	}
}
