// Package scam holds the shared vocabulary of the project: red-flag types,
// transcript lines, analysis results and the spoken messages in each language.
//
// Every other package (prefilter, detector, reasoner, session) speaks in these
// types, so this is the best file to read first.
package scam

import (
	"fmt"
	"strings"
)

// FlagType is one scam tactic. Scam calls in India follow a script:
// authority -> fear -> isolation -> urgency -> extraction (OTP / app / money).
type FlagType string

const (
	Authority    FlagType = "authority_impersonation" // "Main CBI se bol raha hoon"
	Fear         FlagType = "fear_threat"             // "Aapke naam pe arrest warrant hai"
	Urgency      FlagType = "urgency"                 // "30 minute mein karo warna..."
	Isolation    FlagType = "isolation"               // "Video call band mat karna"
	Secrecy      FlagType = "secrecy"                 // "Kisi ko mat batana, confidential hai"
	OTPRequest   FlagType = "otp_request"             // "OTP bataiye"
	RemoteAccess FlagType = "remote_access_app"       // "AnyDesk download kijiye"
	MoneyMove    FlagType = "money_transfer"          // "Safe account mein transfer kijiye"
	PersonalData FlagType = "personal_data_request"   // "Aadhaar number bataiye"
	TooGood      FlagType = "too_good_to_be_true"     // "Aapki lottery lagi hai"
	LinkOrQR     FlagType = "link_or_qr"              // "QR scan kijiye", "is link pe click karo"
)

// AllFlags lists every flag type, in the order they usually appear in a scam.
var AllFlags = []FlagType{
	Authority, Fear, Urgency, Isolation, Secrecy,
	OTPRequest, RemoteAccess, MoneyMove, LinkOrQR, PersonalData, TooGood,
}

// Descriptions explain each flag. They are used in the Evon prompt and alerts.
var Descriptions = map[FlagType]string{
	Authority:    "Caller claims to be police, CBI, customs, TRAI, RBI, a court, a courier or a bank official",
	Fear:         "Threat of arrest, FIR, legal case, drugs in parcel, account/SIM block",
	Urgency:      "Pressure to act immediately or within minutes",
	Isolation:    "Asks to stay on call/video call, go to a closed room, not hang up",
	Secrecy:      "Asks to keep it secret, not tell family or bank",
	OTPRequest:   "Asks for an OTP, PIN, CVV or verification code",
	RemoteAccess: "Asks to install AnyDesk, TeamViewer, QuickSupport or share the screen",
	MoneyMove:    "Asks to transfer money to a 'safe', 'verification' or 'RBI' account, or pay a fee",
	PersonalData: "Asks for Aadhaar, PAN, card number, bank details",
	TooGood:      "Lottery, prize, refund, subsidy, loan or job approval, guaranteed returns, task-based earning",
	LinkOrQR:     "Asks to click a payment/verification link, scan a QR code or approve a UPI collect request",
}

// IsValid reports whether s is a known flag type.
func IsValid(s string) bool {
	_, ok := Descriptions[FlagType(s)]
	return ok
}

// IsExtraction reports whether the flag means the scammer is trying to take
// something right now (OTP, device access, money). These are the most dangerous.
func IsExtraction(t FlagType) bool {
	return t == OTPRequest || t == RemoteAccess || t == MoneyMove || t == LinkOrQR
}

// IsPressure reports whether the flag is a pressure tactic.
func IsPressure(t FlagType) bool {
	return t == Authority || t == Fear || t == Isolation || t == Secrecy
}

// IsSetup reports whether the flag is part of the "setup" before extraction:
// pressure (threats, fake authority), urgency, or a lure (prize, refund).
// Setup followed by an extraction attempt is the core scam pattern.
func IsSetup(t FlagType) bool {
	return IsPressure(t) || t == Urgency || t == TooGood
}

