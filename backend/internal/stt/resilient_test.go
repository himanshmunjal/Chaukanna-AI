package stt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fakePrisma sends one transcript per connection, then drops it (as Prisma does when a
// session hits its limit), except the connection number keepOpen, which it holds open.
func fakePrisma(t *testing.T, keepOpen int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var conns atomic.Int32
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		n := conns.Add(1)
		c.WriteJSON(map[string]any{"type": "transcript", "text": "line " + string(rune('0'+n)), "is_final": true})
		if n == keepOpen {
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &conns
}

func TestReconnectsWhenPrismaEndsTheSession(t *testing.T) {
	srv, conns := fakePrisma(t, 3)
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := New(url, "key", "prisma-v3.0", 8000)
	r, err := c.OpenResilient(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	timeout := time.After(5 * time.Second)
	for len(got) < 3 {
		select {
		case tr := <-r.Results():
			got = append(got, tr.Text)
		case <-timeout:
			t.Fatalf("got only %v after %d connections", got, conns.Load())
		}
	}
	if err := r.Send(make([]int16, 800)); err != nil {
		t.Errorf("send after reconnecting: %v", err)
	}
	r.Close()
	select {
	case _, open := <-r.Results():
		if open {
			for range r.Results() {
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("results should close after Close")
	}
}

func TestLastWordsArriveAfterClose(t *testing.T) {
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			var m map[string]string
			if err := c.ReadJSON(&m); err != nil {
				return
			}
			if m["type"] == "commit" {
				time.Sleep(100 * time.Millisecond) // Prisma finishing the last sentence
				c.WriteJSON(map[string]any{"type": "transcript", "text": "last words", "is_final": true})
				return
			}
		}
	}))
	defer srv.Close()
	s, err := New("ws"+strings.TrimPrefix(srv.URL, "http"), "key", "prisma-v3.0", 8000).Open(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	var got string
	for tr := range s.Results() {
		got = tr.Text
	}
	if got != "last words" {
		t.Errorf("got %q", got)
	}
}
