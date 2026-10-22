import type {FavoritePlan} from '@/stores/favorites.ts'

export function planQuery(plan: FavoritePlan): Record<string, string> {
  const query: Record<string, string> = {lat: String(plan.lat), lon: String(plan.lon), name: plan.name}
  if (plan.originLat !== undefined && plan.originLon !== undefined && plan.originName) {
    query.originLat = String(plan.originLat)
    query.originLon = String(plan.originLon)
    query.originName = plan.originName
  }
  return query
}
