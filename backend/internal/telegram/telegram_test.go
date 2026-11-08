package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSendAndLink(t *testing.T) {
	sent := make(chan string, 4)
	served := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			if served {
				time.Sleep(20 * time.Millisecond)
				w.Write([]byte(`{"result":[]}`))
				return
			}
			served = true
			w.Write([]byte(`{"result":[{"update_id":7,"message":{"text":"/start abc123","chat":{"id":42}}},{"update_id":8,"message":{"text":"hello","chat":{"id":42}}}]}`))
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			var m struct {
				ChatID string `json:"chat_id"`
				Text   string `json:"text"`
			}
			json.NewDecoder(r.Body).Decode(&m)
			sent <- m.ChatID + ":" + m.Text
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c := NewWithBase(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Listen(ctx, func(chatID, code string) string {
		if chatID != "42" || code != "abc123" {
			t.Errorf("got chat %s code %s", chatID, code)
		}
		return "linked"
	})
	select {
	case got := <-sent:
		if got != "42:linked" {
			t.Errorf("reply = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no reply to /start")
	}
}

func TestDisabledWithoutToken(t *testing.T) {
	if New("").Enabled() {
		t.Error("should be disabled")
	}
}

func TestErrorsDoNotLeakToken(t *testing.T) {
	c := New("SECRET")
	c.base = "http://127.0.0.1:1/botSECRET"
	err := c.Send(context.Background(), "1", "x")
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error leaks token or is nil: %v", err)
	}
}

func TestSendRetriesAfterABlip(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if err := NewWithBase(srv.URL).Send(context.Background(), "1", "x"); err != nil || calls != 2 {
		t.Errorf("err %v after %d calls", err, calls)
	}
}
