// 0 the name starts with the query, 1 the own name has it, 2 only the "(Town)" part does, -1 no match.
export function stopMatchRank(name: string, query: string): number {
  const at = name.indexOf(query)
  if (at < 0) return -1
  if (at === 0) return 0
  const own = name.indexOf(' (')
  return own < 0 || at < own ? 1 : 2
}
