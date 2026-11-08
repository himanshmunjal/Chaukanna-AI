// Package clips keeps short audio files in memory for one hour so Twilio can
// download them by URL (used to play the Screen Mode summary on a callback).
package clips

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const ttl = time.Hour

type clip struct {
	data    []byte
	created time.Time
}

// Store is a thread-safe in-memory store of WAV files.
type Store struct {
	mu    sync.Mutex
	clips map[string]clip
}

// New creates an empty store.
func New() *Store { return &Store{clips: map[string]clip{}} }

// Put saves a WAV file and returns its random id.
func (s *Store) Put(wav []byte) string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	id := hex.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	for k, c := range s.clips { // drop expired clips
		if time.Since(c.created) > ttl {
			delete(s.clips, k)
		}
	}
	s.clips[id] = clip{data: wav, created: time.Now()}
	return id
}

// Get returns a WAV file by id.
func (s *Store) Get(id string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clips[id]
	if !ok || time.Since(c.created) > ttl {
		return nil, false
	}
	return c.data, true
}
