package ratelimit

import (
	"testing"
	"time"
)

func TestLimitsPerKeyAndResets(t *testing.T) {
	now := time.Now()
	l := New(2, time.Minute)
	l.now = func() time.Time { return now }
	if !l.Allow("a") || !l.Allow("a") {
		t.Fatal("first two should pass")
	}
	if l.Allow("a") {
		t.Error("third should be refused")
	}
	if !l.Allow("b") {
		t.Error("another key has its own limit")
	}
	now = now.Add(time.Minute)
	if !l.Allow("a") {
		t.Error("a new window should allow again")
	}
}
