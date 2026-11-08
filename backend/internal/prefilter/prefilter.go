// Package prefilter is the fast first layer of detection.
//
// It looks for known scam phrases in every transcript line (Hindi in Devanagari,
// romanised Hinglish, English and a few regional words). It runs in microseconds,
// so it can check every sentence. When it finds something, the session asks
// Evon (the slower, smarter layer) to look at the conversation right away.
//
// To add a phrase: add it to the list for the right flag type below.
// Matching is case-insensitive and on whole words, so "won" does not match "wonderful".
package prefilter

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"chaukanna/internal/scam"
)

// Hit is one phrase found in a line.
type Hit struct {
	Type   scam.FlagType
	Phrase string
	// Strong means a direct demand that no genuine caller makes ("tell me the OTP",
	// "install AnyDesk", "scan this QR"). One is enough to call the call a scam.
	Strong bool
}

// weights says how strongly each flag type points to a scam (used by RiskFromFlags).
var weights = map[scam.FlagType]float64{
	scam.OTPRequest:   0.50,
	scam.RemoteAccess: 0.50,
	scam.MoneyMove:    0.45,
	scam.LinkOrQR:     0.45,
	scam.Isolation:    0.35,
	scam.Fear:         0.30,
	scam.Authority:    0.25,
	scam.TooGood:      0.25,
	scam.Secrecy:      0.20,
	scam.PersonalData: 0.20,
	scam.Urgency:      0.15,
}

