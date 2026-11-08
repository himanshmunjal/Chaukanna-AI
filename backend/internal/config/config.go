// Package config loads every runtime setting from environment variables.
//
// Values are read from backend/.env (if it exists) and then from the real
// environment, so you can override anything with `export KEY=value`.
// Every setting has a safe default so the server can start with an empty .env
// (useful for running the simulator before you have API keys).
package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all settings for the backend.
type Config struct {
	// --- Server ---
	Port              string   // HTTP port, e.g. "8080"
	PublicURL         string   // Public https URL of this backend (ngrok). Twilio/Exotel connect here.
	DashboardURL      string   // URL of the React dashboard. Used in alert links.
	DashboardToken    string   // Optional shared secret for scripts and the API. Empty = no token.
	DashboardPassword string   // Login password for the family dashboard. Empty = no login page.
	DemoMode          bool     // Public demo: visitors are isolated, spend nothing, and alert nobody.
	TrustedProxies    []string // Proxies whose X-Forwarded-For is believed (for rate limits behind a host).
	EncryptionKey     string   // Passphrase that encrypts what was said on calls. Empty = stored readable.
	TelephonyTestMode bool     // Accept unverified telephony streams. Only for cmd/smoketest on a laptop.
	ExotelStreamToken string   // Secret in the Exotel stream URL. Empty = Exotel is off.
	CORSOrigins       []string // Allowed browser origins for the API.

	// --- Database ---
	DatabaseURL string // Postgres DSN. Leave empty to use SQLite.
	SQLitePath  string // SQLite file path (used when DatabaseURL is empty).

	// --- Gnani AI: Prisma (speech-to-text) and Timbre (text-to-speech) ---
	GnaniAPIKey      string
	PrismaWSURL      string // Streaming STT WebSocket URL
	PrismaModel      string // prisma-v3.0 | prisma-v2.5
	PrismaSampleRate int    // Sample rate we send to Prisma (8000 = phone audio, no resampling)
	TimbreURL        string // TTS HTTP endpoint
	TimbreModel      string // timbre-v2.5 | timbre-v3.0
	TimbreVoice      string // Voice name; empty = pick one for the user's language
	TimbreSampleRate int    // Sample rate of raw PCM returned by Timbre (ignored for WAV)

	// --- Gnani AI: Evon (reasoning model behind an OpenAI-compatible server such as vLLM) ---
	LLMPlusRules bool // false: rules only. true: rules, plus the model below if it is configured
	EvonBaseURL  string
	EvonModel    string
	EvonAPIKey   string
	EvonTimeout  time.Duration

	// --- Twilio (telephony + SMS + screen-mode callback) ---
	TwilioAccountSID  string
	TwilioAuthToken   string
	TwilioPhoneNumber string

	// --- WhatsApp Cloud API (family alerts) ---
	WhatsAppToken         string
	WhatsAppPhoneNumberID string
	WhatsAppAPIVersion    string

	// --- Telegram (free family alerts) ---
	TelegramBotToken string

	// --- Default protected user (created on first start) ---
	DefaultUserName     string
	DefaultUserPhone    string
	DefaultUserLanguage string
	DefaultFamilyName   string
	DefaultFamilyPhone  string

	// --- Detection tuning ---
	RiskSuspicious   float64       // risk at which a call becomes "suspicious"
	RiskCaution      float64       // risk at which a soft "be careful" is spoken (no family alert)
	RiskScam         float64       // risk at which a call becomes "scam" and the warning plays
	RiskClear        float64       // risk below which a suspicious call calms down
	EvonWindow       time.Duration // how much recent transcript Evon sees
	EvonInterval     time.Duration // how often Evon re-checks a live call
	WarningCooldown  time.Duration // minimum gap between two spoken warnings
	StoreTranscripts string        // "all" | "flagged" | "none"
	RetentionDays    int           // transcripts older than this are deleted; 0 = keep
	ScreenMaxTurns   int           // max questions the screening bot asks

	// --- Misc ---
	BenchmarkDir string // folder with scenario JSON files (used by the simulator)
	WebDist      string // built dashboard (web/dist) to serve from this binary; empty = don't
}

