// Command smoketest exercises every feature of a RUNNING backend and prints a
// pass/fail report. Run it after any change, and again once Evon is live.
//
//	cd backend
//	go run ./cmd/server            # in another terminal
//	go run ./cmd/gnanicheck        # creates gnanicheck.wav (a spoken scam sentence)
//	go run ./cmd/smoketest
//
// What it checks:
//
//	REST API        /health, people (create/list/delete), scenarios, stats
//	Simulator       every scenario in Guardian Mode (scam -> warned, legit -> not)
//	Screen Mode     the bot greets, asks, hangs up and writes a summary
//	Twilio          webhook routing (guardian vs screen) + media stream with real audio
//	Exotel          media stream with real audio
//	Speakerphone    browser-style microphone stream with real audio
//	Scam numbers    a number caught scamming is blocked on its next call
//	Dashboard       live WebSocket events arrive
//
// The three audio-stream tests use real Prisma + Timbre (a few seconds of credits).
// Skip them with -skip-streams.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"chaukanna/internal/audio"
)

var (
	base        = flag.String("base", "http://localhost:8080", "backend URL")
	wavPath     = flag.String("wav", "gnanicheck.wav", "spoken scam sentence used for stream tests")
	speed       = flag.Float64("speed", 8, "simulator speed-up")
	skipStreams = flag.Bool("skip-streams", false, "skip the Twilio/Exotel/Speakerphone audio tests")
)

// ---------- tiny test framework ----------

type outcome struct {
	name, detail string
	ok           bool
}

var (
	mu      sync.Mutex
	results []outcome
)

func check(name string, ok bool, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	results = append(results, outcome{name, fmt.Sprintf(format, args...), ok})
	mark := "✅"
	if !ok {
		mark = "❌"
	}
	fmt.Printf("%s %-46s %s\n", mark, name, fmt.Sprintf(format, args...))
}

// ---------- HTTP helpers ----------

