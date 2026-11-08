package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"chaukanna/internal/config"
)

func testContext(method, target string, headers map[string]string, remote string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(method, target, nil)
	c.Request.RemoteAddr = remote
	for k, v := range headers {
		c.Request.Header.Set(k, v)
	}
	return c
}

func TestPathIDRejectsSQL(t *testing.T) {
	for _, raw := range []string{"1) OR (1=1", "0 OR 1=1", "1;drop table calls", "", "0", "-1", "abc"} {
		c := testContext("GET", "/", nil, "127.0.0.1:1")
		c.Params = gin.Params{{Key: "id", Value: raw}}
		if _, ok := pathID(c); ok {
			t.Errorf("pathID(%q) accepted", raw)
		}
	}
	c := testContext("GET", "/", nil, "127.0.0.1:1")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	if id, ok := pathID(c); !ok || id != 42 {
		t.Errorf("pathID(42) = %d, %v", id, ok)
	}
}

func TestOpenModeOnlyForDirectLocalRequests(t *testing.T) {
	s := &Server{cfg: config.Config{}} // no password, no token, no demo
	cases := []struct {
		remote  string
		headers map[string]string
		want    bool
	}{
		{"127.0.0.1:5000", nil, true},
		{"[::1]:5000", nil, true},
		{"127.0.0.1:5000", map[string]string{"X-Forwarded-For": "203.0.113.9"}, false}, // ngrok
		{"172.17.0.1:5000", nil, false}, // Docker bridge
		{"203.0.113.9:5000", nil, false},
	}
	for _, tc := range cases {
		if got := s.isFamily(testContext("GET", "/api/calls", tc.headers, tc.remote)); got != tc.want {
			t.Errorf("isFamily(%s, %v) = %v, want %v", tc.remote, tc.headers, got, tc.want)
		}
	}
}

func TestAllowedOriginBlocksCrossSiteWrites(t *testing.T) {
	s := &Server{cfg: config.Config{CORSOrigins: []string{"http://localhost:5173"}}}
	cases := []struct {
		method, origin string
		want           bool
	}{
		{"GET", "https://evil.example", true}, // reads are left to CORS
		{"POST", "https://evil.example", false},
		{"DELETE", "https://evil.example", false},
		{"POST", "http://localhost:5173", true},
		{"POST", "http://chaukanna.test", true}, // same host as the request
		{"POST", "", true},                      // scripts and curl
	}
	for _, tc := range cases {
		h := map[string]string{}
		if tc.origin != "" {
			h["Origin"] = tc.origin
		}
		c := testContext(tc.method, "http://chaukanna.test/api/calls/1/warn", h, "203.0.113.9:1")
		if got := s.allowedOrigin(c); got != tc.want {
			t.Errorf("%s from %q = %v, want %v", tc.method, tc.origin, got, tc.want)
		}
	}
	ws := testContext("GET", "http://chaukanna.test/ws/dashboard", map[string]string{"Upgrade": "websocket", "Origin": "https://evil.example"}, "203.0.113.9:1")
	if s.allowedOrigin(ws) {
		t.Error("cross-site WebSocket allowed")
	}
}

func TestRedactedLogHidesSecrets(t *testing.T) {
	line := redactedLog(gin.LogFormatterParams{
		Request: &http.Request{}, TimeStamp: time.Now(), StatusCode: 101,
		Method: "GET", Path: "/ws/exotel/supersecret123?token=abc&mode=x",
	})
	if strings.Contains(line, "supersecret123") || strings.Contains(line, "abc") {
		t.Errorf("secret in log line: %s", line)
	}
}
