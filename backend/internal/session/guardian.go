package session

import (
	"context"
	"log"
	"strings"
	"time"

	"chaukanna/internal/alerts"
	"chaukanna/internal/prefilter"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/scam"
)

// Guardian Mode
//
// The user merged Chaukanna into a live call. We never speak unless the call
// turns out to be a scam:
//
//	transcript ─> detector (pre-filter) ─┬─> new red flag?   analyze() now
//	                                     └─> every 5 s:      analyze()
//	analyze() ─> LLM (or rule-based fallback) ─> risk ≥ 0.55 ─> caution()  soft spoken advice, once
//	                                            risk ≥ 0.8  ─> warn()     checked warning + family alert

// analyze asks Evon about the recent transcript. Only one request runs at a time.
func (s *Session) analyze() {
	if !s.analyzing.CompareAndSwap(false, true) {
		return // an analysis is already running; the ticker will catch up
	}
	defer s.analyzing.Store(false)

	s.mu.Lock()
	now := s.elapsed()
	s.det.MarkAnalysisStarted(now)
	input := reasoner.GuardianInput{
		Window:       s.det.Window(now),
		KnownFlags:   s.det.Flags(),
		UserLanguage: s.user.Language,
		ElapsedSec:   now,
	}
	s.mu.Unlock()

	// Slow network call: no lock held.
	var result *scam.Analysis
	if s.llm.Enabled() {
		started := time.Now()
		a, err := s.llm.AnalyzeGuardian(s.ctx, input)
		if err != nil {
			log.Printf("session %d: evon failed, using rule-based fallback: %v", s.call.ID, err)
		} else {
			log.Printf("session %d: evon risk=%.2f in %s", s.call.ID, a.Risk, time.Since(started).Round(time.Millisecond))
			result = &a
		}
	}
	if s.ctx.Err() != nil {
		return // call ended while we were waiting
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if result != nil {
		s.afterUpdateLocked(s.det.ApplyAnalysis(s.elapsed(), *result))
	} else {
		s.afterUpdateLocked(s.det.ApplyHeuristic(s.elapsed()))
	}
}

// speak plays one of the pre-written warnings. Speed wins over the voice: the family's
// voice is used if that clip is ready, otherwise the stock voice speaks now and the
// family's clip is prepared for next time. A warning must never wait to sound nicer.
func (s *Session) speak(text string) bool {
	s.rememberSpoken(text)
	if s.user.VoiceEmbedding != "" {
		if samples, ok := s.tts.CachedClone(text, s.user.Language, s.user.VoiceEmbedding); ok {
			return s.play(samples)
		}
		go s.tts.PrewarmCloned(context.Background(), []string{text}, s.user.Language, s.user.VoiceEmbedding)
	}
	samples, err := s.tts.Synthesize(s.ctx, text, s.user.Language)
	if err != nil {
		log.Printf("session %d: could not synthesize: %v", s.call.ID, err)
		return false
	}
	return s.play(samples)
}

func (s *Session) play(samples []int16) bool {
	if s.leg == nil {
		return false
	}
	if err := s.leg.SendAudio(samples); err != nil {
		log.Printf("session %d: could not play audio: %v", s.call.ID, err)
		return false
	}
	return true
}

// familyVoiceWait is how long a family's typed message may wait for their own voice
// before the stock voice says it instead.
const familyVoiceWait = 8 * time.Second

// transcriptLocked joins what was heard on the call. Caller must hold s.mu.
func (s *Session) transcriptLocked() string {
	var b strings.Builder
	for _, l := range s.det.Lines() {
		b.WriteString(l.Text)
		b.WriteString(" ")
	}
	return b.String()
}

// caution speaks the soft warning once. The call is doubtful, not proven, so the
// family is not alarmed; the person is told how to check for themselves.
func (s *Session) caution() {
	text := scam.Caution(s.user.Language)
	played := s.speak(text)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateCallLocked(map[string]any{"cautioned_at": time.Now()})
	s.saveLineLocked(scam.Line{T: s.elapsed(), Speaker: "chaukanna", Text: text})
	s.broadcast("caution", map[string]any{"text": text, "language": s.user.Language, "audio_played": played})
	log.Printf("session %d: caution issued (audio played: %v)", s.call.ID, played)
}

// warn speaks the warning into the call and alerts the family (once per call).
func (s *Session) warn() { s.issueWarning(false) }

// issueWarning does the work. force is the family pressing "Warn now": it skips the
// cooldown, because a person watching the call has decided.
func (s *Session) issueWarning(force bool) {
	s.mu.Lock()
	if !force && !s.lastWarnAt.IsZero() && time.Since(s.lastWarnAt) < s.m.settings.WarningCooldown {
		s.mu.Unlock()
		return
	}
	s.lastWarnAt = time.Now()
	// Always a pre-written, checked line. Model output is never spoken into a call.
	category := prefilter.Classify(s.transcriptLocked())
	text := scam.WarningFor(category, s.user.Language, scam.OTPOnly(s.det.Flags()))
	s.mu.Unlock()

	// 1) Speak the warning into the call.
	played := s.speak(text)

	// 2) Record it.
	s.mu.Lock()
	upd := s.det.MarkWarned(s.elapsed())
	fields := map[string]any{"state": string(upd.State), "warned_at": time.Now(), "warning_played": played, "scam_type": string(category)}
	if at, ok := s.det.DetectedAt(); ok {
		fields["detected_after_sec"] = at
	}
	s.updateCallLocked(fields)
	s.saveLineLocked(scam.Line{T: s.elapsed(), Speaker: "chaukanna", Text: text})
	s.broadcast("warning", map[string]any{"text": text, "language": s.user.Language, "audio_played": played, "scam_type": category})
	s.broadcast("analysis", map[string]any{"state": s.det.State(), "risk": s.det.Risk(), "reason": s.det.Reason()})

	sendAlert := !s.familyAlerted && (s.isDemo() || s.user.FamilyPhone != "" || s.user.TelegramChatID != "")
	s.familyAlerted = true
	body := alerts.ScamAlert(s.user, s.det.Risk(), category, s.det.Reason(), s.det.Flags(), s.dashboardLink(), played)
	s.mu.Unlock()

	if !force {
		s.startFollowUpCheck()
	}

	// 3) Tell the family.
	if sendAlert {
		go s.notifyFamily(body)
	}
	log.Printf("session %d: ⚠️  scam warning issued (%s, audio played: %v, family alerted: %v)", s.call.ID, category, played, sendAlert)
}

// stopCooldown spaces out the interruption: long enough for the clip to finish (about
// 6 s), short enough to catch the next attempt.
const stopCooldown = 10 * time.Second

func (s *Session) lastHeardLocked() string {
	lines := s.det.Lines()
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1].Text
}

