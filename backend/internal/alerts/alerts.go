// Package alerts sends WhatsApp/SMS messages to the family (and, in Screen
// Mode, a text summary to the protected user).
//
// It tries channels in order and stops at the first that works:
//
//  1. WhatsApp Cloud API   (if WHATSAPP_TOKEN + WHATSAPP_PHONE_NUMBER_ID are set)
//  2. Twilio SMS           (if Twilio is configured)
//  3. Log only             (prints the message, so demos still show it)
//
// Every attempt is saved in the alerts table.
package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"chaukanna/internal/models"
	"chaukanna/internal/scam"
	"chaukanna/internal/telegram"
	"chaukanna/internal/twilio"
)

// WhatsAppConfig holds the WhatsApp Cloud API settings.
type WhatsAppConfig struct {
	Token         string
	PhoneNumberID string
	APIVersion    string
}

func (w WhatsAppConfig) enabled() bool { return w.Token != "" && w.PhoneNumberID != "" }

// Notifier sends alerts.
type Notifier struct {
	db       *gorm.DB
	whatsapp WhatsAppConfig
	twilio   *twilio.Client
	telegram *telegram.Client
	http     *http.Client
}

// New creates a Notifier.
func New(db *gorm.DB, wa WhatsAppConfig, tw *twilio.Client, tg *telegram.Client) *Notifier {
	return &Notifier{db: db, whatsapp: wa, twilio: tw, telegram: tg, http: &http.Client{Timeout: 15 * time.Second}}
}

// Send delivers body to `to` and records the result for callID.
func (n *Notifier) Send(ctx context.Context, callID uint, to, body string) {
	if to == "" {
		return
	}
	alert := models.Alert{CallID: callID, Recipient: to, Body: body}

	switch {
	case n.whatsapp.enabled():
		alert.Channel = "whatsapp"
		if err := n.sendWhatsApp(ctx, to, body); err != nil {
			alert.Status, alert.Error = "failed", err.Error()
			// WhatsApp failed (e.g. no open 24h session): fall back to SMS.
			if n.twilio.Enabled() {
				n.record(alert)
				alert = models.Alert{CallID: callID, Recipient: to, Body: body, Channel: "sms"}
				alert.Status, alert.Error = statusOf(n.twilio.SendSMS(ctx, to, body))
			}
		} else {
			alert.Status = "sent"
		}
	case n.twilio.Enabled():
		alert.Channel = "sms"
		alert.Status, alert.Error = statusOf(n.twilio.SendSMS(ctx, to, body))
	default:
		alert.Channel, alert.Status = "log", "logged"
		log.Printf("alerts: (no WhatsApp/SMS configured) message to %s:\n%s", to, body)
	}

	if alert.Error != "" {
		log.Printf("alerts: %s to %s failed: %s", alert.Channel, to, alert.Error)
	}
	n.record(alert)
}

// SendToFamily alerts a protected user's family on Telegram (if they linked a chat)
// as well as WhatsApp/SMS. This is urgent, so one working channel must not stop another.
func (n *Notifier) SendToFamily(ctx context.Context, callID uint, user models.ProtectedUser, body string) {
	if user.TelegramChatID != "" && n.telegram.Enabled() {
		alert := models.Alert{CallID: callID, Recipient: user.FamilyName, Body: body, Channel: "telegram"}
		if alert.Recipient == "" {
			alert.Recipient = "family"
		}
		alert.Status, alert.Error = statusOf(n.telegram.Send(ctx, user.TelegramChatID, body))
		n.record(alert)
	}
	n.Send(ctx, callID, user.FamilyPhone, body)
}

// RecordDemo notes what a demo call would have sent. Nothing leaves the server.
func (n *Notifier) RecordDemo(callID uint, body string) {
	n.record(models.Alert{CallID: callID, Channel: "demo", Recipient: "family", Body: body, Status: "not sent (demo)"})
}

func (n *Notifier) record(a models.Alert) {
	if err := n.db.Create(&a).Error; err != nil {
		log.Printf("alerts: could not save alert: %v", err)
	}
}

func statusOf(err error) (status, errText string) {
	if err != nil {
		return "failed", err.Error()
	}
	return "sent", ""
}

