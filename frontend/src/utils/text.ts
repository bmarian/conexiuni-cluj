export const fold = (s: string): string =>
  s
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLowerCase()

export const dateLocale = (locale: string): string => (locale === 'en' ? 'en-GB' : 'ro-RO')

export const escapeHtml = (text: string): string => text.replace(/[&<>"']/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'})[c]!)
