package detector

import (
	"testing"
	"time"

	"chaukanna/internal/scam"
)

func testConfig() Config {
	return Config{RiskSuspicious: 0.4, RiskScam: 0.8, RiskClear: 0.3, Window: 60 * time.Second, Interval: 5 * time.Second}
}

func TestDigitalArrestBecomesScam(t *testing.T) {
	d := New(testConfig())
	d.AddLine(scam.Line{T: 2, Text: "Namaste, main CBI headquarters se Inspector Sharma bol raha hoon"})
	d.AddLine(scam.Line{T: 9, Text: "Aapke naam par money laundering ka case hai, arrest warrant nikla hai"})
	upd := d.AddLine(scam.Line{T: 20, Text: "Abhi apne account ka paisa RBI safe account mein transfer kijiye"})

	if d.State() != Scam {
		t.Fatalf("expected scam, got %s (risk %.2f)", d.State(), d.Risk())
	}
	if !upd.NewExtraction {
		t.Fatal("expected the transfer request to be flagged as extraction")
	}
	if at, ok := d.DetectedAt(); !ok || at != 20 {
		t.Fatalf("expected detection at 20s, got %v %v", at, ok)
	}
}

func TestRealBankReminderStaysCalm(t *testing.T) {
	d := New(testConfig())
	d.AddLine(scam.Line{T: 1, Text: "Good afternoon, this is a reminder that your loan EMI is due on the 5th"})
	d.AddLine(scam.Line{T: 8, Text: "Please never share your OTP with anyone. Thank you for banking with us"})
	d.ApplyHeuristic(10)

	if d.State() == Scam || d.State() == Warned {
		t.Fatalf("legit call marked as %s (risk %.2f)", d.State(), d.Risk())
	}
}

func TestEvonCannotDowngradeSafetyRule(t *testing.T) {
	d := New(testConfig())
	d.AddLine(scam.Line{T: 1, Text: "Main police se bol raha hoon"})
	d.AddLine(scam.Line{T: 5, Text: "AnyDesk download kijiye"})
	d.ApplyAnalysis(6, scam.Analysis{Risk: 0.1})

	if d.Risk() < 0.8 {
		t.Fatalf("pressure + extraction must keep risk >= scam threshold, got %.2f", d.Risk())
	}
}

func TestRaiseToHeuristicNeverLowers(t *testing.T) {
	d := New(testConfig())
	d.AddLine(scam.Line{T: 1, Text: "Main CBI se bol raha hoon, arrest warrant hai"})
	d.RaiseToHeuristic(1)
	if d.Risk() < 0.5 {
		t.Fatalf("authority + fear should raise risk, got %.2f", d.Risk())
	}
	d.ApplyAnalysis(2, scam.Analysis{Risk: 0.97})
	d.RaiseToHeuristic(3)
	if d.Risk() != 0.97 {
		t.Fatalf("must not lower an Evon verdict, got %.2f", d.Risk())
	}
}

func TestKnownScamNumber(t *testing.T) {
	d := New(testConfig())
	upd := d.MarkKnownScam(0, "reported before")
	if d.State() != Scam || !upd.StateChanged {
		t.Fatalf("known scam number must start as scam, got %s", d.State())
	}
}

func TestScamRiskNeverDrops(t *testing.T) {
	d := New(testConfig())
	d.MarkKnownScam(0, "reported before")
	d.AddLine(scam.Line{T: 1, Text: "दादी नमस्ते, कैसी हो आप?"})
	d.ApplyHeuristic(2)                           // no flags -> rule score 0
	d.ApplyAnalysis(3, scam.Analysis{Risk: 0.05}) // a model saying "looks fine now"
	if d.Risk() < 0.99 {
		t.Fatalf("risk of a detected scam dropped to %.2f", d.Risk())
	}
}

func tieredConfig() Config {
	return Config{RiskSuspicious: 0.4, RiskCaution: 0.55, RiskScam: 0.8, RiskClear: 0.3, Window: time.Minute, Interval: 5 * time.Second}
}

func TestDirectDemandIsAScamOnItsOwn(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "Please tell me the OTP that you received"})
	if d.State() != Scam {
		t.Fatalf("state = %s, risk %.2f", d.State(), d.Risk())
	}
}

func TestLowModelScoreCannotUndoADirectDemand(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "Please install AnyDesk"})
	d.ApplyAnalysis(2, scam.Analysis{Risk: 0.05})
	if d.Risk() < 0.8 {
		t.Errorf("risk fell to %.2f", d.Risk())
	}
}