// Line is one finalised piece of transcript.
type Line struct {
	T       float64 `json:"t"`                 // seconds since call start
	Speaker string  `json:"speaker,omitempty"` // call | caller | chaukanna
	Text    string  `json:"text"`
}

// Flag is one detected red flag.
type Flag struct {
	Type   FlagType `json:"type"`
	T      float64  `json:"t"`
	Quote  string   `json:"quote"`
	Source string   `json:"source,omitempty"` // prefilter | evon
}

// Analysis is what the reasoning step (Evon, or the rule-based fallback) returns.
type Analysis struct {
	Risk        float64 `json:"risk"`         // 0..1
	Stage       string  `json:"stage"`        // which part of the scam script we are in
	Language    string  `json:"language"`     // language of the conversation
	Flags       []Flag  `json:"flags"`        // red flags with quotes
	Reason      string  `json:"reason"`       // one-line explanation
	WarningText string  `json:"warning_text"` // what to say to the user, in their language
}

// ---------------------------------------------------------------------------
// Languages and spoken messages.
// NOTE: please get a native speaker to proof-read the non-Hindi/English lines.
// ---------------------------------------------------------------------------

var languageNames = map[string]string{
	"hi": "Hindi", "en": "English", "ta": "Tamil", "te": "Telugu", "kn": "Kannada",
	"ml": "Malayalam", "bn": "Bengali", "mr": "Marathi", "gu": "Gujarati", "pa": "Punjabi",
}

// KnownLanguage returns a supported short code for what a client sent, or "hi".
func KnownLanguage(code string) string {
	base := BaseLanguage(code)
	if _, ok := languageNames[base]; ok {
		return base
	}
	return "hi"
}

// LanguageName returns the English name of a language code ("hi" -> "Hindi").
func LanguageName(code string) string {
	if name, ok := languageNames[BaseLanguage(code)]; ok {
		return name
	}
	return "Hindi"
}

// BaseLanguage turns "hi-IN" into "hi".
func BaseLanguage(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if i := strings.IndexAny(code, "-_"); i > 0 {
		code = code[:i]
	}
	if code == "" {
		return "hi"
	}
	return code
}

// Locale turns "hi" into "hi-IN" (the format most Indian speech APIs expect).
func Locale(code string) string {
	return BaseLanguage(code) + "-IN"
}

// pick returns the message for a language, falling back to Hindi, then English.
func pick(messages map[string]string, lang string) string {
	if m, ok := messages[BaseLanguage(lang)]; ok {
		return m
	}
	if m, ok := messages["hi"]; ok {
		return m
	}
	return messages["en"]
}

