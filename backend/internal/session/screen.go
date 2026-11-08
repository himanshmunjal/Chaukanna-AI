package session

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"chaukanna/internal/alerts"
	"chaukanna/internal/audio"
	"chaukanna/internal/prefilter"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/scam"
)

// Screen Mode
//
// An unknown caller was forwarded to Chaukanna. The bot:
//  1. greets the caller and asks who they are,
//  2. after each caller sentence (plus a short silence) asks Evon what to say next,
//  3. ends the call when it knows enough (or after SCREEN_MAX_TURNS),
//  4. calls the user back and plays a spoken summary, and sends it as text.

// endOfTurnSilence: how long the caller must be quiet before the bot replies.
const endOfTurnSilence = 1200 * time.Millisecond

type screenState struct {
	conversation []scam.Line // caller + bot lines, in order
	turns        int
	busy         bool // the bot is deciding what to say
	done         bool // the bot has decided to end the call
	timer        *time.Timer
	callerName   string
	purpose      string
	summary      string
	summaryRisk  float64 // the risk when the summary was written
	finishOnce   sync.Once
}

// screenGreet plays the greeting when the call connects.
func (s *Session) screenGreet() {
	time.Sleep(500 * time.Millisecond) // let the media stream settle
	s.say(scam.Greeting(s.user.Language, s.user.Name))
}

// screenCallerSpoke restarts the end-of-turn timer each time the caller says something.
func (s *Session) screenCallerSpoke() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.screen.done {
		return
	}
	if s.screen.timer != nil {
		s.screen.timer.Stop()
	}
	s.screen.timer = time.AfterFunc(endOfTurnSilence, s.screenTurn)
}

// screenTurn runs when the caller has finished speaking: decide and reply.
func (s *Session) screenTurn() {
	if s.ctx.Err() != nil {
		return
	}

	s.mu.Lock()
	if s.screen.done || s.screen.busy {
		s.mu.Unlock()
		return
	}
	s.screen.busy = true
	s.screen.turns++
	input := reasoner.ScreenInput{
		Transcript:   append([]scam.Line(nil), s.screen.conversation...),
		UserName:     s.user.Name,
		UserLanguage: s.user.Language,
		Turn:         s.screen.turns,
		MaxTurns:     s.m.settings.ScreenMaxTurns,
	}
	ruleRisk := s.ruleRiskLocked()
	s.mu.Unlock()

	// Slow network call: no lock held.
	res := s.screenDecide(input, ruleRisk)

	s.mu.Lock()
	s.afterUpdateLocked(s.det.ApplyAnalysis(s.elapsed(), res.Analysis))
	if res.CallerName != "" {
		s.screen.callerName = res.CallerName
	}
	if res.Purpose != "" {
		s.screen.purpose = res.Purpose
	}
	if res.Summary != "" {
		s.screen.summary = res.Summary
		s.screen.summaryRisk = s.det.Risk()
	}
	s.updateCallLocked(map[string]any{"caller_name": s.screen.callerName, "purpose": s.screen.purpose})
	s.broadcast("call_updated", map[string]any{"caller_name": s.screen.callerName, "purpose": s.screen.purpose})

	finished := res.Done || s.screen.turns >= s.m.settings.ScreenMaxTurns
	if finished {
		s.screen.done = true
	}
	s.screen.busy = false
	s.mu.Unlock()

	reply := res.Reply
	if finished && reply == "" {
		reply = scam.Closing(s.user.Language)
	}
	playTime := s.say(reply)

	if finished {
		time.Sleep(playTime + 700*time.Millisecond) // let the goodbye finish playing
		if s.leg != nil {
			if err := s.leg.Hangup(); err != nil {
				log.Printf("session %d: hangup: %v", s.call.ID, err)
			}
		}
		// The telephony handler calls End() when the stream closes, which runs finishScreen().
	}
}

// screenDecide asks Evon for the next move, or uses a simple script without Evon.
func (s *Session) screenDecide(in reasoner.ScreenInput, ruleRisk float64) reasoner.ScreenResult {
	if s.llm.Enabled() {
		res, err := s.llm.Screen(s.ctx, in)
		if err == nil {
			return res
		}
		log.Printf("session %d: evon screen failed, using script: %v", s.call.ID, err)
	}

	// Fallback script: ask one follow-up question, then finish.
	fallback := reasoner.ScreenResult{Analysis: scam.Analysis{Risk: ruleRisk, Reason: "Rule-based screening."}}
	if in.Turn < 2 && ruleRisk < s.m.settings.Detector.RiskScam {
		fallback.Reply = scam.FollowUp(in.UserLanguage)
		return fallback
	}
	// No Summary here on purpose: finishScreen() writes it when the call ends,
	// so it reflects everything the caller said (including their last sentence).
	fallback.Done = true
	return fallback
}

