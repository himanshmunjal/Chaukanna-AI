package telephony

import (
	"encoding/base64"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"chaukanna/internal/audio"
	"chaukanna/internal/session"
)

// exotelMessage is a message on the Exotel Voicebot (bidirectional stream) WebSocket.
// ⚠️ Field names follow Exotel's Voicebot applet docs; double-check them for your account.
type exotelMessage struct {
	Event     string `json:"event"` // connected | start | media | stop | mark | dtmf
	StreamSid string `json:"stream_sid"`
	Start     struct {
		StreamSid        string            `json:"stream_sid"`
		CallSid          string            `json:"call_sid"`
		From             string            `json:"from"`
		To               string            `json:"to"`
		CustomParameters map[string]string `json:"custom_parameters"`
	} `json:"start"`
	Media struct {
		Payload string `json:"payload"` // base64 16-bit PCM, 8 kHz, mono
	} `json:"media"`
	Mark struct {
		Name string `json:"name"`
	} `json:"mark"`
}

// ExotelStream handles one Exotel Voicebot stream.
// In the Exotel flow builder, point the Voicebot applet at wss://<PUBLIC_URL>/ws/exotel.
func (h *Handler) ExotelStream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(maxStreamMessage)
	defer conn.Close()

	leg := &exotelLeg{conn: conn}
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
		var msg exotelMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		switch msg.Event {
		case "start":
			streamSid := msg.Start.StreamSid
			if streamSid == "" {
				streamSid = msg.StreamSid
			}
			leg.setStreamSid(streamSid)

			mode, userPhone, caller := h.sessions.ResolveMode(msg.Start.From, msg.Start.CustomParameters["forwarded_from"])
			sess, err = h.sessions.Start(session.StartParams{
				Provider:       "exotel",
				ProviderCallID: msg.Start.CallSid,
				Mode:           mode,
				UserPhone:      userPhone,
				CallerNumber:   caller,
				Leg:            leg,
			})
			if err != nil {
				log.Printf("telephony: exotel: %v", err)
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
			if err := transcriber.Send(audio.BytesToSamples(raw)); err != nil {
				log.Printf("telephony: exotel: send to prisma: %v", err)
			}

		case "mark":
			// Exotel's own word that our audio finished playing. Without this line in the
			// log after a warning, Exotel accepted the audio but never played it.
			leg.markPlayed(msg.Mark.Name)

		case "stop":
			return
		}
	}
}

// exotelLeg plays audio into an Exotel call.
type exotelLeg struct {
	conn      *websocket.Conn
	mu        sync.Mutex
	streamSid string
	clips     int
	sentAt    map[string]time.Time
}

func (l *exotelLeg) setStreamSid(sid string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.streamSid = sid
}

// exotelChunk is the size of each audio chunk sent back. Exotel asks for at least
// 3.2 KB and a multiple of 320 bytes; smaller chunks can distort the audio.
// https://developer.exotel.com/docs/agentstream/stream-voicebot-applet
const exotelChunk = 3200

// exotelChunks splits PCM into chunks Exotel accepts, padding the last with silence.
func exotelChunks(pcm []byte) [][]byte {
	var out [][]byte
	for start := 0; start < len(pcm); start += exotelChunk {
		piece := pcm[start:min(start+exotelChunk, len(pcm))]
		if len(piece) < exotelChunk {
			piece = append(append([]byte(nil), piece...), make([]byte, exotelChunk-len(piece))...)
		}
		out = append(out, piece)
	}
	return out
}

// SendAudio plays PCM into the call.
func (l *exotelLeg) SendAudio(samples []int16) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, piece := range exotelChunks(audio.SamplesToBytes(samples)) {
		msg := map[string]any{
			"event":      "media",
			"stream_sid": l.streamSid,
			"media":      map[string]string{"payload": base64.StdEncoding.EncodeToString(piece)},
		}
		if err := l.conn.WriteJSON(msg); err != nil {
			return err
		}
	}

	// Ask Exotel to tell us when this clip has really played.
	l.clips++
	name := "clip-" + strconv.Itoa(l.clips)
	if l.sentAt == nil {
		l.sentAt = map[string]time.Time{}
	}
	l.sentAt[name] = time.Now()
	log.Printf("telephony: exotel: sent %s, %.1f s of audio (stream id set: %v)", name, float64(len(samples))/audio.PhoneRate, l.streamSid != "")
	return l.conn.WriteJSON(map[string]any{
		"event":      "mark",
		"stream_sid": l.streamSid,
		"mark":       map[string]string{"name": name},
	})
}

// markPlayed logs Exotel confirming that a clip has finished playing.
func (l *exotelLeg) markPlayed(name string) {
	l.mu.Lock()
	sent, ok := l.sentAt[name]
	l.mu.Unlock()
	if !ok {
		log.Printf("telephony: exotel: played %s", name)
		return
	}
	log.Printf("telephony: exotel: played %s, %.1f s after it was sent", name, time.Since(sent).Seconds())
}

// Hangup closes the stream; the Exotel flow then moves to its next applet
// (add a "Hangup" applet after the Voicebot applet).
func (l *exotelLeg) Hangup() error {
	return l.conn.Close()
}
