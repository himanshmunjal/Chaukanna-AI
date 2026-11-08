// Package ratelimit limits how often one key (a visitor or an IP address) can do
// something. It keeps a count per fixed time window, in memory.
package ratelimit

import (
	"sync"
	"time"
)

// maxKeys bounds memory: past this many keys, finished windows are cleared out.
const maxKeys = 50000

type window struct {
	started time.Time
	count   int
}

// Limiter allows max events per key in each period.
type Limiter struct {
	mu     sync.Mutex
	max    int
	period time.Duration
	keys   map[string]*window
	now    func() time.Time
}

// New creates a Limiter.
func New(max int, period time.Duration) *Limiter {
	return &Limiter{max: max, period: period, keys: map[string]*window{}, now: time.Now}
}

// Allow records one event and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w, ok := l.keys[key]
	if !ok || now.Sub(w.started) >= l.period {
		if !ok && len(l.keys) >= maxKeys {
			for k, old := range l.keys {
				if now.Sub(old.started) >= l.period {
					delete(l.keys, k)
				}
			}
			if len(l.keys) >= maxKeys {
				return false // under attack: refuse new keys rather than grow forever
			}
		}
		l.keys[key] = &window{started: now, count: 1}
		return true
	}
	w.count++
	return w.count <= l.max
}
