// Play only lets members of this group into the closed test.
export const BETA_GROUP_URL = 'https://groups.google.com/g/conexiuni-cluj-beta'
export const PLAY_URL = 'https://play.google.com/store/apps/details?id=online.bmarian.bus'

const HIDDEN_KEY = 'androidBeta.hidden'

export function isAndroid(): boolean {
  if (typeof navigator === 'undefined') return false
  const hints = (navigator as Navigator & { userAgentData?: { platform: string } }).userAgentData
  return hints ? hints.platform === 'Android' : /Android/.test(navigator.userAgent)
}

export function androidAppHidden(): boolean {
  try {
    return localStorage.getItem(HIDDEN_KEY) === 'true'
  } catch {
    return false
  }
}

export function hideAndroidApp() {
  try {
    localStorage.setItem(HIDDEN_KEY, 'true')
  } catch {}
}