// phrases per flag type. Keep them lowercase.
var phrases = map[scam.FlagType][]string{
	scam.Authority: {
		"department se", "electricity board", "verification team", "verification office", "hr department", "irdai", "sebi", "nhai",
		"government", "sarkari", "ministry", "gas agency", "lic", "airtel", "vodafone", "microsoft",
		"support team", "army officer", "विभाग", "डिपार्टमेंट", "सरकारी", "मंत्रालय", "एचआर", "एलआईसी",
		"పోలీస్", "ಪೊಲೀಸ್", "പോലീസ്", "પોલીસ", "ਪੁਲਿਸ",
		"cbi", "police", "crime branch", "cyber cell", "cyber crime", "customs", "narcotics", "ncb",
		// Bank names: a bank claim alone is weak (real banks call too), but a
		// "bank" asking for an OTP/app/transfer is always a scam.
		"sbi", "state bank", "hdfc", "icici", "axis bank", "kotak", "pnb", "punjab national bank",
		"bank of baroda", "canara bank", "union bank", "bank se", "from the bank", "from bank",
		"bank manager", "bank official", "bank officer", "बैंक से", "बैंक मैनेजर",
		"பேங்க்", "வங்கி", "পুলিশ", "ব্যাংক",
		// Tech-support / service impersonation ("phone repair wala", "customer care").
		"phone repair", "mobile repair", "customer care", "customer service", "tech support",
		"technical support", "service center", "service centre", "कस्टमर केयर",
		"trai", "rbi", "reserve bank", "income tax", "enforcement directorate", "ed officer",
		"fedex", "dhl", "courier", "supreme court", "high court", "judge", "inspector", "officer",
		"bijli vibhag", "electricity office", "electricity department", "बिजली विभाग", "बिजली दफ्तर",
		"thane se", "थाने से", "stock exchange", "nse", "bse", "सीबीआई", "पुलिस", "कस्टम", "क्राइम ब्रांच", "साइबर सेल", "इंस्पेक्टर", "कूरियर", "आरबीआई",
		"போலீஸ்", "சிபிஐ", "পুলিশ", "पोलीस", "পুলিশ থেকে", "காவல்துறை", "सीमाशुल्क",
	},
	scam.Fear: {
		"blocked", "block ho", "suspended", "freeze", "terminate", "disconnected", "cut ho jayega", "supply band",
		"band ho jayega", "band ho sakta", "penalty", "impounded", "seized", "lock your", "delete hone", "copyright violation",
		"virus", "trojan", "suspicious transaction", "suspicious activity", "unauthorized", "accident", "locked me up", "ब्लॉक",
		"बंद हो जाएगा", "फ्रीज", "जुर्माना", "पेनल्टी", "एक्सीडेंट", "penalty लगेगी", "అరెస్ట్", "ಬಂಧನ",
		"അറസ്റ്റ്", "ધરપકડ", "ਗ੍ਰਿਫਤਾਰ", "ಅರೆಸ್ಟ್",
		"arrest", "arrest warrant", "warrant", "fir", "case registered", "case darj", "drugs",
		"money laundering", "jail", "giraftaar", "giraftar", "illegal", "sim band", "sim block",
		"account freeze", "account block", "account band", "legal action", "court case",
		"custody", "connection kat", "kat jayega", "disconnected tonight", "kyc expire", "kyc update",
		"connection cut", "कनेक्शन कट", "कट जाएगा", "बिजली कट",
		"गिरफ्तार", "गिरफ़्तार", "वारंट", "ड्रग्स", "जेल", "मनी लॉन्ड्रिंग", "केस दर्ज",
		"केवाईसी", "अकाउंट ब्लॉक", "अकाउंट बंद",
		// English words as Prisma writes them in Tamil script.
		"கேஒய்சி", "கே ஒய் சி", "கே வை சி", "அக்கவுண்ட் பிளாக்", "பிளாக் ஆகிடும்", "எக்ஸ்பையர்",
		"கைது", "গ্রেফতার", "গ্রেফতারি", "পরোয়ানা", "अटक", "वॉरंट", "முடக்கப்படும்", "முடக்கப்படும", "தடை",
	},
	scam.Urgency: {
		"urgent", "urgently", "quickly", "tonight", "today itself", "aaj raat", "jaldi", "in 1 hour",
		"within 2 hours", "2 ghante", "1 ghante", "expiring", "midnight", "आज रात", "घंटे में", "अर्जेंट",
		"turant", "immediately", "right now", "within 30 minutes", "within one hour", "jaldi karo",
		"last warning", "aaj hi", "abhi ke abhi", "time nahi hai", "तुरंत", "अभी के अभी", "जल्दी",
	},
	scam.Isolation: {
		"video surveillance", "press 1", "press 9", "press one", "press nine", "निगरानी",
		"video call", "skype", "stay on the call", "do not disconnect", "don't disconnect",
		"phone mat katna", "call mat kaatna", "call cut mat", "camera on", "room band",
		"digital arrest", "kisi ko mat batana", "don't tell anyone", "do not tell anyone",
		"वीडियो कॉल", "फोन मत काटना", "किसी को मत बताना", "डिजिटल अरेस्ट",
	},
	scam.Secrecy: {
		"batana mat", "do not inform", "don't inform", "don't call mom", "don't call dad", "kisi ko call nahi", "kisi aur ko mat", "किसी को call मत",
		"किसी को मत", "बताना मत",
		"confidential", "secret", "gopniya", "national secret", "गोपनीय", "राज़",
	},
	scam.OTPRequest: {
		"password", "digit code", "digit otp", "login verification code", "read back the code", "card details", "expiry date", "पासवर्ड",
		"కోడ్ చెప్పండి", "ఓటీపీ", "ಒಟಿಪಿ", "ഒടിപി", "ઓટીપી", "ਓਟੀਪੀ", "কোড",
		"otp", "one time password", "verification code", "code bataiye", "code batao",
		"upi pin", "atm pin", "cvv", "pin bataiye", "ओटीपी", "पिन बताइए", "ஓடிபி", "ওটিপি", "ओ टी पी",
	},
	scam.RemoteAccess: {
		"remote support", "remotely", "screen sharing", "रिमोट",
		"anydesk", "any desk", "teamviewer", "team viewer", "quicksupport", "quick support",
		"rustdesk", "screen share", "share screen", "apk", "app download", "एनीडेस्क",
		"स्क्रीन शेयर",
	},
	scam.MoneyMove: {
		"registration fee", "verification fee", "replacement fee", "file charges", "insurance charge", "settlement amount", "pay right now", "pay immediately",
		"send it back", "wallet address", "जमा करना", "भेज दो", "డబ్బు పంపండి", "ಹಣ ಕಳುಹಿಸಿ", "പണം അയക്കൂ", "টাকা পাঠান",
		"पैसे पाठवा", "પૈસા મોકલો", "ਪੈਸੇ ਭੇਜੋ", "பணம் அனுப்பு",
		"safe account", "verification account", "rbi account", "secure account",
		"transfer kar", "transfer kijiye", "paise transfer", "amount transfer", "send money",
		"gift card", "security deposit", "refundable", "processing fee", "fine bharna",
		"paise bhejo", "paise bhej", "payment kijiye", "pay now",
		"पैसे ट्रांसफर", "ट्रांसफर कीजिए", "जमा कीजिए", "सेफ अकाउंट",
	},
	scam.PersonalData: {
		"account details", "16 digit", "net banking",
		"aadhaar", "aadhar", "pan card", "card number", "bank details", "account number",
		"date of birth", "आधार", "पैन कार्ड", "खाता नंबर", "ஆதார்", "আধার",
	},
	scam.LinkOrQR: {
		"qr code", "qr scan", "scan the qr", "click on the link", "link pe click", "link par click", "link bhej", "link bheja",
		"upi link", "telegram link", "verification link", "payment link", "on this link", "link sent", "is link", "usse payment",
		"लिंक", "क्यूआर", "qr कोड", "స్కాన్", "ಲಿಂಕ್", "ലിങ്ക്", "লিংক", "લિંક", "ਲਿੰਕ", "லிங்க்",
	},
	scam.TooGood: {
		"congratulations", "congratulation", "lucky draw", "you won", "jeeta", "winner", "guaranteed", "guarantee",
		"double ho", "doubles", "profit", "zero risk", "zero loss", "selected", "shortlist", "direct selection",
		"no interview", "no experience", "pre approved", "loan approve", "approve ho gaya", "approve hua", "subsidy", "scholarship",
		"bonus", "maturity", "reward points", "refund", "benefit", "by mistake", "galti se", "lifetime free",
		"vip", "usdt", "crypto", "donation", "relief fund", "बधाई", "गारंटी", "मुनाफा",
		"सब्सिडी", "स्कॉलरशिप", "बोनस", "रिफंड", "गलती से", "लोन approve", "चुना गया",
		"lottery", "prize", "kbc", "you have won", "jackpot", "cashback", "double your money",
		"free cash", "free money", "free gift", "फ्री कैश", "मुफ्त पैसे", "part time job", "task based", "guaranteed return", "लॉटरी", "इनाम",
	},
}

