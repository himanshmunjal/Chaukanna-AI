package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"chaukanna/internal/hub"
	"chaukanna/internal/ratelimit"
	"chaukanna/internal/stt"
	"chaukanna/internal/tts"
)

// Who can do what:
//
//	family    Logged in with DASHBOARD_PASSWORD (or DASHBOARD_TOKEN, for scripts). With
//	          neither set and no demo mode, everyone counts as family: a laptop.
//	visitor   DEMO_MODE only, anyone else. Gets a random id in a cookie, sees only their
//	          own calls, spends nothing of ours, and can't reach real people or numbers.
//	nobody    Otherwise: 401, and the dashboard shows its login page.
const (
	sessionCookie  = "chaukanna_session"
	visitorCookie  = "chaukanna_visitor"
	sessionLife    = 7 * 24 * time.Hour
	familyKey      = "is_family"
	gnaniKeyHeader = "X-Gnani-Key"
)

// isFamily checks the token and the login cookie.
func (s *Server) isFamily(c *gin.Context) bool {
	if s.cfg.DashboardToken != "" {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			token = c.Query("token")
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.DashboardToken)) == 1 {
			return true
		}
	}
	if s.cfg.DashboardPassword != "" {
		if cookie, err := c.Cookie(sessionCookie); err == nil && s.validSession(cookie) {
			return true
		}
	}
	return !s.cfg.DemoMode && s.cfg.DashboardPassword == "" && s.cfg.DashboardToken == ""
}

// sessionValue is "<expiry>.<signature>". The key comes from the password, so changing
// the password logs everyone out.
func (s *Server) signSession(expiry int64) string {
	key := sha256.Sum256([]byte("chaukanna-session|" + s.cfg.DashboardPassword))
	mac := hmac.New(sha256.New, key[:])
	fmt.Fprint(mac, expiry)
	return strconv.FormatInt(expiry, 10) + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) validSession(value string) bool {
	expiryText, _, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	expiry, err := strconv.ParseInt(expiryText, 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	return hmac.Equal([]byte(s.signSession(expiry)), []byte(value))
}

func secureRequest(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

// GET /api/me
func (s *Server) me(c *gin.Context) {
	family := s.isFamily(c)
	if s.cfg.DemoMode && !family {
		// Issue the visitor's id now. The live WebSocket can't set a cookie, and it must
		// use the same id as their later requests, or they'd miss their own call's events.
		s.visitorID(c)
	}
	c.JSON(http.StatusOK, gin.H{
		"family":         family,
		"demo":           s.cfg.DemoMode,
		"login_required": !family && !s.cfg.DemoMode,
		"can_login":      s.cfg.DashboardPassword != "",
		"logged_in":      family && s.cfg.DashboardPassword != "",
	})
}

// POST /api/login {password}
func (s *Server) login(c *gin.Context) {
	if s.cfg.DashboardPassword == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "login is not set up"})
		return
	}
	// Counted per address and overall, so guessing is slow even from many addresses.
	if !s.limits.login.Allow(c.ClientIP()) || !s.limits.loginAll.Allow("all") {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many attempts, please wait a few minutes"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	_ = c.ShouldBindJSON(&req)
	want := sha256.Sum256([]byte(s.cfg.DashboardPassword))
	got := sha256.Sum256([]byte(req.Password))
	if subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		log.Printf("api: failed login from %s", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong password"})
		return
	}
	expiry := time.Now().Add(sessionLife).Unix()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(sessionCookie, s.signSession(expiry), int(sessionLife.Seconds()), "/", "", secureRequest(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/logout
func (s *Server) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(sessionCookie, "", -1, "/", "", secureRequest(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// identify works out who is asking, for every dashboard route.
func (s *Server) identify(c *gin.Context) {
	if !s.allowedOrigin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "this site may not connect here"})
		return
	}
	if s.isFamily(c) {
		c.Set(familyKey, true)
		return
	}
	if !s.cfg.DemoMode {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please log in"})
		return
	}
	c.Set(hub.OwnerKey, s.visitorID(c))
}

// allowedOrigin stops another website's page from opening a WebSocket with this
// browser's cookies. Same-site cookies do most of this; it is a second lock.
func (s *Server) allowedOrigin(c *gin.Context) bool {
	if !strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
		return true
	}
	origin := c.GetHeader("Origin")
	if origin == "" || slices.Contains(s.cfg.CORSOrigins, "*") || slices.Contains(s.cfg.CORSOrigins, origin) {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == c.Request.Host
}

// visitorID reads or creates the anonymous id. It identifies a browser, not a person.
func (s *Server) visitorID(c *gin.Context) string {
	if id, err := c.Cookie(visitorCookie); err == nil && validVisitorID(id) {
		return id
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	id := hex.EncodeToString(raw)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(visitorCookie, id, int((24 * time.Hour).Seconds()), "/", "", secureRequest(c), true)
	return id
}

func validVisitorID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

// requireAdmin keeps real people and numbers away from demo visitors.
func requireAdmin(c *gin.Context) {
	if !c.GetBool(familyKey) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not available in the demo"})
	}
}

// ownCalls is the only way handlers should reach calls: family sees real calls, and a
// visitor sees only their own. Family doesn't see visitors' calls either.
func (s *Server) ownCalls(c *gin.Context) *gorm.DB {
	if owner := c.GetString(hub.OwnerKey); owner != "" {
		return s.db.Where("demo_owner = ?", owner)
	}
	// Calls from before this column existed hold NULL, and they're real calls.
	return s.db.Where("(demo_owner IS NULL OR demo_owner = '')")
}

// ownsCall answers 404 itself if this call belongs to someone else.
func (s *Server) ownsCall(c *gin.Context, id uint) bool {
	var count int64
	s.ownCalls(c).Table("calls").Where("id = ?", id).Count(&count)
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "call not found"})
		return false
	}
	return true
}