// finishScreen calls the user back with a spoken summary and sends a text summary.
func (s *Session) finishScreen() {
	s.screen.finishOnce.Do(func() {
		s.mu.Lock()
		words := callerWords(s.screen.conversation)
		if words == "" && s.knownScam == nil {
			s.mu.Unlock()
			return // the caller never spoke; nothing to report
		}
		risk := s.det.Risk()
		summary := s.screen.summary
		if s.knownScam != nil {
			summary = scam.BlockedSummary(s.user.Language, s.knownScam.Reports)
		}
		// Rebuild the summary if there is none, or if the caller revealed red
		// flags after it was written (e.g. said "CBI" after the bot's last question).
		becameSuspicious := risk >= s.m.settings.Detector.RiskSuspicious &&
			s.screen.summaryRisk < s.m.settings.Detector.RiskSuspicious
		if s.knownScam == nil && (summary == "" || becameSuspicious) {
			summary = scam.FallbackScreenSummary(s.user.Language, risk, words)
		}
		callerName, purpose := s.screen.callerName, s.screen.purpose
		s.updateCallLocked(map[string]any{"summary": summary})
		s.mu.Unlock()

		s.broadcast("call_updated", map[string]any{"caller_name": callerName, "purpose": purpose, "summary": summary})

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		// 1) Spoken callback (Twilio calls only; the simulator and speakerphone
		//    mode have no phone line to call back on).
		if s.call.Provider == "twilio" {
			s.callUserWithSummary(ctx, summary)
		}

		// 2) Text summary to the user, and to the family if it was a scam.
		body := alerts.ScreenSummary(s.user, s.call.CallerNumber, callerName, purpose, summary, risk, s.dashboardLink())
		if s.isDemo() {
			s.m.deps.Alerts.RecordDemo(s.call.ID, body)
			return
		}
		s.m.deps.Alerts.Send(ctx, s.call.ID, s.user.Phone, body)
		if risk >= s.m.settings.Detector.RiskScam {
			s.m.deps.Alerts.SendToFamily(ctx, s.call.ID, s.user, body)
		}
	})
}

// callUserWithSummary synthesises the summary and has Twilio call the user to play it.
func (s *Session) callUserWithSummary(ctx context.Context, summary string) {
	cb := s.m.deps.Callback
	if cb == nil || !cb.Enabled() || s.m.settings.PublicURL == "" {
		log.Printf("session %d: callback skipped (Twilio or PUBLIC_URL not configured)", s.call.ID)
		return
	}
	samples, err := s.tts.Synthesize(ctx, summary, s.user.Language)
	if err != nil {
		log.Printf("session %d: callback TTS failed: %v", s.call.ID, err)
		return
	}
	id := s.m.deps.Clips.Put(audio.EncodeWAV(samples, audio.PhoneRate))
	url := s.m.settings.PublicURL + "/audio/" + id + ".wav"
	if err := cb.CallWithAudio(ctx, s.user.Phone, url); err != nil {
		log.Printf("session %d: callback call failed: %v", s.call.ID, err)
	}
}

// say speaks text into the call, records it, and returns how long it plays.
func (s *Session) say(text string) time.Duration {
	if strings.TrimSpace(text) == "" || s.ctx.Err() != nil {
		return 0
	}
	s.mu.Lock()
	line := scam.Line{T: s.elapsed(), Speaker: "chaukanna", Text: text}
	s.saveLineLocked(line)
	s.screen.conversation = append(s.screen.conversation, line)
	s.mu.Unlock()

	s.rememberSpoken(text)
	samples, err := s.tts.Synthesize(s.ctx, text, s.user.Language)
	if err != nil {
		log.Printf("session %d: TTS failed: %v", s.call.ID, err)
		return 0
	}
	if s.leg != nil {
		if err := s.leg.SendAudio(samples); err != nil {
			log.Printf("session %d: play audio: %v", s.call.ID, err)
		}
	}
	return time.Duration(audio.Duration(samples, audio.PhoneRate) * float64(time.Second))
}

// ruleRiskLocked scores the call from detected flags (used without Evon).
func (s *Session) ruleRiskLocked() float64 {
	var types []scam.FlagType
	for _, f := range s.det.Flags() {
		types = append(types, f.Type)
	}
	return prefilter.RiskFromFlags(types)
}

// callerWords joins everything the caller said.
func callerWords(lines []scam.Line) string {
	var parts []string
	for _, l := range lines {
		if l.Speaker == "caller" {
			parts = append(parts, l.Text)
		}
	}
	return strings.Join(parts, " ")
}

// handleKnownScam runs when the caller's number was caught scamming before.
//   - Call screening: tell the caller the number is reported, hang up, notify the user.
//   - Live protection: mark the call as a scam right away, which plays the warning.
func (s *Session) handleKnownScam() {
	reason := fmt.Sprintf("This number was reported for fraud %d time(s) before.", s.knownScam.Reports)

	s.mu.Lock()
	upd := s.det.MarkKnownScam(s.elapsed(), reason)
	s.screen.done = true // never start a conversation with a known scammer
	s.afterUpdateLocked(upd)
	s.mu.Unlock()
	log.Printf("session %d: 🚫 known scam number %s", s.call.ID, s.call.CallerNumber)

	if s.mode == Screen {
		time.Sleep(500 * time.Millisecond) // let the media stream settle
		playTime := s.say(scam.BlockedMessage(s.user.Language))
		time.Sleep(playTime + 500*time.Millisecond)
		if s.leg != nil {
			_ = s.leg.Hangup()
		}
	}
}