// pairs catch requests whose words are split by other words, e.g.
// "screen मेरे साथ share कीजिए" or "paise turant transfer karo".
// A pair matches when one word from A and one from B are in the same segment and
// nothing from Not is. Not lists what genuine reminders say ("you can pay in the app").
type pair struct {
	Type      scam.FlagType
	A, B, Not []string
	Strong    bool
}

var otpAsks = []string{
	"batao", "bataiye", "bata", "बताओ", "बताइए", "बता", "share", "शेयर", "bhejo", "send", "chahiye", "चाहिए",
	"tell", "read", "enter", "dijiye", "दीजिए", "give", "provide", "dalo", "daal", "डालिए",
	"சொல்லுங்கள்", "বলুন", "सांगा", "చెప్పండి", "ಹೇಳಿ", "പറയൂ", "કહો", "ਦੱਸੋ",
}

var moneyNouns = []string{
	"paise", "paisa", "money", "amount", "rupaye", "rupees", "rupaya", "rs", "funds", "fund", "fee", "fees", "fine",
	"charge", "charges", "deposit", "donation", "contribution", "premium", "penalty", "settlement", "tax", "gst", "advance",
	"usdt", "hazaar", "payment", "dollars", "dollar", "bail", "jamanat", "cash",
	"पैसे", "पैसा", "रुपये", "रुपए", "हज़ार", "फीस", "चार्ज", "जमानत", "जुर्माना", "डिपॉजिट", "पेमेंट", "राशि",
}

