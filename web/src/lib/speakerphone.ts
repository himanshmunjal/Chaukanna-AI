// Speakerphone mode: stream the laptop microphone to the backend and play back
// Chaukanna's spoken warnings through the laptop speakers.
//
//   mic -> AudioWorklet (8 kHz PCM) -> WebSocket /ws/speaker -> Prisma -> Evon
//   speakers <- WAV clips <- WebSocket <- Timbre warning
//
// While a clip is playing the microphone is NOT sent, so Chaukanna never
// transcribes (and reacts to) its own voice.

import { speakerSocketURL } from './api'

export interface SpeakerphoneOptions {
  userPhone: string
  mode: 'guardian' | 'screen'
  language?: string // demo visitors have no protected person, so they choose this
  gnaniKey?: string // demo visitors use their own key; sent as a message, never in the URL
  onStarted: (callId: number) => void
  onSpeaking: (speaking: boolean) => void
  onLevel: (level: number) => void // 0..1 microphone loudness, for the meter
  onError: (message: string) => void
  onMicSilent?: (silent: boolean) => void // the mic has heard nothing for a while
  onClosed: () => void
}

export interface SpeakerphoneSession {
  stop: () => void
}

/** Why the microphone can't be used here, or null if it can. */
export function microphoneProblem(): string | null {
  if (typeof window === 'undefined') return null
  if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
    return 'The browser only allows the microphone on https or localhost. On a phone, open the https address (for example from ngrok), not the laptop\'s http address.'
  }
  return null
}

/** How long the microphone may hear nothing before we say so. */
const SILENCE_WARNING_MS = 15000
/** Anything quieter than this counts as nothing being heard (16-bit peak, as 0..1). */
const SILENCE_LEVEL = 0.002

export async function startSpeakerphone(opts: SpeakerphoneOptions): Promise<SpeakerphoneSession> {
  const problem = microphoneProblem()
  if (problem) throw new Error(problem)

  // Created before any await, while the browser still counts this as the user's tap. Phone
  // browsers refuse to play sound from an audio graph started any other way, and that graph
  // is how the warning gets out.
  const ctx = new AudioContext()
  void ctx.resume()

  // 1) Microphone. Echo cancellation stops the laptop hearing its own speakers.
  let media: MediaStream
  try {
    media = await navigator.mediaDevices.getUserMedia({
      audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true, channelCount: 1 },
    })
  } catch (e) {
    void ctx.close()
    throw e
  }

  // 2) Audio graph: mic -> worklet (downsamples to 8 kHz).
  await ctx.audioWorklet.addModule('/pcm-worklet.js')
  const source = ctx.createMediaStreamSource(media)
  const worklet = new AudioWorkletNode(ctx, 'pcm-capture')
  source.connect(worklet)
  worklet.connect(ctx.destination) // the worklet outputs silence; this just keeps it running

  // A phone call can take the audio system away from the page: try to get it back.
  ctx.onstatechange = () => {
    if (ctx.state !== 'running' && ctx.state !== 'closed') void ctx.resume().catch(() => {})
  }

  // 3) WebSocket to the backend.
  const socket = new WebSocket(speakerSocketURL({ user_phone: opts.userPhone, mode: opts.mode, language: opts.language }))
  socket.binaryType = 'arraybuffer'
  socket.onopen = () => {
    if (opts.gnaniKey) socket.send(JSON.stringify({ type: 'auth', key: opts.gnaniKey }))
  }

  let muted = false
  let started = false // no audio until the server says it's ready, so the key goes first
  let stopped = false

  // The mic hearing nothing is a failure, not a quiet call: the phone's own call usually
  // takes the microphone away from a browser on the same phone. Nothing else would show it.
  let lastSound = Date.now()
  let silent = false
  const setSilent = (value: boolean) => {
    if (value !== silent) {
      silent = value
      opts.onMicSilent?.(value)
    }
  }
  media.getAudioTracks().forEach((track) => {
    track.onmute = () => setSilent(true)
    track.onunmute = () => {
      lastSound = Date.now()
      setSilent(false)
    }
    track.onended = () => {
      if (!stopped) opts.onError('The microphone was taken away from this page. Press Stop, then Start again.')
    }
  })

  worklet.port.onmessage = (event: MessageEvent<ArrayBuffer>) => {
    const samples = new Int16Array(event.data)
    let peak = 0
    for (let i = 0; i < samples.length; i++) peak = Math.max(peak, Math.abs(samples[i]))
    opts.onLevel(peak / 32768)

    const now = Date.now()
    if (peak / 32768 > SILENCE_LEVEL) {
      lastSound = now
      setSilent(false)
    } else if (!muted && now - lastSound > SILENCE_WARNING_MS) {
      setSilent(true)
    }

    if (!muted && started && socket.readyState === WebSocket.OPEN) socket.send(event.data)
  }

  // If the page stops getting audio at all, the checks above never run.
  const watchdog = window.setInterval(() => {
    if (!muted && Date.now() - lastSound > SILENCE_WARNING_MS) setSilent(true)
  }, 3000)

  // 4) Play warning clips one after another; keep the mic muted while playing.
  const queue: string[] = []
  let playing = false
  const finishedClip = () => {
    const next = queue.shift()
    if (next === undefined) {
      playing = false
      // Small tail so the end of the clip isn't picked up by the mic.
      window.setTimeout(() => {
        if (!playing) {
          muted = false
          lastSound = Date.now()
          opts.onSpeaking(false)
        }
      }, 400)
      return
    }
    playClip(next)
  }
  const playClip = (clip: string) => {
    playing = true
    muted = true
    opts.onSpeaking(true)
    playThroughGraph(ctx, clip).catch(() => {
      opts.onError('The warning could not be played aloud on this device. It is shown on the screen.')
    }).finally(finishedClip)
  }

  socket.onmessage = (msg) => {
    const m = JSON.parse(msg.data)
    if (m.type === 'started') {
      started = true
      lastSound = Date.now()
      opts.onStarted(m.call_id)
    }
    if (m.type === 'error') opts.onError(m.message)
    if (m.type === 'audio') {
      if (playing) queue.push(m.audio)
      else playClip(m.audio)
    }
  }

  const cleanup = () => {
    if (stopped) return
    stopped = true
    window.clearInterval(watchdog)
    media.getTracks().forEach((t) => {
      t.onended = null
      t.stop()
    })
    source.disconnect()
    worklet.disconnect()
    void ctx.close()
  }

  socket.onclose = () => {
    // Closed without pressing Stop: protection has ended, and they need to know.
    if (!stopped) opts.onError('The connection to Chaukanna was lost, so this call is no longer protected. Press Start listening again.')
    cleanup()
    opts.onClosed()
  }

  return {
    stop: () => {
      if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: 'stop' }))
      socket.close()
      cleanup() // sets stopped first, so the close isn't reported as a lost connection
    },
  }
}

/** Plays a base64 WAV through the audio graph, and resolves when it has finished. */
async function playThroughGraph(ctx: AudioContext, base64: string): Promise<void> {
  const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
  if (ctx.state !== 'running') await ctx.resume()
  const buffer = await ctx.decodeAudioData(bytes.buffer)
  await new Promise<void>((resolve) => {
    const clip = ctx.createBufferSource()
    clip.buffer = buffer
    clip.connect(ctx.destination)
    clip.onended = () => resolve()
    clip.start()
  })
}
