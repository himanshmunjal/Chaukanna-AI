package models

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"
	"sync"

	"gorm.io/gorm/schema"
)

// Sensitive columns (what was said on calls, voice models, chat ids) are encrypted with
// AES-256-GCM when ENCRYPTION_KEY is set. Fields opt in with `gorm:"serializer:enc"`.
//
// Without a key values stay readable, for local development. Rows written before a key
// was set stay readable too, because encrypted values carry a prefix. Phone numbers
// are not encrypted: calls are looked up by them.

const encPrefix = "enc:v1:"

var (
	gcmMu sync.RWMutex
	gcm   cipher.AEAD
)

func init() { schema.RegisterSerializer("enc", encSerializer{}) }

// SetEncryptionKey sets the key from any passphrase. Empty turns encryption off.
func SetEncryptionKey(passphrase string) error {
	gcmMu.Lock()
	defer gcmMu.Unlock()
	if passphrase == "" {
		gcm = nil
		return nil
	}
	sum := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return err
	}
	gcm, err = cipher.NewGCM(block)
	return err
}

// Encrypt protects a value. It is safe to call twice, and leaves "" as "".
// Use it for values written through Updates(map): GORM skips serializers there.
func Encrypt(plain string) string {
	gcmMu.RLock()
	g := gcm
	gcmMu.RUnlock()
	if g == nil || plain == "" || strings.HasPrefix(plain, encPrefix) {
		return plain
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic("crypto/rand failed: " + err.Error()) // never fall back to storing it readable
	}
	return encPrefix + base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(plain), nil))
}

// Decrypt reverses Encrypt. Values without the prefix are returned as they are.
func Decrypt(stored string) (string, error) {
	if !strings.HasPrefix(stored, encPrefix) {
		return stored, nil
	}
	gcmMu.RLock()
	g := gcm
	gcmMu.RUnlock()
	if g == nil {
		return "", fmt.Errorf("value is encrypted but ENCRYPTION_KEY is not set")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, encPrefix))
	if err != nil || len(raw) < g.NonceSize() {
		return "", fmt.Errorf("corrupt encrypted value")
	}
	plain, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("wrong ENCRYPTION_KEY or corrupt value")
	}
	return string(plain), nil
}

// unreadable replaces a value that can't be decrypted, so one bad row can't break a page.
const unreadable = "[could not decrypt]"

type encSerializer struct{}

func (encSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	var stored string
	switch v := dbValue.(type) {
	case nil:
	case string:
		stored = v
	case []byte:
		stored = string(v)
	default:
		return fmt.Errorf("enc: unexpected column type %T", dbValue)
	}
	plain, err := Decrypt(stored)
	if err != nil {
		log.Printf("models: %s: %v", field.Name, err)
		plain = unreadable
	}
	field.ReflectValueOf(ctx, dst).SetString(plain)
	return nil
}

func (encSerializer) Value(_ context.Context, _ *schema.Field, _ reflect.Value, fieldValue any) (any, error) {
	s, _ := fieldValue.(string)
	return Encrypt(s), nil
}