// Spoken warnings are always these pre-written lines, never model output: a warning
// goes straight into a frightened person's ear, so every word must be checked first.
// Evon decides IF we warn and explains why on the dashboard; it does not write the words.
//
// Each one announces that it is Chaukanna's AI voice and ends with the helpline.
var defaultWarnings = map[string]string{
	"hi": "चौकन्ना AI की चेतावनी। यह कॉल धोखाधड़ी लग रही है। कोई भी बैंक, पुलिस या कंपनी फ़ोन पर ओटीपी, पैसे या स्क्रीन शेयर नहीं माँगती। कुछ मत बताइए, फ़ोन काट दीजिए, और 1930 पर कॉल कीजिए।",
	"en": "This is a warning from Chaukanna AI. This call looks like a fraud. No bank, police or company asks for your OTP, money or screen over the phone. Do not share anything. Hang up now, and call 1930 for help.",
	"ta": "இது சௌகன்னா AI எச்சரிக்கை. இந்த அழைப்பு மோசடி போல் தெரிகிறது. யாருக்கும் OTP அல்லது பணம் கொடுக்காதீர்கள். உடனே அழைப்பைத் துண்டித்து, 1930 என்ற எண்ணை அழையுங்கள்.",
	"te": "ఇది చౌకన్నా AI హెచ్చరిక. ఈ కాల్ మోసం లాగా ఉంది. ఎవరికీ OTP లేదా డబ్బు ఇవ్వకండి. వెంటనే ఫోన్ పెట్టేసి, 1930 కి కాల్ చేయండి.",
	"kn": "ಇದು ಚೌಕನ್ನಾ AI ಎಚ್ಚರಿಕೆ. ಈ ಕರೆ ವಂಚನೆಯಂತೆ ಕಾಣುತ್ತಿದೆ. ಯಾರಿಗೂ OTP ಅಥವಾ ಹಣ ಕೊಡಬೇಡಿ. ತಕ್ಷಣ ಕರೆ ಕಡಿತಗೊಳಿಸಿ, 1930 ಗೆ ಕರೆ ಮಾಡಿ.",
	"ml": "ഇത് ചൗകന്ന AI മുന്നറിയിപ്പാണ്. ഈ കോൾ തട്ടിപ്പാണെന്ന് തോന്നുന്നു. ആർക്കും OTP യോ പണമോ നൽകരുത്. ഉടൻ ഫോൺ കട്ട് ചെയ്ത്, 1930 ൽ വിളിക്കുക.",
	"bn": "এটি চৌকান্না AI-এর সতর্কবার্তা। এই কলটি প্রতারণা মনে হচ্ছে। কাউকে OTP বা টাকা দেবেন না। এখনই ফোন কেটে দিন, এবং 1930 নম্বরে ফোন করুন।",
	"mr": "ही चौकन्ना AI ची सूचना आहे. हा कॉल फसवणुकीचा वाटतो. कोणालाही OTP किंवा पैसे देऊ नका. लगेच फोन बंद करा, आणि 1930 वर कॉल करा.",
	"gu": "આ ચૌકન્ના AI ની ચેતવણી છે. આ કોલ છેતરપિંડી લાગે છે. કોઈને OTP કે પૈસા આપશો નહીં. તરત ફોન કાપી નાખો, અને 1930 પર કોલ કરો.",
	"pa": "ਇਹ ਚੌਕੰਨਾ AI ਦੀ ਚੇਤਾਵਨੀ ਹੈ। ਇਹ ਕਾਲ ਧੋਖਾਧੜੀ ਲੱਗਦੀ ਹੈ। ਕਿਸੇ ਨੂੰ OTP ਜਾਂ ਪੈਸੇ ਨਾ ਦਿਓ। ਤੁਰੰਤ ਫ਼ੋਨ ਕੱਟੋ, ਅਤੇ 1930 ਤੇ ਕਾਲ ਕਰੋ।",
}

// helplineDigits says 1930 digit by digit. Timbre otherwise reads it as a quantity
// ("one thousand nine hundred thirty"), and nobody can dial that. Found by speaking
// each warning and transcribing it back (cmd/warningcheck).
var helplineDigits = map[string]string{
	"hi": "एक नौ तीन शून्य", "en": "one nine three zero", "ta": "ஒன்று ஒன்பது மூன்று பூஜ்ஜியம்", "te": "ఒకటి తొమ్మిది మూడు సున్నా",
	"kn": "ಒಂದು ಒಂಬತ್ತು ಮೂರು ಸೊನ್ನೆ", "ml": "ഒന്ന് ഒമ്പത് മൂന്ന് പൂജ്യം", "bn": "এক নয় তিন শূন্য", "mr": "एक नऊ तीन शून्य",
	"gu": "એક નવ ત્રણ શૂન્ય", "pa": "ਇੱਕ ਨੌਂ ਤਿੰਨ ਸਿਫ਼ਰ",
}

// Spoken prepares text for text-to-speech. The screen keeps showing "1930".
func Spoken(text, lang string) string {
	return strings.ReplaceAll(text, "1930", pick(helplineDigits, lang))
}

// DefaultWarning is the general warning.
func DefaultWarning(lang string) string { return pick(defaultWarnings, lang) }