// sendWhatsApp sends a plain text message with the WhatsApp Cloud API.
func (n *Notifier) sendWhatsApp(ctx context.Context, to, body string) error {
	endpoint := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", n.whatsapp.APIVersion, n.whatsapp.PhoneNumberID)
	payload, _ := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                strings.TrimPrefix(to, "+"),
		"type":              "text",
		"text":              map[string]any{"body": body, "preview_url": true},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+n.whatsapp.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp returned %d: %.300s", resp.StatusCode, b)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Message templates
// ---------------------------------------------------------------------------

// ScamAlert is sent to the family when a live call is detected as a scam.
//
// played says whether the warning audio actually reached the call: the family must
// not relax because of a warning nobody heard.
func ScamAlert(user models.ProtectedUser, risk float64, category scam.Category, reason string, flags []scam.Flag, link string, played bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🚨 Chaukanna alert: %s is on a likely SCAM call right now (risk %d%%).\n", user.Name, int(risk*100))
	if label, ok := scam.CategoryLabels[category]; ok && category != scam.General {
		fmt.Fprintf(&b, "Type: %s\n", label)
	}
	b.WriteString("\n")
	if reason != "" {
		fmt.Fprintf(&b, "Why: %s\n\n", reason)
	}
	if len(flags) > 0 {
		b.WriteString("Red flags:\n")
		for _, f := range flags {
			fmt.Fprintf(&b, "• %s (%.0fs): \"%s\"\n", strings.ReplaceAll(string(f.Type), "_", " "), f.T, f.Quote)
		}
		b.WriteString("\n")
	}
	if played {
		fmt.Fprintf(&b, "A warning was played on the call. ")
	} else {
		fmt.Fprintf(&b, "⚠️ The spoken warning could NOT be played. ")
	}
	fmt.Fprintf(&b, "Please call %s now: %s\nIf money was sent, call 1930 immediately.\n\nLive view: %s", user.Name, user.Phone, link)
	return b.String()
}

// ComplyingAlert is the most urgent alert: the person seems to be giving away a code or
// money right now, even after being warned.
func ComplyingAlert(user models.ProtectedUser, heard, link string) string {
	return fmt.Sprintf("🚨🚨 URGENT: %s may be giving a code or money to a scammer RIGHT NOW.\n\nHeard: \"%s\"\n\nCall %s immediately: %s\nIf money was sent, call 1930 at once.\n\nLive view: %s",
		user.Name, heard, user.Name, user.Phone, link)
}

// TalkOutAlert goes out when a caller is trying to talk the person out of the warning. A
// voice from a machine can be argued with. A call from their own child is much harder to dismiss.
func TalkOutAlert(user models.ProtectedUser, heard, link string) string {
	return fmt.Sprintf("⚠️ The caller is trying to talk %s out of the warning.\n\nHeard: \"%s\"\n\nPlease call %s now: %s\n\nLive view: %s",
		user.Name, heard, user.Name, user.Phone, link)
}

// StillOnCallAlert goes out when the person is still on a flagged call after the warning.
func StillOnCallAlert(user models.ProtectedUser, seconds int, link string) string {
	return fmt.Sprintf("⚠️ %s is still on the flagged call, %d seconds after the warning.\n\nPlease call %s now: %s\n\nLive view: %s",
		user.Name, seconds, user.Name, user.Phone, link)
}

// ScreenSummary is sent after Screen Mode answered an unknown call.
func ScreenSummary(user models.ProtectedUser, caller, callerName, purpose, summary string, risk float64, link string) string {
	verdict := "✅ Looks safe"
	if risk >= 0.8 {
		verdict = "🚨 Likely SCAM, do not call back"
	} else if risk >= 0.4 {
		verdict = "⚠️ Suspicious"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "📞 Chaukanna screened a call for %s\n", user.Name)
	if caller != "" {
		fmt.Fprintf(&b, "From: %s\n", caller)
	}
	if callerName != "" {
		fmt.Fprintf(&b, "Caller: %s\n", callerName)
	}
	if purpose != "" {
		fmt.Fprintf(&b, "Purpose: %s\n", purpose)
	}
	fmt.Fprintf(&b, "Verdict: %s (risk %d%%)\n\n%s\n\nDetails: %s", verdict, int(risk*100), summary, link)
	return b.String()
}
