// Typed client for the Go backend. Every request the dashboard makes goes through here.

const API_URL: string = import.meta.env.VITE_API_URL ?? ''
const TOKEN: string = import.meta.env.VITE_DASHBOARD_TOKEN ?? ''

// ---------- Types (mirror backend/internal/models) ----------

export type CallState = 'listening' | 'suspicious' | 'scam' | 'warned'

export interface ProtectedUser {
  id: number
  name: string
  phone: string
  language: string
  family_name: string
  family_phone: string
  consent_at?: string | null
  has_voice?: boolean
  voice_owner?: string
  telegram_linked?: boolean
}

export interface TranscriptLine {
  id?: number
  t: number // seconds since call start
  speaker: string // call | caller | chaukanna
  text: string
}

export interface RedFlag {
  id?: number
  type: string
  t: number
  quote: string
  source?: string // prefilter | evon
}

export interface Alert {
  id: number
  channel: string
  recipient: string
  status: string
  error: string
  created_at: string
}

export interface Call {
  id: number
  provider: string
  mode: 'guardian' | 'screen'
  caller_number: string
  state: CallState
  risk: number
  reason: string
  language: string
  scam_type: string
  warning_played: boolean
  cautioned_at: string | null
  complying_at: string | null
  marked_safe_at: string | null
  outcome: string
  test_label: '' | 'scam' | 'legit'
  test_category: string
  caller_name: string
  purpose: string
  summary: string
  started_at: string
  ended_at: string | null
  warned_at: string | null
  detected_after_sec: number | null
  protected_user?: ProtectedUser
  transcript?: TranscriptLine[]
  flags?: RedFlag[]
  alerts?: Alert[]
}

export interface Stats {
  total_calls: number
  scams_detected: number
  warnings_played: number
  alerts_sent: number
  avg_detection_sec: number | null
  active_calls: number
  labelled_scam?: number
  labelled_legit?: number
}

export interface ScamNumber {
  id: number
  phone: string
  reports: number
  blocked: number
  source: 'auto' | 'manual'
  last_reason: string
  last_call_id: number | null
  updated_at: string
}

export interface Scenario {
  id: string
  title: string
  label: 'scam' | 'legit'
  language: string
  lines: TranscriptLine[]
}

export interface CallReport {
  scam_type: string
  scam_type_label: string
  steps: string[]
  money_lost_steps: string[]
  links: { label: string; url: string }[]
  draft: string
}

export interface CheckResult {
  verdict: 'scam' | 'caution' | 'safe'
  risk: number
  reason: string
  advice: string
  flags: { type: string; quote: string }[]
  transcript: string
  scam_type_label: string
  links?: { label: string; url: string }[]
}

export interface DrillScenario {
  id: string
  title: string
  language: string
}

export interface DrillTurn {
  total: number
  step?: number
  line?: string
  audio?: string
  heard?: string
  verdict?: 'safe' | 'risky' | 'unclear'
  lesson?: string
  done?: boolean
}

/** A message pushed by the backend over /ws/dashboard (see backend/internal/hub). */
export interface LiveEvent {
  type: string
  call_id: number
  data: any // shape depends on `type`
}

// ---------- HTTP ----------

// A demo visitor's own Gnani key. It lives only in this browser tab, goes only to this
// server, and the server never stores it.
const KEY_STORAGE = 'chaukanna-gnani-key'

export function getGnaniKey(): string {
  try {
    return sessionStorage.getItem(KEY_STORAGE) ?? ''
  } catch {
    return ''
  }
}

export function setGnaniKey(key: string) {
  try {
    if (key) sessionStorage.setItem(KEY_STORAGE, key)
    else sessionStorage.removeItem(KEY_STORAGE)
  } catch {
    // Storage blocked: the key simply won't be remembered.
  }
}

function authHeaders(): Record<string, string> {
  const key = getGnaniKey()
  return {
    ...(TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {}),
    ...(key ? { 'X-Gnani-Key': key } : {}),
  }
}

/** Fired when the server says the login has expired, so the app can show the login page. */
export const LOGGED_OUT_EVENT = 'chaukanna-logged-out'

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(API_URL + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders(),
      ...init.headers,
    },
  })
  if (res.status === 401 && !path.includes('/login')) window.dispatchEvent(new Event(LOGGED_OUT_EVENT))
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Request failed (${res.status})`)
  }
  return res.status === 204 ? (undefined as T) : res.json()
}

/** Posts a file. The browser sets the multipart Content-Type itself. */
async function upload<T>(path: string, form: FormData, method = 'POST'): Promise<T> {
  const res = await fetch(API_URL + path, {
    method,
    body: form,
    headers: authHeaders(),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Request failed (${res.status})`)
  }
  return res.json()
}

export interface Access {
  family: boolean
  demo: boolean
  login_required: boolean
  can_login: boolean
  logged_in: boolean
  /** Demo visitors may listen on the server's Gnani key, within a daily allowance. */
  shared_key?: boolean
}