// cautions are the soft tier: the call is doubtful, not proven. It tells them how to
// check for themselves, and does not accuse the caller.
var cautions = map[string]string{
	"hi": "चौकन्ना AI की सलाह। सावधान रहिए। कोई ओटीपी, पैसा या जानकारी मत दीजिए। फ़ोन रखकर, उस संस्था के असली नंबर पर ख़ुद कॉल करके जाँच लीजिए।",
	"en": "A caution from Chaukanna AI. Please be careful. Do not give any OTP, money or details. Hang up and check by calling the organisation's official number yourself.",
	"ta": "இது சௌகன்னா AI ஆலோசனை. கவனமாக இருங்கள். OTP, பணம் அல்லது தகவல் எதுவும் கொடுக்காதீர்கள். அந்த நிறுவனத்தின் உண்மையான எண்ணை நீங்களே அழைத்து சரிபாருங்கள்.",
	"te": "ఇది చౌకన్నా AI సలహా. జాగ్రత్తగా ఉండండి. OTP, డబ్బు లేదా వివరాలు ఇవ్వకండి. ఆ సంస్థ అసలు నంబర్‌కు మీరే కాల్ చేసి తనిఖీ చేసుకోండి.",
	"kn": "ಇದು ಚೌಕನ್ನಾ AI ಸಲಹೆ. ಎಚ್ಚರಿಕೆಯಿಂದಿರಿ. OTP, ಹಣ ಅಥವಾ ಮಾಹಿತಿ ಕೊಡಬೇಡಿ. ಆ ಸಂಸ್ಥೆಯ ನಿಜವಾದ ಸಂಖ್ಯೆಗೆ ನೀವೇ ಕರೆ ಮಾಡಿ ಪರಿಶೀಲಿಸಿ.",
	"ml": "ഇത് ചൗകന്ന AI നിർദ്ദേശമാണ്. ശ്രദ്ധിക്കുക. OTP യോ പണമോ വിവരങ്ങളോ നൽകരുത്. ആ സ്ഥാപനത്തിന്റെ യഥാർത്ഥ നമ്പറിൽ നിങ്ങൾ തന്നെ വിളിച്ച് ഉറപ്പാക്കുക.",
	"bn": "এটি চৌকান্না AI-এর পরামর্শ। সাবধান থাকুন। OTP, টাকা বা কোনো তথ্য দেবেন না। ওই সংস্থার আসল নম্বরে নিজে ফোন করে যাচাই করে নিন।",
	"mr": "ही चौकन्ना AI ची सूचना आहे. सावध राहा. OTP, पैसे किंवा माहिती देऊ नका. त्या संस्थेच्या खऱ्या नंबरवर स्वतः कॉल करून खात्री करा.",
	"gu": "આ ચૌકન્ના AI ની સલાહ છે. સાવધ રહો. OTP, પૈસા કે માહિતી આપશો નહીં. તે સંસ્થાના સાચા નંબર પર જાતે કોલ કરીને ખાતરી કરો.",
	"pa": "ਇਹ ਚੌਕੰਨਾ AI ਦੀ ਸਲਾਹ ਹੈ। ਸਾਵਧਾਨ ਰਹੋ। OTP, ਪੈਸੇ ਜਾਂ ਜਾਣਕਾਰੀ ਨਾ ਦਿਓ। ਉਸ ਸੰਸਥਾ ਦੇ ਅਸਲੀ ਨੰਬਰ ਤੇ ਖ਼ੁਦ ਕਾਲ ਕਰਕੇ ਜਾਂਚ ਕਰੋ।",
}

// Caution is the soft spoken warning for doubtful calls.
func Caution(lang string) string { return pick(cautions, lang) }

// Category is a family of scam. It selects the warning and goes in the report.
type Category string

const (
	General       Category = "general"
	DigitalArrest Category = "digital_arrest"
	Courier       Category = "courier"
	BankKYC       Category = "bank_kyc"
	Relative      Category = "relative_emergency"
	Prize         Category = "prize"
	Job           Category = "job_task"
	Investment    Category = "investment"
	Loan          Category = "loan"
	Utility       Category = "utility_sim"
	TechHelp      Category = "tech_support"
	Refund        Category = "refund_upi"
	Benefit       Category = "govt_benefit"
	Charity       Category = "charity"
)