var moneyDemands = []string{
	"transfer", "bhejo", "bhejiye", "bhejne", "bhejna", "bhej", "send", "submit", "deposit", "jama", "pay", "purchase",
	"karna hoga", "karne honge", "karna padega", "karni hogi", "karni padegi", "deni hogi", "dena hoga", "required", "must",
	"ट्रांसफर", "भेजो", "भेजिए", "भेजने", "भेज", "भेजूं", "भेजें", "भेजना", "जमा", "करना होगा", "करने होंगे", "करना पड़ेगा",
	"देनी होगी", "देना होगा", "देना पड़ेगा",
}

// genuinePayment: how real reminders talk about money. They point to your own app
// or a counter, and never demand the money on this call.
var genuinePayment = []string{
	"counter", "official portal", "official app", "official website", "regular bank app", "bank app", "app se pay", "app settings",
	"auto debit", "automatically", "cash on delivery", "kar sakte hain", "can pay", "can also pay", "pay it in", "pay through the",
	"payment method", "no payment", "koi payment", "कर सकते हैं", "ऑफिशियल", "official", "ऐप से", "काउंटर", "ऑटो डेबिट",
}

var pairs = []pair{
	{Type: scam.RemoteAccess, A: []string{"screen", "स्क्रीन"}, B: []string{"share", "शेयर", "sharing", "dikhao", "dikhaiye", "दिखाइए", "दिखाओ"}},
	{Type: scam.MoneyMove, A: moneyNouns, B: moneyDemands, Not: genuinePayment},
	{Type: scam.OTPRequest, A: []string{"otp", "ओटीपी", "code", "कोड", "pin", "पिन"}, B: otpAsks, Not: notAskingForSecrets},

	// Direct demands. Nobody genuine says these on a call.
	{Type: scam.OTPRequest, Strong: true, Not: notAskingForSecrets,
		A: []string{"otp", "ओटीपी", "ஓடிபி", "ওটিপি", "ओ टी पी", "ఓటీపీ", "ಒಟಿಪಿ", "ഒടിപി", "ઓટીપી", "ਓਟੀਪੀ", "cvv", "upi pin", "atm pin", "verification code", "digit code", "password", "पासवर्ड"},
		B: otpAsks},
	{Type: scam.LinkOrQR, Strong: true, A: []string{"qr", "क्यूआर"}, B: []string{"scan", "स्कैन"}},
	{Type: scam.LinkOrQR, Strong: true, A: []string{"request", "रिक्वेस्ट"}, B: []string{"approve", "अप्रूव"}},
	{Type: scam.OTPRequest, Strong: true, A: []string{"pin", "पिन"}, B: []string{"enter", "daal", "डालिए", "qr"}, Not: notAskingForSecrets},
	{Type: scam.MoneyMove, Strong: true, A: []string{"safe account", "rbi account", "verification account", "secure account", "सेफ अकाउंट"}, B: moneyDemands},

	// Prisma keeps English words in Latin letters inside Hindi: "link भेजा है".
	{Type: scam.LinkOrQR, A: linkWords, B: linkActions},
	// Logging in through a link the caller sent is how accounts get stolen.
	{Type: scam.LinkOrQR, Strong: true, A: linkWords, B: linkLogins, Not: notAskingForSecrets},
	// "Google Pay कर दीजिए": naming a payment app and telling you to use it.
	{Type: scam.MoneyMove, A: payApps, B: payApply, Not: genuinePayment},
	// "वापस भेज दीजिए" about money, not a parcel being returned.
	{Type: scam.MoneyMove, A: []string{"wapas", "vapas", "वापस"}, B: []string{"bhej", "भेज"}, Not: returnedGoods},
}

