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
