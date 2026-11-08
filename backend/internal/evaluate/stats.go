package evaluate

import (
	"fmt"
	"math"
)

// WilsonRange is the 95% confidence range of k successes in n tries, as percentages.
// On 30 calls a single score hides a lot of uncertainty, so reports should show this.
func WilsonRange(k, n int) (low, high float64) {
	if n == 0 {
		return 0, 0
	}
	const z = 1.96
	p, nf := float64(k)/float64(n), float64(n)
	denom := 1 + z*z/nf
	centre := (p + z*z/(2*nf)) / denom
	margin := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf)) / denom
	return 100 * max(0, centre-margin), 100 * min(1, centre+margin)
}

// RangeText formats WilsonRange, e.g. "95% range 60-90%".
func RangeText(k, n int) string {
	low, high := WilsonRange(k, n)
	return fmt.Sprintf("95%% range %.0f-%.0f%%", low, high)
}
