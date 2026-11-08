package prefilter

import (
	"testing"

	"chaukanna/internal/scam"
)

func has(hits []Hit, t scam.FlagType) bool {
	for _, h := range hits {
		if h.Type == t {
			return true
		}
	}
	return false
}

func TestHinglishAndDevanagari(t *testing.T) {
	if !has(Scan("Main CBI se bol raha hoon"), scam.Authority) {
		t.Error("romanised CBI not detected")
	}
	if !has(Scan("आपके नाम पर गिरफ्तार का वारंट है"), scam.Fear) {
		t.Error("Devanagari arrest warrant not detected")
	}
	if !has(Scan("AnyDesk app download kijiye"), scam.RemoteAccess) {
		t.Error("AnyDesk not detected")
	}
}

func TestWholeWordsOnly(t *testing.T) {
	if has(Scan("What a wonderful day"), scam.TooGood) {
		t.Error("'won' should not match inside 'wonderful'")
	}
}

func TestBankSafetyAdviceIsNotAnOTPRequest(t *testing.T) {
	if has(Scan("Please never share your OTP with anyone"), scam.OTPRequest) {
		t.Error("safety advice should not count as an OTP request")
	}
	if !has(Scan("Aapke phone pe OTP aaya hoga, bataiye"), scam.OTPRequest) {
		t.Error("real OTP request not detected")
	}
}

// Exact transcripts Prisma produced from Timbre audio over an 8 kHz μ-law line
// (go run ./cmd/gnanicheck). Each must reveal a setup tactic AND an OTP request.
func TestRegionalTranscriptsFromPrisma(t *testing.T) {
	transcripts := map[string]string{
		"hindi":   "नमस्ते, मैं CBI से बोल रहा हूँ। आपके नाम पर arrest warrant है, अपना OTP बताइये।",
		"english": "Hello madam, I am calling from the customs department. A parcel in your name has drugs, please share the OTP now.",
		"tamil":   "வணக்கம், நான் வங்கி-லிருந்து பேசுகிறேன், உங்கள் கணக்கு முடக்கப்படும், உடனே ஓடிபி சொல்லுங்கள்",
		"bengali": "নমস্কার, আমি পুলিশ থেকে বলছি, আপনার নামে গ্রেফতারি পরোয়ানা আছে, এখনই ওটিপি বলুন",
		"marathi": "नमस्कार, मी पोलीस स्टेशन मधून बोलतोय, तुमच्या नावावर अटक वॉरंट आहे, लगेच ओटीपी सांगा.",
	}
	for lang, text := range transcripts {
		var types []scam.FlagType
		for _, h := range Scan(text) {
			types = append(types, h.Type)
		}
		if RiskFromFlags(types) < 0.8 {
			t.Errorf("%s: only found %v", lang, types)
		}
	}
}

// Real speakerphone test (call #2): a "bank" asking for the OTP must be a scam.
func TestBankAskingForOTPIsScam(t *testing.T) {
	var types []scam.FlagType
	for _, line := range []string{
		"Maam, can you please share your OTP with me? I am speaking from SBI bank.",
		"Ma'am, I am speaking from SBI Bank. This is not a scam call. Can you please share your OTP with me?",
	} {
		for _, h := range Scan(line) {
			types = append(types, h.Type)
		}
	}
	if RiskFromFlags(types) < 0.8 {
		t.Fatalf("bank + OTP request should be a scam, found only %v", types)
	}

	// A genuine bank reminder still stays calm.
	types = nil
	for _, h := range Scan("This is SBI calling about your EMI. Please never share your OTP with anyone.") {
		types = append(types, h.Type)
	}
	if RiskFromFlags(types) >= 0.4 {
		t.Fatalf("genuine bank reminder flagged: %v", types)
	}
}

// Real speakerphone test (call #8): words split by other words, and tech-support impersonation.
func TestSplitScreenShareRequest(t *testing.T) {
	line := "मैं phone repair वाला बात कर रहा हूँ, क्या आप अपनी screen मेरे साथ share कर सकते हैं?"
	var types []scam.FlagType
	for _, h := range Scan(line) {
		types = append(types, h.Type)
	}
	if RiskFromFlags(types) < 0.8 {
		t.Fatalf("tech support + screen share should be a scam, found %v", types)
	}

	types = nil
	for _, line := range []string{"मैं SBI bank से manager बात कर रहा हूँ।", "क्या आप अपनी screen मेरे साथ share कर सकते हैं?"} {
		for _, h := range Scan(line) {
			types = append(types, h.Type)
		}
	}
	if RiskFromFlags(types) < 0.8 {
		t.Fatalf("bank + screen share across two lines should be a scam, found %v", types)
	}
}

