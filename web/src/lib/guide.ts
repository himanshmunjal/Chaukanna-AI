// Whether this browser has seen the in-app guide. On a first visit the dashboard opens
// the guide by itself, so a new reviewer or family member knows where to start.
const GUIDE_SEEN = 'chaukanna_guide_seen'

export function guideSeen(): boolean {
  try {
    return localStorage.getItem(GUIDE_SEEN) === '1'
  } catch {
    return true // storage blocked: don't keep sending them back to the guide
  }
}

export function markGuideSeen() {
  try {
    localStorage.setItem(GUIDE_SEEN, '1')
  } catch {
    /* it may show again next time */
  }
}
