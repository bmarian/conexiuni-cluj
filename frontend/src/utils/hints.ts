// One-time tips shown in the snackbar when someone's own habits show a feature would help.
// Counted on the device only; each tip shows once, and no more than one a day.

export type HintId = 'save' | 'group' | 'remind'

export type HintState = {
  shown: HintId[]
  lastDay?: string
  // Distinct days, newest last, keyed by "stop", "stop@hour" and "stop>line".
  days: Record<string, string[]>
}

export type HintLine = {routeId: number; direction: '0' | '1'; platform: number}

export type Hint = {id: 'save'} | {id: 'group'; lines: [HintLine, HintLine]} | {id: 'remind'}

export const SAVE_DAYS = 3
export const GROUP_DAYS = 2
export const REMIND_DAYS = 3
const KEPT_DAYS = Math.max(SAVE_DAYS, GROUP_DAYS, REMIND_DAYS)

export const emptyHints = (): HintState => ({shown: [], days: {}})

const lineKey = (stop: string, line: HintLine) => `${stop}>${line.routeId}:${line.direction}:${line.platform}`

function parseLine(key: string): HintLine | null {
  const [routeId, direction, platform] = key.slice(key.indexOf('>') + 1).split(':')
  if (direction !== '0' && direction !== '1') return null
  return {routeId: Number(routeId), direction, platform: Number(platform)}
}

function withDay(state: HintState, key: string, day: string): HintState {
  const days = state.days[key] ?? []
  if (days.includes(day)) return state
  return {...state, days: {...state.days, [key]: [...days, day].slice(-KEPT_DAYS)}}
}

export const visitStop = (state: HintState, stop: string, day: string, hour: number) => withDay(withDay(state, stop, day), `${stop}@${hour}`, day)

export const openLineFromStop = (state: HintState, stop: string, line: HintLine, day: string) => withDay(state, lineKey(stop, line), day)

export type HintContext = {
  stop: string
  day: string
  hour: number
  saved: boolean
  // Whether these lines are already together in a saved group.
  grouped: (a: HintLine, b: HintLine) => boolean
  canRemind: boolean
}

export function pickHint(state: HintState, ctx: HintContext): Hint | null {
  if (state.lastDay === ctx.day) return null
  const free = (id: HintId) => !state.shown.includes(id)
  if (free('save') && !ctx.saved && (state.days[ctx.stop]?.length ?? 0) >= SAVE_DAYS) return {id: 'save'}
  if (free('group')) {
    const often = Object.entries(state.days)
      .filter(([key, days]) => key.startsWith(`${ctx.stop}>`) && days.length >= GROUP_DAYS)
      .sort(([, a], [, b]) => b.at(-1)!.localeCompare(a.at(-1)!))
      .flatMap(([key]) => parseLine(key) ?? [])
      .filter((line, i, all) => all.findIndex((l) => l.routeId === line.routeId) === i)
    const [a, b] = often
    if (a && b && !ctx.grouped(a, b)) return {id: 'group', lines: [a, b]}
  }
  if (free('remind') && ctx.canRemind && (state.days[`${ctx.stop}@${ctx.hour}`]?.length ?? 0) >= REMIND_DAYS) return {id: 'remind'}
  return null
}

export const markShown = (state: HintState, id: HintId, day: string): HintState => ({...state, shown: [...state.shown, id], lastDay: day})

export function readHints(raw: unknown): HintState {
  if (!raw || typeof raw !== 'object') return emptyHints()
  const r = raw as Partial<HintState>
  const shown = Array.isArray(r.shown) ? r.shown.filter((id): id is HintId => id === 'save' || id === 'group' || id === 'remind') : []
  const days: Record<string, string[]> = {}
  if (r.days && typeof r.days === 'object') {
    for (const [key, list] of Object.entries(r.days)) if (Array.isArray(list)) days[key] = list.filter((d): d is string => typeof d === 'string')
  }
  return {shown, lastDay: typeof r.lastDay === 'string' ? r.lastDay : undefined, days}
}