export const api = {
  me: () => request<Access>('/api/me'),
  login: (password: string) => request<{ ok: boolean }>('/api/login', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request<{ ok: boolean }>('/api/logout', { method: 'POST' }),

  stats: () => request<Stats>('/api/stats'),
  calls: () => request<Call[]>('/api/calls'),
  call: (id: number | string) => request<Call>(`/api/calls/${id}`),
  activeCalls: () => request<{ ids: number[] }>('/api/calls/active'),

  users: () => request<ProtectedUser[]>('/api/users'),
  createUser: (u: Omit<ProtectedUser, 'id'> & { consent: boolean }) =>
    request<ProtectedUser>('/api/users', { method: 'POST', body: JSON.stringify(u) }),
  deleteUser: (id: number) => request<void>(`/api/users/${id}`, { method: 'DELETE' }),

  scamNumbers: () => request<ScamNumber[]>('/api/scam-numbers'),
  reportScamNumber: (phone: string, reason: string) =>
    request<ScamNumber>('/api/scam-numbers', { method: 'POST', body: JSON.stringify({ phone, reason }) }),
  unblockScamNumber: (id: number) => request<void>(`/api/scam-numbers/${id}`, { method: 'DELETE' }),

  recordConsent: (id: number) => request<ProtectedUser>(`/api/users/${id}/consent`, { method: 'POST' }),
  enrollVoice: (id: number, wav: Blob, owner: string) => {
    const form = new FormData()
    form.append('audio_file', wav, 'voice.wav')
    form.append('owner', owner)
    form.append('consent', 'true')
    return upload<{ voice_owner: string; sample_audio?: string }>(`/api/users/${id}/voice`, form)
  },
  deleteVoice: (id: number) => request<void>(`/api/users/${id}/voice`, { method: 'DELETE' }),
  telegramLink: (id: number) => request<{ enabled: boolean; linked?: boolean; url?: string }>(`/api/users/${id}/telegram-link`),

  warnNow: (id: number) => request<{ ok: boolean }>(`/api/calls/${id}/warn`, { method: 'POST' }),
  markCallSafe: (id: number, safe: boolean) =>
    request<{ safe: boolean }>(`/api/calls/${id}/safe`, { method: 'POST', body: JSON.stringify({ safe }) }),
  speakIntoCall: (id: number, text: string) =>
    request<{ played: boolean }>(`/api/calls/${id}/speak`, { method: 'POST', body: JSON.stringify({ text }) }),
  setOutcome: (id: number, outcome: string) =>
    request<{ outcome: string }>(`/api/calls/${id}/outcome`, { method: 'POST', body: JSON.stringify({ outcome }) }),
  labelCall: (id: number, label: string, category: string) =>
    request<{ test_label: string; test_category: string }>(`/api/calls/${id}/label`, { method: 'POST', body: JSON.stringify({ label, category }) }),
  callReport: (id: number) => request<CallReport>(`/api/calls/${id}/report`),

  checkText: (text: string, language: string) =>
    request<CheckResult>('/api/analyze', { method: 'POST', body: JSON.stringify({ text, language }) }),
  checkFile: (file: File, language: string) => {
    const form = new FormData()
    form.append('audio_file', file)
    form.append('language', language)
    return upload<CheckResult>('/api/analyze', form)
  },

  drillScenarios: () => request<{ scenarios: DrillScenario[]; voice: boolean; listen: boolean }>('/api/drill/scenarios'),
  drillSay: (scenario: string, step: number, reply: string) =>
    request<DrillTurn>('/api/drill/turn', { method: 'POST', body: JSON.stringify({ scenario, step, reply }) }),
  drillSpeak: (scenario: string, step: number, wav: Blob) => {
    const form = new FormData()
    form.append('scenario', scenario)
    form.append('step', String(step))
    form.append('audio_file', wav, 'reply.wav')
    return upload<DrillTurn>('/api/drill/turn', form)
  },

  scenarios: () => request<Scenario[]>('/api/scenarios'),
  simulate: (body: { scenario_id?: string; lines?: TranscriptLine[]; mode: string; speed: number; caller_number?: string; language?: string }) =>
    request<{ call_id: number }>('/api/simulate', { method: 'POST', body: JSON.stringify(body) }),
}

/** Builds a ws:// or wss:// URL to the backend, adding the dashboard token. */
function socketURL(path: string, params: Record<string, string> = {}): string {
  const url = new URL(path, API_URL || window.location.origin)
  url.protocol = url.protocol.replace('http', 'ws')
  for (const [key, value] of Object.entries(params)) if (value) url.searchParams.set(key, value)
  if (TOKEN) url.searchParams.set('token', TOKEN)
  return url.toString()
}

/** WebSocket URL for live events. */
export const dashboardSocketURL = () => socketURL('/ws/dashboard')

/** WebSocket URL for speakerphone mode (microphone audio in, warnings out). */
export const speakerSocketURL = (params: { user_phone: string; mode: string; language?: string }) => socketURL('/ws/speaker', params)
