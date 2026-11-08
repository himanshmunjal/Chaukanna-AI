package report

import (
	"strings"
	"testing"
	"time"

	"chaukanna/internal/models"
)

func TestDraftHasTheFactsAndAsksForWhatItDoesNotKnow(t *testing.T) {
	call := models.Call{
		ScamType:      "digital_arrest",
		StartedAt:     time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
		ProtectedUser: &models.ProtectedUser{Name: "Kamla", Phone: "+919800000000"},
		Reason:        "Caller threatened arrest.",
		Flags:         []models.RedFlag{{Type: "fear_threat", TimeSec: 75, Quote: "arrest warrant hai"}},
	}
	r := Build(call)
	for _, want := range []string{"Digital arrest", "2 Nov 2026, 3:30 PM IST", "Kamla", "not captured", "1:15", "arrest warrant hai", "Money lost: [Yes/No"} {
		if !strings.Contains(r.Draft, want) {
			t.Errorf("draft misses %q:\n%s", want, r.Draft)
		}
	}
	if r.ScamTypeLabel == "" || len(r.Links) == 0 {
		t.Error("label or links missing")
	}
}

func TestUnknownCategoryFallsBack(t *testing.T) {
	if r := Build(models.Call{}); r.ScamType != "general" || r.ScamTypeLabel != "Phone fraud" {
		t.Errorf("got %+v", r)
	}
}
