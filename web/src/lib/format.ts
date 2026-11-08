// Small display helpers shared by pages and components.

import type { CallState } from './api'
import type { Translate } from '../i18n'

/** Human labels for red-flag types (keys match backend/internal/scam). */
export const FLAG_INFO: Record<string, { label: string; icon: string }> = {
  authority_impersonation: { label: 'Fake authority', icon: '👮' },
  fear_threat: { label: 'Threat / fear', icon: '⛓️' },
  urgency: { label: 'Urgency', icon: '⏰' },
  isolation: { label: 'Isolation', icon: '📹' },
  secrecy: { label: 'Secrecy', icon: '🤫' },
  otp_request: { label: 'OTP / PIN request', icon: '🔢' },
  remote_access_app: { label: 'Remote-access app', icon: '🖥️' },
  money_transfer: { label: 'Money transfer', icon: '💸' },
  personal_data_request: { label: 'Personal data', icon: '🪪' },
  too_good_to_be_true: { label: 'Too-good offer', icon: '🎁' },
  link_or_qr: { label: 'Link / QR / UPI request', icon: '🔗' },
}

export function flagInfo(type: string) {
  return FLAG_INFO[type] ?? { label: type.replaceAll('_', ' '), icon: '🚩' }
}

/** Colours and labels for each call state. */
export const STATE_STYLE: Record<CallState, { label: string; badge: string; color: string }> = {
  listening: { label: 'Listening', badge: 'bg-emerald-500/10 text-emerald-300 ring-emerald-500/25', color: '#3f8a5f' },
  suspicious: { label: 'Suspicious', badge: 'bg-amber-500/10 text-amber-300 ring-amber-500/30', color: '#c98a1d' },
  scam: { label: 'Scam', badge: 'bg-red-500/10 text-red-300 ring-red-500/30', color: '#d2473a' },
  warned: { label: 'Warned', badge: 'bg-violet-500/10 text-violet-300 ring-violet-500/25', color: '#6b58b3' },
}

/** 74.2 -> "1:14" */
export function formatSeconds(t: number): string {
  const s = Math.max(0, Math.floor(t))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

/** ISO date -> "5 min ago" */
export function timeAgo(iso: string, t: Translate = (key, vars) => key.replace('{n}', String(vars?.n))): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return t('just now')
  if (diff < 3600) return t('{n} min ago', { n: Math.floor(diff / 60) })
  if (diff < 86400) return t('{n} h ago', { n: Math.floor(diff / 3600) })
  return new Date(iso).toLocaleDateString()
}

/** Each language in its own script, so people can find theirs. */
export const LANGUAGES: Record<string, string> = {
  hi: 'हिन्दी', en: 'English', ta: 'தமிழ்', te: 'తెలుగు', kn: 'ಕನ್ನಡ',
  ml: 'മലയാളം', bn: 'বাংলা', mr: 'मराठी', gu: 'ગુજરાતી', pa: 'ਪੰਜਾਬੀ',
}

export const SCAM_TYPES: Record<string, string> = {
  general: 'Phone fraud', digital_arrest: 'Digital arrest / fake police', courier: 'Courier or customs parcel',
  bank_kyc: 'Bank / KYC / OTP', relative_emergency: 'Relative in an emergency', prize: 'Lottery or prize',
  job_task: 'Job or task offer', investment: 'Investment or trading', loan: 'Instant loan',
  utility_sim: 'Electricity, gas or SIM cut-off', tech_support: 'Tech support / remote access',
  refund_upi: 'Refund or UPI', govt_benefit: 'Government benefit or insurance', charity: 'Fake donation',
}

/** The biggest words on the elder's alert screen, in their language. */
export const HANG_UP: Record<string, string> = {
  hi: 'फ़ोन काट दीजिए', en: 'Hang up now', ta: 'அழைப்பைத் துண்டியுங்கள்', te: 'ఫోన్ పెట్టేయండి', kn: 'ಕರೆ ಕಡಿತಗೊಳಿಸಿ',
  ml: 'ഫോൺ കട്ട് ചെയ്യൂ', bn: 'ফোন কেটে দিন', mr: 'फोन बंद करा', gu: 'ફોન કાપી નાખો', pa: 'ਫ਼ੋਨ ਕੱਟੋ',
}