func TestCautionFiresOnceAndNotForAProvenScam(t *testing.T) {
	d := New(tieredConfig())
	first := d.AddLine(scam.Line{T: 1, Text: "Main customs officer bol raha hoon"})
	if first.CautionDue {
		t.Error("authority alone should not caution")
	}
	second := d.AddLine(scam.Line{T: 5, Text: "Aapke naam par arrest warrant hai"})
	if !second.CautionDue {
		t.Errorf("authority + threat should caution; risk %.2f", d.Risk())
	}
	third := d.AddLine(scam.Line{T: 9, Text: "Jail jaana padega, court case hoga"})
	if third.CautionDue {
		t.Error("caution should fire only once")
	}

	proven := New(tieredConfig())
	if upd := proven.AddLine(scam.Line{T: 1, Text: "Tell me the OTP"}); upd.CautionDue {
		t.Error("a proven scam gets the full warning, not a caution")
	}
}

// The exact lines from call #26, after the warning had been played.
func TestCompliesAfterAWarningIsNoticed(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "दादी, मैं आपका courier वाला बोल रहा हूँ। आप अपना OTP मेरे साथ share करिए।"})
	d.MarkWarned(2)
	for _, line := range []string{"1, 2, 3, 4।", "जो भी OTP वगैरह चाहिए, मैं तुम्हें UPI से payment कर दूंगी।"} {
		if upd := d.AddLine(scam.Line{T: 3, Text: line}); !upd.Complying {
			t.Errorf("%q should count as complying", line)
		}
	}
	if upd := d.AddLine(scam.Line{T: 4, Text: "नहीं, मैं OTP नहीं दूंगी"}); upd.Complying {
		t.Error("a refusal is not complying")
	}
}

func TestComplyingOnAnOrdinaryCallIsNothing(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "Beta, kal subah milte hain"})
	upd := d.AddLine(scam.Line{T: 2, Text: "Haan, main shukravar ko payment kar dunga"})
	if upd.Complying || d.State() != Listening {
		t.Errorf("ordinary call flagged: complying %v, state %s", upd.Complying, d.State())
	}
}

func TestComplyingOnADoubtfulCallRaisesIt(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "Main customs officer bol raha hoon"})
	d.AddLine(scam.Line{T: 2, Text: "Aapke naam par arrest warrant hai"})
	if d.State() == Scam {
		t.Fatal("should only be doubtful so far")
	}
	if upd := d.AddLine(scam.Line{T: 3, Text: "Theek hai, main paise bhej deta hoon"}); !upd.Complying || d.State() != Scam {
		t.Errorf("complying %v, state %s", upd.Complying, d.State())
	}
}

// Call #29: Prisma heard "1 2 3", one digit short of the old limit, and nothing happened.
func TestShortReadOutAfterAWarningIsNoticed(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 8.7, Text: "मैं SBI से manager बोल रहा हूँ। आपके phone पर जो OTP आया है वो बता दीजिए।"})
	d.MarkWarned(9)
	if upd := d.AddLine(scam.Line{T: 32.9, Text: "1 2 3"}); !upd.Complying {
		t.Error("three digits on a flagged call should interrupt")
	}
}

func TestSplitReadOutIsAddedUp(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "Tell me the OTP"})
	d.MarkWarned(2)
	if upd := d.AddLine(scam.Line{T: 3, Text: "चार"}); upd.Complying {
		t.Error("one digit alone should not interrupt")
	}
	if upd := d.AddLine(scam.Line{T: 5, Text: "5"}); upd.Complying {
		t.Error("two digits should not interrupt yet")
	}
	if upd := d.AddLine(scam.Line{T: 7, Text: "seven"}); !upd.Complying {
		t.Error("three digits across lines should interrupt")
	}
}

func TestOldDigitsDoNotCount(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "Tell me the OTP"})
	d.MarkWarned(2)
	d.AddLine(scam.Line{T: 3, Text: "4 5"})
	if upd := d.AddLine(scam.Line{T: 60, Text: "9"}); upd.Complying {
		t.Error("digits a minute apart are not one code")
	}
}

func TestNumbersInSentencesDoNotInterrupt(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "Tell me the OTP"})
	d.MarkWarned(2)
	for _, line := range []string{"Main 10 baje aaungi", "Mere paas 5 lakh nahi hain", "1930 pe call karungi", "एक नौ तीन शून्य"} {
		if upd := d.AddLine(scam.Line{T: 5, Text: line}); upd.Complying {
			t.Errorf("%q should not count as reading out a code", line)
		}
	}
}

func TestDigitsOnAnOrdinaryCallMeanNothing(t *testing.T) {
	d := New(tieredConfig())
	d.AddLine(scam.Line{T: 1, Text: "Dadi, mera naya number likh lo"})
	if upd := d.AddLine(scam.Line{T: 3, Text: "9 8 1 2 3 4 5 6 7 8"}); upd.Complying || d.State() != Listening {
		t.Errorf("complying %v, state %s", upd.Complying, d.State())
	}
}