// clientsFor returns the speech clients for this request. A visitor's own key is used
// if they gave one. Without one they get no listening and only prepared clips, so a
// public demo never spends this server's credits. The key is used for this request
// and never stored or logged.
func (s *Server) clientsFor(c *gin.Context) (*stt.Client, *tts.Client) {
	if key := strings.TrimSpace(c.GetHeader(gnaniKeyHeader)); key != "" {
		return s.svc.Prisma.WithKey(key), s.svc.TTS.WithKey(key)
	}
	if c.GetBool(familyKey) {
		return s.svc.Prisma, s.svc.TTS
	}
	return nil, s.svc.TTS.CacheOnly()
}

func (s *Server) noVoiceMessage(c *gin.Context) string {
	if c.GetBool(familyKey) {
		return "Prisma is not configured, so recordings cannot be transcribed. Paste the words instead."
	}
	return "Add your Gnani API key to use audio, or paste the words instead."
}

// ---------------------------------------------------------------------------
// Limits
// ---------------------------------------------------------------------------

type limits struct {
	login, loginAll                   *ratelimit.Limiter
	analyze, drill, simulate, speaker *ratelimit.Limiter
	perIP                             *ratelimit.Limiter
}

func newLimits() *limits {
	return &limits{
		login:    ratelimit.New(5, 5*time.Minute),
		loginAll: ratelimit.New(50, 5*time.Minute),
		analyze:  ratelimit.New(10, time.Minute),
		drill:    ratelimit.New(40, time.Minute),
		simulate: ratelimit.New(6, time.Minute),
		speaker:  ratelimit.New(4, time.Minute),
		// A generous cap across everything, so clearing cookies doesn't escape the limits.
		perIP: ratelimit.New(120, time.Minute),
	}
}

// visitorLimit rate-limits demo visitors. The family isn't limited.
func (s *Server) visitorLimit(l *ratelimit.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		owner := c.GetString(hub.OwnerKey)
		if owner == "" {
			return
		}
		if !l.Allow(owner) || !s.limits.perIP.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "that's a lot of requests for the demo, please wait a minute"})
		}
	}
}

const (
	maxCallsPerVisitor = 2
	maxDemoCalls       = 20
)

// visitorCallSlot limits how many demo calls can run at once.
func (s *Server) visitorCallSlot(c *gin.Context) {
	owner := c.GetString(hub.OwnerKey)
	if owner == "" {
		return
	}
	mine, all := s.sessions.ActiveDemoCalls(owner)
	if mine >= maxCallsPerVisitor || all >= maxDemoCalls {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many demo calls are running, please wait for one to finish"})
	}
}

// limitBody caps request size. Uploads get room; everything else stays small.
func limitBody(c *gin.Context) {
	limit := int64(1 << 20)
	if strings.HasPrefix(c.ContentType(), "multipart/") {
		limit = maxUploadBytes + 1<<20
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
}
