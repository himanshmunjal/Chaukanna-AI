// Package detector holds the decision logic for one call: which red flags were
// seen, the current risk score, and the risk state machine.
//
//	listening ──(risk ≥ suspicious)──> suspicious ──(risk ≥ scam)──> scam ──(warning played)──> warned
//	    ^                                   │
//	    └──────(risk < clear, 2 windows)────┘
//
// The detector does no network calls and does not read the clock: the caller
// passes the time (seconds since call start) into every method. That keeps it
// easy to test, and lets the benchmark replay calls faster than real time.
//
// It is NOT safe for concurrent use; the session guards it with a mutex.
package detector

import (
	"sort"
	"strings"
	"time"

	"chaukanna/internal/prefilter"
	"chaukanna/internal/scam"
)

// State is where a call is in the state machine.
type State string

const (
	Listening  State = "listening"
	Suspicious State = "suspicious"
	Scam       State = "scam"
	Warned     State = "warned"
)

// Config holds the thresholds. See .env for the defaults.
type Config struct {
	RiskSuspicious float64
	RiskCaution    float64 // risk at which a soft "be careful" is spoken; 0 = no caution tier
	RiskScam       float64
	RiskClear      float64
	Window         time.Duration // how much recent transcript Evon sees
	Interval       time.Duration // how often Evon re-checks
	ModelMode      string        // benchmarks only: ModelAdds (default), ModelDecides or ModelOnly
}

// How a benchmark combines the model and the rules. Live calls use ModelAdds.
const (
	ModelAdds    = "adds"    // the rules' warnings stand, and the model can add more
	ModelDecides = "decides" // rule hits go to the model first; direct demands act at once
	ModelOnly    = "only"    // no rules at all
)

// Update tells the caller what changed after a line or an analysis.
type Update struct {
	NewFlags      []scam.Flag // flags seen for the first time
	NewExtraction bool        // one of NewFlags is an OTP/app/money request
	AnalyzeNow    bool        // ask Evon immediately (don't wait for the timer)
	CautionDue    bool        // risk just reached the caution level (fires once per call)
	Complying     bool        // on a doubtful call, the person seems to be giving a code or money
	RepeatDemand  bool        // after the warning, the caller asked again for a code, app or money
	TalkOut       bool        // after the warning, the caller tried to discredit it
	StateChanged  bool
	State         State
}

// Detector tracks one call.
type Detector struct {
	cfg Config

	lines []scam.Line
	flags map[scam.FlagType]scam.Flag // first occurrence of each flag type

	state        State
	risk         float64
	riskFloor    float64 // once a call is a scam, its risk never drops below the level at detection
	modelDecides bool    // see SetModelDecides
	modelOnly    bool    // see SetModelOnly
	strongDemand bool    // a direct demand was heard ("tell me the OTP"); no model score can undo it
	cautioned    bool
	digitsHeard  int     // digits read out recently, across lines
	lastDigitsT  float64 // when the last ones were heard
	reason       string
	warningText  string
	language     string

	lastAnalysisT       float64 // time of the last Evon/heuristic run
	linesAtLastAnalysis int     // how many lines existed at that time
	calmWindows         int     // consecutive low-risk analyses while suspicious
	detectedAt          float64 // first time we reached "scam" (-1 = never)
}

// New creates a detector for a fresh call.
func New(cfg Config) *Detector {
	return &Detector{
		cfg:        cfg,
		flags:      map[scam.FlagType]scam.Flag{},
		state:      Listening,
		detectedAt: -1,
	}
}

// SetModelDecides is a benchmark setup, not used on live calls. It lets the model judge
// rule hits instead of warning on them straight away. In tests it missed more scams.
// Direct demands and a code being read out still act at once, without waiting for it.
// If a model request fails, ApplyHeuristic scores the call from the rules as usual.
func (d *Detector) SetModelDecides(on bool) { d.modelDecides = on }

