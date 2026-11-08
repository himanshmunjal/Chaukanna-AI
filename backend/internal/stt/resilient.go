package stt

import (
	"context"
	"log"
	"sync"
	"time"
)

// Resilient is a Prisma stream that survives losing its connection. A Prisma session
// lasts at most 15 minutes, and a digital-arrest call can run for hours, so without this
// protection would quietly stop in the middle of the most dangerous calls. It reconnects
// straight away, and only the audio during the gap is lost.
type Resilient struct {
	client   *Client
	language string
	ctx      context.Context
	cancel   context.CancelFunc
	results  chan Transcript

	mu      sync.Mutex
	current *Stream // nil while reconnecting
}

// OpenResilient connects, and returns an error only if the first connection fails.
func (c *Client) OpenResilient(ctx context.Context, language string) (*Resilient, error) {
	first, err := c.Open(ctx, language)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	r := &Resilient{client: c, language: language, ctx: ctx, cancel: cancel, results: make(chan Transcript, 64), current: first}
	go r.pump(first)
	return r, nil
}

// Results delivers transcripts across reconnections. It closes after Close.
func (r *Resilient) Results() <-chan Transcript { return r.results }

// Send streams audio. While reconnecting, audio is dropped rather than queued, because
// transcribing stale speech later would only delay the warning.
func (r *Resilient) Send(samples []int16) error {
	r.mu.Lock()
	s := r.current
	r.mu.Unlock()
	if s == nil {
		return nil
	}
	if err := s.Send(samples); err != nil {
		log.Printf("prisma: send failed, will reconnect: %v", err)
		_ = s.conn.Close() // ends the read loop, which reconnects
	}
	return nil
}

// Close flushes the last words and ends the stream.
func (r *Resilient) Close() {
	r.cancel()
	r.mu.Lock()
	s := r.current
	r.mu.Unlock()
	if s != nil {
		s.Close()
	}
}

func (r *Resilient) pump(s *Stream) {
	defer close(r.results)
	for {
		for t := range s.Results() {
			r.results <- t
		}
		if r.ctx.Err() != nil {
			return
		}
		r.mu.Lock()
		r.current = nil
		r.mu.Unlock()

		if s = r.reconnect(); s == nil {
			return
		}
		r.mu.Lock()
		r.current = s
		r.mu.Unlock()
		if r.ctx.Err() != nil { // closed while reconnecting
			s.Close()
			return
		}
	}
}

// reconnect keeps trying until it works or the call ends.
func (r *Resilient) reconnect() *Stream {
	delay := reconnectFirstDelay
	for {
		s, err := r.client.Open(r.ctx, r.language)
		if err == nil {
			log.Printf("prisma: reconnected, the call stays protected")
			return s
		}
		if r.ctx.Err() != nil {
			return nil
		}
		log.Printf("prisma: reconnect failed, retrying in %s: %v", delay, err)
		select {
		case <-time.After(delay):
		case <-r.ctx.Done():
			return nil
		}
		delay = min(delay*2, 10*time.Second)
	}
}

// reconnectFirstDelay is the wait after the first failed reconnection.
var reconnectFirstDelay = 500 * time.Millisecond
