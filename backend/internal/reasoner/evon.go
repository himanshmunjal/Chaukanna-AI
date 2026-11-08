// Package reasoner talks to the reasoning model: Gnani's Evon, or any other LLM
// behind an OpenAI-compatible API (set by EVON_BASE_URL and EVON_MODEL).
//
// Evon is published on Hugging Face. We serve it with vLLM, which exposes an
// OpenAI-compatible API (POST {EVON_BASE_URL}/chat/completions), so this client
// is a small, dependency-free HTTP client for that API.
//
// Two tasks:
//   - AnalyzeGuardian: "is this live call a scam, and why?"
//   - Screen:          "what should the screening bot say next to this caller?"
//
// Both ask Evon to reply with strict JSON (see prompts/*.md).
package reasoner

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chaukanna/internal/scam"
)

//go:embed prompts/guardian.md
var guardianPrompt string

//go:embed prompts/screen.md
var screenPrompt string

// maxTokens limits Evon's answer length. JSON answers are short; this also
// stops a "thinking" model from rambling and adding latency.
const maxTokens = 800

// defaultRateLimitWait is used when a 429 response has no Retry-After header.
const defaultRateLimitWait = 10 * time.Second

// ErrRateLimited means the provider kept refusing requests with 429 for longer
// than the client may wait, for example because a daily limit ran out.
var ErrRateLimited = errors.New("llm rate limit")

// Client calls Evon, or any other model behind an OpenAI-compatible API.
type Client struct {
	baseURL string
	model   string
	apiKey  string
	http    *http.Client

	// maxRateLimitWait is how long one request may wait out 429 responses.
	// Zero means fail immediately, which is right for a live call: the session
	// falls back to rules instead of holding up a warning.
	maxRateLimitWait time.Duration
}

// WaitOnRateLimit makes the client wait out 429 responses, for up to max per
// request. Benchmarks use it so a rate limit is not scored as a rules verdict.
func (c *Client) WaitOnRateLimit(max time.Duration) {
	if c != nil {
		c.maxRateLimitWait = max
	}
}

// waitCounterKey is the context key for WithWaitCounter.
type waitCounterKey struct{}

// WithWaitCounter returns a context that adds any rate-limit waiting to counter,
// so callers can keep that time out of their latency measurements.
func WithWaitCounter(ctx context.Context, counter *time.Duration) context.Context {
	return context.WithValue(ctx, waitCounterKey{}, counter)
}

// New creates an Evon client. If baseURL or model is empty the client is
// disabled and the session falls back to rule-based scoring.
func New(baseURL, model, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: timeout},
	}
}

// Enabled reports whether Evon is configured.
func (c *Client) Enabled() bool { return c != nil && c.baseURL != "" && c.model != "" }

// ---------------------------------------------------------------------------
// Guardian mode
// ---------------------------------------------------------------------------

// GuardianInput is what Evon sees during a live call.
type GuardianInput struct {
	Window       []scam.Line // recent transcript (last ~60 s)
	KnownFlags   []scam.Flag // flags already detected earlier in the call
	UserLanguage string      // language for the spoken warning
	ElapsedSec   float64
}

// AnalyzeGuardian asks Evon whether the call is a scam.
func (c *Client) AnalyzeGuardian(ctx context.Context, in GuardianInput) (scam.Analysis, error) {
	system := fillPrompt(guardianPrompt, map[string]string{
		"{{USER_LANGUAGE}}": scam.LanguageName(in.UserLanguage),
	})

	var b strings.Builder
	fmt.Fprintf(&b, "Call time so far: %.0f seconds.\n", in.ElapsedSec)
	if len(in.KnownFlags) > 0 {
		b.WriteString("Red flags detected earlier in this call:\n")
		for _, f := range in.KnownFlags {
			fmt.Fprintf(&b, "- [%.1fs] %s: %q\n", f.T, f.Type, f.Quote)
		}
	}
	b.WriteString("\nRecent transcript:\n")
	writeLines(&b, in.Window)

	var out scam.Analysis
	err := c.chat(ctx, system, b.String(), &out)
	return out, err
}

// ---------------------------------------------------------------------------
// Screen mode
// ---------------------------------------------------------------------------

// ScreenInput is what Evon sees while screening an unknown caller.
type ScreenInput struct {
	Transcript   []scam.Line // the whole conversation so far, with speakers
	UserName     string
	UserLanguage string
	Turn         int
	MaxTurns     int
}