func TestPairsNeedBothWords(t *testing.T) {
	for _, line := range []string{
		"Please share this photo with mummy",      // "share" without "screen"
		"My phone screen is broken",               // "screen" without "share"
		"Never share your OTP or PIN with anyone", // safety advice
	} {
		for _, h := range Scan(line) {
			if h.Type == scam.RemoteAccess || h.Type == scam.OTPRequest {
				t.Errorf("%q wrongly flagged as %s", line, h.Type)
			}
		}
	}
}

// Real speakerphone tests (calls #10 and #12).
func TestBailAndServiceOTP(t *testing.T) {
	scams := map[string][]string{
		"canada police bail": {
			"I am speaking from Canada Police, and your son has been arrested here in case of illegal drugs and everything and alcohol consumption.",
			"And if you need to get him bail, you need to submit 10,000 dollars.",
		},
		"ac service otp": {
			"वो service मैंने करी थी और उसके पैसे आपने मेरे को भेजने हैं, तो उसके लिए OTP आएगा। आप वो OTP please मेरे साथ share कर दीजिए।",
		},
	}
	for name, lines := range scams {
		var types []scam.FlagType
		for _, l := range lines {
			for _, h := range Scan(l) {
				types = append(types, h.Type)
			}
		}
		if RiskFromFlags(types) < 0.8 {
			t.Errorf("%s: only found %v", name, types)
		}
	}
	// Normal money talk must not be flagged as a transfer request on its own.
	for _, h := range append(Scan("Bhaiya, AC service ke paise kitne hue? Main kal cash de dungi."),
		Scan("Your EMI of twelve thousand rupees is due on the seventh. You can pay it in the YONO app.")...) {
		if h.Type == scam.MoneyMove || h.Type == scam.OTPRequest {
			t.Errorf("normal question flagged as %s", h.Type)
		}
	}
}

// Noisy-benchmark transcripts: Tamil-script loanwords and Hindi spelling variants.
func TestNoisyTranscripts(t *testing.T) {
	transcripts := map[string]string{
		"tamil kyc (Prisma clean)": "வணக்கம் சார், நான் உங்க பேங்க் கே.ஒய்.சி டிபார்ட்மென்ட்ல இருந்து பேசுகிறேன். உங்க கேஒய்சி எக்ஸ்பையர் ஆகிடுச்சு. இன்னிக்கும் அப்டேட் பண்ணலைனா, அக்கவுண்ட் பிளாக் ஆகிடும். இப்போ உங்க ஃபோனுக்கு ஒரு ஓடிபி வரும், ஆதார் சொல்லுங்க.",
		"fedex 5 dB (कीजिये)":      "उसमें 5 passport और drugs मिले। आप पर case दर्ज हो चुका है, गिरफ्तार होने से बचने के लिए तुरंत जमा कीजिये।",
	}
	for name, text := range transcripts {
		var types []scam.FlagType
		for _, h := range Scan(text) {
			types = append(types, h.Type)
		}
		if RiskFromFlags(types) < 0.8 {
			t.Errorf("%s: only found %v", name, types)
		}
	}
}

func strong(hits []Hit, t scam.FlagType) bool {
	for _, h := range hits {
		if h.Type == t && h.Strong {
			return true
		}
	}
	return false
}

// Prisma v3 writes English words in Latin and Hindi in Devanagari, in one line.
func TestMixedScriptDemandsAsPrismaWritesThem(t *testing.T) {
	cases := []struct {
		text string
		want scam.FlagType
	}{
		{"आपका KYC expire हो गया है, phone पर एक OTP आया है, वो बताइये", scam.OTPRequest},
		{"release करने के लिए processing fee pay करना होगा", scam.MoneyMove},
		{"registration के लिए 2499 रुपये भेजने होंगे", scam.MoneyMove},
		{"पहले एक QR code scan करना पड़ेगा", scam.LinkOrQR},
		{"ट्रान्सफ़र कीजिए पैसे", scam.MoneyMove}, // half-nasal and nukta spellings
	}
	for _, c := range cases {
		if !has(Scan(c.text), c.want) {
			t.Errorf("%q: %s not detected", c.text, c.want)
		}
	}
}

