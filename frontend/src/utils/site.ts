export const SITE_ORIGIN = 'https://ceautobuziau.ro'

const SITE_HOST = new URL(SITE_ORIGIN).host

export const siteUrl = (path: string) => SITE_ORIGIN + path

// Shared links can still point at the old host.
export const isSiteHost = (host: string) => host === SITE_HOST || host === 'bus.bmarian.online'
