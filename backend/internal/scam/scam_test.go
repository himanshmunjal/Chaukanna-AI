package scam

import (
	"strings"
	"testing"
)

// Every warning must name the helpline and say it comes from an AI, in every language.
func TestEveryLanguageHasACheckedWarning(t *testing.T) {
	for code := range languageNames {
		if _, ok := defaultWarnings[code]; !ok {
			t.Errorf("%s: no warning", code)
		}
		if _, ok := cautions[code]; !ok {
			t.Errorf("%s: no caution", code)
		}
		w := DefaultWarning(code)
		if !strings.Contains(w, "1930") || !strings.Contains(w, "AI") {
			t.Errorf("%s warning lacks 1930 or AI disclosure: %s", code, w)
		}
		if !strings.Contains(Caution(code), "AI") {
			t.Errorf("%s caution lacks AI disclosure", code)
		}
	}
}

func TestWarningForUsesTheFactWhereWeHaveOne(t *testing.T) {
	if w := WarningFor(DigitalArrest, "hi", false); !strings.Contains(w, "डिजिटल अरेस्ट") || !strings.Contains(w, "1930") {
		t.Errorf("hi digital arrest: %s", w)
	}
	if w := WarningFor(DigitalArrest, "en", false); !strings.Contains(w, "Digital arrest does not exist") {
		t.Errorf("en digital arrest: %s", w)
	}
	if w := WarningFor(DigitalArrest, "ta", false); w != DefaultWarning("ta") {
		t.Error("languages without a reviewed fact use the general warning")
	}
	if w := WarningFor(General, "en", false); w != DefaultWarning("en") {
		t.Error("general category uses the general warning")
	}
}

func TestHelplineIsSpokenDigitByDigit(t *testing.T) {
	for code := range languageNames {
		if _, ok := helplineDigits[code]; !ok {
			t.Errorf("%s: no spoken helpline", code)
		}
		if strings.Contains(Spoken(DefaultWarning(code), code), "1930") {
			t.Errorf("%s still says 1930 as a number", code)
		}
	}
	if got := Spoken("call 1930", "en"); got != "call one nine three zero" {
		t.Errorf("got %q", got)
	}
}

// Call #26: a "courier" caller demanded an OTP and was answered with a line about money.
func TestAnOTPDemandGetsAnOTPWarningNotAMoneyOne(t *testing.T) {
	w := WarningFor(Courier, "hi", true)
	if strings.Contains(w, "केस ख़त्म") || !strings.Contains(w, "ओटीपी") {
		t.Errorf("otp demand got: %s", w)
	}
	if w := WarningFor(Courier, "hi", false); !strings.Contains(w, "केस ख़त्म") {
		t.Errorf("money demand got: %s", w)
	}
	if w := WarningFor(DigitalArrest, "hi", true); !strings.Contains(w, "डिजिटल अरेस्ट") {
		t.Errorf("a fact that isn't about money should stay: %s", w)
	}
	otp, money := Flag{Type: OTPRequest}, Flag{Type: MoneyMove}
	if !OTPOnly([]Flag{otp}) || OTPOnly([]Flag{otp, money}) || OTPOnly([]Flag{money}) {
		t.Error("OTPOnly is wrong")
	}
}

func TestEveryLanguageCanInterrupt(t *testing.T) {
	for code := range languageNames {
		if _, ok := stopNow[code]; !ok {
			t.Errorf("%s: no stop line", code)
		}
		if !strings.Contains(StopNow(code), "AI") {
			t.Errorf("%s: stop line lacks AI disclosure", code)
		}
	}
}

func TestEveryLanguageCanAnswerATalkOut(t *testing.T) {
	for code := range languageNames {
		if _, ok := answerTalkOut[code]; !ok {
			t.Errorf("%s: no answer", code)
		}
		if !strings.Contains(AnswerTalkOut(code), "AI") {
			t.Errorf("%s: lacks AI disclosure", code)
		}
	}
}
