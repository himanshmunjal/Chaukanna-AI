// Package noise makes realistic background noise and mixes it into speech at
// a chosen signal-to-noise ratio (SNR), to test Chaukanna on noisy phone calls.
//
// SNR in decibels: how much louder the voice is than the noise.
//
//	20 dB  quiet room         10 dB  busy street
//	 5 dB  loud traffic        0 dB  noise as loud as the voice
//
// Noise types:
//
//	street  traffic rumble + engine hum + occasional horns (synthetic)
//	babble  several other people talking at once (built from other test calls)
//	white   plain hiss
//	file    any WAV you record yourself (e.g. 30 s of a real market on your phone)
package noise

import (
	"math"
	"math/rand"
	"os"

	"chaukanna/internal/audio"
)

const rate = audio.PhoneRate

// Street returns n samples of synthetic street noise with RMS 1.
func Street(n int, seed int64) []float64 {
	rng := rand.New(rand.NewSource(seed))
	out := make([]float64, n)

	// 1) Traffic rumble: "brown" noise (white noise, heavily low-passed).
	brown := 0.0
	for i := range out {
		brown = 0.985*brown + 0.015*rng.NormFloat64()
		out[i] = brown
	}
	normalize(out)

	// 2) Engine hum: low tones whose loudness slowly rises and falls (passing vehicles).
	for i := range out {
		t := float64(i) / rate
		swell := 0.5 + 0.5*math.Sin(2*math.Pi*0.15*t+1.3)
		out[i] += 0.35 * swell * (math.Sin(2*math.Pi*58*t) + 0.5*math.Sin(2*math.Pi*116*t) + 0.25*math.Sin(2*math.Pi*174*t))
	}

	// 3) Horns: a two-tone honk every 2–6 seconds.
	for start := int(rng.Float64() * 2 * rate); start < n; start += int((2 + 4*rng.Float64()) * rate) {
		length := int((0.25 + 0.45*rng.Float64()) * rate)
		f1, f2 := 400+80*rng.Float64(), 500+80*rng.Float64()
		for j := 0; j < length && start+j < n; j++ {
			t := float64(j) / rate
			env := math.Min(1, math.Min(t*40, float64(length-j)/rate*40)) // soft attack/release
			out[start+j] += 1.6 * env * (sign(math.Sin(2*math.Pi*f1*t)) + sign(math.Sin(2*math.Pi*f2*t))) / 2
		}
	}

	normalize(out)
	return out
}

// White returns n samples of white noise with RMS 1.
func White(n int, seed int64) []float64 {
	rng := rand.New(rand.NewSource(seed))
	out := make([]float64, n)
	for i := range out {
		out[i] = rng.NormFloat64()
	}
	normalize(out)
	return out
}

// Babble overlaps several speech clips (other people talking) into n samples with RMS 1.
func Babble(n int, clips [][]int16, voices int, seed int64) []float64 {
	rng := rand.New(rand.NewSource(seed))
	out := make([]float64, n)
	if len(clips) == 0 {
		return White(n, seed)
	}
	for v := 0; v < voices; v++ {
		clip := clips[rng.Intn(len(clips))]
		if len(clip) == 0 {
			continue
		}
		offset := rng.Intn(len(clip))
		gain := 0.6 + 0.4*rng.Float64()
		for i := range out {
			out[i] += gain * float64(clip[(offset+i)%len(clip)])
		}
	}
	normalize(out)
	return out
}

// FromWAV loads a recorded noise file and loops it to n samples, with RMS 1.
func FromWAV(path string, n int) ([]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	samples, r, err := audio.ParseWAV(data)
	if err != nil {
		return nil, err
	}
	samples = audio.Resample(samples, r, rate)
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(samples[i%len(samples)])
	}
	normalize(out)
	return out, nil
}

// Mix adds noise (RMS 1) to speech so that the voice is snrDB louder than the noise.
// The voice level is measured only where someone is talking, not in pauses.
func Mix(speech []int16, noise []float64, snrDB float64) []int16 {
	scale := speechRMS(speech) / math.Pow(10, snrDB/20)
	out := make([]int16, len(speech))
	for i, s := range speech {
		v := float64(s) + scale*noise[i%len(noise)]
		out[i] = int16(math.Max(-32768, math.Min(32767, v)))
	}
	return out
}

// speechRMS is the RMS of the 20 ms frames that contain speech.
func speechRMS(s []int16) float64 {
	const frame = rate / 50
	var frames []float64
	loudest := 0.0
	for i := 0; i+frame <= len(s); i += frame {
		sum := 0.0
		for _, v := range s[i : i+frame] {
			sum += float64(v) * float64(v)
		}
		r := math.Sqrt(sum / frame)
		frames = append(frames, r)
		loudest = math.Max(loudest, r)
	}
	sum, count := 0.0, 0
	for _, r := range frames {
		if r > 0.1*loudest { // ignore silence between sentences
			sum += r * r
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return math.Sqrt(sum / float64(count))
}

func normalize(x []float64) {
	sum := 0.0
	for _, v := range x {
		sum += v * v
	}
	if rms := math.Sqrt(sum / float64(len(x))); rms > 0 {
		for i := range x {
			x[i] /= rms
		}
	}
}

func sign(v float64) float64 {
	if v >= 0 {
		return 1
	}
	return -1
}