// Load reads the configuration. It never fails: missing values get defaults.
func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("config: no .env file found, using environment variables only")
	}

	// With this off, nothing below gets a model, so every call runs on the rules alone.
	llmPlusRules := getString("LLM_PLUS_RULES", "true") != "false"
	modelSetting := func(key string) string {
		if !llmPlusRules {
			return ""
		}
		return getString(key, "")
	}

	return Config{
		Port:              getString("PORT", "8080"),
		PublicURL:         strings.TrimRight(getString("PUBLIC_URL", ""), "/"),
		DashboardURL:      strings.TrimRight(getString("DASHBOARD_URL", "http://localhost:5173"), "/"),
		DashboardToken:    getString("DASHBOARD_TOKEN", ""),
		DashboardPassword: getString("DASHBOARD_PASSWORD", ""),
		DemoMode:          getString("DEMO_MODE", "") == "true",
		TrustedProxies:    getList("TRUSTED_PROXIES", nil),
		EncryptionKey:     getString("ENCRYPTION_KEY", ""),
		TelephonyTestMode: getString("TELEPHONY_TEST_MODE", "") == "true",
		ExotelStreamToken: getString("EXOTEL_STREAM_TOKEN", ""),
		CORSOrigins:       getList("CORS_ORIGINS", []string{"http://localhost:5173"}),

		DatabaseURL: getString("DATABASE_URL", ""),
		SQLitePath:  getString("SQLITE_PATH", "chaukanna.db"),

		GnaniAPIKey:      getString("GNANI_API_KEY", ""),
		PrismaWSURL:      getString("PRISMA_WS_URL", "wss://api.vachana.ai/stt/v3/stream"),
		PrismaModel:      getString("PRISMA_MODEL", "prisma-v3.0"),
		PrismaSampleRate: getInt("PRISMA_SAMPLE_RATE", 8000),
		TimbreURL:        getString("TIMBRE_URL", "https://api.vachana.ai/api/v1/tts/inference"),
		TimbreModel:      getString("TIMBRE_MODEL", "timbre-v2.5"),
		TimbreVoice:      getString("TIMBRE_VOICE", ""),
		TimbreSampleRate: getInt("TIMBRE_SAMPLE_RATE", 16000),

		LLMPlusRules: llmPlusRules,
		EvonBaseURL:  strings.TrimRight(modelSetting("EVON_BASE_URL"), "/"),
		EvonModel:    modelSetting("EVON_MODEL"),
		EvonAPIKey:   getString("EVON_API_KEY", ""),
		EvonTimeout:  getSeconds("EVON_TIMEOUT_SECONDS", 15),

		TwilioAccountSID:  getString("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:   getString("TWILIO_AUTH_TOKEN", ""),
		TwilioPhoneNumber: getString("TWILIO_PHONE_NUMBER", ""),

		WhatsAppToken:         getString("WHATSAPP_TOKEN", ""),
		WhatsAppPhoneNumberID: getString("WHATSAPP_PHONE_NUMBER_ID", ""),
		WhatsAppAPIVersion:    getString("WHATSAPP_API_VERSION", "v21.0"),

		TelegramBotToken: getString("TELEGRAM_BOT_TOKEN", ""),

		DefaultUserName:     getString("DEFAULT_USER_NAME", "Dadi"),
		DefaultUserPhone:    getString("DEFAULT_USER_PHONE", ""),
		DefaultUserLanguage: getString("DEFAULT_USER_LANGUAGE", "hi"),
		DefaultFamilyName:   getString("DEFAULT_FAMILY_NAME", ""),
		DefaultFamilyPhone:  getString("DEFAULT_FAMILY_PHONE", ""),

		RiskSuspicious:   getFloat("RISK_SUSPICIOUS", 0.4),
		RiskCaution:      getFloat("RISK_CAUTION", 0.55),
		RiskScam:         getFloat("RISK_SCAM", 0.8),
		RiskClear:        getFloat("RISK_CLEAR", 0.3),
		EvonWindow:       getSeconds("EVON_WINDOW_SECONDS", 60),
		EvonInterval:     getSeconds("EVON_INTERVAL_SECONDS", 5),
		WarningCooldown:  getSeconds("WARNING_COOLDOWN_SECONDS", 30),
		StoreTranscripts: getString("STORE_TRANSCRIPTS", "flagged"),
		RetentionDays:    getInt("RETENTION_DAYS", 30),
		ScreenMaxTurns:   getInt("SCREEN_MAX_TURNS", 3),

		BenchmarkDir: getString("BENCHMARK_DIR", "../benchmark/cases"),
		WebDist:      getString("WEB_DIST", ""),
	}
}

// ---------- small helpers ----------

func getString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v, err := strconv.Atoi(getString(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func getFloat(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(getString(key, ""), 64)
	if err != nil {
		return fallback
	}
	return v
}

func getSeconds(key string, fallback int) time.Duration {
	return time.Duration(getInt(key, fallback)) * time.Second
}

func getList(key string, fallback []string) []string {
	raw := getString(key, "")
	if raw == "" {
		return fallback
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