// interrupt runs when the person seems to be giving a code or money. It speaks a short
// "stop" at once, and sends the family one urgent alert telling them to call.
func (s *Session) interrupt(heard string) {
	s.mu.Lock()
	if !s.lastStopAt.IsZero() && time.Since(s.lastStopAt) < stopCooldown {
		s.mu.Unlock()
		return
	}
	s.lastStopAt = time.Now()
	s.lastWarnAt = s.lastStopAt // a warning right after this would only talk over it
	text := scam.StopNow(s.user.Language)
	s.mu.Unlock()

	played := s.speak(text)

	s.mu.Lock()
	upd := s.det.MarkWarned(s.elapsed())
	now := time.Now()
	category := prefilter.Classify(s.transcriptLocked())
	fields := map[string]any{"state": string(upd.State), "warned_at": now, "scam_type": string(category), "complying_at": now}
	if played {
		fields["warning_played"] = true
	}
	if at, ok := s.det.DetectedAt(); ok {
		fields["detected_after_sec"] = at
	}
	s.updateCallLocked(fields)
	s.saveLineLocked(scam.Line{T: s.elapsed(), Speaker: "chaukanna", Text: text})
	s.broadcast("warning", map[string]any{"text": text, "language": s.user.Language, "audio_played": played, "scam_type": category})
	s.broadcast("complying", map[string]any{"heard": heard})
	s.broadcast("analysis", map[string]any{"state": s.det.State(), "risk": s.det.Risk(), "reason": s.det.Reason()})

	sendAlert := !s.urgentSent && (s.isDemo() || s.user.FamilyPhone != "" || s.user.TelegramChatID != "")
	s.urgentSent = true
	s.familyAlerted = true // this says everything the first alert would have
	body := alerts.ComplyingAlert(s.user, heard, s.dashboardLink())
	s.mu.Unlock()

	s.startFollowUpCheck()
	if sendAlert {
		go s.notifyFamily(body)
	}
	log.Printf("session %d: 🛑 person seems to be complying (audio played: %v, family alerted: %v)", s.call.ID, played, sendAlert)
}

// remindCooldown keeps a caller asking again and again from making this talk over itself.
const remindCooldown = 15 * time.Second

// remind answers the caller asking again after the warning, with the short line. The
// family already has the first alert, and it only escalates if the person starts to comply.
func (s *Session) remind() {
	s.mu.Lock()
	now := time.Now()
	// Measured from the last thing said, so this never queues up behind the full warning.
	if now.Sub(s.lastRemindAt) < remindCooldown || now.Sub(s.lastStopAt) < stopCooldown || now.Sub(s.lastWarnAt) < remindCooldown {
		s.mu.Unlock()
		return
	}
	s.lastRemindAt = now
	text := scam.StopNow(s.user.Language)
	s.mu.Unlock()

	played := s.speak(text)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveLineLocked(scam.Line{T: s.elapsed(), Speaker: "chaukanna", Text: text})
	s.broadcast("warning", map[string]any{"text": text, "language": s.user.Language, "audio_played": played})
	log.Printf("session %d: reminded after the caller asked again (audio played: %v)", s.call.ID, played)
}

