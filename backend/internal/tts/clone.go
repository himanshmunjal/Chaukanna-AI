package tts

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"chaukanna/internal/gnanihttp"
	"chaukanna/internal/scam"
)

// Family voice: warnings spoken in a family member's cloned voice.
//
// Scammers work by isolating the victim, and a stranger's warning is easy to
// dismiss ("the officer said to ignore that"). A daughter's voice is not.
//
// Protocol (https://docs.gnani.ai/api/VC/voice-clone-embeddings):
//
//	POST /api/v1/tts/voice-clone/embeddings  multipart audio_file (5-30 s)
//	  -> {"data":{"voice_clone_embedding":{"embedding":"...","shape":[1,768],"dtype":"..."}}}
//	POST /api/v1/tts/inference  {text, model:"vachana-vc-v1", speaker_embedding, audio_config}

const cloneModel = "vachana-vc-v1"

func (c *Client) embeddingsURL() (string, error) {
	u, err := url.Parse(c.url)
	if err != nil {
		return "", err
	}
	u.Path = "/api/v1/tts/voice-clone/embeddings"
	return u.String(), nil
}

// CreateEmbedding turns 5-30 s of clean speech into a reusable voice embedding,
// returned as the JSON text to store and pass to SynthesizeCloned.
func (c *Client) CreateEmbedding(ctx context.Context, wav []byte) (string, error) {
	if !c.Enabled() {
		return "", ErrNotConfigured
	}
	endpoint, err := c.embeddingsURL()
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("audio_file", "voice.wav")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(wav); err != nil {
		return "", err
	}
	form.Close()

	resp, err := gnanihttp.Do(ctx, gnanihttp.Speech(c.apiKey), c.http, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body.Bytes()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.Header.Set("X-API-Key-ID", c.apiKey)
		return req, nil
	})
	if err != nil {
		return "", fmt.Errorf("voice clone request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("voice clone returned %d: %.300s", resp.StatusCode, raw)
	}
	return parseEmbedding(raw)
}

// parseEmbedding accepts the nested form from the API reference and the flat form
// from the quick start.
func parseEmbedding(raw []byte) (string, error) {
	var nested struct {
		Data struct {
			Embedding json.RawMessage `json:"voice_clone_embedding"`
		} `json:"data"`
	}
	candidate := raw
	if json.Unmarshal(raw, &nested) == nil && len(nested.Data.Embedding) > 0 {
		candidate = nested.Data.Embedding
	}
	var e struct {
		Embedding string `json:"embedding"`
		Shape     []int  `json:"shape"`
		DType     string `json:"dtype"`
	}
	if err := json.Unmarshal(candidate, &e); err != nil || e.Embedding == "" {
		return "", fmt.Errorf("voice clone: no embedding in response: %.200s", raw)
	}
	out, err := json.Marshal(e)
	return string(out), err
}

// SynthesizeCloned speaks text in a cloned voice. Results are cached like stock clips.
func (c *Client) SynthesizeCloned(ctx context.Context, text, language, embedding string) ([]int16, error) {
	if !c.Enabled() {
		return nil, ErrNotConfigured
	}
	if c.parent != nil {
		return nil, ErrNotPrepared
	}
	text = scam.Spoken(text, language)
	key := cloneKey(text, embedding)

	c.cacheMu.Lock()
	if samples, ok := c.cache[key]; ok {
		c.cacheMu.Unlock()
		return samples, nil
	}
	c.cacheMu.Unlock()

	samples, err := c.post(ctx, map[string]any{
		"text":              text,
		"model":             cloneModel,
		"speaker_embedding": json.RawMessage(embedding),
		"audio_config":      c.audioConfig(),
	})
	if err != nil {
		return nil, err
	}
	c.cacheMu.Lock()
	if len(c.cache) >= maxCachedClips {
		c.cache = map[string][]int16{}
	}
	c.cache[key] = samples
	c.cacheMu.Unlock()
	return samples, nil
}

// CachedClone returns a cloned clip only if it is already prepared: making one takes
// many seconds, and a warning cannot wait that long.
func (c *Client) CachedClone(text, language, embedding string) ([]int16, bool) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	samples, ok := c.cache[cloneKey(scam.Spoken(text, language), embedding)]
	return samples, ok
}

// PrewarmCloned prepares clips in a cloned voice at low priority.
func (c *Client) PrewarmCloned(ctx context.Context, texts []string, language, embedding string) (ok int) {
	ctx = gnanihttp.Background(ctx)
	for _, text := range texts {
		if _, err := c.SynthesizeCloned(ctx, text, language, embedding); err == nil {
			ok++
		}
	}
	return ok
}

func cloneKey(text, embedding string) string {
	sum := sha1.Sum([]byte(embedding))
	return "clone:" + hex.EncodeToString(sum[:8]) + "|" + text
}

// callerVoices are male Timbre v2.5 voices, used for the scammer in Scam Drill.
var callerVoices = map[string]string{"hi": "Deepak", "en": "Pranav"}

// SynthesizeCaller speaks in a "caller" voice. Stock voices differ per model, so
// any model other than v2.5 uses its default voice.
func (c *Client) SynthesizeCaller(ctx context.Context, text, language string) ([]int16, error) {
	voice := ""
	if strings.HasPrefix(c.model, "timbre-v2") {
		voice = callerVoices[language]
	}
	return c.SynthesizeWithVoice(ctx, text, language, voice)
}
