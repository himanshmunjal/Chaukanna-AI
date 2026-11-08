// Package api defines every HTTP route (Gin).
//
//	GET    /health                      liveness check + which integrations are configured
//
//	Telephony (called by Twilio / Exotel, no auth):
//	POST   /telephony/twilio/voice      Twilio "a call comes in" webhook
//	GET    /ws/twilio                   Twilio Media Stream
//	GET    /ws/exotel                   Exotel Voicebot stream
//	GET    /audio/:file                 short-lived WAV clips (screen-mode callback)
//
//	Who is asking (always open):
//	GET    /api/me                      am I logged in? is this a demo?
//	POST   /api/login, /api/logout      family dashboard password
//
//	Dashboard (see access.go: open locally, login when configured, isolated visitors in demo mode):
//	GET    /ws/dashboard                live events
//	GET    /ws/speaker                  speakerphone mode (browser microphone audio)
//	GET    /api/stats                   headline numbers
//	GET    /api/calls                   recent calls
//	GET    /api/calls/active            ids of calls in progress
//	GET    /api/calls/:id               one call with transcript, flags, alerts
//	GET    /api/users                   protected users
//	POST   /api/users                   add a protected user
//	DELETE /api/users/:id               remove a protected user
//	GET    /api/scam-numbers            numbers caught scamming (blocked on their next call)
//	POST   /api/scam-numbers            report a number manually
//	DELETE /api/scam-numbers/:id        unblock a number
//	GET    /api/scenarios               simulator scenarios (benchmark/cases)
//	POST   /api/simulate                replay a scenario as a live call
//	POST   /api/users/:id/consent       record that the person agreed to protection
//	POST   /api/users/:id/voice         enrol a family voice (multipart audio_file + owner + consent)
//	DELETE /api/users/:id/voice         delete it
//	GET    /api/users/:id/telegram-link link for the family to connect Telegram
//	POST   /api/calls/:id/warn          family: speak the warning now
//	POST   /api/calls/:id/speak         family: speak their own words into the call
//	POST   /api/calls/:id/safe          family: this call is fine, stop warning {safe: bool}
//	POST   /api/calls/:id/outcome       family verdict: confirmed_scam | false_alarm | money_lost
//	GET    /api/calls/:id/report        what to do next + complaint draft
//	POST   /api/analyze                 check pasted text or an uploaded recording
//	GET    /api/drill/scenarios         Scam Drill practice calls
//	POST   /api/drill/turn              one turn of a Scam Drill
package api

import (
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"chaukanna/internal/clips"
	"chaukanna/internal/config"
	"chaukanna/internal/detector"
	"chaukanna/internal/hub"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/session"
	"chaukanna/internal/stt"
	"chaukanna/internal/telegram"
	"chaukanna/internal/telephony"
	"chaukanna/internal/tts"
)

// Server holds what the handlers need.
type Server struct {
	cfg      config.Config
	db       *gorm.DB
	hub      *hub.Hub
	sessions *session.Manager
	clips    *clips.Store
	status   map[string]bool // which integrations are configured (shown on /health)
	svc      Services
	limits   *limits
}

// Services are the AI and messaging clients the REST handlers use directly.
type Services struct {
	Evon     *reasoner.Client
	Prisma   *stt.Client
	TTS      *tts.Client
	Telegram *telegram.Client
	Detector detector.Config
}

