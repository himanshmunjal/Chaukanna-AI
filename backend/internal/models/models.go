// Package models contains the GORM database tables.
//
//	ProtectedUser 1 ──< Call 1 ──< TranscriptLine
//	                           ├──< RedFlag
//	                           └──< Alert
//	ScamNumber (numbers caught scamming; blocked on their next call)
package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ProtectedUser is a person Chaukanna protects (usually an elderly parent),
// together with the family member who should be alerted.
type ProtectedUser struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`
	Phone       string    `gorm:"uniqueIndex" json:"phone"` // E.164, e.g. +919812345678
	Language    string    `json:"language"`                 // short code: hi, en, ta, bn, mr ...
	FamilyName  string    `json:"family_name"`
	FamilyPhone string    `json:"family_phone"`
	CreatedAt   time.Time `json:"created_at"`

	// ConsentAt is when the person (not only the family) agreed to be protected.
	ConsentAt *time.Time `json:"consent_at"`

	// Family voice: warnings are spoken in this person's cloned voice. Stored only
	// after they confirmed it is their own voice and they agree.
	VoiceEmbedding string     `gorm:"serializer:enc" json:"-"`
	VoiceOwner     string     `json:"voice_owner"`
	VoiceConsentAt *time.Time `json:"voice_consent_at"`

	// Telegram: the family links a chat by opening t.me/<bot>?start=<code>.
	TelegramChatID   string `gorm:"serializer:enc" json:"-"`
	TelegramLinkCode string `json:"-"`

	HasVoice       bool `gorm:"-" json:"has_voice"`
	TelegramLinked bool `gorm:"-" json:"telegram_linked"`
}

// AfterFind fills the fields the dashboard may see instead of the secrets.
func (u *ProtectedUser) AfterFind(*gorm.DB) error {
	u.HasVoice = u.VoiceEmbedding != ""
	u.TelegramLinked = u.TelegramChatID != ""
	return nil
}

// DemoUserName and DemoUserPhone are the made-up person used on the public demo. There is
// no family on this record, so nothing can ever be sent to a real person from a demo call.
const (
	DemoUserName  = "Dadi (demo)"
	DemoUserPhone = "+910000000000"
)

// Call is one protected or screened phone call.
type Call struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Provider       string `json:"provider"` // twilio | exotel | simulator
	ProviderCallID string `gorm:"index" json:"provider_call_id"`
	Mode           string `json:"mode"` // guardian | screen

	ProtectedUserID *uint          `json:"protected_user_id"`
	ProtectedUser   *ProtectedUser `json:"protected_user,omitempty"`
	CallerNumber    string         `json:"caller_number"`

	State    string  `json:"state"` // listening | suspicious | scam | warned
	Risk     float64 `json:"risk"`  // 0..1
	Reason   string  `gorm:"serializer:enc" json:"reason"`
	Language string  `json:"language"`
	ScamType string  `json:"scam_type"` // see scam.Category

	// WarningPlayed is false when a warning was due but no audio reached the call.
	WarningPlayed bool       `json:"warning_played"`
	CautionedAt   *time.Time `json:"cautioned_at"`
	// ComplyingAt is when the person first seemed to be giving a code or money.
	ComplyingAt *time.Time `json:"complying_at"`
	// DemoOwner is the anonymous visitor who started this call on a public demo. Empty
	// for real calls. Each side only ever sees its own.
	DemoOwner string `gorm:"index" json:"-"`
	// MarkedSafeAt is when the family said this call is fine, during the call.
	MarkedSafeAt *time.Time `json:"marked_safe_at"`
	// TestLabel and TestCategory are the truth about a test call, set by hand for scoring:
	// scam | legit, and the scam family. Empty on calls that aren't part of a test.
	TestLabel    string `json:"test_label"`
	TestCategory string `json:"test_category"`
	// Outcome is the family's verdict afterwards: confirmed_scam | false_alarm | money_lost.
	Outcome string `json:"outcome"`

	// Filled in by Screen Mode.
	CallerName string `gorm:"serializer:enc" json:"caller_name"`
	Purpose    string `gorm:"serializer:enc" json:"purpose"`
	Summary    string `gorm:"serializer:enc" json:"summary"`

	StartedAt        time.Time  `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
	WarnedAt         *time.Time `json:"warned_at"`
	DetectedAfterSec *float64   `json:"detected_after_sec"` // seconds from call start to "scam"

	Transcript []TranscriptLine `json:"transcript,omitempty"`
	Flags      []RedFlag        `json:"flags,omitempty"`
	Alerts     []Alert          `json:"alerts,omitempty"`
}

// TranscriptLine is one finalised sentence heard on the call (or spoken by the bot).
type TranscriptLine struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CallID    uint      `gorm:"index" json:"call_id"`
	TimeSec   float64   `json:"t"`       // seconds since call start
	Speaker   string    `json:"speaker"` // call | caller | chaukanna
	Text      string    `gorm:"serializer:enc" json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// RedFlag is one scam tactic detected on a call (e.g. "otp_request").
type RedFlag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CallID    uint      `gorm:"index" json:"call_id"`
	Type      string    `json:"type"`
	TimeSec   float64   `json:"t"` // seconds since call start
	Quote     string    `gorm:"serializer:enc" json:"quote"`
	Source    string    `json:"source"` // prefilter | evon
	CreatedAt time.Time `json:"created_at"`
}

// Alert is a WhatsApp/SMS message we tried to send.
type Alert struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CallID    uint      `gorm:"index" json:"call_id"`
	Channel   string    `json:"channel"` // whatsapp | sms | log
	Recipient string    `json:"recipient"`
	Body      string    `gorm:"serializer:enc" json:"body"`
	Status    string    `json:"status"` // sent | failed | logged
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
}

// ScamNumber is a caller number that was caught scamming (or reported by the
// family). If it calls any protected user again, Chaukanna blocks it at once.
type ScamNumber struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Phone      string    `gorm:"uniqueIndex" json:"phone"`
	Reports    int       `json:"reports"` // times it was flagged as a scam (or reported)
	Blocked    int       `json:"blocked"` // times Chaukanna stopped it at the start of a call
	Source     string    `json:"source"`  // auto | manual
	LastReason string    `gorm:"serializer:enc" json:"last_reason"`
	LastCallID *uint     `json:"last_call_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NormalizePhone turns Indian phone numbers in any common format into E.164.
//
//	"98123 45678"   -> "+919812345678"
//	"09812345678"   -> "+919812345678"
//	"919812345678"  -> "+919812345678"
//	"+14155550100"  -> "+14155550100"
func NormalizePhone(raw string) string {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch {
	case d == "":
		return ""
	case strings.HasPrefix(strings.TrimSpace(raw), "+"):
		return "+" + d
	case len(d) == 10:
		return "+91" + d
	case len(d) == 11 && d[0] == '0':
		return "+91" + d[1:]
	case len(d) == 12 && strings.HasPrefix(d, "91"):
		return "+" + d
	default:
		return "+" + d
	}
}
