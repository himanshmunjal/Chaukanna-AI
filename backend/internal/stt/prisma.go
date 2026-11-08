// Package stt streams call audio to Gnani Prisma (speech-to-text) and returns
// live transcripts.
//
// Flow:  Open() -> Send(audio) many times -> read Results() -> Close()
//
// Protocol (https://docs.gnani.ai/api/STT/stt-websocket):
//   - Connect to wss://api.vachana.ai/stt/v3/stream. ALL settings are sent as
//     WebSocket headers (x-api-key-id, language_code, model, x-sample-rate, ...).
//     There is no "start" message.
//   - Send raw 16-bit little-endian mono PCM in binary frames of EXACTLY 1024
//     bytes (64 ms at 8 kHz), at real-time pace.
//   - Prisma's voice-activity detection splits speech into segments and sends one
//     {"type":"transcript","text":...,"is_final":true} per segment. There are no
//     partial results.
//   - Send {"type":"commit"} to flush the last segment before closing.
//   - Sessions last at most 15 minutes.
package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"chaukanna/internal/audio"
	"chaukanna/internal/scam"
)

// frameBytes is the exact binary frame size Prisma requires.
const frameBytes = 1024

// Transcript is one recognition result.
type Transcript struct {
	Text    string
	IsFinal bool
}

// Client creates Prisma streams.
type Client struct {
	url        string
	apiKey     string
	model      string
	sampleRate int
}

// New creates a Prisma client. sampleRate is the rate we send (8000 for phone audio).
func New(url, apiKey, model string, sampleRate int) *Client {
	if sampleRate <= 0 {
		sampleRate = audio.PhoneRate
	}
	return &Client{url: url, apiKey: apiKey, model: model, sampleRate: sampleRate}
}

// WithKey returns a client that uses someone else's API key.
func (c *Client) WithKey(key string) *Client {
	return New(c.url, key, c.model, c.sampleRate)
}

// Enabled reports whether Prisma is configured.
func (c *Client) Enabled() bool { return c != nil && c.url != "" && c.apiKey != "" }

// Stream is one live recognition session (one per phone call).
type Stream struct {
	conn       *websocket.Conn
	writeMu    sync.Mutex // gorilla/websocket allows only one writer at a time
	pending    []byte     // audio waiting to fill a 1024-byte frame
	results    chan Transcript
	sampleRate int
	closed     atomic.Bool
	done       chan struct{} // closed when the read loop has finished
}

// Open connects to Prisma and starts a recognition session.
// language is a short code like "hi".
func (c *Client) Open(ctx context.Context, language string) (*Stream, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("prisma is not configured")
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, c.url, c.headers(language))
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("prisma connect: %w (HTTP %d; 401 means a wrong GNANI_API_KEY)", err, resp.StatusCode)
		}
		return nil, fmt.Errorf("prisma connect: %w", err)
	}

	s := &Stream{conn: conn, results: make(chan Transcript, 64), sampleRate: c.sampleRate, done: make(chan struct{})}
	go s.readLoop()
	return s, nil
}

// headers builds the connection settings Prisma reads from the WebSocket handshake.
func (c *Client) headers(language string) http.Header {
	h := http.Header{}
	h.Set("x-api-key-id", c.apiKey)
	h.Set("language_code", languageCodes(c.model, language))
	if c.model != "" {
		h.Set("model", c.model)
	}
	h.Set("x-sample-rate", strconv.Itoa(c.sampleRate))
	h.Set("x-idle-timeout-sec", "300") // a guardian call can have long quiet stretches
	h.Set("x-min-silence-ms", "500")   // 0.5 s of silence ends a segment
	h.Set("x-min-speech-ms", "200")    // recommended for 8 kHz phone audio
	h.Set("x-max-speech-s", "15")      // scammers monologue; don't wait forever for a pause
	if c.model == "prisma-v3.0" {
		// Word boosting: tell Prisma which words matter most on these calls, so
		// "AnyDesk" isn't heard as "any desk" or "OTP" as "OTB" on a noisy line.
		h.Set("keyterms", strings.Join(scamKeyterms, ","))
	}
	return h
}

// scamKeyterms are boosted in Prisma v3 (max 200). Brand names, acronyms and
// English words that scammers use inside Hindi/regional sentences.
// ASCII only: they travel in an HTTP header.
var scamKeyterms = []string{
	"OTP", "PIN", "CVV", "UPI", "KYC", "Aadhaar", "PAN card",
	"AnyDesk", "TeamViewer", "QuickSupport", "RustDesk", "screen share", "APK",
	"CBI", "RBI", "TRAI", "NCB", "FIR", "customs", "cyber crime", "crime branch",
	"arrest warrant", "digital arrest", "money laundering", "bail",
	"FedEx", "DHL", "courier", "parcel",
	"SBI", "HDFC", "ICICI", "Axis Bank", "Kotak", "PNB", "YONO", "Paytm", "PhonePe", "Google Pay",
	"safe account", "verification account", "refund", "cashback", "lottery", "KBC",
}

// languageCodes returns the language_code header. The live API accepts exactly
// one locale (e.g. "hi-IN"), even though the docs show comma-separated lists.
// Prisma handles Hinglish code-mixing within the Hindi locale.
func languageCodes(_ string, language string) string {
	return scam.Locale(language)
}

// Send streams phone audio (8 kHz PCM samples) to Prisma, in 1024-byte frames.
func (s *Stream) Send(samples []int16) error {
	if s.closed.Load() {
		return nil
	}
	if s.sampleRate != audio.PhoneRate {
		samples = audio.Resample(samples, audio.PhoneRate, s.sampleRate)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.pending = append(s.pending, audio.SamplesToBytes(samples)...)
	for len(s.pending) >= frameBytes {
		if err := s.conn.WriteMessage(websocket.BinaryMessage, s.pending[:frameBytes]); err != nil {
			return err
		}
		s.pending = s.pending[frameBytes:]
	}
	return nil
}

// Results delivers transcripts. The channel is closed when the stream ends.
func (s *Stream) Results() <-chan Transcript { return s.results }

// Close flushes the last segment ("commit") and ends the session.
func (s *Stream) Close() {
	if s.closed.Swap(true) {
		return
	}
	s.writeMu.Lock()
	_ = s.conn.WriteJSON(map[string]string{"type": "commit"})
	s.writeMu.Unlock()
	// Give Prisma a moment to send the last sentence, or the end of the call is lost.
	select {
	case <-s.done:
	case <-time.After(lastWordsWait):
	}
	_ = s.conn.Close()
}

// lastWordsWait is how long Close waits for the final transcript.
const lastWordsWait = 1500 * time.Millisecond

func (s *Stream) readLoop() {
	defer close(s.done)
	defer close(s.results)
	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		if t, ok := parseMessage(msg); ok {
			s.results <- t
		}
	}
}

// parseMessage reads a transcript from a server message.
// Other message types (connected, speech_start, processing) are ignored;
// errors are logged.
func parseMessage(raw []byte) (Transcript, bool) {
	var m struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		IsFinal *bool  `json:"is_final"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return Transcript{}, false
	}
	switch m.Type {
	case "transcript":
		text := strings.TrimSpace(m.Text)
		if text == "" {
			return Transcript{}, false
		}
		final := m.IsFinal == nil || *m.IsFinal // Prisma results are always final
		return Transcript{Text: text, IsFinal: final}, true
	case "error":
		log.Printf("prisma: error from server: %s", m.Message)
	}
	return Transcript{}, false
}
