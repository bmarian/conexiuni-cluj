// iPadOS reports a Mac user agent.
export const isIOS = /iP(hone|od|ad)/.test(navigator.userAgent)
  || (/Macintosh/.test(navigator.userAgent) && navigator.maxTouchPoints > 1)

export const isIOSStandalone = isIOS && (navigator as Navigator & {standalone?: boolean}).standalone === true

// Since iOS 26 a Home Screen app's viewport can end above the home indicator, short by the status bar's height.
export function standaloneBottomCutoffPx(): number {
  if (!isIOSStandalone || !window.matchMedia('(orientation: portrait)').matches) return 0
  return Math.max(0, Math.max(screen.width, screen.height) - window.innerHeight)
}

export function lockIOSPageZoom() {
  if (!isIOS) return
  const viewport = document.querySelector<HTMLMetaElement>('meta[name="viewport"]')
  if (viewport && !viewport.content.includes('maximum-scale')) viewport.content += ', maximum-scale=1'
  const block = (e: Event) => e.preventDefault()
  document.addEventListener('gesturestart', block)
  document.addEventListener('gesturechange', block)
}
