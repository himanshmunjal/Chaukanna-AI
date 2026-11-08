package session

import (
	"encoding/base64"
	"sync"
	"sync/atomic"

	"chaukanna/internal/audio"
	"chaukanna/internal/hub"
)

// Leg is our connection to the phone call: how Chaukanna's voice gets into it.
// Each telephony provider implements it (see internal/telephony).
type Leg interface {
	// SendAudio plays 8 kHz PCM samples into the call.
	SendAudio(samples []int16) error
	// Hangup ends the call (Screen Mode uses this after the goodbye).
	Hangup() error
}

// SimulatorLeg is used by the dashboard simulator. There is no phone call, so
// the audio is sent to the dashboard instead, which plays it in the browser.
type SimulatorLeg struct {
	hub    *hub.Hub
	callID atomic.Uint64
	once   sync.Once
	done   chan struct{}
}

// NewSimulatorLeg creates a leg for a simulated call.
func NewSimulatorLeg(h *hub.Hub) *SimulatorLeg {
	return &SimulatorLeg{hub: h, done: make(chan struct{})}
}

// Attach links the leg to its call once the session has been created.
func (l *SimulatorLeg) Attach(callID uint) { l.callID.Store(uint64(callID)) }

// SendAudio sends the audio to the dashboard as a WAV file.
func (l *SimulatorLeg) SendAudio(samples []int16) error {
	wav := audio.EncodeWAV(samples, audio.PhoneRate)
	l.hub.Broadcast(hub.Event{
		Type:   "call_audio",
		CallID: uint(l.callID.Load()),
		Data:   map[string]string{"audio": base64.StdEncoding.EncodeToString(wav)},
	})
	return nil
}

// Hangup stops the simulated call.
func (l *SimulatorLeg) Hangup() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

// Done is closed when the bot hangs up.
func (l *SimulatorLeg) Done() <-chan struct{} { return l.done }