var (
	linkWords   = []string{"link", "लिंक"}
	linkActions = []string{"bheja", "bhej", "भेजा", "भेज", "click", "क्लिक", "khol", "खोल", "open", "ओपन", "जाकर", "jakar"}
	linkLogins  = []string{"login", "log in", "लॉगिन", "लॉग इन", "sign in", "password", "पासवर्ड", "otp", "ओटीपी"}
	payApps     = []string{"google pay", "gpay", "phonepe", "phone pe", "paytm", "bhim", "गूगल पे", "फोनपे", "फोन पे", "पेटीएम"}
	payApply    = []string{"kar dijiye", "kar do", "kijiye", "karo", "कर दीजिए", "कर दो", "कीजिए", "करो"}
	// returnedGoods: a delivery or shop can genuinely ask for an item to be sent back.
	returnedGoods = []string{"parcel", "पार्सल", "saman", "samaan", "सामान", "item", "product", "order", "ऑर्डर", "package", "courier", "कूरियर", "document", "form", "photo", "फोटो"}
)

var (
	amount      = regexp.MustCompile(` [0-9][0-9,]{2,} `)
	amountVerbs = []string{"bhejna", "bhej", "bhejo", "send", "transfer", "lagao", "invest", "de dijiye", "de do", "भेजना", "भेज", "लगाओ", "लगाइए", "ट्रांसफर", "दे दीजिए", "दे दो"}
)

// notAskingForSecrets: "pin code" is an address, and a delivery agent at the door
// may genuinely ask for the delivery OTP of an order you placed.
var notAskingForSecrets = []string{"pin code", "pincode", "पिन कोड", "postal code", "delivery otp", "delivery code", "डिलीवरी ओटीपी", "डिलीवरी कोड"}

// remoteApps are named outright: asking anyone to install one is a direct demand.
var remoteApps = []string{"anydesk", "any desk", "teamviewer", "team viewer", "quicksupport", "quick support", "rustdesk", "remote support", "एनीडेस्क"}

// normalized holds the same phrases after normalize(), computed once at start-up.
var normalized = func() map[scam.FlagType][]string {
	out := map[scam.FlagType][]string{}
	for t, list := range phrases {
		for _, p := range list {
			out[t] = append(out[t], normalize(p))
		}
	}
	return out
}()

// safetyAdvice are phrases genuine callers use ("never share your OTP", "we do not
// need your OTP"). A segment containing one does not count as asking for a secret,
// money or an app, unless the caller then asks anyway ("...just tell me the code").
var safetyAdvice = []string{
	"never share", "do not share", "don't share", "kabhi share mat", "share mat kijiye", "share na kare",
	"never ask", "will never", "do not need", "don't need", "do not install", "no otp", "no pin", "zarurat nahi", "jarurat nahi",
	"किसी के साथ साझा न करें", "शेयर मत कीजिए", "शेयर न करें", "कभी शेयर न करें", "जरूरत नहीं", "कभी नहीं मांग", "नहीं मांगते", "नहीं मांगता",
}

// askAnyway overrides safetyAdvice in the same segment.
var askAnyway = []string{"tell me", "give me", "mujhe bata", "mujhe de", "मुझे बता", "मुझे दे"}

// adviceHides are the flag types safety advice can switch off for a segment.
var adviceHides = map[scam.FlagType]bool{
	scam.OTPRequest: true, scam.PersonalData: true, scam.MoneyMove: true, scam.RemoteAccess: true, scam.LinkOrQR: true,
}

