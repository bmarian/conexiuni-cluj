import {ref} from 'vue'

export const ANDROID_APP_URL = 'https://github.com/bmarian/conexiuni-cluj-mobile/releases/latest/download/conexiuni-cluj.apk'

const MIN_ANDROID = 14
const HIDDEN_KEY = 'androidApp.hidden'

interface UserAgentData {
  platform: string
  getHighEntropyValues(hints: string[]): Promise<{ platformVersion?: string }>
}

/** Whether this is an Android phone new enough for the app. */
export async function canInstallAndroidApp(): Promise<boolean> {
  if (typeof navigator === 'undefined') return false
  const hints = (navigator as Navigator & { userAgentData?: UserAgentData }).userAgentData
  if (hints) {
    if (hints.platform !== 'Android') return false
    try {
      const {platformVersion} = await hints.getHighEntropyValues(['platformVersion'])
      return parseInt(platformVersion ?? '', 10) >= MIN_ANDROID
    } catch {
      return false
    }
  }
  // Chrome freezes this at "Android 10; K"; browsers without client hints report the real version.
  const match = /Android (\d+)/.exec(navigator.userAgent)
  return !!match && parseInt(match[1]!, 10) >= MIN_ANDROID
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

export const WEAR_APP_URL = 'https://github.com/bmarian/conexiuni-cluj-wearos/releases/latest/download/conexiuni-cluj-wear.apk'
export const WEAR_INSTALLER_URL = 'https://play.google.com/store/apps/details?id=org.freepoc.wearinstaller2'

export const androidAppDialog = ref(false)

export function openAndroidApp() {
  androidAppDialog.value = true
}

export interface AppRelease {
  version: string
  /** ISO date-time. */
  date: string
}

export type AppRepo = 'conexiuni-cluj-mobile' | 'conexiuni-cluj-wearos'

// GitHub allows 60 anonymous requests an hour per address.
const RELEASE_TTL_MS = 6 * 60 * 60_000

const releases = new Map<AppRepo, Promise<AppRelease | null>>()

function cached(repo: AppRepo): AppRelease | null {
  try {
    const saved = JSON.parse(localStorage.getItem(`release.${repo}`) ?? 'null') as (AppRelease & { at: number }) | null
    return saved && Date.now() - saved.at < RELEASE_TTL_MS ? {version: saved.version, date: saved.date} : null
  } catch {
    return null
  }
}

export function latestRelease(repo: AppRepo): Promise<AppRelease | null> {
  let release = releases.get(repo)
  if (!release) {
    const saved = cached(repo)
    release = saved ? Promise.resolve(saved) : fetch(`https://api.github.com/repos/bmarian/${repo}/releases/latest`)
      .then((r) => (r.ok ? r.json() : null))
      .then((json: { tag_name?: string, published_at?: string } | null) => {
        if (!json?.tag_name || !json.published_at) return null
        const found = {version: json.tag_name.replace(/^v/, ''), date: json.published_at}
        try {
          localStorage.setItem(`release.${repo}`, JSON.stringify({...found, at: Date.now()}))
        } catch {}
        return found
      })
      .catch(() => null)
    releases.set(repo, release)
  }
  return release
}
