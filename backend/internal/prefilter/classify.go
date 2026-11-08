package prefilter

import (
	"regexp"
	"strings"

	"chaukanna/internal/scam"
)

// categoryWords says which scam family a call belongs to. Whichever family has the
// most distinct words in the transcript wins. Used for the scam-specific warning
// ("digital arrest does not exist"), the family alert and the cybercrime report.
var categoryWords = map[scam.Category][]string{
	scam.DigitalArrest: {"digital arrest", "cbi", "narcotics", "ncb", "cyber cell", "cyber crime", "crime branch", "arrest", "warrant", "giraftar",
		"money laundering", "video call", "police", "डिजिटल अरेस्ट", "सीबीआई", "गिरफ्तार", "वारंट", "पुलिस", "वीडियो कॉल", "मनी लॉन्ड्रिंग", "போலீஸ்", "পুলিশ"},
	scam.Courier: {"courier", "parcel", "fedex", "dhl", "customs", "shipment", "package", "कूरियर", "पार्सल", "कस्टम"},
	scam.BankKYC: {"kyc", "otp", "bank", "sbi", "hdfc", "icici", "debit card", "credit card", "cvv", "net banking", "aadhaar", "reward points",
		"केवाईसी", "बैंक", "ओटीपी", "आधार", "கேஒய்சி", "ஓடிபி", "ওটিপি"},
	scam.Relative: {"accident", "hospital", "papa", "beta", "uncle", "bhatija", "dad", "mom", "son", "custody", "bail", "doosre number", "phone kharab",
		"एक्सीडेंट", "अस्पताल", "पापा", "बेटा", "जमानत"},
	scam.Prize: {"lottery", "lucky draw", "kbc", "prize", "jackpot", "jeeta", "you won", "won", "लॉटरी", "इनाम", "जीता"},
	scam.Job:   {"job", "resume", "interview", "offer letter", "hr", "hr department", "part time", "work from home", "task", "tasks", "vip", "visa", "जॉब", "नौकरी"},
	scam.Investment: {"investment", "trading", "stock", "stocks", "profit", "guaranteed", "double", "doubles", "crypto", "usdt", "percent", "sebi",
		"निवेश", "ट्रेडिंग", "मुनाफा", "गारंटी"},
	scam.Loan:     {"loan", "cibil", "emi", "pre approved", "लोन"},
	scam.Utility:  {"electricity", "bijli", "meter", "gas", "sim", "connection", "fastag", "airtel", "bill", "बिजली", "मीटर", "गैस", "सिम", "कनेक्शन"},
	scam.TechHelp: {"anydesk", "teamviewer", "virus", "trojan", "microsoft", "screen share", "screen sharing", "remote support", "remotely", "एनीडेस्क"},
	scam.Refund:   {"refund", "cashback", "by mistake", "galti se", "wrong", "google pay", "amazon", "रिफंड", "गलती से"},
	scam.Benefit:  {"subsidy", "scholarship", "bonus", "maturity", "government", "ministry", "lic", "irdai", "सब्सिडी", "स्कॉलरशिप", "बोनस"},
	scam.Charity:  {"donation", "ngo", "relief", "charity", "flood", "दान", "चंदा"},
}

// categoryOrder decides ties: the more specific and dangerous family comes first.
var categoryOrder = []scam.Category{
	scam.DigitalArrest, scam.Courier, scam.Relative, scam.TechHelp, scam.Prize, scam.Job, scam.Investment,
	scam.Loan, scam.Charity, scam.Benefit, scam.Refund, scam.Utility, scam.BankKYC,
}

