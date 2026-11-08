// Records the microphone as mono WAV: a format Prisma and voice cloning accept without
// conversion (browser MediaRecorder gives WebM, which they do not list). It records at the
// device's own rate, because some browsers refuse a microphone in a context at another rate.

export interface WavRecording {
  /** Stops recording and returns the WAV file. */
  stop: () => Blob
  /** Stops and throws the audio away. */
  cancel: () => void
}

export async function recordWav(onLevel?: (level: number) => void): Promise<WavRecording> {
  if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
    throw new Error('The browser only allows the microphone on https or localhost.')
  }
  const ctx = new AudioContext() // before any await, while this still counts as the user's tap
  void ctx.resume()
  const media = await navigator.mediaDevices.getUserMedia({
    audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true, channelCount: 1 },
  })
  const source = ctx.createMediaStreamSource(media)
  const node = ctx.createScriptProcessor(4096, 1, 1)
  const chunks: Float32Array[] = []

  node.onaudioprocess = (e) => {
    const samples = new Float32Array(e.inputBuffer.getChannelData(0))
    chunks.push(samples)
    if (onLevel) {
      let peak = 0
      for (const v of samples) peak = Math.max(peak, Math.abs(v))
      onLevel(peak)
    }
  }
  source.connect(node)
  node.connect(ctx.destination) // the node only runs while connected; its output is silent

  const release = () => {
    node.disconnect()
    source.disconnect()
    media.getTracks().forEach((t) => t.stop())
    void ctx.close()
  }
  return {
    stop: () => {
      release()
      return encodeWav(chunks, ctx.sampleRate)
    },
    cancel: release,
  }
}

function encodeWav(chunks: Float32Array[], rate: number): Blob {
  const count = chunks.reduce((n, c) => n + c.length, 0)
  const view = new DataView(new ArrayBuffer(44 + count * 2))
  const text = (at: number, s: string) => [...s].forEach((ch, i) => view.setUint8(at + i, ch.charCodeAt(0)))
  text(0, 'RIFF')
  view.setUint32(4, 36 + count * 2, true)
  text(8, 'WAVEfmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true) // PCM
  view.setUint16(22, 1, true) // mono
  view.setUint32(24, rate, true)
  view.setUint32(28, rate * 2, true)
  view.setUint16(32, 2, true)
  view.setUint16(34, 16, true)
  text(36, 'data')
  view.setUint32(40, count * 2, true)
  let at = 44
  for (const chunk of chunks) {
    for (const v of chunk) {
      view.setInt16(at, Math.max(-1, Math.min(1, v)) * 0x7fff, true)
      at += 2
    }
  }
  return new Blob([view], { type: 'audio/wav' })
}

/** Plays a base64 WAV and resolves when it ends (or cannot play). */
export function playWav(base64: string): Promise<void> {
  return new Promise((resolve) => {
    const el = new Audio(`data:audio/wav;base64,${base64}`)
    el.onended = () => resolve()
    el.onerror = () => resolve()
    el.play().catch(() => resolve())
  })
}
