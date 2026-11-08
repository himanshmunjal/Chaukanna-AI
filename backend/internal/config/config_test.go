package config

import "testing"

func TestLLMPlusRulesSwitch(t *testing.T) {
	t.Setenv("EVON_BASE_URL", "http://model/v1")
	t.Setenv("EVON_MODEL", "evon")

	t.Setenv("LLM_PLUS_RULES", "false")
	if c := Load(); c.LLMPlusRules || c.EvonBaseURL != "" || c.EvonModel != "" {
		t.Errorf("false should leave no model configured; got %+v, %q, %q", c.LLMPlusRules, c.EvonBaseURL, c.EvonModel)
	}

	t.Setenv("LLM_PLUS_RULES", "true")
	if c := Load(); !c.LLMPlusRules || c.EvonBaseURL == "" || c.EvonModel != "evon" {
		t.Errorf("true should keep the model; got %+v, %q, %q", c.LLMPlusRules, c.EvonBaseURL, c.EvonModel)
	}
}
