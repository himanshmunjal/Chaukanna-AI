package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"chaukanna/internal/audio"
	"chaukanna/internal/gnanihttp"
	"chaukanna/internal/scam"
)

// maxRESTSeconds: the REST endpoint takes clips of up to 30 s, so longer WAVs are cut.
const maxRESTSeconds = 25

// restURL derives the REST endpoint from the streaming URL:
// wss://api.vachana.ai/stt/v3/stream -> https://api.vachana.ai/stt/v3
func (c *Client) restURL() (string, error) {
	u, err := url.Parse(c.url)
	if err != nil {
		return "", err
	}
	u.Scheme = strings.Replace(strings.Replace(u.Scheme, "wss", "https", 1), "ws", "http", 1)
	u.Path = strings.TrimSuffix(u.Path, "/stream")
	return u.String(), nil
}

// TranscribeFile transcribes a recording. WAV files of any length are split into
// short clips. Other formats (MP3, OGG, M4A, AAC, FLAC) are sent as they are, so
// Prisma's 30-second limit applies to them.
func (c *Client) TranscribeFile(ctx context.Context, filename string, data []byte, language string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("prisma is not configured")
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) {
		return c.transcribeClip(ctx, filename, data, language)
	}
	samples, rate, err := audio.ParseWAV(data)
	if err != nil {
		return "", fmt.Errorf("could not read the WAV file: %w", err)
	}
	var parts []string
	step := rate * maxRESTSeconds
	for start := 0; start < len(samples); start += step {
		clip := samples[start:min(start+step, len(samples))]
		text, err := c.transcribeClip(ctx, "clip.wav", audio.EncodeWAV(clip, rate), language)
		if err != nil {
			return "", err
		}
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " "), nil
}

func (c *Client) transcribeClip(ctx context.Context, filename string, data []byte, language string) (string, error) {
	endpoint, err := c.restURL()
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("audio_file", path.Base(filename))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	_ = form.WriteField("language_code", scam.Locale(language))
	if c.model != "" {
		_ = form.WriteField("model", c.model)
	}
	form.Close()

	resp, err := gnanihttp.Do(ctx, gnanihttp.Listen(c.apiKey), &http.Client{Timeout: time.Minute}, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body.Bytes()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.Header.Set("X-API-Key-ID", c.apiKey)
		return req, nil
	})
	if err != nil {
		return "", fmt.Errorf("prisma request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("prisma returned %d: %.300s", resp.StatusCode, raw)
	}
	var out struct {
		Transcript string `json:"transcript"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("prisma: unexpected response: %.300s", raw)
	}
	return strings.TrimSpace(out.Transcript), nil
}