// maxTalkOutAnswers: past this, repeating ourselves only teaches people to tune it out.
const maxTalkOutAnswers = 2

// answerTalkOut replies when the caller tries to discredit the warning, and tells the family.
func (s *Session) answerTalkOut(heard string) {
	s.mu.Lock()
	sendAlert := !s.familyFollowUp && !s.urgentSent && (s.isDemo() || s.user.FamilyPhone != "" || s.user.TelegramChatID != "")
	if sendAlert {
		s.familyFollowUp = true
	}
	body := alerts.TalkOutAlert(s.user, heard, s.dashboardLink())

	now := time.Now()
	speak := s.talkOutAnswers < maxTalkOutAnswers && now.Sub(s.lastRemindAt) >= remindCooldown &&
		now.Sub(s.lastStopAt) >= stopCooldown && now.Sub(s.lastWarnAt) >= remindCooldown
	if speak {
		s.talkOutAnswers++
		s.lastRemindAt = now
	}
	text := scam.AnswerTalkOut(s.user.Language)
	s.mu.Unlock()

	// The family hears even when we stay quiet: this is the moment a person can help.
	if sendAlert {
		go s.notifyFamily(body)
	}
	if !speak {
		return
	}
	played := s.speak(text)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveLineLocked(scam.Line{T: s.elapsed(), Speaker: "chaukanna", Text: text})
	s.broadcast("warning", map[string]any{"text": text, "language": s.user.Language, "audio_played": played})
	log.Printf("session %d: answered a talk-out (audio played: %v, family alerted: %v)", s.call.ID, played, sendAlert)
}

// stillOnCallAfter is how long someone may stay on a flagged call before the family
// hears again. Long enough to hang up after the warning, short enough to still help.
const stillOnCallAfter = 45 * time.Second

// startFollowUpCheck tells the family if the person is still on the call a while after
// the warning. A call that ends in time cancels it, through the session's context.
func (s *Session) startFollowUpCheck() {
	s.mu.Lock()
	started := s.followUpTimer
	s.followUpTimer = true
	s.mu.Unlock()
	if started {
		return
	}
	go func() {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(stillOnCallAfter):
		}
		s.mu.Lock()
		send := !s.safe && !s.familyFollowUp && !s.urgentSent && (s.isDemo() || s.user.FamilyPhone != "" || s.user.TelegramChatID != "")
		if send {
			s.familyFollowUp = true
		}
		body := alerts.StillOnCallAlert(s.user, int(stillOnCallAfter.Seconds()), s.dashboardLink())
		s.mu.Unlock()
		if send {
			s.notifyFamily(body)
			log.Printf("session %d: family told the person is still on the call", s.call.ID)
		}
	}()
}

// SetSafe is the family saying the call is fine, or taking that back. While set, Chaukanna
// keeps listening but stops speaking and alerting, and the number isn't remembered as a scam.
func (s *Session) SetSafe(safe bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.safe = safe
	fields := map[string]any{"marked_safe_at": nil}
	if safe {
		fields["marked_safe_at"] = time.Now()
	}
	s.updateCallLocked(fields)
	s.broadcast("call_safe", map[string]any{"safe": safe})
	log.Printf("session %d: family marked the call safe: %v", s.call.ID, safe)
}

// WarnNow is the family pressing "Warn now" on the dashboard.
func (s *Session) WarnNow() {
	if s.mode == Guardian {
		s.issueWarning(true)
	}
}

// SpeakFamilyMessage plays the family's own words into the live call.
func (s *Session) SpeakFamilyMessage(text string) bool {
	s.rememberSpoken(text)
	var samples []int16
	if s.user.VoiceEmbedding != "" {
		ctx, cancel := context.WithTimeout(s.ctx, familyVoiceWait)
		defer cancel()
		samples, _ = s.tts.SynthesizeCloned(ctx, text, s.user.Language, s.user.VoiceEmbedding)
	}
	if len(samples) == 0 {
		var err error
		if samples, err = s.tts.Synthesize(s.ctx, text, s.user.Language); err != nil {
			log.Printf("session %d: could not synthesize family message: %v", s.call.ID, err)
		}
	}
	played := len(samples) > 0 && s.play(samples)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveLineLocked(scam.Line{T: s.elapsed(), Speaker: "family", Text: text})
	return played
}
