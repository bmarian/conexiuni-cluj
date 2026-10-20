// "P-ta" must not wrap at its hyphen, so swap in a non-breaking one.
export const keepHyphenatedWords = (text: string): string => text.replace(/(\S)-(\S)/g, '$1‑$2')

export const escapeHtml = (text: string): string =>
  text.replace(/[&<>"']/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'})[c]!)