// CategoryLabels are shown on the dashboard, alerts and the report.
var CategoryLabels = map[Category]string{
	General:       "Phone fraud",
	DigitalArrest: "Digital arrest / fake police",
	Courier:       "Courier or customs parcel",
	BankKYC:       "Bank / KYC / OTP",
	Relative:      "Relative in an emergency",
	Prize:         "Lottery or prize",
	Job:           "Job or task offer",
	Investment:    "Investment or trading",
	Loan:          "Instant loan",
	Utility:       "Electricity, gas or SIM cut-off",
	TechHelp:      "Tech support / remote access",
	Refund:        "Refund or UPI",
	Benefit:       "Government benefit or insurance",
	Charity:       "Fake donation",
}

// categoryFacts is the one line that breaks the scammer's story. A specific fact
// persuades better than a general "beware". Hindi and English only for now: the
// other languages use the general warning until a native speaker can review more.
var categoryFacts = map[Category]map[string]string{
	DigitalArrest: {"hi": "डिजिटल अरेस्ट नाम की कोई चीज़ नहीं होती। पुलिस वीडियो कॉल पर गिरफ़्तार नहीं करती।", "en": "Digital arrest does not exist. Police never arrest anyone over a video call."},
	Courier:       {"hi": "कूरियर कंपनी या कस्टम फ़ोन पर केस ख़त्म करने के पैसे नहीं माँगते।", "en": "Courier companies and customs never ask for money over the phone to close a case."},
	BankKYC:       {"hi": "बैंक कभी भी फ़ोन पर ओटीपी, पिन या कार्ड नंबर नहीं माँगता।", "en": "A bank never asks for your OTP, PIN or card number on a call."},
	Relative:      {"hi": "पैसे भेजने से पहले, अपने रिश्तेदार को उनके पुराने नंबर पर ख़ुद फ़ोन कीजिए।", "en": "Before sending money, call your relative yourself on their usual number."},
	Prize:         {"hi": "असली इनाम के लिए पहले पैसे नहीं देने पड़ते।", "en": "A real prize never asks you to pay first."},
	Job:           {"hi": "असली नौकरी के लिए पहले पैसे नहीं देने पड़ते।", "en": "A real job never asks you to pay first."},
	Investment:    {"hi": "पक्के मुनाफ़े का वादा हमेशा झूठ होता है।", "en": "A promise of guaranteed profit is always false."},
	Loan:          {"hi": "असली लोन के लिए पहले से फ़ीस फ़ोन पर नहीं माँगी जाती।", "en": "A real lender does not ask for an upfront fee on a call."},
	Utility:       {"hi": "बिजली, गैस या सिम कंपनी किसी लिंक या निजी नंबर पर पैसे नहीं माँगती।", "en": "Electricity, gas and SIM companies do not ask for payment on a private link or number."},
	TechHelp:      {"hi": "कोई भी कंपनी आपसे फ़ोन पर ऐसा ऐप नहीं डलवाती जिससे वह आपका फ़ोन देख सके।", "en": "No company asks you to install an app that lets them see your phone."},
	Refund:        {"hi": "पैसे पाने के लिए कभी पिन डालना या क्यूआर स्कैन करना नहीं पड़ता।", "en": "You never enter a PIN or scan a QR code to receive money."},
	Benefit:       {"hi": "सरकारी योजना या बीमे की रक़म पाने के लिए पहले पैसे नहीं देने पड़ते।", "en": "You never pay first to receive a government benefit or insurance money."},
	Charity:       {"hi": "दान करना हो तो ख़ुद संस्था की असली वेबसाइट पर जाँचिए।", "en": "If you want to donate, check the organisation's real website yourself."},
}