// SetModelOnly switches the rules off completely, so a benchmark can measure the model on
// its own. Every line goes to the model. Live calls never use this, because it also drops
// the direct-demand and read-out checks.
func (d *Detector) SetModelOnly(on bool) {
	d.modelOnly = on
	d.modelDecides = on
}

// AddLine records a final transcript line and runs the fast pre-filter on it.
func (d *Detector) AddLine(line scam.Line) Update {
	d.lines = append(d.lines, line)

	var upd Update
	if d.modelOnly {
		upd.AnalyzeNow = true
		d.transition(line.T, false, &upd)
		return upd
	}
	upd.RepeatDemand = d.state == Warned && prefilter.AsksForSomething(line.Text)
	upd.TalkOut = d.state == Warned && prefilter.DiscreditsWarning(line.Text)
	for _, hit := range prefilter.Scan(line.Text) {
		d.strongDemand = d.strongDemand || hit.Strong
		f := scam.Flag{Type: hit.Type, T: line.T, Quote: shorten(line.Text), Source: "prefilter"}
		if d.addFlag(f) {
			upd.NewFlags = append(upd.NewFlags, f)
			upd.NewExtraction = upd.NewExtraction || scam.IsExtraction(f.Type)
		}
	}
	upd.AnalyzeNow = len(upd.NewFlags) > 0

	// Safety rule: a setup tactic (threat, fake authority, urgency, prize) followed by
	// an OTP/app/money request is a scam,
	// even before Evon answers. A real bank may mention "OTP" ("never share your
	// OTP"), which is why an extraction word *alone* is not enough.
	if !d.modelDecides && d.hasSetupAndExtraction() && d.risk < d.cfg.RiskScam {
		d.risk = d.cfg.RiskScam
		d.reason = "Caller used pressure tactics and then asked for an OTP, app install or money transfer."
	}
	// A direct demand ("tell me the OTP", "install AnyDesk") is a scam on its own.
	if d.strongDemand && d.risk < d.cfg.RiskScam {
		d.risk = d.cfg.RiskScam
		d.reason = "Caller directly asked for an OTP, PIN, remote-access app or QR/UPI approval. No genuine caller does this."
	}
	if !d.modelDecides && d.risk < d.cfg.RiskScam { // keep the running rule score visible between Evon checks
		if r := prefilter.RiskFromFlags(d.flagTypes()); r > d.risk {
			d.risk = r
		}
	}

	// Handing over a code on a call that already looks wrong is the emergency this whole
	// system exists for. On a call with no other red flags it means nothing ("I'll pay you
	// on Friday"), so it needs the call to be doubtful first.
	if (prefilter.Complying(line.Text) || d.readingOutDigits(line)) && d.complyingMatters() {
		upd.Complying = true
		if d.risk < d.cfg.RiskScam {
			d.risk = d.cfg.RiskScam
			d.reason = "The person seems to be giving a code or money on a doubtful call."
		}
	}

	d.transition(line.T, false, &upd)
	return upd
}

// A code read out slowly can arrive split over several lines.
const (
	digitsWindow     = 20.0 // seconds between lines that count as one read-out
	digitsWhenScam   = 3    // once the call is flagged, this many digits is enough to interrupt
	digitsWhenUnsure = 4    // on a merely doubtful call, want a code's worth
)

// readingOutDigits adds up digits across lines made only of digits, and reports whether
// the person seems to be reading out a code. It stops the moment a code begins, because
// each digit that gets out is one the scammer already has.
func (d *Detector) readingOutDigits(line scam.Line) bool {
	run, only := prefilter.DigitRun(line.Text)
	total := run
	if only {
		if line.T-d.lastDigitsT > digitsWindow {
			d.digitsHeard = 0
		}
		d.digitsHeard += run
		d.lastDigitsT = line.T
		total = d.digitsHeard
	}
	if d.IsDangerous() {
		return total >= digitsWhenScam
	}
	return total >= digitsWhenUnsure
}

