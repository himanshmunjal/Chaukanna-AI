package gnanihttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRetriesWhileBusyAndResendsTheBody(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if body, _ := io.ReadAll(r.Body); string(body) != "payload" {
			t.Errorf("attempt %d: body = %q", calls, body)
		}
		if calls < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	build := func() (*http.Request, error) {
		return http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("payload"))
	}
	resp, err := Do(context.Background(), NewGate(), srv.Client(), build)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || calls != 3 {
		t.Errorf("status %d after %d calls", resp.StatusCode, calls)
	}
}

func TestStopsWhenTheCallEnds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	build := func() (*http.Request, error) { return http.NewRequest(http.MethodGet, srv.URL, nil) }
	if _, err := Do(ctx, NewGate(), srv.Client(), build); err == nil {
		t.Error("expected the context error")
	}
}

func TestOtherErrorsAreNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	build := func() (*http.Request, error) { return http.NewRequest(http.MethodGet, srv.URL, nil) }
	resp, err := Do(context.Background(), NewGate(), srv.Client(), build)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if calls != 1 {
		t.Errorf("401 was retried: %d calls", calls)
	}
}

func TestOneAtATimeAndLiveRequestsGoFirst(t *testing.T) {
	var mu sync.Mutex
	active, peak := 0, 0
	var order []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		active++
		peak = max(peak, active)
		order = append(order, r.URL.Query().Get("who"))
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
	}))
	defer srv.Close()

	g := NewGate()
	send := func(ctx context.Context, who string, wg *sync.WaitGroup) {
		defer wg.Done()
		resp, err := Do(ctx, g, srv.Client(), func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, srv.URL+"?who="+who, nil)
		})
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
	}

	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 4; i++ {
		go send(Background(context.Background()), "bg", &wg)
	}
	time.Sleep(10 * time.Millisecond) // the background queue has formed
	go send(context.Background(), "live", &wg)
	wg.Wait()

	if peak != 1 {
		t.Errorf("%d requests ran at once", peak)
	}
	if pos := slices.Index(order, "live"); pos > 1 {
		t.Errorf("the live request ran at position %d: %v", pos, order)
	}
}
