package session

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"chaukanna/internal/detector"
	"chaukanna/internal/hub"
	"chaukanna/internal/models"
	"chaukanna/internal/prefilter"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/scam"
	"chaukanna/internal/tts"
)

// Session is one live call.
//
// Concurrency: transcripts, the ticker, Evon answers and TTS all run on
// different goroutines. Everything that touches `det` or the screen state
// holds `mu`. Slow network calls (Evon, Timbre, alerts) are made WITHOUT
// holding the lock.
type Session struct {
	m         *Manager
	call      models.Call
	user      models.ProtectedUser
	mode      Mode
	leg       Leg
	startedAt time.Time
	ctx       context.Context // cancelled when the call ends
	cancel    context.CancelFunc

	mu             sync.Mutex
	det            *detector.Detector
	analyzing      atomic.Bool // true while an Evon request is in flight
	lastWarnAt     time.Time
	familyAlerted  bool
	lastStopAt     time.Time    // last "stop" interruption
	lastRemindAt   time.Time    // last short warning after the caller asked again
	talkOutAnswers int          // how many times a talk-out was answered
	familyFollowUp bool         // the family got the follow-up alert
	followUpTimer  bool         // the still-on-call check has been started
	safe           bool         // the family marked this call safe: Chaukanna stops speaking and alerting
	urgentSent     bool         // the family got the urgent alert
	spoken         []spokenLine // what Chaukanna said recently
	screen         screenState
	knownScam      *models.ScamNumber // set if the caller's number was caught scamming before
	tts            *tts.Client        // this call's voice (a demo visitor's key, or the server's)
	llm            *reasoner.Client   // this call's reasoning model; nil on demo calls

	endOnce sync.Once
}

// ID returns the database id of the call.
func (s *Session) ID() uint { return s.call.ID }

// Language returns the protected user's language (used to configure Prisma).
func (s *Session) Language() string { return s.user.Language }

// OnTranscript is called for every result from Prisma (or the simulator).
func (s *Session) OnTranscript(text string, final bool) {
	text = strings.TrimSpace(text)
	if text == "" || s.ctx.Err() != nil {
		return
	}
	if final && s.isOwnEcho(text) {
		log.Printf("session %d: ignored our own voice picked up by the microphone", s.call.ID)
		return
	}

	// Partial results are only live captions for the dashboard.
	if !final {
		s.broadcast("transcript_partial", map[string]any{"t": s.elapsed(), "text": text})
		return
	}

	speaker := "call" // guardian: both people are mixed in one audio stream
	if s.mode == Screen {
		speaker = "caller"
	}

	s.mu.Lock()
	line := scam.Line{T: s.elapsed(), Speaker: speaker, Text: text}
	upd := s.det.AddLine(line)
	if s.mode == Screen && len(upd.NewFlags) > 0 {
		// Screen Mode has no periodic Evon check, so score new red flags right away.
		raised := s.det.RaiseToHeuristic(line.T)
		upd.StateChanged = upd.StateChanged || raised.StateChanged
		upd.State = s.det.State()
	}
	s.saveLineLocked(line)
	if s.mode == Screen {
		s.screen.conversation = append(s.screen.conversation, line)
	}
	s.afterUpdateLocked(upd)
	s.mu.Unlock()

	if s.mode == Guardian && upd.AnalyzeNow {
		go s.analyze()
	}
	if s.mode == Screen {
		s.screenCallerSpoke()
	}
}

// End finishes the call: saves the final state, applies the transcript
// retention policy and (in Screen Mode) sends the summary. Safe to call twice.
func (s *Session) End() {
	s.endOnce.Do(func() {
		s.cancel()

		s.mu.Lock()
		if s.screen.timer != nil {
			s.screen.timer.Stop()
		}
		state, risk := s.det.State(), s.det.Risk()
		markedSafe := s.safe
		fields := map[string]any{"ended_at": time.Now(), "state": string(state), "risk": risk}
		if state == detector.Scam || state == detector.Warned {
			fields["scam_type"] = string(prefilter.Classify(s.transcriptLocked()))
		}
		s.updateCallLocked(fields)
		s.applyRetentionLocked(state)
		s.mu.Unlock()

		if s.mode == Screen {
			s.finishScreen()
		}
		// Remember the number so it is blocked next time (only calls with caller ID).
		if !s.isDemo() && !markedSafe && s.call.CallerNumber != "" && (state == detector.Scam || state == detector.Warned) {
			s.m.recordScamNumber(s.call.CallerNumber, s.det.Reason(), s.call.ID, s.knownScam != nil)
		}

		s.broadcast("call_ended", map[string]any{"state": state, "risk": risk})
		s.m.remove(s.call.ID)
		if s.isDemo() {
			s.m.deps.Hub.ClearOwner(s.call.ID)
		}
		log.Printf("session %d: ended (state %s, risk %.2f)", s.call.ID, state, risk)
	})
}

// run is the background loop: in Guardian Mode it re-checks the call with
// Evon every EVON_INTERVAL_SECONDS (as long as something new was said).
func (s *Session) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if s.mode != Guardian {
				continue
			}
			s.mu.Lock()
			due := s.det.DueForAnalysis(s.elapsed())
			s.mu.Unlock()
			if due {
				go s.analyze()
			}
		}
	}
}