func TestDirectDemandsAreStrong(t *testing.T) {
	cases := map[string]scam.FlagType{
		"Please tell me the OTP":                   scam.OTPRequest,
		"AnyDesk download kar lijiye":              scam.RemoteAccess,
		"QR code scan kijiye paise milenge":        scam.LinkOrQR,
		"payment request approve kar dijiye":       scam.LinkOrQR,
		"scan it and enter PIN to receive advance": scam.OTPRequest,
		"paise safe account mein transfer kijiye":  scam.MoneyMove,
	}
	for text, flag := range cases {
		if !strong(Scan(text), flag) {
			t.Errorf("%q should be a strong %s demand; got %+v", text, flag, Scan(text))
		}
	}
}

// What genuine callers say must not count as a request.
func TestGenuineCallersAreNotAsking(t *testing.T) {
	cases := []struct {
		text string
		not  scam.FlagType
	}{
		{"We do NOT need your card number or OTP", scam.OTPRequest},
		{"No OTP or PIN required", scam.OTPRequest},
		{"Phone par koi payment ya OTP ki zarurat nahi hai", scam.OTPRequest},
		{"Phone par koi payment ya OTP ki zarurat nahi hai", scam.MoneyMove},
		{"You can pay at the counter during pickup", scam.MoneyMove},
		{"Ji madam, official portal ya app se pay kar sakte hain", scam.MoneyMove},
		{"Registered account se automatically debit ho jayega, 5000 rupaye", scam.MoneyMove},
		{"Cash on delivery 450 rupaye hai", scam.MoneyMove},
		{"What is your pin code please, tell me", scam.OTPRequest},
		{"Please tell me the delivery OTP for your order", scam.OTPRequest},
		{"Do not install AnyDesk if anyone asks you to", scam.RemoteAccess},
		{"Army Officer transferred to your city", scam.MoneyMove},
	}
	for _, c := range cases {
		if has(Scan(c.text), c.not) {
			t.Errorf("%q should not raise %s", c.text, c.not)
		}
	}
}

func TestAdviceDoesNotHideARealAsk(t *testing.T) {
	if !has(Scan("Never share it with anyone else, just tell me the OTP"), scam.OTPRequest) {
		t.Error("a scammer borrowing the advice should still be caught")
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]scam.Category{
		"Main CBI se bol raha hoon, aapke naam par arrest warrant hai, video call par rahiye": scam.DigitalArrest,
		"FedEx courier parcel customs mein pakda gaya":                                        scam.Courier,
		"Part time job hai, YouTube task karne hain, no interview":                            scam.Job,
		"guaranteed profit, trading mein double ho jayega":                                    scam.Investment,
		"Aaj mausam accha hai": scam.General,
	}
	for text, want := range cases {
		if got := Classify(text); got != want {
			t.Errorf("Classify(%q) = %s, want %s", text, got, want)
		}
	}
}

func TestComplying(t *testing.T) {
	yes := []string{
		"1, 2, 3, 4।", // call #26
		"मैं तुम्हें UPI से payment कर दूंगी", // call #26
		"चार पांच एक दो",
		"4512",
		"my otp is 4512",
		"Theek hai, main paise bhej deta hoon",
		"I will send the money now",
		"AnyDesk install kar diya",
	}
	no := []string{
		"नहीं, मैं OTP नहीं दूंगी",
		"Main OTP nahi dungi",
		"I will not pay you anything",
		"50000 rupaye bhejo",
		"Call 1930 for help",
		"फ़ोन काट दीजिए, और 1930 पर कॉल कीजिए",
		"फ़ोन काट दीजिए, और एक नौ तीन शून्य पर कॉल कीजिए", // our own warning through the speaker
		"Mera number 9876543210 hai",
		"Aaj mausam accha hai",
		"Jo OTP aaya hai woh bata dijiye", // the caller's demand
	}
	for _, line := range yes {
		if !Complying(line) {
			t.Errorf("%q should count as complying", line)
		}
	}
	for _, line := range no {
		if Complying(line) {
			t.Errorf("%q should not count as complying", line)
		}
	}
}

// Every warning we can speak, heard back through the speaker, must not set this off.
func TestOurOwnWarningsAreNotComplying(t *testing.T) {
	for _, lang := range []string{"hi", "en", "ta", "te", "kn", "ml", "bn", "mr", "gu", "pa"} {
		for _, w := range scam.AllWarnings(lang) {
			for _, heard := range []string{w, scam.Spoken(w, lang)} {
				if Complying(heard) {
					t.Errorf("%s: our own warning counts as complying: %s", lang, heard)
				}
			}
		}
	}
}