// Call #29: "कृपया अपना OTP बताइए" after the warning got no response.
func TestAskingAgainAfterTheWarningIsNoticed(t *testing.T) {
	d := New(tieredConfig())
	first := d.AddLine(scam.Line{T: 1, Text: "मैं SBI से manager बोल रहा हूँ। आपके phone पर जो OTP आया है वो बता दीजिए।"})
	if first.RepeatDemand {
		t.Error("the first demand gets the full warning, not a repeat")
	}
	d.MarkWarned(2)
	if upd := d.AddLine(scam.Line{T: 20, Text: "ये एक fraud call नहीं है दादी। कृपया अपना OTP बताइए।"}); !upd.RepeatDemand {
		t.Error("asking again should be noticed")
	}
	for _, line := range []string{"OTP", "OTP kya hota hai", "Nahi, main OTP nahi dungi", "Mujhe kuch nahi samajh aa raha", "Haan beta, khana kha liya"} {
		if upd := d.AddLine(scam.Line{T: 30, Text: line}); upd.RepeatDemand {
			t.Errorf("%q is not the caller asking again", line)
		}
	}
}

func TestTalkOutOnlyCountsAfterAWarning(t *testing.T) {
	d := New(tieredConfig())
	if upd := d.AddLine(scam.Line{T: 1, Text: "Ye scam nahi hai madam"}); upd.TalkOut {
		t.Error("before any warning there is nothing to talk them out of")
	}
	d.AddLine(scam.Line{T: 2, Text: "OTP bata dijiye"})
	d.MarkWarned(3)
	if upd := d.AddLine(scam.Line{T: 4, Text: "Ye scam nahi hai madam"}); !upd.TalkOut {
		t.Error("after a warning this is a talk-out")
	}
}

// With a model deciding, words about a scam don't trigger a warning before it answers.
func TestModelDecidesOnRuleHits(t *testing.T) {
	d := New(testConfig())
	d.SetModelDecides(true)
	line := scam.Line{T: 8, Text: "दादी, आजकल लोग CBI बनकर phone करते हैं और digital arrest बोलते हैं"}
	if upd := d.AddLine(line); upd.State != Listening || !upd.AnalyzeNow {
		t.Fatalf("should keep listening and ask the model; got %+v", upd)
	}
	if upd := d.ApplyAnalysis(9, scam.Analysis{Risk: 0.1}); upd.State != Listening {
		t.Fatalf("a low model score should stand; got %s", upd.State)
	}
	if upd := d.ApplyAnalysis(10, scam.Analysis{Risk: 0.9}); upd.State != Scam {
		t.Fatalf("a high model score should flag the call; got %s", upd.State)
	}
}

func TestModelDecidesKeepsImmediateSafetyRules(t *testing.T) {
	d := New(testConfig())
	d.SetModelDecides(true)
	if upd := d.AddLine(scam.Line{T: 3, Text: "Please tell me the OTP"}); upd.State != Scam {
		t.Fatalf("a direct demand must not wait for the model; got %s", upd.State)
	}

	cfg := testConfig()
	cfg.RiskCaution = 0.55
	d = New(cfg)
	d.SetModelDecides(true)
	d.AddLine(scam.Line{T: 3, Text: "Main CBI se bol raha hoon, arrest warrant hai"})
	if upd := d.AddLine(scam.Line{T: 6, Text: "4 7 2 9"}); !upd.Complying {
		t.Fatal("a code read out on a doubtful call must not wait for the model")
	}
}

func TestModelFailureFallsBackToRules(t *testing.T) {
	d := New(testConfig())
	d.SetModelDecides(true)
	d.AddLine(scam.Line{T: 3, Text: "Main CBI se bol raha hoon, arrest warrant hai"})
	d.AddLine(scam.Line{T: 6, Text: "paise safe account mein bhejo"})
	if upd := d.ApplyHeuristic(7); upd.State != Scam {
		t.Fatalf("rules should still protect when the model is down; got %s", upd.State)
	}
}

func TestModelOnlyIgnoresTheRules(t *testing.T) {
	d := New(testConfig())
	d.SetModelOnly(true)
	upd := d.AddLine(scam.Line{T: 3, Text: "Please tell me the OTP"})
	if upd.State != Listening || !upd.AnalyzeNow || len(upd.NewFlags) != 0 {
		t.Fatalf("rules should do nothing, and the model should be asked; got %+v", upd)
	}
	if upd := d.ApplyAnalysis(4, scam.Analysis{Risk: 0.1}); upd.State != Listening {
		t.Fatalf("a low model score should stand; got %s", upd.State)
	}
	if upd := d.ApplyAnalysis(5, scam.Analysis{Risk: 0.9}); upd.State != Scam {
		t.Fatalf("a high model score should flag the call; got %s", upd.State)
	}
}