// Scan returns the red flags found in text: at most one hit per flag type, marked
// Strong if any match for that type was a direct demand.
func Scan(text string) []Hit {
	padded := normalize(text)
	isAdvice := containsAny(padded, safetyAdvice) && !containsAny(padded, askAnyway)

	byType := map[scam.FlagType]*Hit{}
	var order []scam.FlagType
	add := func(t scam.FlagType, phrase string, strong bool) {
		if h, ok := byType[t]; ok {
			h.Strong = h.Strong || strong
			return
		}
		byType[t] = &Hit{Type: t, Phrase: phrase, Strong: strong}
		order = append(order, t)
	}

	for _, t := range scam.AllFlags {
		if isAdvice && adviceHides[t] {
			continue
		}
		if t == scam.OTPRequest && containsAny(padded, notAskingForSecrets) {
			continue
		}
		for _, p := range normalized[t] {
			if strings.Contains(padded, p) {
				add(t, strings.TrimSpace(p), false)
				break
			}
		}
	}

	if !isAdvice {
		if w, ok := firstWord(padded, remoteApps); ok {
			add(scam.RemoteAccess, w, true)
		}
	}
	for _, pr := range pairs {
		if (isAdvice && adviceHides[pr.Type]) || containsAny(padded, pr.Not) {
			continue
		}
		a, okA := firstWord(padded, pr.A)
		b, okB := firstWord(padded, pr.B)
		if okA && okB {
			add(pr.Type, a+" … "+b, pr.Strong)
		}
	}

	// "1500 bhejna hai", "5000 lagao": an amount plus a send verb needs no money word.
	if _, seen := byType[scam.MoneyMove]; !seen && !isAdvice && !containsAny(padded, genuinePayment) && amount.MatchString(padded) {
		if w, ok := firstWord(padded, amountVerbs); ok {
			add(scam.MoneyMove, "amount … "+w, false)
		}
	}

	hits := make([]Hit, 0, len(order))
	for _, t := range order {
		hits = append(hits, *byType[t])
	}
	return hits
}

func containsAny(padded string, words []string) bool {
	_, ok := firstWord(padded, words)
	return ok
}

// firstWord returns the first word from the list that appears in padded text.
func firstWord(padded string, words []string) (string, bool) {
	for _, w := range words {
		if strings.Contains(padded, normalize(w)) {
			return w, true
		}
	}
	return "", false
}

// Weight returns how strongly a flag type points to a scam (0..1).
func Weight(t scam.FlagType) float64 { return weights[t] }

// RiskFromFlags turns a set of flag types into a rough 0..1 risk score.
// It is used when Evon is not configured or fails.
//
// Rule of thumb:
//   - each distinct tactic adds its weight;
//   - a setup tactic (pressure, urgency, lure) followed by an extraction attempt is almost always a scam.
func RiskFromFlags(types []scam.FlagType) float64 {
	seen := map[scam.FlagType]bool{}
	risk := 0.0
	hasSetup, hasExtraction := false, false
	for _, t := range types {
		if seen[t] {
			continue
		}
		seen[t] = true
		risk += weights[t]
		hasSetup = hasSetup || scam.IsSetup(t)
		hasExtraction = hasExtraction || scam.IsExtraction(t)
	}
	if hasSetup && hasExtraction && risk < 0.9 {
		risk = 0.9
	}
	if risk > 0.95 {
		risk = 0.95
	}
	return risk
}

// SortedTypes returns the keys of a flag set in a stable order.
func SortedTypes(set map[scam.FlagType]bool) []scam.FlagType {
	out := make([]scam.FlagType, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// normalize lowercases text, replaces punctuation with spaces, collapses
// whitespace and pads with spaces so we can match whole words with Contains.
func normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		// Keep letters, digits and combining marks (Indic vowel signs are marks).
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) {
			return r
		}
		return ' '
	}, s)
	s = halfNasal.ReplaceAllString(hindiSpelling.Replace(s), "ं$1")
	return " " + strings.Join(strings.Fields(s), " ") + " "
}

// hindiSpelling unifies spellings that are both correct and that speech
// recognition uses interchangeably: कीजिये/कीजिए, बताइये/बताइए, ज़रूरत/जरूरत, ड़/ड.
var hindiSpelling = strings.NewReplacer(
	"िये", "िए", "ाइये", "ाइए", "ाये", "ाए",
	"\u093c", "", // nukta as a separate mark
	"\u0958", "क", "\u0959", "ख", "\u095a", "ग", "\u095b", "ज", "\u095c", "ड", "\u095d", "ढ", "\u095e", "फ", "\u095f", "य", // precomposed
)

// halfNasal: ट्रान्सफर and ट्रांसफर are the same word.
var halfNasal = regexp.MustCompile(`[नमणङञ]\x{094D}([क-ह])`)
