// Package evaluate scores Chaukanna on test calls. It is shared by
// cmd/benchmark (scripts / recordings) and cmd/noisebench (synthetic noisy calls),
// so both measure with exactly the same logic as the live server.
package evaluate

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"chaukanna/internal/audio"
	"chaukanna/internal/cases"
	"chaukanna/internal/detector"
	"chaukanna/internal/prefilter"
	"chaukanna/internal/reasoner"
	"chaukanna/internal/scam"
	"chaukanna/internal/stt"
)

// Result is the outcome of one test call.
type Result struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	Language     string  `json:"language"`
	Detected     bool    `json:"detected"`
	Cautioned    bool    `json:"cautioned"` // a soft "be careful" was spoken (includes detected calls)
	ScamType     string  `json:"scam_type,omitempty"`
	DetectedAt   float64 `json:"detected_at_sec,omitempty"`
	FinalRisk    float64 `json:"final_risk"`
	FinalState   string  `json:"final_state"`
	Reason       string  `json:"reason"`
	EvonCalls    int     `json:"evon_calls"`
	EvonAvgMs    float64 `json:"evon_avg_ms"`
	EvonFailures int     `json:"evon_failures"`
	// EvonErrors holds the message of each failed model request.
	EvonErrors []string `json:"evon_errors,omitempty"`
	// RateLimited means the provider's limit ran out during this call, so the
	// result is incomplete and must not be scored.
	RateLimited bool   `json:"-"`
	Transcript  string `json:"transcript,omitempty"`
}

// Correct reports whether the verdict matches the label.
func (r Result) Correct() bool { return r.Detected == (r.Label == "scam") }

// Run replays a conversation on a simulated clock, exactly like a live session:
// every line goes through the pre-filter; Evon (if configured) runs on new red
// flags and every interval; otherwise the rule-based score is used.
func Run(c cases.Case, lines []scam.Line, cfg detector.Config, evon *reasoner.Client) Result {
	det := detector.New(cfg)
	switch cfg.ModelMode {
	case detector.ModelDecides:
		det.SetModelDecides(evon.Enabled())
	case detector.ModelOnly:
		det.SetModelOnly(true)
	default: // ModelAdds, as on live calls
	}
	r := Result{ID: c.ID, Label: c.Label, Language: c.Language}
	var evonTotal time.Duration

	analyze := func(now float64) {
		if r.RateLimited {
			return
		}
		det.MarkAnalysisStarted(now)
		if !evon.Enabled() {
			det.ApplyHeuristic(now)
			return
		}
		started := time.Now()
		var rateLimitWait time.Duration
		ctx := reasoner.WithWaitCounter(context.Background(), &rateLimitWait)
		a, err := evon.AnalyzeGuardian(ctx, reasoner.GuardianInput{
			Window: det.Window(now), KnownFlags: det.Flags(), UserLanguage: c.Language, ElapsedSec: now,
		})
		took := time.Since(started) - rateLimitWait // waiting out a rate limit is not model latency
		if err != nil {
			r.EvonFailures++
			r.EvonErrors = append(r.EvonErrors, err.Error())
			r.RateLimited = errors.Is(err, reasoner.ErrRateLimited)
			det.ApplyHeuristic(now)
			return
		}
		r.EvonCalls++
		evonTotal += took
		det.ApplyAnalysis(now+took.Seconds(), a) // in a real call the verdict arrives `took` later
	}

	var texts []string
	for _, line := range lines {
		texts = append(texts, line.Text)
		upd := det.AddLine(line)
		if upd.AnalyzeNow || det.DueForAnalysis(line.T) {
			analyze(line.T)
		}
	}
	if n := len(lines); n > 0 {
		if end := lines[n-1].T + cfg.Interval.Seconds(); det.DueForAnalysis(end) {
			analyze(end) // final check after the last line
		}
	}

	r.Transcript = strings.Join(texts, " ")
	r.DetectedAt, r.Detected = det.DetectedAt()
	if !r.Detected {
		r.DetectedAt = 0
	}
	r.Cautioned = det.Cautioned() || r.Detected
	r.ScamType = string(prefilter.Classify(r.Transcript))
	r.FinalRisk = det.Risk()
	r.FinalState = string(det.State())
	r.Reason = det.Reason()
	if r.EvonCalls > 0 {
		r.EvonAvgMs = float64(evonTotal.Milliseconds()) / float64(r.EvonCalls)
	}
	return r
}

