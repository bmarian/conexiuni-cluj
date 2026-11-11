import type {Stop} from '@/types/tranzy.ts'
import type {NetworkIndex, NetworkRoute} from '@/utils/network.ts'
import type {DestinationSummary} from '@/utils/destinationPages.ts'

export type SearchExamples = {line?: NetworkRoute; stop?: Stop; place?: string}

function busiest<T>(items: readonly T[], score: (item: T) => number): T | undefined {
  let best: T | undefined
  let bestScore = 0
  for (const item of items) {
    const s = score(item)
    if (s > bestScore) {
      best = item
      bestScore = s
    }
  }
  return best
}

// Taken from the city's own busiest line, station and neighbourhood, so each one finds something there.
// Outside neighbourhoods the biggest area is the city itself, a poor example.
export function searchExamples(index: NetworkIndex, areas: readonly DestinationSummary[]): SearchExamples {
  return {
    line: busiest(index.bundle.routes, (r) => r.timetable?.weekdays.entries.length ?? 0),
    stop: busiest(index.bundle.stops, (s) => index.look(s.stop_id)?.routes.length ?? 0),
    place: busiest(
      areas.filter((a) => a.district),
      (a) => a.stops,
    )?.name,
  }
}
