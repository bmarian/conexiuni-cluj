import {afterEach, describe, expect, it, vi} from 'vitest'

async function offered(userAgent: string, maxTouchPoints = 0) {
  vi.resetModules()
  vi.stubGlobal('navigator', {userAgent, maxTouchPoints})
  return (await import('@/utils/androidApp.ts')).offersAndroidApp
}

describe('android app offer', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows on Android and desktop', async () => {
    expect(await offered('Mozilla/5.0 (Linux; Android 14; Pixel 8)')).toBe(true)
    expect(await offered('Mozilla/5.0 (Windows NT 10.0; Win64; x64)')).toBe(true)
    expect(await offered('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)')).toBe(true)
  })

  it('stays away from iPhones and iPads', async () => {
    expect(await offered('Mozilla/5.0 (iPhone; CPU iPhone OS 26_0 like Mac OS X)')).toBe(false)
    expect(await offered('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', 5)).toBe(false)
  })
})