// ScreenResult is Evon's next move as the screening assistant.
type ScreenResult struct {
	scam.Analysis
	Reply      string `json:"reply"`
	Done       bool   `json:"done"`
	CallerName string `json:"caller_name"`
	Purpose    string `json:"purpose"`
	Summary    string `json:"summary"`
}

// Screen asks Evon what to say next to an unknown caller.
func (c *Client) Screen(ctx context.Context, in ScreenInput) (ScreenResult, error) {
	system := fillPrompt(screenPrompt, map[string]string{
		"{{USER_NAME}}":     in.UserName,
		"{{USER_LANGUAGE}}": scam.LanguageName(in.UserLanguage),
		"{{MAX_TURNS}}":     fmt.Sprint(in.MaxTurns),
	})

	var b strings.Builder
	fmt.Fprintf(&b, "This is turn %d of at most %d.\n\nConversation so far:\n", in.Turn, in.MaxTurns)
	writeLines(&b, in.Transcript)

	var out ScreenResult
	err := c.chat(ctx, system, b.String(), &out)
	return out, err
}

// ---------------------------------------------------------------------------
// HTTP plumbing
// ---------------------------------------------------------------------------

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
	// vLLM/SGLang extension. Evon "thinks" before answering by default; on a
	// live call we need the verdict in ~1 s, so thinking is turned off.
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// chat sends one request and decodes the JSON object in the answer into out.
func (c *Client) chat(ctx context.Context, system, user string, out any) error {
	if !c.Enabled() {
		return errors.New("reasoning model is not configured")
	}

	request := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature:    0.1, // we want consistent verdicts, not creativity
		MaxTokens:      maxTokens,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	if strings.Contains(strings.ToLower(c.model), "evon") {
		// Only Evon's vLLM server understands this; other providers may reject it.
		request.ChatTemplateKwargs = map[string]any{"enable_thinking": false}
	}
	body, _ := json.Marshal(request)

	raw, err := c.post(ctx, body)
	if err != nil {
		return err
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil || len(cr.Choices) == 0 {
		return fmt.Errorf("llm: unexpected response: %s", truncate(string(raw), 300))
	}

	jsonText, err := extractJSON(cr.Choices[0].Message.Content)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(jsonText), out); err != nil {
		return fmt.Errorf("llm: invalid JSON: %w: %s", err, truncate(jsonText, 300))
	}
	return nil
}

// post sends one chat request, waiting out rate limits if the client allows it.
func (c *Client) post(ctx context.Context, body []byte) ([]byte, error) {
	var waited time.Duration
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("llm request: %w", err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return raw, nil
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return nil, fmt.Errorf("llm returned %d: %s", resp.StatusCode, truncate(string(raw), 300))
		}
		wait := retryAfter(resp.Header.Get("Retry-After"))
		if waited+wait > c.maxRateLimitWait {
			return nil, fmt.Errorf("%w (retry after %v): %s", ErrRateLimited, wait, truncate(string(raw), 300))
		}

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		waited += wait
		if counter, ok := ctx.Value(waitCounterKey{}).(*time.Duration); ok {
			*counter += wait
		}
	}
}

// retryAfter reads a Retry-After header given in seconds.
func retryAfter(header string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(header), 64)
	if err != nil || seconds <= 0 {
		return defaultRateLimitWait
	}
	return time.Duration(seconds * float64(time.Second))
}

var thinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)

// extractJSON pulls the JSON object out of a model answer. Reasoning models
// sometimes wrap it in <think>...</think> or ```json fences.
func extractJSON(content string) (string, error) {
	content = thinkBlock.ReplaceAllString(content, "")
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return "", fmt.Errorf("llm: no JSON object in answer: %s", truncate(content, 300))
	}
	return content[start : end+1], nil
}

// fillPrompt replaces {{PLACEHOLDERS}} in a prompt template.
func fillPrompt(tmpl string, values map[string]string) string {
	var flags strings.Builder
	for _, t := range scam.AllFlags {
		fmt.Fprintf(&flags, "- %s: %s\n", t, scam.Descriptions[t])
	}
	values["{{FLAGS}}"] = strings.TrimSpace(flags.String())
	for k, v := range values {
		tmpl = strings.ReplaceAll(tmpl, k, v)
	}
	return tmpl
}

func writeLines(b *strings.Builder, lines []scam.Line) {
	if len(lines) == 0 {
		b.WriteString("(nothing yet)\n")
	}
	for _, l := range lines {
		if l.Speaker != "" && l.Speaker != "call" {
			fmt.Fprintf(b, "[%.1fs] %s: %s\n", l.T, l.Speaker, l.Text)
		} else {
			fmt.Fprintf(b, "[%.1fs] %s\n", l.T, l.Text)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