// afterUpdateLocked saves and broadcasts what changed, and triggers a warning
// when the call becomes a scam. Caller must hold s.mu.
func (s *Session) afterUpdateLocked(upd detector.Update) {
	if len(upd.NewFlags) > 0 {
		s.saveFlagsLocked(upd.NewFlags)
		s.broadcast("flags", map[string]any{"flags": upd.NewFlags})
	}

	fields := map[string]any{"state": string(s.det.State()), "risk": s.det.Risk(), "reason": s.det.Reason()}
	if lang := s.det.Language(); lang != "" {
		fields["language"] = lang
	}
	s.updateCallLocked(fields)
	s.broadcast("analysis", map[string]any{"state": s.det.State(), "risk": s.det.Risk(), "reason": s.det.Reason()})

	// Only Guardian Mode speaks warnings into the call. In Screen Mode the bot
	// is already talking to the scammer, and the user gets the summary.
	if s.mode != Guardian {
		return
	}
	if s.safe {
		return // the family has said this call is fine
	}
	if upd.Complying {
		go s.interrupt(s.lastHeardLocked()) // it includes the warning, so nothing else is needed
		return
	}
	if upd.TalkOut {
		go s.answerTalkOut(s.lastHeardLocked())
		return
	}
	if upd.RepeatDemand {
		go s.remind()
		return
	}
	if upd.CautionDue {
		go s.caution()
	}
	becameScam := upd.StateChanged && upd.State == detector.Scam
	newAttemptAfterWarning := s.det.State() == detector.Warned && upd.NewExtraction
	if becameScam || newAttemptAfterWarning {
		go s.warn()
	}
}

// ---------- persistence ----------

func (s *Session) saveLineLocked(line scam.Line) {
	row := models.TranscriptLine{CallID: s.call.ID, TimeSec: line.T, Speaker: line.Speaker, Text: line.Text}
	if err := s.m.deps.DB.Create(&row).Error; err != nil {
		log.Printf("session %d: save transcript: %v", s.call.ID, err)
	}
	s.broadcast("transcript", line)
}

func (s *Session) saveFlagsLocked(flags []scam.Flag) {
	for _, f := range flags {
		row := models.RedFlag{CallID: s.call.ID, Type: string(f.Type), TimeSec: f.T, Quote: f.Quote, Source: f.Source}
		if err := s.m.deps.DB.Create(&row).Error; err != nil {
			log.Printf("session %d: save flag: %v", s.call.ID, err)
		}
	}
}

func (s *Session) updateCallLocked(fields map[string]any) {
	// GORM skips the encrypting serializer on map updates.
	for _, column := range []string{"reason", "summary", "caller_name", "purpose"} {
		if v, ok := fields[column].(string); ok {
			fields[column] = models.Encrypt(v)
		}
	}
	if err := s.m.deps.DB.Model(&models.Call{}).Where("id = ?", s.call.ID).Updates(fields).Error; err != nil {
		log.Printf("session %d: update call: %v", s.call.ID, err)
	}
}

// applyRetentionLocked deletes transcripts we promised not to keep (privacy).
func (s *Session) applyRetentionLocked(state detector.State) {
	policy := s.m.settings.StoreTranscripts
	if policy == "all" || (policy == "flagged" && state != detector.Listening) {
		return
	}
	s.m.deps.DB.Where("call_id = ?", s.call.ID).Delete(&models.TranscriptLine{})
}

// ---------- helpers ----------

func (s *Session) broadcast(eventType string, data any) {
	s.m.deps.Hub.Broadcast(hub.Event{Type: eventType, CallID: s.call.ID, Data: data})
}

// spokenLine remembers one thing Chaukanna said.
type spokenLine struct {
	text string
	at   time.Time
}

// echoWindow is how long after speaking a line can still be our own voice.
const echoWindow = 45 * time.Second

// rememberSpoken notes a line just spoken into the call.
func (s *Session) rememberSpoken(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spoken = append(s.spoken, spokenLine{scam.Spoken(text, s.user.Language), time.Now()})
}

// isOwnEcho stops Chaukanna reacting to its own warning. A warning mentions OTPs and
// money, so hearing one back could otherwise turn a mere caution into a scam verdict.
func (s *Session) isOwnEcho(heard string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, line := range s.spoken {
		if time.Since(line.at) < echoWindow && prefilter.EchoOf(heard, line.text) {
			return true
		}
	}
	return false
}

// isDemo reports whether this is an anonymous visitor's demo call.
func (s *Session) isDemo() bool { return s.call.DemoOwner != "" }

// notifyFamily sends an alert, or on a demo call records what would have been sent.
func (s *Session) notifyFamily(body string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if s.isDemo() {
		s.m.deps.Alerts.RecordDemo(s.call.ID, body)
		return
	}
	s.m.deps.Alerts.SendToFamily(ctx, s.call.ID, s.user, body)
}

// elapsed returns seconds since the call started, rounded to 0.1 s.
func (s *Session) elapsed() float64 {
	return math.Round(time.Since(s.startedAt).Seconds()*10) / 10
}

func (s *Session) dashboardLink() string {
	return fmt.Sprintf("%s/calls/%d", s.m.settings.DashboardURL, s.call.ID)
}
