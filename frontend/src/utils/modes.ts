import type {RouteType} from '@/types/tranzy.ts'

export type Mode = 'metro' | 'rail' | 'tram' | 'trolleybus' | 'bus'

// Biggest first: a station is named after the biggest thing that stops there, and lists group in this order.
export const MODES: Mode[] = ['metro', 'rail', 'tram', 'trolleybus', 'bus']

// The legacy blue theme draws in emoji.
export const MODE_EMOJI: Record<Mode, string> = {metro: '🚇', rail: '🚆', tram: '🚋', trolleybus: '🚎', bus: '🚌'}

export function modeOf(routeType: RouteType | number | undefined): Mode {
  switch (routeType) {
    case 1:
    case 12:
      return 'metro'
    case 2:
      return 'rail'
    case 0:
    case 5:
      return 'tram'
    case 11:
      return 'trolleybus'
    default:
      return 'bus'
  }
}

export function mainMode(routeTypes: Iterable<RouteType | number>): Mode {
  let best = MODES.length - 1
  for (const type of routeTypes) best = Math.min(best, MODES.indexOf(modeOf(type)))
  return MODES[best]!
}

type Line = {route_type: RouteType | number, route_short_name: string}

// By mode, then as the numbers read.
export function byModeThenNumber(a: Line, b: Line): number {
  return MODES.indexOf(modeOf(a.route_type)) - MODES.indexOf(modeOf(b.route_type))
    || a.route_short_name.localeCompare(b.route_short_name, undefined, {numeric: true})
}

export function groupByMode<T extends Line>(lines: T[]): Array<{mode: Mode, lines: T[]}> {
  const out: Array<{mode: Mode, lines: T[]}> = []
  for (const line of [...lines].sort(byModeThenNumber)) {
    const mode = modeOf(line.route_type)
    if (out.at(-1)?.mode !== mode) out.push({mode, lines: []})
    out.at(-1)!.lines.push(line)
  }
  return out
}