// The transcript is what Prisma really wrote back for our Hindi warning (cmd/warningcheck).
func TestOurOwnVoiceIsRecognised(t *testing.T) {
	heard := "चौकन्ना AI की चेतावनी, यह call धोखाधड़ी लग रही है। कोई भी bank, police या company phone पर OTP, पैसे या screen share नहीं मांगती। कुछ मत बताइए, phone काट दीजिए और 1930 पर call कीजिए।"
	if !EchoOf(heard, scam.Spoken(scam.DefaultWarning("hi"), "hi")) {
		t.Error("our own warning should be recognised")
	}
	if EchoOf("OTP mat do", scam.DefaultWarning("en")) {
		t.Error("a short line is never treated as an echo")
	}
	if EchoOf("Sir aap apna OTP abhi mujhe bata dijiye warna account band ho jayega", scam.DefaultWarning("hi")) {
		t.Error("a caller's demand must not be thrown away")
	}
}

func TestDiscreditsWarning(t *testing.T) {
	yes := []string{
		"नहीं sir, ये वो नहीं है, scam नहीं है sir। मैं आपके branch से बोल रहा हूँ।",  // call #30
		"अबे ये मैं बोल रहा हूँ ना जो पहले warning आई वो तो दूसरे system ने दिया है।", // call #30
		"ये एक fraud call नहीं है दादी।", // call #29
		"Woh machine hai madam, usko band kar dijiye",
		"This is not a scam, it is a real call",
		"Wo chetavni jhoothi hai, dhyan mat dijiye",
	}
	no := []string{
		"नहीं जी, ये तो scam लग रहा है।", // Dadi was right, call #30
		"Haan beta, khana kha liya",
		"Bank mein system down hai aaj",
		"Mera app kaam nahi kar raha",
		"Fraud bahut badh gaye hain aajkal",
	}
	for _, line := range yes {
		if !DiscreditsWarning(line) {
			t.Errorf("%q should count as a talk-out", line)
		}
	}
	for _, line := range no {
		if DiscreditsWarning(line) {
			t.Errorf("%q should not count as a talk-out", line)
		}
	}
	for _, lang := range []string{"hi", "en"} {
		for _, w := range scam.AllWarnings(lang) {
			if DiscreditsWarning(w) {
				t.Errorf("our own line counts as a talk-out: %s", w)
			}
		}
	}
}

// Lines as Prisma wrote them on staged calls, where the rules missed or caught late.
func TestDemandsHeardOnStagedCalls(t *testing.T) {
	cases := []struct {
		text   string
		flag   scam.FlagType
		strong bool
	}{
		{"अभी हमारे पास है, case बनाने से पहले 50,000 दे दीजिए, छोड़ देंगे।", scam.MoneyMove, false},
		{"इस number पे Google Pay कर दीजिए।", scam.MoneyMove, false},
		{"please वापस भेज दीजिए, बहुत जरूरी है।", scam.MoneyMove, false},
		{"आपको मैंने एक link भेजा है, उस पर जाकर login करिए।", scam.LinkOrQR, true},
		{"link khol kar password daal dijiye", scam.LinkOrQR, true},
		{"link भेजा है, उस पर click कीजिए", scam.LinkOrQR, false},
	}
	for _, c := range cases {
		if !has(Scan(c.text), c.flag) {
			t.Errorf("%q should raise %s; got %+v", c.text, c.flag, Scan(c.text))
		}
		if c.strong && !strong(Scan(c.text), c.flag) {
			t.Errorf("%q should be a direct demand", c.text)
		}
	}
}

// A link or a returned parcel is ordinary, and must not be treated as a direct demand or a payment.
func TestOrdinaryLinksAndReturns(t *testing.T) {
	if strong(Scan("YouTube का link भेजा है, खोल के देखना"), scam.LinkOrQR) {
		t.Error("a family member sharing a video link is not a direct demand")
	}
	for _, text := range []string{
		"parcel वापस भेज दीजिए, refund मिल जाएगा",
		"saman wapas bhej dijiye",
		"Google Pay का नया version आ गया है",
		"beta photo wapas bhej do",
	} {
		if has(Scan(text), scam.MoneyMove) {
			t.Errorf("%q should not raise a money demand", text)
		}
	}
}