// Summary aggregates results.
type Summary struct {
	Scams, Legit       int
	TP, FN, FP, TN     int
	CautionedScams     int     // scams that got at least the soft caution
	CautionedLegit     int     // genuine calls that got at least the soft caution
	MedianDetect       float64 // seconds into the call, over caught scams
	P95Detect          float64
	EvonMedianMs       float64
	EvonCalls          int // successful model requests
	EvonFailures       int // failed requests: those decisions used rules instead
	HasDetect, HasEvon bool
}

// Summarize computes recall / false positives / time-to-detect.
func Summarize(results []Result) Summary {
	var s Summary
	var detect, evonMs []float64
	for _, r := range results {
		isScam := r.Label == "scam"
		if r.Cautioned && isScam {
			s.CautionedScams++
		} else if r.Cautioned {
			s.CautionedLegit++
		}
		switch {
		case isScam && r.Detected:
			s.TP++
			detect = append(detect, r.DetectedAt)
		case isScam:
			s.FN++
		case r.Detected:
			s.FP++
		default:
			s.TN++
		}
		s.EvonCalls += r.EvonCalls
		s.EvonFailures += r.EvonFailures
		if r.EvonCalls > 0 {
			evonMs = append(evonMs, r.EvonAvgMs)
		}
	}
	s.Scams, s.Legit = s.TP+s.FN, s.FP+s.TN
	if len(detect) > 0 {
		s.HasDetect = true
		s.MedianDetect, s.P95Detect = Percentile(detect, 50), Percentile(detect, 95)
	}
	if len(evonMs) > 0 {
		s.HasEvon, s.EvonMedianMs = true, Percentile(evonMs, 50)
	}
	return s
}

// Recall, FPR and Precision as "88% (7/8)" strings.
func (s Summary) Recall() string    { return Pct(s.TP, s.TP+s.FN) }
func (s Summary) FPR() string       { return Pct(s.FP, s.FP+s.TN) }
func (s Summary) Precision() string { return Pct(s.TP, s.TP+s.FP) }

// Pct formats a/b as a percentage.
func Pct(a, b int) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%% (%d/%d)", 100*float64(a)/float64(b), a, b)
}

// Percentile returns the p-th percentile (0..100) of values.
func Percentile(values []float64, p float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted[int(float64(len(sorted)-1)*p/100)]
}

// Transcribe streams 8 kHz phone audio to Prisma at real-time pace (like a
// phone call) and returns the final transcripts, timestamped by when they
// arrived (seconds since the call started).
func Transcribe(prisma *stt.Client, samples []int16, language string) ([]scam.Line, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	stream, err := prisma.Open(ctx, language)
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	var lines []scam.Line
	start := time.Now()
	done := make(chan struct{})
	go func() {
		for t := range stream.Results() {
			if t.IsFinal {
				mu.Lock()
				lines = append(lines, scam.Line{T: time.Since(start).Seconds(), Text: t.Text})
				mu.Unlock()
			}
		}
		close(done)
	}()

	const frame = 160 // 20 ms at 8 kHz
	padded := append(append([]int16(nil), samples...), make([]int16, audio.PhoneRate*3/2)...)
	next := time.Now()
	for i := 0; i < len(padded); i += frame {
		if err := stream.Send(padded[i:min(i+frame, len(padded))]); err != nil {
			break
		}
		next = next.Add(20 * time.Millisecond) // keep real-time pace without drifting
		time.Sleep(time.Until(next))
	}
	time.Sleep(2 * time.Second) // let the last result arrive
	stream.Close()
	<-done

	mu.Lock()
	defer mu.Unlock()
	return lines, nil
}
