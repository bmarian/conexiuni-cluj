// "P-ta" must not wrap at its hyphen, so swap in a non-breaking one.
export const keepHyphenatedWords = (text: string): string => text.replace(/(\S)-(\S)/g, '$1‑$2')