func (d *Detector) complyingMatters() bool {
	if d.IsDangerous() {
		return true
	}
	threshold := d.cfg.RiskCaution
	if threshold <= 0 {
		threshold = d.cfg.RiskScam
	}
	// The model may not have answered yet, and a code getting out can't wait for it.
	return max(d.risk, prefilter.RiskFromFlags(d.flagTypes())) >= threshold
}

// DueForAnalysis reports whether the periodic Evon check should run now:
// there are new lines and enough time has passed since the last check.
func (d *Detector) DueForAnalysis(now float64) bool {
	return len(d.lines) > d.linesAtLastAnalysis &&
		now-d.lastAnalysisT >= d.cfg.Interval.Seconds()
}

// MarkAnalysisStarted must be called right before running an analysis.
func (d *Detector) MarkAnalysisStarted(now float64) {
	d.lastAnalysisT = now
	d.linesAtLastAnalysis = len(d.lines)
}

// ApplyAnalysis merges an Evon result into the call state.
func (d *Detector) ApplyAnalysis(now float64, a scam.Analysis) Update {
	var upd Update
	for _, f := range a.Flags {
		if !scam.IsValid(string(f.Type)) {
			continue // ignore made-up flag names
		}
		f.Source = "evon"
		f.Quote = shorten(f.Quote)
		if d.addFlag(f) {
			upd.NewFlags = append(upd.NewFlags, f)
			upd.NewExtraction = upd.NewExtraction || scam.IsExtraction(f.Type)
		}
	}

	d.risk = clamp(a.Risk)
	if (d.strongDemand || (!d.modelDecides && d.hasSetupAndExtraction())) && d.risk < d.cfg.RiskScam {
		d.risk = d.cfg.RiskScam // the safety rule wins over a low model score
	}
	if a.Reason != "" {
		d.reason = a.Reason
	}
	if a.WarningText != "" {
		d.warningText = a.WarningText
	}
	if a.Language != "" {
		d.language = a.Language
	}

	d.applyFloor()
	d.transition(now, true, &upd)
	return upd
}

// ApplyHeuristic scores the call from its flags alone. Used when Evon is not
// configured or its request failed, so protection never stops.
func (d *Detector) ApplyHeuristic(now float64) Update {
	types := make([]scam.FlagType, 0, len(d.flags))
	names := make([]string, 0, len(d.flags))
	for _, f := range d.Flags() {
		types = append(types, f.Type)
		names = append(names, strings.ReplaceAll(string(f.Type), "_", " "))
	}
	d.risk = prefilter.RiskFromFlags(types)
	if d.strongDemand && d.risk < d.cfg.RiskScam {
		d.risk = d.cfg.RiskScam
	}
	if len(names) > 0 {
		d.reason = "Rule-based check found: " + strings.Join(names, ", ") + "."
	}
	var upd Update
	d.applyFloor()
	d.transition(now, true, &upd)
	return upd
}

// RaiseToHeuristic raises the risk to at least the rule-based score of the
// flags seen so far (it never lowers it, so an Evon verdict is kept).
// Screen Mode uses it for red flags that arrive after the bot stopped asking
// questions, so a late "CBI ... arrest warrant" still counts.
func (d *Detector) RaiseToHeuristic(now float64) Update {
	types := make([]scam.FlagType, 0, len(d.flags))
	for t := range d.flags {
		types = append(types, t)
	}
	var upd Update
	if r := prefilter.RiskFromFlags(types); r > d.risk {
		d.risk = r
		d.reason = "Rule-based check of the red flags heard on the call."
		d.transition(now, true, &upd)
	}
	upd.State = d.state
	return upd
}

// MarkKnownScam flags the call from the start because the caller's number was
// caught scamming before.
func (d *Detector) MarkKnownScam(now float64, reason string) Update {
	d.risk = 0.99
	d.reason = reason
	var upd Update
	d.transition(now, true, &upd)
	return upd
}

