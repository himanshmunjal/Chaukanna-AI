package audio

import "testing"

func TestMulawRoundTrip(t *testing.T) {
	in := []int16{0, 100, -100, 1000, -1000, 12000, -12000, 32000, -32000}
	out := MulawDecode(MulawEncode(in))
	for i := range in {
		diff := int(in[i]) - int(out[i])
		if diff < 0 {
			diff = -diff
		}
		// μ-law is lossy; error grows with amplitude but stays within ~3%.
		if limit := abs(int(in[i]))/30 + 8; diff > limit {
			t.Errorf("sample %d: in=%d out=%d (diff %d > %d)", i, in[i], out[i], diff, limit)
		}
	}
}

func TestWAVRoundTrip(t *testing.T) {
	in := []int16{1, -2, 3, -4, 5}
	out, rate, err := ParseWAV(EncodeWAV(in, 16000))
	if err != nil || rate != 16000 || len(out) != len(in) {
		t.Fatalf("got rate=%d len=%d err=%v", rate, len(out), err)
	}
}

func TestResampleLength(t *testing.T) {
	if got := len(Resample(make([]int16, 16000), 16000, 8000)); got != 8000 {
		t.Fatalf("16k->8k: got %d samples", got)
	}
	if got := len(Resample(make([]int16, 8000), 8000, 16000)); got != 16000 {
		t.Fatalf("8k->16k: got %d samples", got)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
