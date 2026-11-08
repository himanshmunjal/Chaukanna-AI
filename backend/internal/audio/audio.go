// Package audio converts between the audio formats used in this project.
//
// Inside Chaukanna, audio is always 16-bit signed PCM, mono, 8000 Hz,
// stored as []int16 ("samples"). Conversions happen at the edges:
//
//	Twilio  <-> μ-law 8 kHz bytes (MulawDecode / MulawEncode)
//	Exotel  <-> 16-bit little-endian PCM 8 kHz bytes (BytesToSamples / SamplesToBytes)
//	Prisma   -> PCM at PRISMA_SAMPLE_RATE (Resample)
//	Timbre   -> WAV or PCM at any rate (ParseWAV + Resample)
package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// PhoneRate is the sample rate of telephone audio.
const PhoneRate = 8000

// ---------------------------------------------------------------------------
// μ-law (G.711), used by Twilio Media Streams
// ---------------------------------------------------------------------------

const (
	mulawBias = 0x84
	mulawClip = 32635
)

// MulawDecode converts μ-law bytes to PCM samples.
func MulawDecode(data []byte) []int16 {
	out := make([]int16, len(data))
	for i, b := range data {
		u := ^b
		sign := u & 0x80
		exponent := (u >> 4) & 0x07
		mantissa := u & 0x0F
		sample := ((int(mantissa) << 3) + mulawBias) << exponent
		sample -= mulawBias
		if sign != 0 {
			sample = -sample
		}
		out[i] = int16(sample)
	}
	return out
}

// MulawEncode converts PCM samples to μ-law bytes.
func MulawEncode(samples []int16) []byte {
	out := make([]byte, len(samples))
	for i, s := range samples {
		sample := int(s)
		sign := 0
		if sample < 0 {
			sample = -sample
			sign = 0x80
		}
		if sample > mulawClip {
			sample = mulawClip
		}
		sample += mulawBias
		exponent := 7
		for mask := 0x4000; sample&mask == 0 && exponent > 0; mask >>= 1 {
			exponent--
		}
		mantissa := (sample >> (exponent + 3)) & 0x0F
		out[i] = ^byte(sign | exponent<<4 | mantissa)
	}
	return out
}

// ---------------------------------------------------------------------------
// Raw 16-bit little-endian PCM, used by Exotel and Prisma
// ---------------------------------------------------------------------------

// BytesToSamples converts little-endian 16-bit PCM bytes to samples.
func BytesToSamples(data []byte) []int16 {
	out := make([]int16, len(data)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return out
}

// SamplesToBytes converts samples to little-endian 16-bit PCM bytes.
func SamplesToBytes(samples []int16) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(s))
	}
	return out
}

// ---------------------------------------------------------------------------
// Resampling
// ---------------------------------------------------------------------------

// Resample changes the sample rate. When going down (e.g. 16 kHz -> 8 kHz) it
// averages neighbouring samples, a simple low-pass filter that avoids most
// aliasing. When going up it interpolates linearly. Good enough for speech.
func Resample(in []int16, from, to int) []int16 {
	if from == to || from <= 0 || to <= 0 || len(in) == 0 {
		return in
	}
	outLen := int(int64(len(in)) * int64(to) / int64(from))
	out := make([]int16, outLen)
	ratio := float64(from) / float64(to)

	for i := range out {
		pos := float64(i) * ratio
		if from > to {
			// Downsample: average every input sample that falls into this output slot.
			start, end := int(pos), int(pos+ratio)
			if end > len(in) {
				end = len(in)
			}
			if end <= start {
				end = start + 1
			}
			sum := 0
			for j := start; j < end && j < len(in); j++ {
				sum += int(in[j])
			}
			out[i] = int16(sum / (end - start))
		} else {
			// Upsample: linear interpolation between two neighbours.
			j := int(pos)
			frac := pos - float64(j)
			a, b := in[j], in[j]
			if j+1 < len(in) {
				b = in[j+1]
			}
			out[i] = int16(float64(a)*(1-frac) + float64(b)*frac)
		}
	}
	return out
}

// Duration returns how long the samples play at the given rate, in seconds.
func Duration(samples []int16, rate int) float64 {
	return float64(len(samples)) / float64(rate)
}

// ---------------------------------------------------------------------------
// WAV files
// ---------------------------------------------------------------------------

// ParseWAV reads a 16-bit PCM WAV file. Stereo files are mixed down to mono.
func ParseWAV(data []byte) (samples []int16, rate int, err error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, errors.New("not a WAV file")
	}

	var channels, bits int
	pos := 12
	for pos+8 <= len(data) {
		id := string(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		body := pos + 8
		if size < 0 || body+size > len(data) {
			size = len(data) - body // streamed WAVs may have a bogus size
		}

		switch id {
		case "fmt ":
			if size < 16 {
				return nil, 0, errors.New("bad WAV fmt chunk")
			}
			format := binary.LittleEndian.Uint16(data[body:])
			channels = int(binary.LittleEndian.Uint16(data[body+2:]))
			rate = int(binary.LittleEndian.Uint32(data[body+4:]))
			bits = int(binary.LittleEndian.Uint16(data[body+14:]))
			if format != 1 && format != 0xFFFE {
				return nil, 0, errors.New("only PCM WAV is supported")
			}
		case "data":
			if bits != 16 || channels == 0 {
				return nil, 0, errors.New("only 16-bit WAV is supported")
			}
			all := BytesToSamples(data[body : body+size])
			if channels == 1 {
				return all, rate, nil
			}
			mono := make([]int16, len(all)/channels)
			for i := range mono {
				sum := 0
				for c := 0; c < channels; c++ {
					sum += int(all[i*channels+c])
				}
				mono[i] = int16(sum / channels)
			}
			return mono, rate, nil
		}
		pos = body + size + size%2 // chunks are padded to even sizes
	}
	return nil, 0, errors.New("WAV has no data chunk")
}

// EncodeWAV writes samples as a mono 16-bit PCM WAV file.
func EncodeWAV(samples []int16, rate int) []byte {
	pcm := SamplesToBytes(samples)
	var buf bytes.Buffer
	w := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }

	buf.WriteString("RIFF")
	w(uint32(36 + len(pcm)))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	w(uint32(16))       // fmt chunk size
	w(uint16(1))        // PCM
	w(uint16(1))        // mono
	w(uint32(rate))     // sample rate
	w(uint32(rate * 2)) // byte rate
	w(uint16(2))        // block align
	w(uint16(16))       // bits per sample
	buf.WriteString("data")
	w(uint32(len(pcm)))
	buf.Write(pcm)
	return buf.Bytes()
}
