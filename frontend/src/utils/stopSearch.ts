export const RESULTS_PER_KIND = 3

// 0 the name starts with the query, 1 the own name has it, 2 only the "(Town)" part does, -1 no match.
export function stopMatchRank(name: string, query: string): number {
  const at = name.indexOf(query)
  if (at < 0) return -1
  if (at === 0) return 0
  const own = name.indexOf(' (')
  return own < 0 || at < own ? 1 : 2
}

const WORDS_RANK = 3

const STREET_KINDS: Record<string, string> = {
  strada: 'strada',
  str: 'strada',
  bulevardul: 'bulevardul',
  bulevard: 'bulevardul',
  bd: 'bulevardul',
  bld: 'bulevardul',
  blvd: 'bulevardul',
  bdul: 'bulevardul',
  calea: 'calea',
  cal: 'calea',
  aleea: 'aleea',
  al: 'aleea',
  piata: 'piata',
  pta: 'piata',
  soseaua: 'soseaua',
  sos: 'soseaua',
  splaiul: 'splaiul',
  spl: 'splaiul',
  intrarea: 'intrarea',
  intr: 'intrarea',
  pasajul: 'pasajul',
  drumul: 'drumul',
}

const WORD = /[\p{L}\p{N}]+/gu
const CONTRACTED = /(?<![\p{L}\p{N}])(\p{L}{1,2})-(\p{L}{2,3})(?![\p{L}\p{N}])/gu

const ONE_EDIT_LETTERS = 4
const TWO_EDITS_LETTERS = 8

const allowedEdits = (letters: number) => (letters >= TWO_EDITS_LETTERS ? 2 : letters >= ONE_EDIT_LETTERS ? 1 : 0)

function queryWords(query: string): string[] {
  const words = (query.replaceAll('b-dul', 'bulevardul').replaceAll('p-ta', 'piata').match(WORD) ?? []).map((w) => STREET_KINDS[w] ?? w)
  const naming = words.filter((w) => !Object.values(STREET_KINDS).includes(w))
  return naming.length ? naming : words
}

export function editDistance(a: string, b: string, limit: number): number {
  if (Math.abs(a.length - b.length) > limit) return limit + 1
  let before: number[] = []
  let previous = Array.from({length: b.length + 1}, (_, j) => j)
  for (let i = 1; i <= a.length; i++) {
    const current = [i]
    let lowest = i
    for (let j = 1; j <= b.length; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1
      let edits = Math.min(previous[j]! + 1, current[j - 1]! + 1, previous[j - 1]! + cost)
      if (i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) edits = Math.min(edits, before[j - 2]! + 1)
      current[j] = edits
      lowest = Math.min(lowest, edits)
    }
    if (lowest > limit) return limit + 1
    before = previous
    previous = current
  }
  return previous[b.length]!
}

function misspells(word: string, nameWord: string): boolean {
  const allowed = allowedEdits(word.length)
  if (!allowed || /\p{N}/u.test(word)) return false
  return editDistance(word, nameWord, allowed) <= allowed || editDistance(word, nameWord.slice(0, word.length), allowed) <= allowed
}

// "M." stands for "mihai" only beside a word matched in full, or "marasti" would find it.
export function nameMatches(name: string, query: string, typos = false): boolean {
  const words = queryWords(query)
  if (!words.length) return false
  const found = [...name.matchAll(WORD)].map((m) => ({word: m[0], short: name[m.index + m[0].length] === '.'}))
  const contracted = [...name.matchAll(CONTRACTED)]
  let whole = false
  for (const word of words) {
    const full =
      found.some((f) => f.word.startsWith(word)) ||
      contracted.some((c) => word.startsWith(c[1]!) && word.endsWith(c[2]!)) ||
      (typos && found.some((f) => !f.short && misspells(word, f.word)))
    if (full) whole = true
    else if (!found.some((f) => f.short && word.startsWith(f.word))) return false
  }
  return whole
}

export function matching<T>(items: T[], nameOf: (item: T) => string, query: string): {item: T; rank: number}[] {
  const exact = items
    .map((item) => {
      const name = nameOf(item)
      const rank = stopMatchRank(name, query)
      return {item, rank: rank >= 0 || !nameMatches(name, query) ? rank : WORDS_RANK}
    })
    .filter((m) => m.rank >= 0)
  if (exact.length) return exact
  return items.filter((item) => nameMatches(nameOf(item), query, true)).map((item) => ({item, rank: WORDS_RANK}))
}
