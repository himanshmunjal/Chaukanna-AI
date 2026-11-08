// Package tts turns text into speech with Gnani Timbre and returns phone-ready
// audio (8 kHz PCM samples).
//
// Protocol (https://docs.gnani.ai/api/TTS/tts-inference):
//
//	POST https://api.vachana.ai/api/v1/tts/inference
//	X-API-Key-ID: <key>
//	{ "text": "...", "voice": "Nalini", "model": "timbre-v2.5", "language": "hi-IN",
//	  "speed": 1.0, "audio_config": { "sample_rate": 16000, "num_channels": 1,
//	  "sample_width": 2, "encoding": "linear_pcm", "container": "wav" } }
//	-> 200 OK, body = WAV file
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"chaukanna/internal/audio"
	"chaukanna/internal/gnanihttp"
	"chaukanna/internal/scam"
)

// ErrNotConfigured is returned when TIMBRE_URL or GNANI_API_KEY is missing.
var ErrNotConfigured = errors.New("timbre is not configured")

// defaultVoices: Timbre v2.5 voices each speak one preferred language.
// (Full list: https://docs.gnani.ai/api/TTS/available-voices)
var defaultVoices = map[string]string{
	"hi": "Nalini", "en": "Kaveri", "ta": "Asmita", "te": "Suhana", "kn": "Saanvi",
	"ml": "Reshma", "mr": "Zahira", "bn": "Kirra", "gu": "Falak", "pa": "Mehuli",
}

// Client calls Timbre.
type Client struct {
	url        string
	apiKey     string
	model      string
	voice      string // fixed voice; empty = pick per language
	sampleRate int    // rate we ask Timbre for (we resample to 8 kHz afterwards)
	http       *http.Client

	// cache remembers every clip we synthesised (key: voice|language|text), so
	// repeated phrases such as the standard warning play instantly.
	cacheMu sync.Mutex
	cache   map[string][]int16

	// parent makes this a read-only view of another client's prepared clips.
	parent *Client
}

// ErrNotPrepared means a read-only client was asked for a clip that isn't ready.
var ErrNotPrepared = errors.New("this clip is not prepared, and no key was given to make it")

// WithKey returns a client that uses someone else's API key, with its own small cache.
func (c *Client) WithKey(key string) *Client {
	return New(c.url, key, c.model, c.voice, c.sampleRate)
}

// CacheOnly returns a client that plays only what is already prepared. A public demo
// visitor without a key can hear the warnings without spending this server's credits.
func (c *Client) CacheOnly() *Client {
	return &Client{parent: c, model: c.model, voice: c.voice}
}

// maxCachedClips bounds memory use (a 6 s clip is about 100 KB).
const maxCachedClips = 300

// New creates a Timbre client.
func New(url, apiKey, model, voice string, sampleRate int) *Client {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	return &Client{url: url, apiKey: apiKey, model: model, voice: voice, sampleRate: sampleRate,
		http: &http.Client{Timeout: 60 * time.Second}, cache: map[string][]int16{}}
}

// Enabled reports whether Timbre is configured.
func (c *Client) Enabled() bool {
	if c != nil && c.parent != nil {
		return c.parent.Enabled()
	}
	return c != nil && c.url != "" && c.apiKey != ""
}

// Synthesize converts text to 8 kHz PCM samples ready to play into a phone call.
// Results are cached, so a phrase is only sent to Timbre once.
func (c *Client) Synthesize(ctx context.Context, text, language string) ([]int16, error) {
	return c.SynthesizeWithVoice(ctx, text, language, "")
}

// SynthesizeWithVoice is Synthesize with a specific voice ("" = default for the language).
func (c *Client) SynthesizeWithVoice(ctx context.Context, text, language, voice string) ([]int16, error) {
	if !c.Enabled() {
		return nil, ErrNotConfigured
	}
	if voice == "" {
		voice = c.voiceFor(language)
	}
	text = scam.Spoken(text, language)
	key := voice + "|" + scam.Locale(language) + "|" + text

	if c.parent != nil {
		c.parent.cacheMu.Lock()
		defer c.parent.cacheMu.Unlock()
		if samples, ok := c.parent.cache[key]; ok {
			return samples, nil
		}
		return nil, ErrNotPrepared
	}

	c.cacheMu.Lock()
	if samples, ok := c.cache[key]; ok {
		c.cacheMu.Unlock()
		return samples, nil
	}
	c.cacheMu.Unlock()

	samples, err := c.request(ctx, text, language, voice)
	if err != nil {
		return nil, err
	}

	c.cacheMu.Lock()
	if len(c.cache) >= maxCachedClips {
		c.cache = map[string][]int16{} // simple reset; the important clips are re-warmed on use
	}
	c.cache[key] = samples
	c.cacheMu.Unlock()
	return samples, nil
}

// Prewarm synthesises phrases ahead of time (e.g. warnings at startup) so the
// first real use is instant. Each item is {text, language}.
//
// It runs at low priority, so a live warning never waits behind it.
func (c *Client) Prewarm(ctx context.Context, items [][2]string) (ok int) {
	ctx = gnanihttp.Background(ctx)
	for _, it := range items {
		if _, err := c.Synthesize(ctx, it[0], it[1]); err == nil {
			ok++
		}
	}
	return ok
}

// request makes the actual HTTP call to Timbre.
func (c *Client) request(ctx context.Context, text, language, voice string) ([]int16, error) {
	return c.post(ctx, map[string]any{
		"text":         text,
		"voice":        voice,
		"model":        c.model,
		"language":     scam.Locale(language),
		"speed":        1.0,
		"audio_config": c.audioConfig(),
	})
}

func (c *Client) audioConfig() map[string]any {
	return map[string]any{
		"sample_rate":  c.sampleRate,
		"num_channels": 1,
		"sample_width": 2,
		"encoding":     "linear_pcm",
		"container":    "wav",
	}
}

func (c *Client) post(ctx context.Context, payload map[string]any) ([]int16, error) {
	body, _ := json.Marshal(payload)
	resp, err := gnanihttp.Do(ctx, gnanihttp.Speech(c.apiKey), c.http, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key-ID", c.apiKey)
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("timbre request: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("timbre returned %d: %.300s", resp.StatusCode, data)
	}

	// The response is a WAV file; anything else is treated as raw PCM at the requested rate.
	if bytes.HasPrefix(data, []byte("RIFF")) {
		samples, rate, err := audio.ParseWAV(data)
		if err != nil {
			return nil, err
		}
		return audio.Resample(samples, rate, audio.PhoneRate), nil
	}
	return audio.Resample(audio.BytesToSamples(data), c.sampleRate, audio.PhoneRate), nil
}

// voiceFor returns the configured voice, or a default voice for the language.
func (c *Client) voiceFor(language string) string {
	if c.voice != "" {
		return c.voice
	}
	if v, ok := defaultVoices[scam.BaseLanguage(language)]; ok {
		return v
	}
	return defaultVoices["hi"]
}
