export const SITE_ORIGIN = 'https://ceautobuziau.ro'

const SITE_HOST = new URL(SITE_ORIGIN).host

export const siteUrl = (path: string) => SITE_ORIGIN + path

export const urlLocale = (): 'ro' | 'en' | null => {
  const lang = new URLSearchParams(window.location.search).get('lang')
  return lang === 'ro' || lang === 'en' ? lang : null
}

// Same rule as selfURL in backend/pages/og.go.
export const canonicalUrl = (path: string) => siteUrl(path) + (urlLocale() === 'en' ? '?lang=en' : '')

// Shared links can still point at the old host.
export const isSiteHost = (host: string) => host === SITE_HOST || host === 'bus.bmarian.online'