// NewRouter builds the Gin engine with every route.
func NewRouter(cfg config.Config, db *gorm.DB, h *hub.Hub, sessions *session.Manager,
	tel *telephony.Handler, clipStore *clips.Store, status map[string]bool, svc Services) *gin.Engine {

	s := &Server{cfg: cfg, db: db, hub: h, sessions: sessions, clips: clipStore, status: status, svc: svc}

	r := gin.New()
	// Without a list, X-Forwarded-For is ignored, so nobody can fake their address.
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Printf("api: TRUSTED_PROXIES: %v", err)
	}
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware(cfg.CORSOrigins))
	s.limits = newLimits()

	r.GET("/health", s.health)

	// Telephony providers call these directly.
	r.POST("/telephony/twilio/voice", tel.RequireTwilio, tel.VerifyTwilio, tel.TwilioVoice)
	r.GET("/ws/twilio", tel.RequireTwilio, tel.TwilioStream)
	r.GET("/ws/exotel", tel.RequireExotel, tel.ExotelStream)
	r.GET("/ws/exotel/:token", tel.RequireExotel, tel.ExotelStream)
	r.GET("/audio/:file", s.serveClip)

	// Who is asking? Open to everyone: the dashboard needs this before logging in.
	r.GET("/api/me", s.me)
	r.POST("/api/login", s.login)
	r.POST("/api/logout", s.logout)

	// Dashboard routes.
	protected := r.Group("/", s.identify)
	protected.GET("/ws/dashboard", h.ServeWS)
	protected.GET("/ws/speaker", s.visitorLimit(s.limits.speaker), s.visitorCallSlot, tel.SpeakerStream) // browser microphone (speakerphone mode)

	api := protected.Group("/api", limitBody)
	api.GET("/stats", s.stats)
	api.GET("/calls", s.listCalls)
	api.GET("/calls/active", s.activeCalls)
	api.GET("/calls/:id", s.getCall)
	api.GET("/scenarios", s.listScenarios)
	api.POST("/simulate", s.visitorLimit(s.limits.simulate), s.visitorCallSlot, s.simulate)

	// Real people and real numbers: never for demo visitors.
	family := api.Group("/", requireAdmin)
	family.GET("/users", s.listUsers)
	family.POST("/users", s.createUser)
	family.DELETE("/users/:id", s.deleteUser)
	family.GET("/scam-numbers", s.listScamNumbers)
	family.POST("/scam-numbers", s.reportScamNumber)
	family.DELETE("/scam-numbers/:id", s.deleteScamNumber)
	family.POST("/users/:id/consent", s.recordConsent)
	family.POST("/users/:id/voice", s.enrollVoice)
	family.DELETE("/users/:id/voice", s.deleteVoice)
	family.GET("/users/:id/telegram-link", s.telegramLink)
	family.POST("/calls/:id/label", s.labelCall)

	api.POST("/calls/:id/warn", s.warnNow)
	api.POST("/calls/:id/speak", s.speakIntoCall)
	api.POST("/calls/:id/safe", s.markCallSafe)
	api.POST("/calls/:id/outcome", s.setOutcome)
	api.GET("/calls/:id/report", s.callReport)
	api.POST("/analyze", s.visitorLimit(s.limits.analyze), s.analyze)
	api.GET("/drill/scenarios", s.drillScenarios)
	api.POST("/drill/turn", s.visitorLimit(s.limits.drill), s.drillTurn)

	s.serveDashboard(r)

	return r
}

// corsMiddleware allows the dashboard (on another port/domain) to call the API.
func corsMiddleware(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (slices.Contains(origins, "*") || slices.Contains(origins, origin)) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "integrations": s.status})
}

// serveClip returns a WAV clip so Twilio can play it on the callback call.
func (s *Server) serveClip(c *gin.Context) {
	id := strings.TrimSuffix(c.Param("file"), ".wav")
	wav, ok := s.clips.Get(id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "audio/wav", wav)
}

// serveDashboard serves the built React app when WEB_DIST is set, so one binary can
// host everything. Unknown paths get index.html because routing happens in the browser.
func (s *Server) serveDashboard(r *gin.Engine) {
	if s.cfg.WebDist == "" {
		return
	}
	// Go doesn't know this type, and browsers expect it for an installable app.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	index := filepath.Join(s.cfg.WebDist, "index.html")
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/ws/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// Clean a rooted path, so ".." can never leave WEB_DIST.
		file := filepath.Join(s.cfg.WebDist, filepath.Clean("/"+c.Request.URL.Path))
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			c.File(file)
			return
		}
		c.File(index)
	})
}
