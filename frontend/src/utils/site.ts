// Overridable to try the host move locally on two *.localhost names.
export const SITE_ORIGIN: string = import.meta.env.VITE_SITE_ORIGIN || 'https://ceautobuziau.ro'
export const LEGACY_HOST: string = import.meta.env.VITE_LEGACY_HOST || 'bus.bmarian.online'

const SITE_HOST = new URL(SITE_ORIGIN).host

export const siteUrl = (path: string) => SITE_ORIGIN + path

// Shared links can still point at the legacy host.
export const isSiteHost = (host: string) => host === SITE_HOST || host === LEGACY_HOST
