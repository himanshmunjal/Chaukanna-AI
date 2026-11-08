package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"chaukanna/internal/cases"
	"chaukanna/internal/hub"
	"chaukanna/internal/scam"
	"chaukanna/internal/session"
)

// The simulator replays a scripted conversation through the REAL pipeline
// (pre-filter -> Evon -> state machine -> Timbre -> alerts -> dashboard),
// skipping only the phone line and Prisma. It is perfect for:
//   - building and testing without a phone number,
//   - recording a reliable demo video,
//   - showing judges many languages in one minute.

// GET /api/scenarios
func (s *Server) listScenarios(c *gin.Context) {
	list, err := cases.LoadDir(s.cfg.BenchmarkDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

type simulateRequest struct {
	ScenarioID string      `json:"scenario_id"` // a file in benchmark/cases, or...
	Lines      []scam.Line `json:"lines"`       // ...your own lines
	Mode       string      `json:"mode"`        // guardian (default) | screen
	Speed      float64     `json:"speed"`       // 1 = real time, 2 = twice as fast
	UserPhone  string      `json:"user_phone"`  // optional: which protected user
	// Optional caller ID. Scam calls with a number are remembered, and the next
	// call from the same number is blocked straight away (try it twice!).
	CallerNumber string `json:"caller_number"`
	// Language is honoured only for demo visitors, who have no protected person of their own.
	Language string `json:"language"`
}

// Limits on a script, so one request can't hold a call open for hours.
const (
	maxScriptLines  = 60
	maxScriptChars  = 1000
	maxScriptSecond = 300
)

// POST /api/simulate
func (s *Server) simulate(c *gin.Context) {
	var req simulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lines := req.Lines
	if req.ScenarioID != "" {
		list, err := cases.LoadDir(s.cfg.BenchmarkDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, sc := range list {
			if sc.ID == req.ScenarioID {
				lines = sc.Lines
			}
		}
	}
	if len(lines) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown scenario_id or empty lines"})
		return
	}
	if req.Speed <= 0 {
		req.Speed = 1
	}
	if len(lines) > maxScriptLines {
		c.JSON(http.StatusBadRequest, gin.H{"error": "that script is too long"})
		return
	}
	for _, l := range lines {
		if l.T < 0 || l.T > maxScriptSecond || len([]rune(l.Text)) > maxScriptChars {
			c.JSON(http.StatusBadRequest, gin.H{"error": "that script is too long"})
			return
		}
	}

	leg := session.NewSimulatorLeg(s.hub)
	params := session.StartParams{
		Provider:     "simulator",
		Mode:         session.Mode(req.Mode),
		UserPhone:    req.UserPhone,
		CallerNumber: req.CallerNumber,
		Leg:          leg,
	}
	if owner := c.GetString(hub.OwnerKey); owner != "" {
		params.Owner, params.Language = owner, scam.KnownLanguage(req.Language)
		_, params.TTS = s.clientsFor(c)
	}
	sess, err := s.sessions.Start(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	leg.Attach(sess.ID())

	go playScript(sess, leg, lines, req.Speed)
	c.JSON(http.StatusAccepted, gin.H{"call_id": sess.ID()})
}

// playScript feeds the lines to the session at their timestamps, as if Prisma
// had just transcribed them (a partial result first, then the final one, just
// like Prisma sends them).
func playScript(sess *session.Session, leg *session.SimulatorLeg, lines []scam.Line, speed float64) {
	defer sess.End()

	start := time.Now()
	for _, line := range lines {
		wait := time.Duration(line.T/speed*float64(time.Second)) - time.Since(start)
		select {
		case <-leg.Done(): // the screening bot hung up
			return
		case <-time.After(max(wait, 0)):
		}
		sess.OnTranscript(line.Text, false) // live caption
		sess.OnTranscript(line.Text, true)  // final sentence
	}

	// Give Evon / the screening bot a moment to finish before ending the call.
	select {
	case <-leg.Done():
	case <-time.After(6 * time.Second):
	}
}
