export const SITE_ORIGIN = 'https://ceautobuziau.ro'

const SITE_HOST = new URL(SITE_ORIGIN).host

export const siteUrl = (path: string) => SITE_ORIGIN + path

// The language the page was opened with (?lang=), which the server's page tags follow too.
export const urlLocale = (): 'ro' | 'en' | null => {
  const lang = new URLSearchParams(window.location.search).get('lang')
  return lang === 'ro' || lang === 'en' ? lang : null
}

// A ?lang=en page is its own canonical: the hreflang="en" alternate of the Romanian one.
export const canonicalUrl = (path: string) => siteUrl(path) + (urlLocale() === 'en' ? '?lang=en' : '')

// Shared links can still point at the old host.
export const isSiteHost = (host: string) => host === SITE_HOST || host === 'bus.bmarian.online'
