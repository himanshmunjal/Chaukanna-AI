package evaluate

import "testing"

func TestWilsonRange(t *testing.T) {
	// Known values: 22/28 is about 60-90%, 2/12 about 5-45%.
	if low, high := WilsonRange(22, 28); low < 59 || low > 61 || high < 88 || high > 91 {
		t.Errorf("22/28: %.1f-%.1f", low, high)
	}
	if low, high := WilsonRange(2, 12); low < 3 || low > 6 || high < 43 || high > 46 {
		t.Errorf("2/12: %.1f-%.1f", low, high)
	}
	if low, _ := WilsonRange(0, 10); low != 0 {
		t.Errorf("0/10 low = %.1f", low)
	}
	if _, high := WilsonRange(10, 10); high != 100 {
		t.Errorf("10/10 high = %.1f", high)
	}
	if low, high := WilsonRange(0, 0); low != 0 || high != 0 {
		t.Error("no data")
	}
}
