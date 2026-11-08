// AudioWorklet that turns microphone audio into phone-quality PCM:
// 8 kHz, mono, 16-bit signed integers, posted in 100 ms chunks (800 samples).
//
// The browser's microphone usually runs at 44.1 or 48 kHz. We downsample by
// averaging the input samples that fall into each output sample (a simple
// low-pass filter), which is plenty for speech recognition.

const TARGET_RATE = 8000
const CHUNK = 800 // 100 ms at 8 kHz

class PcmCapture extends AudioWorkletProcessor {
  constructor() {
    super()
    this.step = sampleRate / TARGET_RATE // input samples per output sample
    this.progress = 0
    this.sum = 0
    this.count = 0
    this.out = new Int16Array(CHUNK)
    this.filled = 0
  }

  process(inputs) {
    const input = inputs[0] && inputs[0][0]
    if (!input) return true

    for (let i = 0; i < input.length; i++) {
      this.sum += input[i]
      this.count++
      this.progress++
      if (this.progress >= this.step) {
        this.progress -= this.step
        const v = Math.max(-1, Math.min(1, this.sum / this.count))
        this.sum = 0
        this.count = 0
        this.out[this.filled++] = v < 0 ? v * 0x8000 : v * 0x7fff
        if (this.filled === CHUNK) {
          const chunk = this.out
          this.port.postMessage(chunk.buffer, [chunk.buffer])
          this.out = new Int16Array(CHUNK)
          this.filled = 0
        }
      }
    }
    return true
  }
}

registerProcessor('pcm-capture', PcmCapture)