// Classify names the scam family of a transcript (scam.General if nothing matches).
func Classify(transcript string) scam.Category {
	padded := normalize(transcript)
	best, bestScore := scam.General, 0
	for _, c := range categoryOrder {
		score := 0
		for _, w := range categoryWords[c] {
			if strings.Contains(padded, normalize(w)) {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

// EchoOf reports whether heard is mostly the words of said: our own warning picked up
// again through a speaker. Short lines don't count, so a person saying "OTP mat do"
// after the warning is never thrown away.
func EchoOf(heard, said string) bool {
	heardWords := strings.Fields(normalize(heard))
	if len(heardWords) < 6 {
		return false
	}
	saidWords := map[string]bool{}
	for _, w := range strings.Fields(normalize(said)) {
		saidWords[w] = true
	}
	matched := 0
	for _, w := range heardWords {
		if saidWords[w] {
			matched++
		}
	}
	return float64(matched) >= 0.6*float64(len(heardWords))
}

// ---------------------------------------------------------------------------
// Is the person handing something over?
// ---------------------------------------------------------------------------

var (
	secretOrMoney = []string{
		"otp", "ओटीपी", "code", "कोड", "pin", "पिन", "cvv", "password", "पासवर्ड", "upi", "payment", "पेमेंट", "pay", "paise", "paisa", "पैसे", "पैसा",
		"money", "rupaye", "rupees", "रुपये", "रुपए", "transfer", "ट्रांसफर", "anydesk", "एनीडेस्क", "app", "ऐप", "qr", "क्यूआर", "card", "कार्ड",
	}
	handingOver = []string{
		"dungi", "dunga", "deti hoon", "deta hoon", "de rahi", "de raha", "de diya", "de di", "kar rahi hoon", "kar raha hoon", "karti hoon", "karta hoon",
		"kar deti", "kar deta", "kar diya", "kar di", "bhej rahi", "bhej raha", "bhej deti", "bhej deta", "bhej diya", "bhej dungi", "bhej dunga",
		"bata rahi", "bata raha", "bata deti", "bata deta", "bata diya",
		"दूंगी", "दूंगा", "दूँगी", "दूँगा", "देती हूं", "देता हूं", "देती हूँ", "देता हूँ", "दे रही", "दे रहा", "दे दिया", "दे दी",
		"कर रही हूं", "कर रहा हूं", "कर रही हूँ", "कर रहा हूँ", "कर देती", "कर देता", "कर दिया", "कर दी",
		"भेज रही", "भेज रहा", "भेज देती", "भेज देता", "भेज दिया", "बता रही", "बता रहा", "बता देती", "बता देता", "बता दिया",
		"i will send", "i will pay", "i will share", "i will tell", "i'll send", "i'll pay", "i am sending", "i am paying", "i have sent", "i have paid",
		"my otp is", "the otp is", "the code is", "mera otp", "मेरा ओटीपी", "मेरा otp",
	}
	refusing = []string{
		"nahi", "nahin", "नहीं", "mat", "मत", "not", "won't", "wont", "never", "don't", "na dungi", "na dunga", "ना दूंगी", "ना दूंगा",
	}
	numberWords = `(?:zero|one|two|three|four|five|six|seven|eight|nine|ek|do|teen|char|paanch|chhah|saat|aath|nau|शून्य|एक|दो|तीन|चार|पांच|पाँच|छह|सात|आठ|नौ)`
	// Digits read one at a time ("1, 2, 3, 4" or "चार पांच एक दो"), or one short number ("4512").
	spokenDigits = regexp.MustCompile(` (?:(?:[0-9०-९]|` + numberWords + `) ){4,8}`)
	shortNumber  = regexp.MustCompile(` [0-9०-९]{4,8} `)
)

var digitToken = regexp.MustCompile(`^[0-9०-९]{1,8}$`)
var numberWord = regexp.MustCompile(`^` + numberWords + `$`)

// DigitRun counts the digits read out in a row in a line, and whether the line holds
// nothing else. People read an OTP slowly, so speech recognition may split it over
// several lines, and "1 2 3" alone is short. The detector adds these up across lines.
func DigitRun(text string) (digits int, onlyDigits bool) {
	padded := withoutHelpline(text)
	onlyDigits = true
	run := 0
	for _, word := range strings.Fields(padded) {
		switch {
		case digitToken.MatchString(word):
			run += len([]rune(word))
		case numberWord.MatchString(word):
			run++
		default:
			onlyDigits = false
			run = 0
		}
		digits = max(digits, run)
	}
	return digits, onlyDigits && digits > 0
}

// withoutHelpline normalises text and removes the helpline number: saying it is the
// right answer, and our own warning may be heard through the speaker.
func withoutHelpline(text string) string {
	padded := normalize(text)
	for _, s := range []string{"1930", "१९३०"} {
		padded = strings.ReplaceAll(padded, " "+s+" ", " ")
	}
	for _, spoken := range scam.HelplineSpoken() {
		padded = strings.ReplaceAll(padded, normalize(spoken), " ")
	}
	return padded
}

var sentenceEnd = regexp.MustCompile(`[.?!।|]+`)

var (
	notAScam = []string{
		"scam nahi", "scam nahin", "scam नहीं", "scam call nahi", "scam call नहीं", "fraud nahi", "fraud nahin", "fraud नहीं",
		"fraud call nahi", "fraud call नहीं", "scammer nahi", "scammer नहीं", "धोखाधड़ी नहीं", "धोखा नहीं", "फ्रॉड नहीं", "स्कैम नहीं",
		"not a scam", "not scam", "not a fraud", "not fraud", "not a scammer",
		"real call", "genuine call", "asli call", "असली कॉल", "असली call", "official call", "this is genuine", "this call is real",
	}
	warningWords = []string{"warning", "वार्निंग", "chetavni", "चेतावनी", "machine", "मशीन", "system", "सिस्टम", "app", "ऐप", "robot", "रोबोट", "ai"}
	dismissals   = []string{
		"galat", "गलत", "jhooth", "jhoothi", "jhoot", "झूठ", "झूठी", "bakwas", "बकवास", "ignore", "band kar", "बंद कर", "mat sun", "मत सुन",
		"dhyan mat", "ध्यान मत", "doosre", "dusre", "दूसरे", "wrong", "false", "don't listen", "do not listen", "turn off", "switch off",
	}
)

// DiscreditsWarning reports whether a line tries to talk the person out of the warning:
// "ye scam nahi hai", "wo warning jhoothi hai", "machine ko band kar do".
func DiscreditsWarning(text string) bool {
	if containsAny(normalize(text), notAScam) {
		return true
	}
	// In one sentence: our own warnings mention both a warning and something false, apart.
	for _, sentence := range sentenceEnd.Split(text, -1) {
		padded := normalize(sentence)
		if containsAny(padded, warningWords) && containsAny(padded, dismissals) {
			return true
		}
	}
	return false
}

// AsksForSomething reports whether a line demands a code, app or money. Each sentence is
// judged alone, so "OTP nahi dungi" isn't a demand, but a scammer's "Ye fraud nahi hai.
// OTP bataiye." still is: the "nahi" there isn't refusing anything.
func AsksForSomething(text string) bool {
	for _, sentence := range sentenceEnd.Split(text, -1) {
		if containsAny(normalize(sentence), refusing) {
			continue
		}
		for _, hit := range Scan(sentence) {
			// A bare "OTP?" may be the person puzzling over it. A code needs a real demand.
			if hit.Strong || (scam.IsExtraction(hit.Type) && hit.Type != scam.OTPRequest) {
				return true
			}
		}
	}
	return false
}

// Complying reports whether a line sounds like someone giving away a code or money.
// A refusal never counts. Both voices are mixed on the line, so this can also match the
// caller's words, which is why it only counts on a call that is already doubtful.
func Complying(text string) bool {
	padded := withoutHelpline(text)
	if spokenDigits.MatchString(padded) {
		return true
	}
	if containsAny(padded, refusing) {
		return false
	}
	if shortNumber.MatchString(padded) && !containsAny(padded, moneyNouns) && !containsAny(padded, amountVerbs) {
		return true
	}
	return containsAny(padded, secretOrMoney) && containsAny(padded, handingOver)
}
