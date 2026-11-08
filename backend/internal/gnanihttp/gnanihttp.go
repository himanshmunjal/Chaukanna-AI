// Package gnanihttp sends REST requests to Gnani's speech APIs.
//
// An API key may have only one REST request active at a time; a second one gets
// 429 RATE_LIMITED. That must never cost a warning: if a scam is detected while
// another clip is being made, the warning waits a moment and goes through.
package gnanihttp

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Gate queues requests to one Gnani service, so this process never competes with
// itself. Live requests always go ahead of background ones (see Background): a
// warning must not wait behind a queue of clips being prepared in advance.
type Gate struct {
	slot    chan struct{}
	waiting atomic.Int32 // live requests waiting for the slot
}

// NewGate creates a gate.
func NewGate() *Gate { return &Gate{slot: make(chan struct{}, 1)} }

// The limit is per API key and appeared per service in testing: Prisma answered while
// Timbre returned 429. Visitors using their own key must not queue behind each other.
var (
	gatesMu sync.Mutex
	gates   = map[[32]byte]*Gate{}
)

func gateFor(service, key string) *Gate {
	id := sha256.Sum256([]byte(service + "|" + key)) // never keep a key itself in memory longer than needed
	gatesMu.Lock()
	defer gatesMu.Unlock()
	g, ok := gates[id]
	if !ok {
		g = NewGate()
		gates[id] = g
	}
	return g
}

// Speech is the gate for Timbre and voice cloning on this key.
func Speech(key string) *Gate { return gateFor("speech", key) }

// Listen is the gate for Prisma REST on this key.
func Listen(key string) *Gate { return gateFor("listen", key) }

type backgroundKey struct{}

// Background marks a context as low priority: prepared in advance, nobody waiting.
func Background(ctx context.Context) context.Context {
	return context.WithValue(ctx, backgroundKey{}, true)
}

func (g *Gate) acquire(ctx context.Context) error {
	if ctx.Value(backgroundKey{}) == nil {
		g.waiting.Add(1)
		defer g.waiting.Add(-1)
		select {
		case g.slot <- struct{}{}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for {
		select {
		case g.slot <- struct{}{}:
			if g.waiting.Load() == 0 {
				return nil
			}
			<-g.slot // someone live is waiting: let them go first
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (g *Gate) release() { <-g.slot }

// heldBody keeps the slot until the response has been read: the service counts a
// request as active until then.
type heldBody struct {
	io.ReadCloser
	once sync.Once
	g    *Gate
}

func (b *heldBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.g.release)
	return err
}

// retryDelays are the waits after each 429. The total is short enough to still warn
// during the call, and longer than a typical clip takes to generate.
var retryDelays = []time.Duration{
	200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond,
	1500 * time.Millisecond, 2500 * time.Millisecond, 4 * time.Second,
}

// Do sends the request built by build, one at a time through the gate, retrying while
// the key is busy elsewhere (another process may share it). build is called again for
// each attempt because a request body can only be read once. The caller must close
// the response body.
func Do(ctx context.Context, g *Gate, client *http.Client, build func() (*http.Request, error)) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if err := g.acquire(ctx); err != nil {
			return nil, err
		}
		req, err := build()
		if err != nil {
			g.release()
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			g.release()
			return nil, err
		}
		resp.Body = &heldBody{ReadCloser: resp.Body, g: g}
		if resp.StatusCode != http.StatusTooManyRequests || attempt >= len(retryDelays) {
			return resp, nil
		}
		resp.Body.Close()
		select {
		case <-time.After(retryDelays[attempt]):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
