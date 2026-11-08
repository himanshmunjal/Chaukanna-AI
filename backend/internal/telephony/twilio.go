package telephony

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"chaukanna/internal/audio"
	"chaukanna/internal/session"
	"chaukanna/internal/twilio"
)

// TwilioVoice is the "A call comes in" webhook of your Twilio number.
// It decides Guardian vs Screen mode and answers with TwiML that connects the
// call to our Media Stream WebSocket.
func (h *Handler) TwilioVoice(c *gin.Context) {
	mode, userPhone, caller := h.sessions.ResolveMode(c.PostForm("From"), c.PostForm("ForwardedFrom"))
	callSid := c.PostForm("CallSid")

	streamURL := strings.Replace(h.publicURL, "https://", "wss://", 1) + "/ws/twilio"
	if h.publicURL == "" {
		log.Println("telephony: PUBLIC_URL is empty; Twilio cannot reach the media stream")
	}

	twiml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Connect>
    <Stream url="%s">
      <Parameter name="mode" value="%s"/>
      <Parameter name="user_phone" value="%s"/>
      <Parameter name="caller" value="%s"/>
      <Parameter name="auth" value="%s"/>
    </Stream>
  </Connect>
</Response>`, html.EscapeString(streamURL), mode, html.EscapeString(userPhone), html.EscapeString(caller), h.twilio.StreamAuth(callSid))

	c.Data(http.StatusOK, "application/xml", []byte(twiml))
}

// VerifyTwilio rejects webhook requests that were not signed by Twilio. They decide
// who counts as the protected user, so they must not be forgeable.
func (h *Handler) VerifyTwilio(c *gin.Context) {
	if h.opts.TestMode {
		return
	}
	if h.publicURL == "" {
		log.Printf("telephony: twilio: rejected webhook: PUBLIC_URL is not set, so it can't be verified")
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	url := h.publicURL + c.Request.URL.RequestURI()
	if !h.twilio.ValidSignature(url, c.Request.PostForm, c.GetHeader("X-Twilio-Signature")) {
		log.Printf("telephony: twilio: rejected webhook with invalid signature")
		c.AbortWithStatus(http.StatusForbidden)
	}
}

// twilioMessage is a message on the Twilio Media Stream WebSocket.
type twilioMessage struct {
	Event string `json:"event"` // connected | start | media | stop | mark
	Start struct {
		StreamSid        string            `json:"streamSid"`
		CallSid          string            `json:"callSid"`
		CustomParameters map[string]string `json:"customParameters"`
	} `json:"start"`
	Media struct {
		Payload string `json:"payload"` // base64 μ-law 8 kHz
	} `json:"media"`
}

// TwilioStream handles the bidirectional Media Stream for one call.
func (h *Handler) TwilioStream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	leg := &twilioLeg{conn: conn, rest: h.twilio}
	var sess *session.Session
	var transcriber interface {
		Send([]int16) error
		Close()
	}

	defer func() {
		if transcriber != nil {
			transcriber.Close()
		}
		if sess != nil {
			sess.End()
		}
	}()

	for {
		var msg twilioMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return // call ended or socket closed
		}

		switch msg.Event {
		case "start":
			leg.setIDs(msg.Start.StreamSid, msg.Start.CallSid)
			p := msg.Start.CustomParameters
			// Only a stream our own webhook set up is accepted: without this, anyone
			// could open this socket and spend Prisma credits as a fake call.
			if !h.twilio.ValidStreamAuth(msg.Start.CallSid, p["auth"]) {
				log.Printf("telephony: twilio: rejected stream with invalid auth (call %s)", msg.Start.CallSid)
				return
			}
			sess, err = h.sessions.Start(session.StartParams{
				Provider:       "twilio",
				ProviderCallID: msg.Start.CallSid,
				Mode:           session.Mode(p["mode"]),
				UserPhone:      p["user_phone"],
				CallerNumber:   p["caller"],
				Leg:            leg,
			})
			if err != nil {
				log.Printf("telephony: twilio: %v", err)
				return
			}
			if stream := h.startTranscription(sess); stream != nil {
				transcriber = stream
			}

		case "media":
			if transcriber == nil {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(msg.Media.Payload)
			if err != nil {
				continue
			}
			if err := transcriber.Send(audio.MulawDecode(raw)); err != nil {
				log.Printf("telephony: twilio: send to prisma: %v", err)
			}

		case "stop":
			return
		}
	}
}

// twilioLeg plays audio into a Twilio call through the same WebSocket.
type twilioLeg struct {
	conn      *websocket.Conn
	rest      *twilio.Client
	mu        sync.Mutex // one writer at a time
	streamSid string
	callSid   string
}

func (l *twilioLeg) setIDs(streamSid, callSid string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.streamSid, l.callSid = streamSid, callSid
}

// SendAudio sends μ-law audio in 20 ms frames (160 bytes each).
func (l *twilioLeg) SendAudio(samples []int16) error {
	payload := audio.MulawEncode(samples)
	l.mu.Lock()
	defer l.mu.Unlock()
	for start := 0; start < len(payload); start += 160 {
		end := min(start+160, len(payload))
		msg := map[string]any{
			"event":     "media",
			"streamSid": l.streamSid,
			"media":     map[string]string{"payload": base64.StdEncoding.EncodeToString(payload[start:end])},
		}
		if err := l.conn.WriteJSON(msg); err != nil {
			return err
		}
	}
	return nil
}

// Hangup ends the call through the REST API (or just closes the stream).
func (l *twilioLeg) Hangup() error {
	l.mu.Lock()
	callSid := l.callSid
	l.mu.Unlock()
	if l.rest.Enabled() && callSid != "" {
		return l.rest.Hangup(context.Background(), callSid)
	}
	return l.conn.Close()
}