// otpFact replaces a fact about money when the caller is asking for an OTP instead: a
// "courier" caller demanding a code must not be answered with a line about payments.
var otpFact = map[string]string{
	"hi": "कोई भी कंपनी या अधिकारी फ़ोन पर ओटीपी नहीं माँगता। ओटीपी बताते ही आपके पैसे जा सकते हैं।",
	"en": "No company or official ever asks for an OTP on a call. Sharing it can empty your account.",
}

// moneyFacts are the families whose fact is about paying.
var moneyFacts = map[Category]bool{
	Courier: true, Prize: true, Job: true, Investment: true, Loan: true, Utility: true, Benefit: true, Charity: true,
}

// WarningFor is the warning for a scam family: the specific fact where we have one
// in this language, then the general warning. otpOnly says the caller asked for an
// OTP or PIN but not for money.
func WarningFor(c Category, lang string, otpOnly bool) string {
	base := BaseLanguage(lang)
	fact, ok := categoryFacts[c][base]
	if ok && otpOnly && moneyFacts[c] {
		fact = otpFact[base]
	}
	if ok {
		if base == "en" {
			return "This is a warning from Chaukanna AI. This call looks like a fraud. " + fact + " Do not share anything. Hang up now, and call 1930 for help."
		}
		return "चौकन्ना AI की चेतावनी। यह कॉल धोखाधड़ी लग रही है। " + fact + " कुछ मत बताइए, फ़ोन काट दीजिए, और 1930 पर कॉल कीजिए।"
	}
	return DefaultWarning(lang)
}

// stopNow interrupts when the person seems to be handing over a code or money. It is
// short on purpose: every second counts here.
var stopNow = map[string]string{
	"hi": "चौकन्ना AI: रुकिए! कोई नंबर या पैसे मत दीजिए। यह धोखाधड़ी है। अभी फ़ोन काट दीजिए।",
	"en": "Chaukanna AI: Stop! Do not give any number or money. This is a fraud. Hang up now.",
	"ta": "சௌகன்னா AI: நிறுத்துங்கள்! எந்த எண்ணையும் பணத்தையும் கொடுக்காதீர்கள். இது மோசடி. உடனே அழைப்பைத் துண்டியுங்கள்.",
	"te": "చౌకన్నా AI: ఆగండి! ఏ నంబర్ గానీ డబ్బు గానీ ఇవ్వకండి. ఇది మోసం. వెంటనే ఫోన్ పెట్టేయండి.",
	"kn": "ಚೌಕನ್ನಾ AI: ನಿಲ್ಲಿ! ಯಾವುದೇ ಸಂಖ್ಯೆ ಅಥವಾ ಹಣ ಕೊಡಬೇಡಿ. ಇದು ವಂಚನೆ. ತಕ್ಷಣ ಕರೆ ಕಡಿತಗೊಳಿಸಿ.",
	"ml": "ചൗകന്ന AI: നിർത്തൂ! ഒരു നമ്പറും പണവും നൽകരുത്. ഇത് തട്ടിപ്പാണ്. ഉടൻ ഫോൺ കട്ട് ചെയ്യുക.",
	"bn": "চৌকান্না AI: থামুন! কোনো নম্বর বা টাকা দেবেন না। এটি প্রতারণা। এখনই ফোন কেটে দিন।",
	"mr": "चौकन्ना AI: थांबा! कोणताही नंबर किंवा पैसे देऊ नका. ही फसवणूक आहे. लगेच फोन बंद करा.",
	"gu": "ચૌકન્ના AI: અટકો! કોઈ નંબર કે પૈસા આપશો નહીં. આ છેતરપિંડી છે. તરત ફોન કાપી નાખો.",
	"pa": "ਚੌਕੰਨਾ AI: ਰੁਕੋ! ਕੋਈ ਨੰਬਰ ਜਾਂ ਪੈਸੇ ਨਾ ਦਿਓ। ਇਹ ਧੋਖਾਧੜੀ ਹੈ। ਤੁਰੰਤ ਫ਼ੋਨ ਕੱਟੋ।",
}

