package twilio

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// ValidSignature checks Twilio's X-Twilio-Signature: base64(HMAC-SHA1(authToken,
// URL + each POST parameter's name and value, sorted by name)).
func (c *Client) ValidSignature(fullURL string, form url.Values, signature string) bool {
	if signature == "" {
		return false
	}
	keys := make([]string, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(fullURL)
	for _, k := range keys {
		b.WriteString(k)
		if len(form[k]) > 0 {
			b.WriteString(form[k][0])
		}
	}
	mac := hmac.New(sha1.New, []byte(c.authToken))
	mac.Write([]byte(b.String()))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(signature))
}

// StreamAuth is a tag tying a media stream to the signed webhook that created it.
// An empty string means no auth token is configured (local development).
func (c *Client) StreamAuth(callSid string) string {
	if !c.Enabled() {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(c.authToken))
	mac.Write([]byte("stream:" + callSid))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidStreamAuth checks the tag from StreamAuth.
func (c *Client) ValidStreamAuth(callSid, tag string) bool {
	if !c.Enabled() {
		return true
	}
	return hmac.Equal([]byte(c.StreamAuth(callSid)), []byte(tag))
}