func getJSON(path string, out any) error {
	resp, err := http.Get(*base + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func postJSON(path string, body, out any) (int, error) {
	data, _ := json.Marshal(body)
	resp, err := http.Post(*base+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

func wsURL(path string) string {
	return strings.Replace(*base, "http", "ws", 1) + path
}

type call struct {
	ID               uint     `json:"id"`
	Provider         string   `json:"provider"`
	ProviderCallID   string   `json:"provider_call_id"`
	Mode             string   `json:"mode"`
	State            string   `json:"state"`
	Risk             float64  `json:"risk"`
	Reason           string   `json:"reason"`
	Summary          string   `json:"summary"`
	EndedAt          *string  `json:"ended_at"`
	DetectedAfterSec *float64 `json:"detected_after_sec"`
	Transcript       []struct {
		Speaker string `json:"speaker"`
		Text    string `json:"text"`
	} `json:"transcript"`
	Flags []struct {
		Type   string `json:"type"`
		Source string `json:"source"`
	} `json:"flags"`
	Alerts []struct {
		Channel string `json:"channel"`
		Status  string `json:"status"`
	} `json:"alerts"`
}

// waitForEnd polls a call until it has ended (or the timeout passes).
func waitForEnd(id uint, timeout time.Duration) (call, bool) {
	deadline := time.Now().Add(timeout)
	var c call
	for time.Now().Before(deadline) {
		if getJSON(fmt.Sprintf("/api/calls/%d", id), &c) == nil && c.EndedAt != nil {
			time.Sleep(1500 * time.Millisecond) // let alerts / summary finish
			_ = getJSON(fmt.Sprintf("/api/calls/%d", id), &c)
			return c, true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return c, false
}

// findCall returns the most recent call with this provider call id.
func findCall(providerCallID string) (call, bool) {
	var calls []call
	if getJSON("/api/calls?limit=50", &calls) != nil {
		return call{}, false
	}
	for _, c := range calls {
		if c.ProviderCallID == providerCallID {
			return c, true
		}
	}
	return call{}, false
}

// ---------- main ----------

func main() {
	flag.Parse()
	// The Twilio and Exotel stream tests need the server started with TELEPHONY_TEST_MODE=true.
	fmt.Printf("Smoke-testing %s\n\n", *base)

	// Health
	var health struct {
		OK           bool            `json:"ok"`
		Integrations map[string]bool `json:"integrations"`
	}
	if err := getJSON("/health", &health); err != nil {
		fmt.Printf("❌ backend not reachable at %s: %v\n", *base, err)
		os.Exit(1)
	}
	check("health", health.OK, "integrations: %v", health.Integrations)

	events := watchDashboard()
	userPhone := testPeople()
	testSimulatorGuardian()
	testSimulatorScreen()
	testScamNumbers()
	testTwilioWebhook(userPhone)

	if *skipStreams {
		fmt.Println("(skipping audio stream tests)")
	} else if samples, err := loadWAV(); err != nil {
		check("audio streams", false, "%v (run `go run ./cmd/gnanicheck` first)", err)
	} else {
		testTwilioStream(userPhone, samples)
		testExotelStream(userPhone, samples)
		testSpeakerphone(userPhone, samples)
	}

	// Stats
	var stats map[string]any
	err := getJSON("/api/stats", &stats)
	check("GET /api/stats", err == nil && stats["total_calls"] != nil, "%v", stats)

	// Dashboard events
	counts := events()
	needed := []string{"call_started", "transcript", "flags", "analysis", "warning", "call_ended"}
	missing := []string{}
	for _, t := range needed {
		if counts[t] == 0 {
			missing = append(missing, t)
		}
	}
	check("dashboard WebSocket events", len(missing) == 0, "received %v missing %v", counts, missing)

	// Summary
	failed := 0
	for _, r := range results {
		if !r.ok {
			failed++
		}
	}
	fmt.Printf("\n%d checks, %d failed\n", len(results), failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// watchDashboard counts live events until the returned function is called.
func watchDashboard() func() map[string]int {
	counts := map[string]int{}
	var cmu sync.Mutex
	conn, _, err := websocket.DefaultDialer.Dial(wsURL("/ws/dashboard"), nil)
	if err != nil {
		check("dashboard WebSocket connect", false, "%v", err)
		return func() map[string]int { return counts }
	}
	go func() {
		for {
			var ev struct{ Type string }
			if err := conn.ReadJSON(&ev); err != nil {
				return
			}
			cmu.Lock()
			counts[ev.Type]++
			cmu.Unlock()
		}
	}()
	return func() map[string]int {
		conn.Close()
		cmu.Lock()
		defer cmu.Unlock()
		return counts
	}
}

// testPeople checks create/list/delete and returns a protected user's phone.
func testPeople() string {
	var created struct {
		ID    uint   `json:"id"`
		Phone string `json:"phone"`
	}
	status, err := postJSON("/api/users", map[string]string{
		"name": "Smoke Test", "phone": "98765 00000", "language": "hi", "family_phone": "98765 00001",
	}, &created)
	check("POST /api/users (phone normalised)", err == nil && status == 201 && created.Phone == "+919876500000",
		"status %d phone %q", status, created.Phone)

	var users []struct {
		ID    uint   `json:"id"`
		Phone string `json:"phone"`
	}
	_ = getJSON("/api/users", &users)
	check("GET /api/users", len(users) > 0, "%d people", len(users))

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/users/%d", *base, created.ID), nil)
	resp, err := http.DefaultClient.Do(req)
	check("DELETE /api/users/:id", err == nil && resp.StatusCode == 204, "")

	_ = getJSON("/api/users", &users)
	if len(users) == 0 {
		return ""
	}
	return users[0].Phone // the default protected user from .env
}

// testSimulatorGuardian replays every scenario at once and checks the verdicts.
func testSimulatorGuardian() {
	var scenarios []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	err := getJSON("/api/scenarios", &scenarios)
	check("GET /api/scenarios", err == nil && len(scenarios) > 0, "%d scenarios", len(scenarios))

	var wg sync.WaitGroup
	for _, sc := range scenarios {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var res struct {
				CallID uint `json:"call_id"`
			}
			if _, err := postJSON("/api/simulate", map[string]any{"scenario_id": sc.ID, "speed": *speed}, &res); err != nil || res.CallID == 0 {
				check("simulate "+sc.ID, false, "could not start: %v", err)
				return
			}
			c, ended := waitForEnd(res.CallID, 90*time.Second)
			warned := c.State == "warned"
			want := sc.Label == "scam"
			detail := fmt.Sprintf("state=%s risk=%.2f", c.State, c.Risk)
			if warned {
				evonFlags := 0
				for _, f := range c.Flags {
					if f.Source == "evon" {
						evonFlags++
					}
				}
				detail += fmt.Sprintf(" flags=%d (evon %d) alerts=%d", len(c.Flags), evonFlags, len(c.Alerts))
			}
			check("guardian: "+sc.ID, ended && warned == want, "%s", detail)
		}()
	}
	wg.Wait()
}

// testSimulatorScreen checks the screening bot on a scam and a legit call.
func testSimulatorScreen() {
	for _, id := range []string{"scam-01-digital-arrest-hinglish", "legit-02-delivery-hinglish"} {
		var res struct {
			CallID uint `json:"call_id"`
		}
		_, _ = postJSON("/api/simulate", map[string]any{"scenario_id": id, "mode": "screen", "speed": 2}, &res)
		c, ended := waitForEnd(res.CallID, 120*time.Second)
		bot, caller := 0, 0
		for _, l := range c.Transcript {
			if l.Speaker == "chaukanna" {
				bot++
			} else {
				caller++
			}
		}
		riskOK := c.Risk >= 0.4 // a scam must look at least suspicious...
		if strings.HasPrefix(id, "legit") {
			riskOK = c.Risk < 0.4 // ...and a normal call must not
		}
		check("screen: "+id, ended && c.Mode == "screen" && bot >= 2 && c.Summary != "" && riskOK,
			"bot lines=%d caller lines=%d risk=%.2f summary=%q", bot, caller, c.Risk, shorten(c.Summary, 70))
	}
}

// testTwilioWebhook checks that the protected user gets Guardian and strangers get Screen.
func testTwilioWebhook(userPhone string) {
	post := func(from string) string {
		resp, err := http.PostForm(*base+"/telephony/twilio/voice", url.Values{"From": {from}, "CallSid": {"CAtest"}})
		if err != nil {
			return err.Error()
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if userPhone != "" {
		twiml := post(userPhone)
		check("twilio webhook: protected user -> guardian", strings.Contains(twiml, `value="guardian"`) && strings.Contains(twiml, "<Stream"), "")
	}
	twiml := post("+15550001111")
	check("twilio webhook: stranger -> screen", strings.Contains(twiml, `value="screen"`), "")
}

// ---------- audio stream tests (real Prisma + Timbre) ----------

func loadWAV() ([]int16, error) {
	data, err := os.ReadFile(*wavPath)
	if err != nil {
		return nil, err
	}
	samples, rate, err := audio.ParseWAV(data)
	if err != nil {
		return nil, err
	}
	samples = audio.Resample(samples, rate, audio.PhoneRate)
	return append(samples, make([]int16, audio.PhoneRate*2)...), nil // + 2 s silence
}

// streamTest is shared by the three providers: send audio in real time,
// count audio coming back, then check the saved call.
func streamTest(name, providerCallID string, conn *websocket.Conn, samples []int16, frame int,
	encode func([]int16) any, isAudioBack func(map[string]any) bool, stop any) {

	back := 0
	var callID uint
	var bmu sync.Mutex
	go func() {
		for {
			var m map[string]any
			if err := conn.ReadJSON(&m); err != nil {
				return
			}
			bmu.Lock()
			if isAudioBack(m) {
				back++
			}
			if id, ok := m["call_id"].(float64); ok {
				callID = uint(id)
			}
			bmu.Unlock()
		}
	}()

	frameDur := time.Duration(frame) * time.Second / audio.PhoneRate
	for i := 0; i < len(samples); i += frame {
		if err := conn.WriteMessage(websocket.TextMessage, mustJSON(encode(samples[i:min(i+frame, len(samples))]))); err != nil {
			break
		}
		time.Sleep(frameDur)
	}
	time.Sleep(5 * time.Second) // Prisma + detector + Timbre
	if stop != nil {
		_ = conn.WriteMessage(websocket.TextMessage, mustJSON(stop))
	}
	conn.Close()

	var c call
	var ok bool
	if providerCallID != "" {
		time.Sleep(time.Second)
		c, ok = findCall(providerCallID)
		if ok {
			c, ok = waitForEnd(c.ID, 15*time.Second)
		}
	} else {
		bmu.Lock()
		id := callID
		bmu.Unlock()
		c, ok = waitForEnd(id, 15*time.Second)
	}
	bmu.Lock()
	defer bmu.Unlock()
	heard := ""
	if len(c.Transcript) > 0 {
		heard = c.Transcript[0].Text
	}
	check(name, ok && c.State == "warned" && back > 0,
		"state=%s audio-back=%d heard=%q", c.State, back, shorten(heard, 60))
}

func testTwilioStream(userPhone string, samples []int16) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL("/ws/twilio"), nil)
	if err != nil {
		check("twilio media stream", false, "%v", err)
		return
	}
	sid := fmt.Sprintf("CAsmoke%d", time.Now().Unix())
	_ = conn.WriteJSON(map[string]any{"event": "connected"})
	_ = conn.WriteJSON(map[string]any{"event": "start", "start": map[string]any{
		"streamSid": "MZsmoke", "callSid": sid,
		"customParameters": map[string]string{"mode": "guardian", "user_phone": userPhone},
	}})
	streamTest("twilio media stream (μ-law, real Prisma)", sid, conn, samples, 160,
		func(s []int16) any {
			return map[string]any{"event": "media", "media": map[string]string{
				"payload": base64.StdEncoding.EncodeToString(audio.MulawEncode(s))}}
		},
		func(m map[string]any) bool { return m["event"] == "media" },
		map[string]any{"event": "stop"})
}

func testExotelStream(userPhone string, samples []int16) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL("/ws/exotel"), nil)
	if err != nil {
		check("exotel stream", false, "%v", err)
		return
	}
	sid := fmt.Sprintf("EXsmoke%d", time.Now().Unix())
	_ = conn.WriteJSON(map[string]any{"event": "connected"})
	_ = conn.WriteJSON(map[string]any{"event": "start", "stream_sid": "s1", "start": map[string]any{
		"stream_sid": "s1", "call_sid": sid, "from": userPhone, "to": "08000000000",
	}})
	streamTest("exotel stream (PCM, real Prisma)", sid, conn, samples, 320,
		func(s []int16) any {
			return map[string]any{"event": "media", "stream_sid": "s1", "media": map[string]string{
				"payload": base64.StdEncoding.EncodeToString(audio.SamplesToBytes(s))}}
		},
		func(m map[string]any) bool { return m["event"] == "media" },
		map[string]any{"event": "stop"})
}

func testSpeakerphone(userPhone string, samples []int16) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL("/ws/speaker?mode=guardian&user_phone="+url.QueryEscape(userPhone)), nil)
	if err != nil {
		check("speakerphone stream", false, "%v", err)
		return
	}
	// Speakerphone sends raw binary PCM, not JSON, so it gets its own loop.
	back := 0
	var callID uint
	var bmu sync.Mutex
	go func() {
		for {
			var m map[string]any
			if err := conn.ReadJSON(&m); err != nil {
				return
			}
			bmu.Lock()
			if m["type"] == "audio" {
				back++
			}
			if id, ok := m["call_id"].(float64); ok {
				callID = uint(id)
			}
			bmu.Unlock()
		}
	}()
	for i := 0; i < len(samples); i += 800 {
		_ = conn.WriteMessage(websocket.BinaryMessage, audio.SamplesToBytes(samples[i:min(i+800, len(samples))]))
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(5 * time.Second)
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"stop"}`))
	conn.Close()

	bmu.Lock()
	id, audioBack := callID, back
	bmu.Unlock()
	c, ok := waitForEnd(id, 15*time.Second)
	check("speakerphone stream (browser mic, real Prisma)", ok && c.State == "warned" && audioBack > 0,
		"state=%s audio-back=%d", c.State, audioBack)
}

// ---------- utils ----------

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// testScamNumbers: a scam from a number is remembered; the next call from it is blocked.
func testScamNumbers() {
	number := fmt.Sprintf("+9190%08d", time.Now().Unix()%100000000) // unique per run
	run := func(scenario, mode string) call {
		var res struct {
			CallID uint `json:"call_id"`
		}
		_, _ = postJSON("/api/simulate", map[string]any{"scenario_id": scenario, "mode": mode, "speed": 4, "caller_number": number}, &res)
		c, _ := waitForEnd(res.CallID, 90*time.Second)
		return c
	}

	first := run("scam-09-sbi-otp-english", "guardian")
	var numbers []struct {
		ID    uint   `json:"id"`
		Phone string `json:"phone"`
	}
	_ = getJSON("/api/scam-numbers", &numbers)
	var id uint
	for _, n := range numbers {
		if n.Phone == number {
			id = n.ID
		}
	}
	check("scam number remembered after a scam", first.State == "warned" && id != 0, "number %s", number)

	second := run("legit-02-delivery-hinglish", "screen")
	blocked := false
	for _, l := range second.Transcript {
		blocked = blocked || (l.Speaker == "chaukanna" && strings.Contains(l.Text, "रिपोर्ट"))
	}
	check("known scam number blocked on next call", blocked && second.Risk >= 0.99, "state=%s risk=%.2f", second.State, second.Risk)

	if id != 0 { // clean up
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/scam-numbers/%d", *base, id), nil)
		_, _ = http.DefaultClient.Do(req)
	}
}
