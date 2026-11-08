package noise

import (
	"math"
	"testing"
)

func TestMixHitsTargetSNR(t *testing.T) {
	// A 1-second 300 Hz "voice" and street noise mixed at 10 dB.
	speech := make([]int16, rate)
	for i := range speech {
		speech[i] = int16(8000 * math.Sin(2*math.Pi*300*float64(i)/rate))
	}
	n := Street(len(speech), 1)
	mixed := Mix(speech, n, 10)

	var sig, noisePow float64
	for i := range speech {
		sig += float64(speech[i]) * float64(speech[i])
		d := float64(mixed[i]) - float64(speech[i])
		noisePow += d * d
	}
	snr := 10 * math.Log10(sig/noisePow)
	if math.Abs(snr-10) > 1 {
		t.Fatalf("wanted ~10 dB SNR, got %.1f dB", snr)
	}
}

func TestNoiseIsReproducible(t *testing.T) {
	a, b := Street(1000, 42), Street(1000, 42)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("same seed must give the same noise")
		}
	}
}
