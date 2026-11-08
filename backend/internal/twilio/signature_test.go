package twilio

import (
	"net/url"
	"testing"
)

// The worked example from Twilio's request-validation documentation.
func TestValidSignatureMatchesTwilioDocsExample(t *testing.T) {
	c := New("AC123", "12345", "+1")
	form := url.Values{
		"CallSid": {"CA1234567890ABCDE"}, "Caller": {"+14158675309"}, "Digits": {"1234"},
		"From": {"+14158675309"}, "To": {"+18005551212"},
	}
	const u = "https://mycompany.com/myapp.php?foo=1&bar=2"
	if !c.ValidSignature(u, form, "RSOYDt4T1cUTdK1PDd93/VVr8B8=") {
		t.Error("the documented example should validate")
	}
	if c.ValidSignature(u, form, "bad") || c.ValidSignature(u, form, "") {
		t.Error("a wrong or empty signature must fail")
	}
	form.Set("From", "+19999999999")
	if c.ValidSignature(u, form, "RSOYDt4T1cUTdK1PDd93/VVr8B8=") {
		t.Error("a changed parameter must fail")
	}
}

func TestStreamAuth(t *testing.T) {
	c := New("AC123", "secret", "+1")
	if !c.ValidStreamAuth("CA1", c.StreamAuth("CA1")) {
		t.Error("own tag should validate")
	}
	if c.ValidStreamAuth("CA2", c.StreamAuth("CA1")) || c.ValidStreamAuth("CA1", "") {
		t.Error("a tag for another call, or none, must fail")
	}
}
