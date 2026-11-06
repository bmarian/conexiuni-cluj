// "🏡 Home" → {icon: '🏡', name: 'Home'}: a first word made only of symbols is the place's icon.
export function placeIcon(label: string): {icon?: string; name: string} {
  const [first = '', ...rest] = label.trim().split(/\s+/)
  const name = rest.join(' ')
  if (!name || !/\p{So}/u.test(first) || /[\p{L}\p{N}]/u.test(first)) return {name: label.trim()}
  return {icon: first, name}
}
