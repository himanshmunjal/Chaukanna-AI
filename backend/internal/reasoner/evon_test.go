package reasoner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"chaukanna/internal/scam"
)

func TestExtractJSON(t *testing.T) {
	in := "<think>Let me reason...</think>\n```json\n{\"risk\": 0.9, \"flags\": []}\n```"
	got, err := extractJSON(in)
	if err != nil || got != `{"risk": 0.9, "flags": []}` {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := extractJSON("no json here"); err == nil {
		t.Fatal("expected an error")
	}
}

// TestAnalyzeGuardianAgainstFakeServer checks the full HTTP round trip with a
// fake OpenAI-compatible server (what vLLM exposes for Evon).
func TestAnalyzeGuardianAgainstFakeServer(t *testing.T) {
	var gotPath, gotSystem string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotSystem = req.Messages[0].Content
		answer := `{"risk":0.92,"stage":"extraction","language":"hi","flags":[{"type":"otp_request","t":13,"quote":"OTP bataiye"}],"reason":"Asks for OTP.","warning_text":"सावधान!"}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": answer}}},
		})
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "evon-test", "", 5*time.Second)
	a, err := client.AnalyzeGuardian(context.Background(), GuardianInput{
		Window:       []scam.Line{{T: 13, Text: "OTP bataiye"}},
		UserLanguage: "hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("wrong path %q", gotPath)
	}
	if !strings.Contains(gotSystem, "otp_request") || !strings.Contains(gotSystem, "Hindi") {
		t.Error("system prompt should list flag types and the user's language")
	}
	if a.Risk != 0.92 || len(a.Flags) != 1 || a.Flags[0].Type != scam.OTPRequest {
		t.Errorf("unexpected analysis: %+v", a)
	}
}

// fakeAnswer is a minimal valid model reply.
func fakeAnswer(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": `{"risk":0.5}`}}},
	})
}

// Only Evon's server understands chat_template_kwargs.
func TestThinkingSwitchOnlySentToEvon(t *testing.T) {
	for model, want := range map[string]bool{"evon": true, "openai/gpt-oss-120b": false} {
		var body map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&body)
			fakeAnswer(w)
		}))
		client := New(server.URL, model, "", 5*time.Second)
		if _, err := client.AnalyzeGuardian(context.Background(), GuardianInput{UserLanguage: "hi"}); err != nil {
			t.Fatal(err)
		}
		server.Close()
		if _, got := body["chat_template_kwargs"]; got != want {
			t.Errorf("%s: chat_template_kwargs sent = %v, want %v", model, got, want)
		}
	}
}

// A benchmark client waits out a 429 and reports the wait; a live client fails at once.
func TestRateLimit(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests%2 == 1 {
			w.Header().Set("Retry-After", "0.01")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fakeAnswer(w)
	}))
	defer server.Close()

	client := New(server.URL, "model", "", 5*time.Second)
	if _, err := client.AnalyzeGuardian(context.Background(), GuardianInput{UserLanguage: "hi"}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("a live client should fail on 429 with ErrRateLimited, got %v", err)
	}

	requests = 0
	client.WaitOnRateLimit(time.Second)
	var waited time.Duration
	ctx := WithWaitCounter(context.Background(), &waited)
	if _, err := client.AnalyzeGuardian(ctx, GuardianInput{UserLanguage: "hi"}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || waited != 10*time.Millisecond {
		t.Errorf("requests = %d, waited = %v", requests, waited)
	}
}
