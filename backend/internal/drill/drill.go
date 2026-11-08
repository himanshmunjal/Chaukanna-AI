// Package drill is Scam Drill: a practice call. A "scammer" (Timbre) speaks, the
// person answers out loud (Prisma), and they learn to refuse and hang up while it
// is safe to get it wrong. Advice alone is forgotten under pressure; rehearsal is not.
//
// All logic is here and needs no network, so it can be tested on its own.
package drill

import (
	"regexp"
	"strings"
)

// Step is one thing the scammer says, and what to learn from it.
type Step struct {
	Line   string `json:"line"`
	Lesson string `json:"lesson"`
}

// Scenario is a practice call in one language.
type Scenario struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Lang  string `json:"language"`
	Steps []Step `json:"-"`
}

// Scenarios are the practice calls. All of them are fictional.
var Scenarios = []Scenario{
	{ID: "arrest-hi", Title: "डिजिटल अरेस्ट", Lang: "hi", Steps: []Step{
		{"नमस्ते, मैं मुंबई क्राइम ब्रांच से इंस्पेक्टर शर्मा बोल रहा हूँ। आपके आधार पर एक केस दर्ज हुआ है।", "पुलिस ऐसे फ़ोन पर केस नहीं बताती। नाम और पद सुनकर डरिए मत।"},
		{"आपके नाम पर अरेस्ट वारंट है। वीडियो कॉल पर रहिए और किसी को मत बताइए, वरना अभी गिरफ़्तार होंगे।", "डिजिटल अरेस्ट नाम की कोई चीज़ नहीं होती। 'किसी को मत बताना' सबसे बड़ा संकेत है।"},
		{"केस से नाम हटाने के लिए, जाँच पूरी होने तक अपने पैसे इस सुरक्षित खाते में ट्रांसफ़र कर दीजिए।", "कोई सरकारी विभाग पैसे 'सुरक्षित खाते' में नहीं मँगवाता। फ़ोन काटिए और 1930 पर कॉल कीजिए।"},
	}},
	{ID: "kyc-hi", Title: "बैंक KYC और OTP", Lang: "hi", Steps: []Step{
		{"नमस्ते, मैं आपके बैंक से बोल रहा हूँ। आपका KYC ख़त्म हो गया है, आज रात खाता बंद हो जाएगा।", "जल्दी का दबाव एक चाल है। बैंक से ख़ुद, कार्ड के पीछे लिखे नंबर पर बात कीजिए।"},
		{"घबराइए मत, घर बैठे हो जाएगा। बस अपना कार्ड नंबर बता दीजिए।", "असली बैंक फ़ोन पर कार्ड नंबर नहीं पूछता।"},
		{"अभी आपके फ़ोन पर एक OTP आया होगा, वह जल्दी से बता दीजिए।", "OTP किसी को कभी मत बताइए। जिसे OTP मिला, उसे आपका पैसा मिला।"},
	}},
	{ID: "relative-hi", Title: "रिश्तेदार मुसीबत में", Lang: "hi", Steps: []Step{
		{"हैलो, पहचाना? मैं बोल रहा हूँ। मेरा एक्सीडेंट हो गया है, मैं अस्पताल में हूँ।", "आवाज़ नक़ली हो सकती है। उनसे ऐसी बात पूछिए जो सिर्फ़ वही जानते हों।"},
		{"मेरा फ़ोन टूट गया है, यह किसी और का नंबर है। घर पर किसी को मत बताना, सब परेशान होंगे।", "नया नंबर और 'किसी को मत बताना' दोनों एक साथ, यह बड़ा संकेत है।"},
		{"डॉक्टर अभी बीस हज़ार माँग रहे हैं। जल्दी इस नंबर पर भेज दो, मैं बाद में लौटा दूँगा।", "फ़ोन रखकर उनके पुराने नंबर पर ख़ुद कॉल कीजिए। सच्चा रिश्तेदार एक मिनट इंतज़ार कर सकता है।"},
	}},
	{ID: "arrest-en", Title: "Digital arrest", Lang: "en", Steps: []Step{
		{"This is Inspector Sharma from Mumbai Crime Branch. A case has been registered against your Aadhaar number.", "Police don't announce cases like this over the phone. A title is not proof."},
		{"There is an arrest warrant in your name. Stay on this video call and do not tell anyone, or you will be arrested now.", "Digital arrest does not exist. 'Tell nobody' is the biggest red flag of all."},
		{"To clear your name, transfer your savings to this safe account until the investigation is over.", "No government agency asks you to move money to a 'safe account'. Hang up and call 1930."},
	}},
	{ID: "kyc-en", Title: "Bank KYC and OTP", Lang: "en", Steps: []Step{
		{"Hello, I am calling from your bank. Your KYC has expired and your account will be blocked tonight.", "Rushing you is the trick. Call the bank yourself on the number on your card."},
		{"Don't worry, we can do it right now. Just tell me your card number.", "A real bank never asks for your card number on a call."},
		{"You will have received an OTP on your phone. Please read it out quickly.", "Never tell anyone an OTP. Whoever has the OTP has your money."},
	}},
}

