// Package telephony connects phone calls to sessions.
//
// Both supported providers stream call audio to us over a WebSocket:
//
//	Twilio: POST /telephony/twilio/voice  (webhook, returns TwiML that opens the stream)
//	        GET  /ws/twilio                (Media Stream: μ-law 8 kHz, base64 in JSON)
//	Exotel: GET  /ws/exotel                (Voicebot applet stream: 16-bit PCM 8 kHz, base64 in JSON)
//	Laptop: GET  /ws/speaker               (speakerphone mode: browser microphone, binary PCM 8 kHz)
//
// For each call the handler:
//  1. starts a Session,
//  2. opens a Prisma stream and forwards every audio frame to it,
//  3. forwards every transcript to the Session,
//  4. ends the Session when the stream stops.
package telephony

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"chaukanna/internal/session"
	"chaukanna/internal/stt"
	"chaukanna/internal/tts"
	"chaukanna/internal/twilio"
)

// maxStreamMessage caps one WebSocket message. Audio frames are a few KB; without a
// cap, one huge message could use up the server's memory.
const maxStreamMessage = 1 << 20

// Handler serves the telephony endpoints.
type Handler struct {
	sessions  *session.Manager
	prisma    *stt.Client
	twilio    *twilio.Client
	publicURL string
	upgrader  websocket.Upgrader
	opts      Options
}

// Options are the telephony safety settings.
type Options struct {
	// TestMode accepts unverified streams, for cmd/smoketest on a laptop only.
	TestMode bool
	// ExotelToken must appear in the Exotel stream URL. Empty turns Exotel off.
	ExotelToken string
	// DemoTTS is the voice for visitors without a key: only prepared clips.
	DemoTTS *tts.Client
	// SharedListening, when set, lets a visitor without a key use this server's Gnani
	// key for one microphone session, if the shared daily allowance has room.
	SharedListening func() bool
}

// New creates a Handler.
func New(sessions *session.Manager, prisma *stt.Client, tw *twilio.Client, publicURL string, opts Options) *Handler {
	return &Handler{
		sessions:  sessions,
		prisma:    prisma,
		twilio:    tw,
		publicURL: publicURL,
		opts:      opts,
		upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
}

// RequireTwilio hides the Twilio endpoints when Twilio isn't set up. Without it, they
// couldn't be verified, and anyone could open one and spend Prisma credits.
func (h *Handler) RequireTwilio(c *gin.Context) {
	if !h.twilio.Enabled() && !h.opts.TestMode {
		c.AbortWithStatus(http.StatusNotFound)
	}
}

// RequireExotel checks the secret in the stream URL. Exotel doesn't sign its streams,
// so this is what stops strangers connecting. No token set means Exotel is off.
func (h *Handler) RequireExotel(c *gin.Context) {
	if h.opts.TestMode {
		return
	}
	if h.opts.ExotelToken == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// In the path as well as the query: Exotel treats query values as its own custom
	// parameters, so a query token may not arrive in the address.
	token := c.Param("token")
	if token == "" {
		token = c.Query("token")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.opts.ExotelToken)) != 1 {
		log.Printf("telephony: exotel: refused a stream with a missing or wrong token")
		c.AbortWithStatus(http.StatusForbidden)
	}
}

// startTranscription opens a Prisma stream for the session and forwards its
// transcripts. Returns nil if Prisma is not configured (the call still runs,
// but nothing will be transcribed).
func (h *Handler) startTranscription(sess *session.Session) *stt.Resilient {
	return h.transcribeWith(h.prisma, sess)
}

func (h *Handler) transcribeWith(prisma *stt.Client, sess *session.Session) *stt.Resilient {
	if !prisma.Enabled() {
		log.Printf("telephony: call %d: PRISMA_WS_URL / GNANI_API_KEY not set, audio will not be transcribed", sess.ID())
		return nil
	}
	stream, err := prisma.OpenResilient(context.Background(), sess.Language())
	if err != nil {
		log.Printf("telephony: call %d: %v", sess.ID(), err)
		return nil
	}
	go func() {
		for t := range stream.Results() {
			sess.OnTranscript(t.Text, t.IsFinal)
		}
	}()
	return stream
}
