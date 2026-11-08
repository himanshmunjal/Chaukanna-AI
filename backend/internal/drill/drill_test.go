package drill

import "testing"

func TestJudge(t *testing.T) {
	cases := map[string]Verdict{
		"नहीं, मैं नहीं बताऊंगी":          Safe,
		"मैं फ़ोन काट रहा हूँ":            Safe,
		"No, I will call the bank myself": Safe,
		"aap kaun ho":                     Safe,
		"I will call 1930":                Safe,
		"ok, but no, I won't share it":    Safe,
		"haan bhej deta hoon":             Risky,
		"my otp is 4512":                  Risky,
		"four five one two":               Risky,
		"चार पांच एक दो":                  Risky,
		"no it is 4512":                   Risky,
		"ठीक है, अभी कर देती हूँ":         Risky,
		"hmm the weather":                 Unclear,
		"":                                Unclear,
	}
	for reply, want := range cases {
		if got := Judge(reply); got != want {
			t.Errorf("Judge(%q) = %s, want %s", reply, got, want)
		}
	}
}

func TestEveryScenarioHasSteps(t *testing.T) {
	for _, s := range Scenarios {
		if len(s.Steps) < 3 || s.Lang == "" {
			t.Errorf("%s is incomplete", s.ID)
		}
		if _, ok := Find(s.ID); !ok {
			t.Errorf("Find(%s) failed", s.ID)
		}
	}
}
