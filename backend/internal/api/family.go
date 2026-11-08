package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"chaukanna/internal/audio"
	"chaukanna/internal/cases"
	"chaukanna/internal/drill"
	"chaukanna/internal/evaluate"
	"chaukanna/internal/hub"
	"chaukanna/internal/models"
	"chaukanna/internal/prefilter"
	"chaukanna/internal/report"
	"chaukanna/internal/scam"
	"chaukanna/internal/telegram"
)

const (
	maxUploadBytes = 25 << 20
	maxCheckBytes  = 20000
)

// ---------------------------------------------------------------------------
// Consent and family voice
// ---------------------------------------------------------------------------

func (s *Server) findUser(c *gin.Context) (models.ProtectedUser, bool) {
	var u models.ProtectedUser
	if err := s.db.First(&u, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "person not found"})
		return u, false
	}
	return u, true
}

// POST /api/users/:id/consent
func (s *Server) recordConsent(c *gin.Context) {
	u, ok := s.findUser(c)
	if !ok {
		return
	}
	now := time.Now()
	s.db.Model(&u).Update("consent_at", now)
	u.ConsentAt = &now
	c.JSON(http.StatusOK, u)
}

// POST /api/users/:id/voice  multipart: audio_file (5-30 s), owner, consent=true.
// Only the owner's own voice, with their agreement: cloning anyone else is impersonation.
func (s *Server) enrollVoice(c *gin.Context) {
	u, ok := s.findUser(c)
	if !ok {
		return
	}
	owner := strings.TrimSpace(c.PostForm("owner"))
	if c.PostForm("consent") != "true" || owner == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the speaker's name and their consent are required"})
		return
	}
	data, _, err := readUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if samples, rate, err := audio.ParseWAV(data); err == nil {
		if sec := float64(len(samples)) / float64(rate); sec < 5 || sec > 30 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("the recording must be 5 to 30 seconds long (this one is %.0f s)", sec)})
			return
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	embedding, err := s.svc.TTS.CreateEmbedding(ctx, data)
	if err != nil {
		log.Printf("voice: enrol failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not create the voice: " + err.Error()})
		return
	}
	// The recording itself is not kept: only the embedding, which can be deleted.
	now := time.Now()
	if err := s.db.Model(&u).Updates(map[string]any{"voice_embedding": models.Encrypt(embedding), "voice_owner": owner, "voice_consent_at": now}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Prepare the warning now, so the first real one plays at once, and return it to listen to.
	sample := scam.DefaultWarning(u.Language)
	go func() {
		bg, done := context.WithTimeout(context.Background(), 5*time.Minute)
		defer done()
		s.svc.TTS.PrewarmCloned(bg, scam.AllWarnings(u.Language), u.Language, embedding)
	}()
	resp := gin.H{"voice_owner": owner, "has_voice": true}
	if clip, err := s.svc.TTS.SynthesizeCloned(ctx, sample, u.Language, embedding); err == nil {
		resp["sample_audio"] = base64.StdEncoding.EncodeToString(audio.EncodeWAV(clip, audio.PhoneRate))
		resp["sample_text"] = sample
	}
	c.JSON(http.StatusOK, resp)
}

// DELETE /api/users/:id/voice
func (s *Server) deleteVoice(c *gin.Context) {
	u, ok := s.findUser(c)
	if !ok {
		return
	}
	s.db.Model(&u).Updates(map[string]any{"voice_embedding": "", "voice_owner": "", "voice_consent_at": nil})
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Telegram
// ---------------------------------------------------------------------------

// GET /api/users/:id/telegram-link
func (s *Server) telegramLink(c *gin.Context) {
	u, ok := s.findUser(c)
	if !ok {
		return
	}
	if !s.svc.Telegram.Enabled() {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	if u.TelegramLinkCode == "" {
		code := make([]byte, 12)
		if _, err := rand.Read(code); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		u.TelegramLinkCode = hex.EncodeToString(code)
		s.db.Model(&u).Update("telegram_link_code", u.TelegramLinkCode)
	}
	bot := s.svc.Telegram.Username(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"linked":  u.TelegramChatID != "",
		"url":     fmt.Sprintf("https://t.me/%s?start=%s", bot, u.TelegramLinkCode),
	})
}

// TelegramLinker connects a family's chat when they open their link.
func TelegramLinker(db *gorm.DB) telegram.LinkHandler {
	return func(chatID, code string) string {
		var users []models.ProtectedUser
		db.Where("telegram_link_code = ? AND telegram_link_code <> ''", code).Limit(1).Find(&users)
		if len(users) == 0 {
			return "This link is not valid. Please open a new one from the Chaukanna dashboard."
		}
		db.Model(&users[0]).Update("telegram_chat_id", models.Encrypt(chatID))
		return fmt.Sprintf("Linked. You will get an alert here if %s is on a likely scam call.", users[0].Name)
	}
}

// ---------------------------------------------------------------------------
// Family actions on a call
// ---------------------------------------------------------------------------

func callID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad call id"})
		return 0, false
	}
	return uint(id), true
}

