// Package twilio is a tiny client for the three Twilio REST calls we need:
// send an SMS, hang up a call, and place a call that plays an audio file.
// (No SDK needed; Twilio's REST API is plain form-encoded HTTP.)
package twilio

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client calls the Twilio REST API.
type Client struct {
	accountSID string
	authToken  string
	from       string // our Twilio phone number
	http       *http.Client
}

// New creates a Twilio client.
func New(accountSID, authToken, from string) *Client {
	return &Client{accountSID: accountSID, authToken: authToken, from: from,
		http: &http.Client{Timeout: 15 * time.Second}}
}

// Enabled reports whether Twilio credentials are configured.
func (c *Client) Enabled() bool {
	return c != nil && c.accountSID != "" && c.authToken != "" && c.from != ""
}

// SendSMS sends a text message.
func (c *Client) SendSMS(ctx context.Context, to, body string) error {
	return c.post(ctx, "Messages.json", url.Values{"To": {to}, "From": {c.from}, "Body": {body}})
}

// Hangup ends an in-progress call.
func (c *Client) Hangup(ctx context.Context, callSID string) error {
	return c.post(ctx, "Calls/"+callSID+".json", url.Values{"Status": {"completed"}})
}

// CallWithAudio rings `to` and plays the audio file at audioURL when answered.
// Used by Screen Mode to read the call summary to the user.
func (c *Client) CallWithAudio(ctx context.Context, to, audioURL string) error {
	twiml := fmt.Sprintf(`<Response><Pause length="1"/><Play>%s</Play></Response>`, html.EscapeString(audioURL))
	return c.post(ctx, "Calls.json", url.Values{"To": {to}, "From": {c.from}, "Twiml": {twiml}})
}

func (c *Client) post(ctx context.Context, path string, form url.Values) error {
	if !c.Enabled() {
		return fmt.Errorf("twilio is not configured")
	}
	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/%s", c.accountSID, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.accountSID, c.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("twilio %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio %s returned %d: %.300s", path, resp.StatusCode, body)
	}
	return nil
}
