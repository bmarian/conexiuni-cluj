import {computed, type Ref} from 'vue'
import {matchingPlans, useFavoritesStore} from '@/stores/favorites.ts'
import {useRecents} from '@/composables/useRecents.ts'
import type {Place} from '@/utils/places.ts'

const PICKS_SHOWN = 5

export type PlacePick = Place & {recent?: boolean}

function pick(lat: number, lon: number, label: string, recent?: boolean): PlacePick {
  return {id: `${lat}_${lon}`, lat: String(lat), lon: String(lon), label, name: label, recent}
}

export function usePlacePicks(query: Ref<string>) {
  const favorites = useFavoritesStore()
  const recents = useRecents(PICKS_SHOWN)

  return computed<PlacePick[]>(() => {
    const typed = !!query.value.trim()
    const plans = typed ? matchingPlans(favorites.favoritePlans, query.value) : favorites.favoritePlans.slice(0, PICKS_SHOWN)
    const saved = plans.map((p) => pick(p.lat, p.lon, p.label ?? p.name))
    if (typed) return saved
    const picks = [...saved]
    for (const {plan, stop} of recents.value) {
      const recent = plan ? pick(plan.lat, plan.lon, plan.name, true) : stop && pick(stop.stop_lat, stop.stop_lon, stop.stop_name, true)
      if (recent && !picks.some((p) => p.id === recent.id)) picks.push(recent)
    }
    return picks
  })
}