// MarkWarned records that the spoken warning was played.
func (d *Detector) MarkWarned(now float64) Update {
	upd := Update{State: Warned, StateChanged: d.state != Warned}
	if d.detectedAt < 0 {
		d.detectedAt = now
	}
	d.state = Warned
	return upd
}

// ---------- getters ----------

func (d *Detector) State() State                { return d.state }
func (d *Detector) Risk() float64               { return d.risk }
func (d *Detector) Reason() string              { return d.reason }
func (d *Detector) WarningText() string         { return d.warningText }
func (d *Detector) Language() string            { return d.language }
func (d *Detector) Lines() []scam.Line          { return append([]scam.Line(nil), d.lines...) }
func (d *Detector) Cautioned() bool             { return d.cautioned }
func (d *Detector) IsDangerous() bool           { return d.state == Scam || d.state == Warned }
func (d *Detector) DetectedAt() (float64, bool) { return d.detectedAt, d.detectedAt >= 0 }

// Flags returns all detected flags, oldest first.
func (d *Detector) Flags() []scam.Flag {
	out := make([]scam.Flag, 0, len(d.flags))
	for _, f := range d.flags {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

// Window returns the lines from the last cfg.Window seconds.
func (d *Detector) Window(now float64) []scam.Line {
	from := now - d.cfg.Window.Seconds()
	var out []scam.Line
	for _, l := range d.lines {
		if l.T >= from {
			out = append(out, l)
		}
	}
	return out
}

// ---------- internals ----------

// transition moves the state machine based on the current risk.
// isAnalysis is true when called after an Evon/heuristic run (only those count
// towards "calming down", single lines do not).
func (d *Detector) transition(now float64, isAnalysis bool, upd *Update) {
	prev := d.state

	switch {
	case d.state == Warned:
		// Stay warned until the call ends. Re-warnings are handled by the session.
	case d.risk >= d.cfg.RiskScam:
		d.state = Scam
	case d.state == Scam:
		// Never downgrade a scam during the same call: safety first.
	case d.risk >= d.cfg.RiskSuspicious:
		d.state = Suspicious
		d.calmWindows = 0
	case d.state == Suspicious && isAnalysis && d.risk < d.cfg.RiskClear:
		d.calmWindows++
		if d.calmWindows >= 2 {
			d.state = Listening
			d.calmWindows = 0
		}
	}

	if d.state == Scam && d.detectedAt < 0 {
		d.detectedAt = now
	}
	if d.state == Scam || d.state == Warned {
		d.riskFloor = max(d.riskFloor, d.risk) // remember how sure we were
	}
	if !d.cautioned && d.cfg.RiskCaution > 0 && d.risk >= d.cfg.RiskCaution {
		d.cautioned = true
		// A call going straight to "scam" gets the full warning instead.
		upd.CautionDue = d.state != Scam && d.state != Warned
	}
	upd.State = d.state
	upd.StateChanged = d.state != prev
}

func (d *Detector) flagTypes() []scam.FlagType {
	types := make([]scam.FlagType, 0, len(d.flags))
	for t := range d.flags {
		types = append(types, t)
	}
	return types
}

// addFlag stores a flag if its type was not seen before. Returns true if new.
func (d *Detector) addFlag(f scam.Flag) bool {
	if _, exists := d.flags[f.Type]; exists {
		return false
	}
	d.flags[f.Type] = f
	return true
}

// applyFloor keeps the risk of a detected scam from falling when later,
// harmless sentences are scored (e.g. the victim talking after the warning).
func (d *Detector) applyFloor() {
	if d.risk < d.riskFloor {
		d.risk = d.riskFloor
	}
}

func (d *Detector) hasSetupAndExtraction() bool {
	setup, extraction := false, false
	for t := range d.flags {
		setup = setup || scam.IsSetup(t)
		extraction = extraction || scam.IsExtraction(t)
	}
	return setup && extraction
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func shorten(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > 160 {
		return string(r[:157]) + "..."
	}
	return string(r)
}