// POST /api/calls/:id/warn
func (s *Server) warnNow(c *gin.Context) {
	id, ok := callID(c)
	if !ok {
		return
	}
	if !s.ownsCall(c, id) {
		return
	}
	sess, live := s.sessions.Get(id)
	if !live {
		c.JSON(http.StatusConflict, gin.H{"error": "this call has ended"})
		return
	}
	go sess.WarnNow()
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

// POST /api/calls/:id/speak {text}
func (s *Server) speakIntoCall(c *gin.Context) {
	id, ok := callID(c)
	if !ok {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}
	if len([]rune(req.Text)) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please keep the message under 200 characters"})
		return
	}
	if !s.ownsCall(c, id) {
		return
	}
	sess, live := s.sessions.Get(id)
	if !live {
		c.JSON(http.StatusConflict, gin.H{"error": "this call has ended"})
		return
	}
	played := sess.SpeakFamilyMessage(strings.TrimSpace(req.Text))
	c.JSON(http.StatusOK, gin.H{"played": played})
}

// POST /api/calls/:id/safe {safe}
func (s *Server) markCallSafe(c *gin.Context) {
	id, ok := callID(c)
	if !ok {
		return
	}
	var req struct {
		Safe bool `json:"safe"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !s.ownsCall(c, id) {
		return
	}
	sess, live := s.sessions.Get(id)
	if !live {
		c.JSON(http.StatusConflict, gin.H{"error": "this call has ended"})
		return
	}
	sess.SetSafe(req.Safe)
	outcome := ""
	if req.Safe {
		outcome = "false_alarm"
	}
	s.db.Model(&models.Call{}).Where("id = ?", id).Update("outcome", outcome)
	c.JSON(http.StatusOK, gin.H{"safe": req.Safe})
}

// POST /api/calls/:id/outcome {outcome}
// A false alarm also releases the number, so a wrong verdict does not block someone forever.
func (s *Server) setOutcome(c *gin.Context) {
	id, ok := callID(c)
	if !ok {
		return
	}
	var req struct {
		Outcome string `json:"outcome"`
	}
	_ = c.ShouldBindJSON(&req)
	switch req.Outcome {
	case "confirmed_scam", "false_alarm", "money_lost", "":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown outcome"})
		return
	}
	var call models.Call
	if err := s.ownCalls(c).First(&call, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "call not found"})
		return
	}
	s.db.Model(&call).Update("outcome", req.Outcome)
	if req.Outcome == "false_alarm" && call.CallerNumber != "" {
		s.db.Where("phone = ? AND source = ? AND last_call_id = ?", call.CallerNumber, "auto", id).Delete(&models.ScamNumber{})
	}
	c.JSON(http.StatusOK, gin.H{"outcome": req.Outcome})
}

// GET /api/calls/:id/report
func (s *Server) callReport(c *gin.Context) {
	var call models.Call
	err := s.ownCalls(c).Preload("ProtectedUser").
		Preload("Flags", func(db *gormDB) *gormDB { return db.Order("time_sec, id") }).
		First(&call, c.Param("id")).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "call not found"})
		return
	}
	c.JSON(http.StatusOK, report.Build(call))
}

// ---------------------------------------------------------------------------
// Check a call: paste what was said, or upload a recording
// ---------------------------------------------------------------------------

var sentenceEnd = regexp.MustCompile(`[.?!।\n]+`)

// splitLines makes transcript lines from free text, about 5 s apart.
func splitLines(text string) []scam.Line {
	var lines []scam.Line
	for _, part := range sentenceEnd.Split(text, -1) {
		if part = strings.TrimSpace(part); part != "" {
			lines = append(lines, scam.Line{T: float64(1 + len(lines)*5), Text: part})
		}
	}
	return lines
}

// POST /api/analyze  JSON {text, language} or multipart audio_file + language.
// Nothing is stored: this is for a quick second opinion.
func (s *Server) analyze(c *gin.Context) {
	language, text := "hi", ""
	if strings.HasPrefix(c.ContentType(), "multipart/") {
		if l := c.PostForm("language"); l != "" {
			language = l
		}
		data, name, err := readUpload(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		prisma, _ := s.clientsFor(c)
		if !prisma.Enabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": s.noVoiceMessage(c)})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
		defer cancel()
		if text, err = prisma.TranscribeFile(ctx, name, data, language); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "could not transcribe the recording: " + err.Error()})
			return
		}
	} else {
		var req struct {
			Text     string `json:"text"`
			Language string `json:"language"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		text = req.Text
		if req.Language != "" {
			language = req.Language
		}
	}

	language = scam.KnownLanguage(language)
	lines := splitLines(text)
	if len(lines) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no speech or text to check"})
		return
	}
	if len(text) > maxCheckBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "that text is too long to check"})
		return
	}
	evon := s.svc.Evon
	if c.GetString(hub.OwnerKey) != "" {
		evon = nil // a demo visitor never spends this server's LLM budget
	}
	result := evaluate.Run(cases.Case{ID: "check", Language: language}, lines, s.svc.Detector, evon)

	type flag struct {
		Type  string `json:"type"`
		Quote string `json:"quote"`
	}
	flags := []flag{}
	seen := map[scam.FlagType]bool{}
	for _, l := range lines {
		for _, h := range prefilter.Scan(l.Text) {
			if !seen[h.Type] {
				seen[h.Type] = true
				flags = append(flags, flag{string(h.Type), l.Text})
			}
		}
	}

	otpOnly := seen[scam.OTPRequest] && !seen[scam.MoneyMove]
	verdict, advice := "safe", ""
	category := scam.Category(result.ScamType)
	switch {
	case result.Detected:
		verdict, advice = "scam", scam.WarningFor(category, language, otpOnly)
	case result.Cautioned:
		verdict, advice = "caution", scam.Caution(language)
	}
	resp := gin.H{
		"verdict": verdict, "risk": result.FinalRisk, "reason": result.Reason, "advice": advice,
		"flags": flags, "transcript": text, "scam_type": category, "scam_type_label": scam.CategoryLabels[category],
	}
	if verdict == "scam" {
		resp["links"] = report.Links
		resp["scam_type_label"] = scam.CategoryLabels[category]
	}
	c.JSON(http.StatusOK, resp)
}

