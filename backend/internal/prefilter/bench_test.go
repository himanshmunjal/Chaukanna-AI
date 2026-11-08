package prefilter

import "testing"

// The pre-filter runs on every transcript segment: it must stay well under 5 ms.
func BenchmarkScan(b *testing.B) {
	const segment = "नमस्ते, मैं CBI headquarters से inspector बोल रहा हूँ। आपके नाम पर arrest warrant है, video call पर रहिए और किसी को मत बताइए। case बंद करने के लिए 50000 रुपये safe account में transfer करना होगा।"
	for i := 0; i < b.N; i++ {
		Scan(segment)
	}
}