// Find returns a scenario by id.
func Find(id string) (Scenario, bool) {
	for _, s := range Scenarios {
		if s.ID == id {
			return s, true
		}
	}
	return Scenario{}, false
}

// Verdict is how the person's reply went.
type Verdict string

const (
	Safe    Verdict = "safe"    // refused, questioned or wants to check first
	Risky   Verdict = "risky"   // gave information or agreed to do it
	Unclear Verdict = "unclear" // could not tell
)

var (
	digitRun = regexp.MustCompile(`[0-9०-९]{4,}`)
	// Words for digits, spoken in a row ("four five one two").
	numberWord = `(?:zero|one|two|three|four|five|six|seven|eight|nine|ek|do|teen|char|paanch|chhah|saat|aath|nau|शून्य|एक|दो|तीन|चार|पांच|पाँच|छह|सात|आठ|नौ)`
	wordDigits = regexp.MustCompile(numberWord + `(?:\s+` + numberWord + `){3,}`)

	safeWords = []string{
		"nahi", "nahin", "नहीं", "no", "not", "don't", "wont", "won't", "never", "mat", "मत", "na",
		"hang up", "cut the call", "kaat", "काट", "rakh", "रख", "band kar", "बंद कर",
		"1930", "police station", "complaint", "शिकायत", "branch", "ब्रांच", "bank jaunga", "बैंक जा",
		"verify", "check", "जाँच", "जांच", "who are you", "kaun", "कौन", "fraud", "scam", "धोखा", "फ्रॉड",
		"family", "beta", "बेटा", "बेटी", "son", "daughter", "pehle", "पहले पूछ",
	}
	riskyWords = []string{
		"yes", "okay", "ok", "sure", "haan", "हाँ", "हां", "ji haan", "जी हाँ", "theek", "ठीक", "achha", "अच्छा",
		"bhej", "भेज", "transfer", "ट्रांसफर", "send", "sending", "bata", "बता रहा", "बता रही", "बताता", "बताती", "kar deta", "कर देता", "कर देती",
		"my otp", "my card", "mera otp", "मेरा ओटीपी", "मेरा कार्ड", "मेरा otp", "install", "इंस्टॉल",
	}
)

// Judge decides how a spoken reply went. Handing over digits is always risky, even
// if polite. A refusal beats agreement words ("ok, but no, I won't share it").
func Judge(reply string) Verdict {
	text := " " + strings.ToLower(strings.NewReplacer(",", " ", ".", " ", "?", " ", "!", " ", "।", " ").Replace(reply)) + " "
	if strings.TrimSpace(text) == "" {
		return Unclear
	}
	// 1930 is the helpline: saying it is the right answer, not handing over a code.
	if digitRun.MatchString(strings.ReplaceAll(strings.ReplaceAll(text, "1930", ""), " ", "")) || wordDigits.MatchString(text) {
		return Risky
	}
	if hasWord(text, safeWords) {
		return Safe
	}
	if hasWord(text, riskyWords) {
		return Risky
	}
	return Unclear
}

func hasWord(text string, words []string) bool {
	for _, w := range words {
		if strings.Contains(text, " "+w+" ") {
			return true
		}
		// Devanagari verbs take many endings (काटता, काट दूँगा): match the stem.
		if w[0] >= 0x80 && strings.Contains(text, " "+w) {
			return true
		}
	}
	return false
}