func readUpload(c *gin.Context) (data []byte, name string, err error) {
	header, err := c.FormFile("audio_file")
	if err != nil {
		return nil, "", fmt.Errorf("audio_file is required")
	}
	if header.Size > maxUploadBytes {
		return nil, "", fmt.Errorf("the file is too large (25 MB at most)")
	}
	f, err := header.Open()
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	data, err = io.ReadAll(f)
	return data, header.Filename, err
}

// ---------------------------------------------------------------------------
// Labelling test calls
// ---------------------------------------------------------------------------

// POST /api/calls/:id/label {label, category}
// Records what a test call really was, for scoring. Label by what the caller intended,
// never by what Chaukanna decided. Empty label removes it.
func (s *Server) labelCall(c *gin.Context) {
	var req struct {
		Label    string `json:"label"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	switch req.Label {
	case "scam":
		if _, ok := scam.CategoryLabels[scam.Category(req.Category)]; !ok {
			req.Category = string(scam.General)
		}
	case "legit", "":
		req.Category = ""
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "label must be scam or legit"})
		return
	}
	result := s.ownCalls(c).Model(&models.Call{}).Where("id = ?", c.Param("id")).
		Updates(map[string]any{"test_label": req.Label, "test_category": req.Category})
	if result.Error != nil || result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "call not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"test_label": req.Label, "test_category": req.Category})
}

// ---------------------------------------------------------------------------
// Scam Drill
// ---------------------------------------------------------------------------

// GET /api/drill/scenarios
func (s *Server) drillScenarios(c *gin.Context) {
	prisma, voice := s.clientsFor(c)
	c.JSON(http.StatusOK, gin.H{"scenarios": drill.Scenarios, "voice": voice.Enabled(), "listen": prisma.Enabled()})
}

// POST /api/drill/turn  JSON {scenario, step, reply} or multipart with audio_file.
// step is the line just answered; step -1 starts the call. The browser keeps the score.
func (s *Server) drillTurn(c *gin.Context) {
	var req struct {
		Scenario string `json:"scenario" form:"scenario"`
		Step     int    `json:"step" form:"step"`
		Reply    string `json:"reply" form:"reply"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sc, ok := drill.Find(req.Scenario)
	if !ok || req.Step < -1 || req.Step >= len(sc.Steps) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown scenario or step"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Minute)
	defer cancel()
	prisma, voice := s.clientsFor(c)
	resp := gin.H{"total": len(sc.Steps)}

	if req.Step >= 0 {
		heard := req.Reply
		if _, err := c.FormFile("audio_file"); err == nil {
			data, name, err := readUpload(c)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if !prisma.Enabled() {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": s.noVoiceMessage(c)})
				return
			}
			if heard, err = prisma.TranscribeFile(ctx, name, data, sc.Lang); err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": "could not hear the reply: " + err.Error()})
				return
			}
		}
		resp["heard"] = heard
		resp["verdict"] = drill.Judge(heard)
		resp["lesson"] = sc.Steps[req.Step].Lesson
	}

	next := req.Step + 1
	if next >= len(sc.Steps) {
		resp["done"] = true
		c.JSON(http.StatusOK, resp)
		return
	}
	resp["step"] = next
	resp["line"] = sc.Steps[next].Line
	if samples, err := voice.SynthesizeCaller(ctx, sc.Steps[next].Line, sc.Lang); err == nil {
		resp["audio"] = base64.StdEncoding.EncodeToString(audio.EncodeWAV(samples, audio.PhoneRate))
	}
	c.JSON(http.StatusOK, resp)
}