// StopNow is the urgent interruption when the person starts to comply.
func StopNow(lang string) string { return pick(stopNow, lang) }

// answerTalkOut answers a caller trying to talk the person out of the warning. Scammers
// say this, and so might a genuine caller after a false alarm, so it accuses nobody: it
// gives a test that works either way.
var answerTalkOut = map[string]string{
	"hi": "चौकन्ना AI: असली बैंक या पुलिस को इस चेतावनी से कोई परेशानी नहीं होती। फ़ोन काटकर ख़ुद जाँच कीजिए।",
	"en": "Chaukanna AI: A real bank or police officer has no problem with this warning. Hang up and check for yourself.",
	"ta": "சௌகன்னா AI: உண்மையான வங்கி அல்லது காவல்துறைக்கு இந்த எச்சரிக்கையால் பிரச்சினை இல்லை. அழைப்பைத் துண்டித்து நீங்களே சரிபாருங்கள்.",
	"te": "చౌకన్నా AI: నిజమైన బ్యాంకు లేదా పోలీసులకు ఈ హెచ్చరికతో సమస్య ఉండదు. ఫోన్ పెట్టేసి మీరే తనిఖీ చేసుకోండి.",
	"kn": "ಚೌಕನ್ನಾ AI: ನಿಜವಾದ ಬ್ಯಾಂಕ್ ಅಥವಾ ಪೊಲೀಸರಿಗೆ ಈ ಎಚ್ಚರಿಕೆಯಿಂದ ತೊಂದರೆ ಇಲ್ಲ. ಕರೆ ಕಡಿತಗೊಳಿಸಿ ನೀವೇ ಪರಿಶೀಲಿಸಿ.",
	"ml": "ചൗകന്ന AI: യഥാർത്ഥ ബാങ്കിനോ പോലീസിനോ ഈ മുന്നറിയിപ്പ് കൊണ്ട് പ്രശ്നമില്ല. ഫോൺ കട്ട് ചെയ്ത് നിങ്ങൾ തന്നെ ഉറപ്പാക്കുക.",
	"bn": "চৌকান্না AI: আসল ব্যাংক বা পুলিশের এই সতর্কবার্তায় কোনো সমস্যা নেই। ফোন কেটে নিজে যাচাই করুন।",
	"mr": "चौकन्ना AI: खऱ्या बँकेला किंवा पोलिसांना या सूचनेचा त्रास नसतो. फोन बंद करून स्वतः खात्री करा.",
	"gu": "ચૌકન્ના AI: સાચી બેંક કે પોલીસને આ ચેતવણીથી કોઈ વાંધો નથી. ફોન કાપીને જાતે ખાતરી કરો.",
	"pa": "ਚੌਕੰਨਾ AI: ਅਸਲੀ ਬੈਂਕ ਜਾਂ ਪੁਲਿਸ ਨੂੰ ਇਸ ਚੇਤਾਵਨੀ ਤੋਂ ਕੋਈ ਦਿੱਕਤ ਨਹੀਂ। ਫ਼ੋਨ ਕੱਟ ਕੇ ਖ਼ੁਦ ਜਾਂਚ ਕਰੋ।",
}

// AnswerTalkOut is the reply when a caller tries to discredit the warning.
func AnswerTalkOut(lang string) string { return pick(answerTalkOut, lang) }

// HelplineSpoken lists how the helpline is said aloud, so hearing our own warning
// through the speaker is not mistaken for someone reading out a code.
func HelplineSpoken() []string {
	out := make([]string, 0, len(helplineDigits))
	for _, words := range helplineDigits {
		out = append(out, words)
	}
	return out
}

