package telephony

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"chaukanna/internal/audio"
	"chaukanna/internal/hub"
	"chaukanna/internal/scam"
	"chaukanna/internal/session"
)

// Speakerphone mode: no phone number needed.
//
// The call is on speaker next to a laptop. The dashboard's "Speakerphone" page
// captures the microphone and streams it here; warnings are sent back and
// played through the laptop speakers, so everyone on the call hears them.
//
//	GET /ws/speaker?mode=guardian&user_phone=+91...
//
//	browser -> server : binary frames, 16-bit little-endian PCM, 8 kHz, mono
//	                    text {"type":"stop"} to end
//	server  -> browser: {"type":"started","call_id":12}
//	                    {"type":"audio","audio":"<base64 WAV>"}   play this out loud
//	                    {"type":"error","message":"..."}
func (h *Handler) SpeakerStream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(maxStreamMessage)
	defer conn.Close()
	leg := &speakerLeg{conn: conn}

	prisma := h.prisma
	params := session.StartParams{
		Provider:  "speakerphone",
		Mode:      session.Mode(c.Query("mode")),
		UserPhone: c.Query("user_phone"),
		Leg:       leg,
	}
	if owner := c.GetString(hub.OwnerKey); owner != "" {
		// A demo visitor pays for their own listening. The key arrives as the first
		// message, not in the URL, because URLs end up in server logs.
		key := readVisitorKey(conn)
		switch {
		case key != "":
			prisma = h.prisma.WithKey(key)
			if h.opts.DemoTTS != nil {
				params.TTS = h.opts.DemoTTS.WithKey(key)
			}
		case h.opts.SharedListening != nil && h.prisma.Enabled():
			// The demo's shared key: one session from today's allowance.
			if !h.opts.SharedListening() {
				leg.send(map[string]any{"type": "error", "message": "The demo's shared Gnani key has reached today's limit. Add your own Gnani key to use the microphone."})
				return
			}
			params.TTS = h.opts.DemoTTS
		default:
			leg.send(map[string]any{"type": "error", "message": "Add your Gnani API key to use the microphone."})
			return
		}
		params.Owner = owner
		params.Language = scam.KnownLanguage(c.Query("language"))
	}

	if !prisma.Enabled() {
		leg.send(map[string]any{"type": "error", "message": "Prisma is not configured: set GNANI_API_KEY in backend/.env"})
		return
	}

	sess, err := h.sessions.Start(params)
	if err != nil {
		leg.send(map[string]any{"type": "error", "message": err.Error()})
		return
	}
	defer sess.End()

	stream := h.transcribeWith(prisma, sess)
	if stream == nil {
		leg.send(map[string]any{"type": "error", "message": "Could not connect to Prisma. Check the API key."})
		return
	}
	defer stream.Close() // runs before sess.End(): flush the last words first

	leg.send(map[string]any{"type": "started", "call_id": sess.ID()})

	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			return // browser closed the page or pressed Stop
		}
		if kind == websocket.TextMessage {
			return // {"type":"stop"}
		}
		if err := stream.Send(audio.BytesToSamples(data)); err != nil {
			log.Printf("telephony: speakerphone: send to prisma: %v", err)
			return
		}
	}
}

// readVisitorKey waits briefly for {"type":"auth","key":"..."}, ignoring any audio
// frames that arrive first. It returns "" if none comes.
func readVisitorKey(conn *websocket.Conn) string {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			return ""
		}
		if kind != websocket.TextMessage {
			continue
		}
		var m struct {
			Type string `json:"type"`
			Key  string `json:"key"`
		}
		if json.Unmarshal(data, &m) != nil || m.Type != "auth" {
			return ""
		}
		return strings.TrimSpace(m.Key)
	}
}

// speakerLeg "plays audio into the call" by sending it to the browser.
type speakerLeg struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (l *speakerLeg) send(msg any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.conn.WriteJSON(msg)
}

// SendAudio sends the clip to the browser, which plays it through the speakers.
func (l *speakerLeg) SendAudio(samples []int16) error {
	wav := audio.EncodeWAV(samples, audio.PhoneRate)
	l.send(map[string]any{"type": "audio", "audio": base64.StdEncoding.EncodeToString(wav)})
	return nil
}

// Hangup ends the session (used by Screen Mode after its goodbye).
func (l *speakerLeg) Hangup() error { return l.conn.Close() }
