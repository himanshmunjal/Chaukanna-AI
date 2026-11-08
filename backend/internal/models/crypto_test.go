package models

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRoundTripAndLegacyValues(t *testing.T) {
	t.Cleanup(func() { SetEncryptionKey("") })
	if got := Encrypt("hello"); got != "hello" {
		t.Errorf("no key should leave it readable, got %q", got)
	}
	SetEncryptionKey("secret")
	c := Encrypt("OTP bata dijiye")
	if !strings.HasPrefix(c, encPrefix) || strings.Contains(c, "OTP") {
		t.Fatalf("not encrypted: %q", c)
	}
	if Encrypt(c) != c {
		t.Error("encrypting twice must not wrap it twice")
	}
	if Encrypt("") != "" {
		t.Error("empty stays empty")
	}
	if p, err := Decrypt(c); err != nil || p != "OTP bata dijiye" {
		t.Errorf("got %q, %v", p, err)
	}
	if p, _ := Decrypt("old plain row"); p != "old plain row" {
		t.Error("rows from before the key must stay readable")
	}
	SetEncryptionKey("other")
	if _, err := Decrypt(c); err == nil {
		t.Error("a wrong key must fail")
	}
}

// The real test: read the raw column, as someone with the database file would.
func TestSensitiveColumnsAreEncryptedOnDisk(t *testing.T) {
	t.Cleanup(func() { SetEncryptionKey("") })
	SetEncryptionKey("secret")
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "t.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProtectedUser{}, &Call{}, &TranscriptLine{}, &RedFlag{}, &Alert{}); err != nil {
		t.Fatal(err)
	}
	const secret = "mera OTP 4512 hai"
	db.Create(&TranscriptLine{CallID: 1, Text: secret})
	db.Create(&RedFlag{CallID: 1, Quote: secret})
	db.Create(&Alert{CallID: 1, Body: secret})
	db.Create(&Call{ID: 1, Reason: secret, Summary: secret})
	db.Create(&ProtectedUser{Phone: "+911", VoiceEmbedding: secret, TelegramChatID: secret})

	for _, q := range []string{
		"SELECT text FROM transcript_lines", "SELECT quote FROM red_flags", "SELECT body FROM alerts",
		"SELECT reason FROM calls", "SELECT summary FROM calls",
		"SELECT voice_embedding FROM protected_users", "SELECT telegram_chat_id FROM protected_users",
	} {
		var raw string
		db.Raw(q).Scan(&raw)
		if strings.Contains(raw, "4512") || !strings.HasPrefix(raw, encPrefix) {
			t.Errorf("%s: stored readable: %q", q, raw)
		}
	}

	var line TranscriptLine
	db.First(&line)
	var call Call
	db.First(&call)
	var user ProtectedUser
	db.First(&user)
	if line.Text != secret || call.Reason != secret || user.TelegramChatID != secret {
		t.Errorf("did not decrypt: %q %q %q", line.Text, call.Reason, user.TelegramChatID)
	}

	// Values written through a map need Encrypt; check that survives a read too.
	db.Model(&call).Updates(map[string]any{"reason": Encrypt("new reason")})
	var again Call
	db.First(&again)
	if again.Reason != "new reason" {
		t.Errorf("map update read back as %q", again.Reason)
	}
	var raw string
	db.Raw("SELECT reason FROM calls").Scan(&raw)
	if strings.Contains(raw, "new reason") {
		t.Error("map update stored readable")
	}
}
