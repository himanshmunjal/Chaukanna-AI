package stt

import (
	"strings"
	"testing"
)

func TestParseMessage(t *testing.T) {
	got, ok := parseMessage([]byte(`{"type":"transcript","text":" otp bataiye ","is_final":true,"segment_index":1}`))
	if !ok || got.Text != "otp bataiye" || !got.IsFinal {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	for _, raw := range []string{
		`{"type":"connected","config":{"sample_rate":8000}}`,
		`{"type":"speech_start"}`,
		`{"type":"processing"}`,
		`{"type":"transcript","text":""}`,
		`{"type":"error","message":"bad header"}`,
	} {
		if _, ok := parseMessage([]byte(raw)); ok {
			t.Errorf("%s should be ignored", raw)
		}
	}
}

func TestLanguageCodes(t *testing.T) {
	if got := languageCodes("prisma-v3.0", "hi"); got != "hi-IN" {
		t.Errorf("v3 Hindi: got %q", got)
	}
	if got := languageCodes("prisma-v2.5", "hi"); got != "hi-IN" {
		t.Errorf("v2.5 Hindi: got %q", got)
	}
	if got := languageCodes("prisma-v3.0", "en"); got != "en-IN" {
		t.Errorf("v3 English: got %q", got)
	}
}

func TestKeytermsAreSentForV3(t *testing.T) {
	h := New("wss://x", "key", "prisma-v3.0", 8000).headers("hi")
	if !strings.Contains(h.Get("keyterms"), "AnyDesk") {
		t.Fatal("keyterms header missing")
	}
	if len(scamKeyterms) > 200 {
		t.Fatal("Prisma accepts at most 200 keyterms")
	}
	for _, k := range scamKeyterms {
		for _, r := range k {
			if r > 127 {
				t.Fatalf("keyterm %q is not ASCII", k)
			}
		}
	}
	if New("wss://x", "key", "prisma-v2.5", 8000).headers("hi").Get("keyterms") != "" {
		t.Fatal("keyterms are a v3.0 feature")
	}
}