// AllWarnings lists every clip worth preparing ahead of time for one language.
func AllWarnings(lang string) []string {
	out := []string{DefaultWarning(lang), Caution(lang), StopNow(lang), AnswerTalkOut(lang)}
	seen := map[string]bool{out[0]: true}
	for c := range categoryFacts {
		for _, otpOnly := range []bool{false, true} {
			if w := WarningFor(c, lang, otpOnly); !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	return out
}

// OTPOnly reports whether the caller asked for an OTP or PIN but not for money.
func OTPOnly(flags []Flag) bool {
	otp, money := false, false
	for _, f := range flags {
		otp = otp || f.Type == OTPRequest
		money = money || f.Type == MoneyMove
	}
	return otp && !money
}

var greetings = map[string]string{
	"hi": "नमस्ते, मैं चौकन्ना हूँ, %s की AI सहायक। कृपया बताइए आप कौन बोल रहे हैं और किस काम से कॉल किया है?",
	"en": "Hello, I am Chaukanna, %s's AI assistant. May I know who is calling and what this call is about?",
}

// Greeting is the first thing the screening bot says to an unknown caller.
func Greeting(lang, userName string) string {
	return strings.Replace(pick(greetings, lang), "%s", userName, 1)
}

var closings = map[string]string{
	"hi": "धन्यवाद, मैं आपका संदेश पहुँचा दूँगी। नमस्ते।",
	"en": "Thank you, I will pass on your message. Goodbye.",
}

// Closing is the last thing the screening bot says before hanging up.
func Closing(lang string) string { return pick(closings, lang) }

var followUps = map[string]string{
	"hi": "जी, क्या आप अपना नाम और अपनी संस्था का नाम बता सकते हैं?",
	"en": "Could you please tell me your name and which organisation you are calling from?",
}

// FollowUp is a generic second question used when Evon is not configured.
func FollowUp(lang string) string { return pick(followUps, lang) }

var blockedMessages = map[string]string{
	"hi": "यह नंबर धोखाधड़ी के लिए रिपोर्ट हो चुका है, इसलिए यह कॉल आगे नहीं जोड़ी जाएगी। नमस्ते।",
	"en": "This number has been reported for fraud, so this call will not be connected. Goodbye.",
}

// BlockedMessage is said to a caller whose number is a known scam number.
func BlockedMessage(lang string) string { return pick(blockedMessages, lang) }

// BlockedSummary tells the user that a known scam number was stopped.
func BlockedSummary(lang string, reports int) string {
	if BaseLanguage(lang) == "en" {
		return fmt.Sprintf("Chaukanna blocked a call from a number that was reported for fraud %d time(s) before. You don't need to do anything.", reports)
	}
	return fmt.Sprintf("एक ऐसे नंबर से कॉल आई जो पहले %d बार धोखाधड़ी के लिए रिपोर्ट हो चुका है। चौकन्ना ने कॉल रोक दी, आपको कुछ करने की ज़रूरत नहीं है।", reports)
}

// FallbackScreenSummary builds a spoken summary for the user when Evon is not
// available. It is intentionally simple.
func FallbackScreenSummary(lang string, risk float64, callerWords string) string {
	if r := []rune(callerWords); len(r) > 140 {
		callerWords = string(r[:140]) + "…"
	}
	if BaseLanguage(lang) == "en" {
		verdict := "It looks safe."
		if risk >= 0.8 {
			verdict = "This looks like a scam. Please do not call back."
		} else if risk >= 0.4 {
			verdict = "This looks suspicious. Be careful."
		}
		return "You missed a call. The caller said: " + callerWords + ". " + verdict
	}
	verdict := "यह कॉल सुरक्षित लगती है।"
	if risk >= 0.8 {
		verdict = "यह कॉल धोखाधड़ी लगती है। कृपया वापस कॉल मत कीजिए।"
	} else if risk >= 0.4 {
		verdict = "यह कॉल संदिग्ध लगती है। सावधान रहिए।"
	}
	return "आपको एक कॉल आई थी। कॉल करने वाले ने कहा: " + callerWords + "। " + verdict
}
