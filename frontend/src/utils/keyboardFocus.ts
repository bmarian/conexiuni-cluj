export type Axis = 'x' | 'y' | 'grid'
export type Edge = 'start' | 'end'

interface Run {
  section: HTMLElement | null
  items: HTMLElement[]
}

const ITEM = '[data-kbd-item]'
const SECTION = '[data-kbd-section]'
const NON_TEXT_INPUTS = new Set(['button', 'checkbox', 'radio', 'range', 'submit', 'reset', 'file', 'color', 'image'])

export function isTypingTarget(el: Element | null): boolean {
  if (!(el instanceof HTMLElement)) return false
  if (el.isContentEditable) return true
  if (el instanceof HTMLTextAreaElement) return true
  return el instanceof HTMLInputElement && !NON_TEXT_INPUTS.has(el.type)
}

export function isEmptyField(el: Element | null): boolean {
  return (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) && el.value === ''
}

export function isNativeActivatable(el: Element | null): boolean {
  return el instanceof HTMLButtonElement
    || el instanceof HTMLInputElement
    || el instanceof HTMLSelectElement
    || (el instanceof HTMLAnchorElement && el.hasAttribute('href'))
}

function isNavigable(el: HTMLElement): boolean {
  if ((el as HTMLButtonElement).disabled || el.closest('[inert]')) return false
  if (typeof el.checkVisibility === 'function') return el.checkVisibility({visibilityProperty: true})
  return el.getClientRects().length > 0
}

function isOn(el: HTMLElement, attr: string): boolean {
  const v = el.getAttribute(attr)
  return v !== null && v !== 'false'
}

export function getItems(scope: ParentNode): HTMLElement[] {
  return Array.from(scope.querySelectorAll<HTMLElement>(ITEM)).filter(isNavigable)
}

export function itemOf(el: Element | null, scope: Element): HTMLElement | null {
  const item = el?.closest<HTMLElement>(ITEM) ?? null
  return item && scope.contains(item) && isNavigable(item) ? item : null
}

export function findItem(scope: ParentNode, key: string): HTMLElement | null {
  const matches = scope.querySelectorAll<HTMLElement>(`[data-kbd-item="${CSS.escape(key)}"]`)
  return Array.from(matches).find(isNavigable) ?? null
}

function axisOf(section: Element | null): Axis {
  const axis = section?.getAttribute('data-kbd-axis')
  return axis === 'x' || axis === 'grid' ? axis : 'y'
}

function toRuns(items: HTMLElement[]): Run[] {
  const runs: Run[] = []
  for (const item of items) {
    const section = item.closest<HTMLElement>(SECTION)
    const last = runs[runs.length - 1]
    if (last && last.section === section) last.items.push(item)
    else runs.push({section, items: [item]})
  }
  return runs
}

function centerX(el: Element): number {
  const r = el.getBoundingClientRect()
  return r.left + r.width / 2
}

function nearestX(items: HTMLElement[], x: number): HTMLElement | undefined {
  let best: HTMLElement | undefined
  let bestDist = Infinity
  for (const el of items) {
    const d = Math.abs(centerX(el) - x)
    if (d < bestDist) {
      bestDist = d
      best = el
    }
  }
  return best
}

function rowAt(items: HTMLElement[], edge: Edge): HTMLElement[] {
  const tops = items.map(el => el.getBoundingClientRect().top)
  const target = edge === 'start' ? Math.min(...tops) : Math.max(...tops)
  return items.filter((_, i) => Math.abs(tops[i]! - target) < 4)
}

function gridStep(items: HTMLElement[], cur: HTMLElement, dir: 1 | -1): HTMLElement | undefined {
  const r = cur.getBoundingClientRect()
  const half = r.height / 2
  const candidates = items.filter(el => {
    const top = el.getBoundingClientRect().top
    return dir > 0 ? top > r.top + half : top < r.top - half
  })
  if (!candidates.length) return undefined
  return nearestX(rowAt(candidates, dir > 0 ? 'start' : 'end'), r.left + r.width / 2)
}

function activeIn(items: HTMLElement[]): HTMLElement | undefined {
  return items.find(el => isOn(el, 'data-kbd-active'))
}

function enterRun(run: Run, edge: Edge, x: number | null, preferActive: boolean): HTMLElement | undefined {
  const axis = axisOf(run.section)
  const active = activeIn(run.items)
  if (axis === 'x') return active ?? run.items[0]
  if (active && preferActive) return active
  if (axis === 'grid' && x !== null) return nearestX(rowAt(run.items, edge), x)
  return edge === 'start' ? run.items[0] : run.items[run.items.length - 1]
}

function locate(items: HTMLElement[], el: Element): { run: number, index: number } | null {
  const runs = toRuns(items)
  for (let r = 0; r < runs.length; r++) {
    const index = runs[r]!.items.indexOf(el as HTMLElement)
    if (index >= 0) return {run: r, index}
  }
  return null
}

function itemNear(items: HTMLElement[], el: Element, dir: 1 | -1): HTMLElement | undefined {
  const following = (item: HTMLElement) => !!(el.compareDocumentPosition(item) & Node.DOCUMENT_POSITION_FOLLOWING)
  return dir > 0 ? items.find(following) : [...items].reverse().find(item => !following(item))
}

export function stepVertical(scope: Element, from: Element | null, dir: 1 | -1): HTMLElement | undefined {
  const items = getItems(scope)
  const cur = from ? itemOf(from, scope) : null
  if (!cur) return from && scope.contains(from) ? itemNear(items, from, dir) : undefined
  const runs = toRuns(items)
  const pos = locate(items, cur)
  if (!pos) return undefined
  const run = runs[pos.run]!
  const axis = axisOf(run.section)
  if (axis === 'grid') {
    const next = gridStep(run.items, cur, dir)
    if (next) return next
  } else if (axis === 'y') {
    const next = run.items[pos.index + dir]
    if (next) return next
  }
  const nextRun = runs[pos.run + dir]
  return nextRun ? enterRun(nextRun, dir > 0 ? 'start' : 'end', centerX(cur), false) : cur
}

export function stepInline(scope: Element, from: Element | null, dir: 1 | -1): HTMLElement | undefined {
  const cur = from ? itemOf(from, scope) : null
  if (!cur) return undefined
  const items = getItems(scope)
  const pos = locate(items, cur)
  if (!pos) return undefined
  const run = toRuns(items)[pos.run]!
  if (axisOf(run.section) === 'y') return cur
  return run.items[pos.index + dir] ?? cur
}

export function stepSection(scope: Element, from: Element | null, dir: 1 | -1): HTMLElement | undefined {
  const items = getItems(scope)
  const runs = toRuns(items)
  if (!runs.length) return undefined
  const cur = from ? itemOf(from, scope) : null
  const pos = cur ? locate(items, cur) : null
  if (!pos) return undefined
  const next = runs[(pos.run + dir + runs.length) % runs.length]!
  return enterRun(next, 'start', null, true)
}

function entryOfSection(section: HTMLElement): HTMLElement | undefined {
  const items = getItems(section).filter(el => el.closest(SECTION) === section)
  return items.length ? enterRun({section, items}, 'start', null, true) : undefined
}

export function sectionEntry(scope: ParentNode, name: string): HTMLElement | undefined {
  const sections = scope.querySelectorAll<HTMLElement>(`[data-kbd-section="${CSS.escape(name)}"]`)
  for (const section of sections) {
    const item = entryOfSection(section)
    if (item) return item
  }
  return undefined
}

export function entryItem(scope: Element, bestRankOnly = false): HTMLElement | undefined {
  const entries = Array.from(scope.querySelectorAll<HTMLElement>('[data-kbd-entry]'))
    .map((el, i) => ({el, i, rank: Number(el.getAttribute('data-kbd-entry')) || 0}))
    .filter(({el}) => isOn(el, 'data-kbd-entry'))
    .sort((a, b) => a.rank - b.rank || a.i - b.i)
  for (const {el, rank} of entries) {
    if (bestRankOnly && rank > entries[0]!.rank) return undefined
    if (el.matches(ITEM)) {
      if (isNavigable(el)) return el
      continue
    }
    const item = el.matches(SECTION) ? entryOfSection(el) : undefined
    if (item) return item
  }
  return undefined
}

export function firstItem(scope: Element): HTMLElement | undefined {
  return getItems(scope)[0]
}

export function neighborOf(scope: Element, el: Element): HTMLElement | undefined {
  const items = getItems(scope)
  const pos = locate(items, el)
  if (!pos) return undefined
  const run = toRuns(items)[pos.run]!
  return run.items[pos.index + 1] ?? run.items[pos.index - 1]
}

function dropTempTabindex(this: HTMLElement) {
  if (this.hasAttribute('data-kbd-tabindex')) {
    this.removeAttribute('tabindex')
    this.removeAttribute('data-kbd-tabindex')
  }
}

export function focusElement(el: HTMLElement) {
  if (el.tabIndex < 0 && !el.hasAttribute('tabindex')) {
    el.setAttribute('tabindex', '-1')
    el.setAttribute('data-kbd-tabindex', '')
    el.addEventListener('blur', dropTempTabindex, {once: true})
  }
  el.focus({preventScroll: true})
}

export function focusItem(item: HTMLElement) {
  const inner = item.hasAttribute('data-kbd-inner')
    ? item.querySelector<HTMLElement>('input, select, textarea, button, [tabindex]')
    : null
  focusElement(inner ?? item)
  item.scrollIntoView({block: 'nearest', inline: 'nearest'})
}

export function activateItem(item: HTMLElement) {
  const target = item.querySelector<HTMLElement>('[data-kbd-click]') ?? item
  target.click()
}
