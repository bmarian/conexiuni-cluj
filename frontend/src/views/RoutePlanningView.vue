<script lang="ts">
export default {
  name: 'RoutePlanningView'
}
</script>

<script setup lang="ts">
import {computed, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch} from 'vue'
import {RouterLink, useRoute, useRouter} from 'vue-router'
import HeaderNavigation from "@/components/HeaderNavigation.vue"
import {useI18n} from 'vue-i18n'
import {useMapStore, type HighlightedStop} from '@/stores/map.ts'
import {useUserStore} from '@/stores/user.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {usePlannerStore} from '@/stores/planner.ts'
import IconHeartFilled from "@/components/icons/IconHeartFilled.vue"
import IconHeartOutline from "@/components/icons/IconHeartOutline.vue"
import {haversineMeters} from "@/utils/geo.ts"
import {apiRequest} from "@/utils/request_cache.ts"
import type {Stop, TimeEntry, ShapeInfo, Shape, Vehicle} from "@/types/tranzy.ts"
import {storeToRefs} from "pinia"
import ClosestStopsList from "@/components/ClosestStopsList.vue"
import ViewErrorState from "@/components/ViewErrorState.vue"

import {useVehicleStream} from "@/composables/useVehicleStream.ts"
import type {DisplayShape} from "@/stores/map.ts"
import {
  buildShapeIndex,
  getIndexedVehicles,
  findClosestShapeIdx,
  buildStopShapeIdxByStopId,
  etaForStop,
  type IndexedVehicle,
  type ShapeIndex
} from "@/composables/useVehicleTracking.ts"
import {formatMinutesFromNow} from "@/utils/time.ts"
import {decodePolyline} from "@/utils/geo.ts"

const MAX_MINUTES = 60
const WALK_SPEED = 80 // m/min
const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const mapStore = useMapStore()
const userStore = useUserStore()
const settings = useSettingsStore()
const favoritesStore = useFavoritesStore()
const plannerStore = usePlannerStore()
const {userLocation, hasLocationPermission, userTime} = storeToRefs(userStore)
const {timeMode, timeValue} = storeToRefs(plannerStore)

interface PlanStop { stop_id: number; stop_name: string; stop_lat: number; stop_lon: number }
interface PlanWalkSeg { geometry: string; distance_m: number; duration_sec: number }
interface ApiLeg { route_id: number; trip_id: string; start_stop_id: number; dest_stop_id: number; ride_seconds: number; intermediate_stop_ids?: number[] }
interface ApiPlan { legs: ApiLeg[]; is_direct: boolean; walk_start_meters: number; walk_end_meters: number; walk_transfer_meters: number; transit_duration_sec: number; total_distance: number; walk_segments?: PlanWalkSeg[] }
interface ApiResp { plans: ApiPlan[]; stops: Record<string, PlanStop>; shapes: Record<string, ShapeInfo> }

interface RichLeg { routeIds: number[]; tripIds: string[]; startStopId: number; destStopId: number; rideSeconds: number; intermediateStopIds: number[] }
interface RichPlan { legs: RichLeg[]; isDirect: boolean; walkStartMeters: number; walkEndMeters: number; walkTransferMeters: number; walkSegments: PlanWalkSeg[]; nextTimes: TimeEntry[]; isLive: boolean; key: string }

interface NominatimResult {
  place_id: number
  display_name: string
  lat: string
  lon: string
}

const customOrigin = ref<{ name: string, lat: number, lon: number } | null>(null)
const activeSearchField = ref<'origin' | 'destination' | null>(null)
const searchQuery = ref('')
const searchResults = ref<NominatimResult[]>([])
const isSearching = ref(false)
let searchTimeout: ReturnType<typeof setTimeout> | null = null

const vFocus = {
  mounted: (el: HTMLElement) => el.focus()
}

const destName = computed(() => (route.query.name as string | undefined) ?? '')
const destLat = computed(() => parseFloat((route.query.lat as string) ?? 'NaN'))
const destLon = computed(() => parseFloat((route.query.lon as string) ?? 'NaN'))

const isFavorite = computed(() => favoritesStore.isPlanFavorite(
  destLat.value,
  destLon.value,
  customOrigin.value?.lat,
  customOrigin.value?.lon
))

function toggleFavorite() {
  if (isNaN(destLat.value) || isNaN(destLon.value)) return
  favoritesStore.togglePlanFavorite({
    name: destName.value || t('planTitleGeneric'),
    lat: destLat.value,
    lon: destLon.value,
    originName: customOrigin.value?.name,
    originLat: customOrigin.value?.lat,
    originLon: customOrigin.value?.lon
  })
}

const allStops = ref<Stop[]>([])
const planData = ref<ApiResp | null>(null)
const currentQueryKey = ref<string | null>(null)
const selectedPlanIndex = ref(0)
const selectedPlanKey = ref<string | null>(null)
const isCalculating = ref(false)
const hasFlownToFallback = ref(false)
const routesWithTimes = ref<RichPlan[]>([])
const expandedLegs = ref<Record<string, boolean>>({})

function toggleLegExpansion(id: string) {
  expandedLegs.value[id] = !expandedLegs.value[id]
}

const selectedRouteIsLive = ref(false)
const selectedRouteLiveEtaMin = ref<number | null>(null)
const liveEtaByKey = ref<Map<string, number>>(new Map())
const mapActivationKey = ref(0)
const isActive = ref(false)

const allStopsMap = computed(() => {
  const m = new Map<number, Stop>()
  for (const s of allStops.value) {
    m.set(s.stop_id, s)
  }
  return m
})

function getStop(id: number | string | undefined): Stop | PlanStop | undefined {
  if (id === undefined) return undefined
  const numId = typeof id === 'string' ? parseInt(id) : id
  return allStopsMap.value.get(numId) ?? stops.value[String(numId)]
}

function getStopName(id: number | string | undefined): string {
  return getStop(id)?.stop_name ?? (id ? `Stop ${id}` : '')
}

const stops = computed(() => planData.value?.stops ?? {} as Record<string, PlanStop>)
const shapes = computed(() => planData.value?.shapes ?? {} as Record<string, ShapeInfo>)

const selectedPlan = computed(() => routesWithTimes.value[selectedPlanIndex.value])
const selectedPlanSignature = computed(() => selectedPlan.value?.key ?? null)

function selectPlanAt(idx: number) {
  selectedPlanIndex.value = idx
  const p = routesWithTimes.value[idx]
  selectedPlanKey.value = p ? p.key : null
  mapStore.fitWalkingPolylines = true
}

// --- Timetable helpers ---
function getNextDepartures(shape: ShapeInfo, tripId: string, boardingStopId: number, now: Date, limit = 3, maxMinutes = 60, arriveBy = false): TimeEntry[] {
  const timetable = shape.timetable
  if (!timetable) return []

  const isOutgoing = tripId.endsWith('_0')
  const stopTimes = (shape.stop_times ?? shape.stop_time ?? []).filter(st => st.trip_id === tripId)

  // Sum offset_arrival_time/60 for stops before boarding stop to get terminus->boarding offset
  let offsetMin = 0
  for (const st of stopTimes) {
    if (st.stop_id === boardingStopId) break
    offsetMin += st.offset_arrival_time / 60
  }

  const dayOfWeek = now.getDay()
  const daySchedule = dayOfWeek === 0 ? timetable.sunday : dayOfWeek === 6 ? timetable.saturday : timetable.weekdays
  if (!daySchedule?.entries) return []

  const nowMins = now.getHours() * 60 + now.getMinutes() + now.getSeconds() / 60
  const results: TimeEntry[] = []

  for (const entry of daySchedule.entries) {
    const timeStr = isOutgoing ? entry.departure_in : entry.departure_out
    if (!timeStr) continue
    const [hStr, mStr] = timeStr.split(':')
    const terminusMinutes = parseInt(hStr ?? '0') * 60 + parseInt(mStr ?? '0')
    const arrivalAtBoardingMin = terminusMinutes + offsetMin
    const diff = arrivalAtBoardingMin - nowMins

    if (arriveBy) {
      if (diff > 0) continue
      if (diff < -maxMinutes) continue
      results.push({ minutes: Math.round(diff), is_live: false })
    } else {
      if (diff < 0) continue
      if (diff > maxMinutes) break
      results.push({ minutes: Math.round(diff), is_live: false })
    }
  }

  if (arriveBy) {
    return results.sort((a, b) => b.minutes - a.minutes).slice(0, limit).sort((a, b) => a.minutes - b.minutes)
  }

  return results
}

function computeNextTimesForPlan(plan: { legs: RichLeg[], isDirect: boolean, walkStartMeters: number, walkEndMeters: number }, shapesMap: Record<string, ShapeInfo>, now: Date, arriveBy = false): TimeEntry[] {
  const walkStartMin = plan.walkStartMeters / WALK_SPEED
  const walkEndMin = plan.walkEndMeters / WALK_SPEED
  if (plan.isDirect) {
    const leg = plan.legs[0]!
    const times: TimeEntry[] = []
    for (let i = 0; i < leg.routeIds.length; i++) {
      const shape = shapesMap[String(leg.routeIds[i]!)]
      const tripId = leg.tripIds[i]
      if (!shape || !tripId) continue

      // For arriveBy, the "now" is the requested arrival time at destination.
      // We must arrive at boarding stop by now - rideTime - walkEnd.
      const refTime = arriveBy ? new Date(now.getTime() - (leg.rideSeconds / 60 + walkEndMin) * 60_000) : now
      const deps = getNextDepartures(shape, tripId, leg.startStopId, refTime, 4, MAX_MINUTES, arriveBy)
      const offset = (refTime.getTime() - now.getTime()) / 60_000
      times.push(...deps.map(t => ({ ...t, minutes: t.minutes + offset }))
        .filter(t => arriveBy || t.minutes >= walkStartMin))
    }
    return times.sort((a, b) => a.minutes - b.minutes).slice(0, 3)
  } else {
    const leg1 = plan.legs[0]!
    const leg2 = plan.legs[1]!
    const shape1 = shapesMap[String(leg1.routeIds[0]!)]
    const tripId1 = leg1.tripIds[0]
    if (!shape1 || !tripId1) return []

    if (arriveBy) {
      // ArriveBy for transfers:
      // 1. Find leg2 departures that arrive before (now - walkEnd)
      // 2. Find leg1 departures that arrive before leg2 departure
      const walkEndMin = plan.walkEndMeters / WALK_SPEED
      const refTime2Base = new Date(now.getTime() - walkEndMin * 60_000 - (leg2.rideSeconds / 60) * 60_000)

      const allLeg2Deps: Array<TimeEntry & { routeId: number, tripId: string }> = []
      for (let i = 0; i < leg2.routeIds.length; i++) {
        const rid = leg2.routeIds[i]!
        const tid = leg2.tripIds[i]!
        const shape = shapesMap[String(rid)]
        if (!shape) continue
        const deps = getNextDepartures(shape, tid, leg2.startStopId, refTime2Base, 5, MAX_MINUTES, true)
        allLeg2Deps.push(...deps.map(t => ({ ...t, routeId: rid, tripId: tid })))
      }
      const leg2Deps = allLeg2Deps.sort((a, b) => b.minutes - a.minutes).slice(0, 5)

      const valid: TimeEntry[] = []
      for (const t2 of leg2Deps) {
        const arrivalAtTransfer = new Date(refTime2Base.getTime() + t2.minutes * 60_000)
        const refTime1Base = new Date(arrivalAtTransfer.getTime() - (leg1.rideSeconds / 60) * 60_000)

        let bestT1: TimeEntry | null = null
        for (let i = 0; i < leg1.routeIds.length; i++) {
          const rid = leg1.routeIds[i]!
          const tid = leg1.tripIds[i]!
          const shape = shapesMap[String(rid)]
          if (!shape) continue
          const leg1Deps = getNextDepartures(shape, tid, leg1.startStopId, refTime1Base, 1, 30, true)
          if (leg1Deps.length > 0) {
            const t = leg1Deps[0]!
            if (!bestT1 || t.minutes > bestT1.minutes) {
              bestT1 = t
            }
          }
        }

        if (bestT1) {
          valid.push({
            minutes: bestT1.minutes + (refTime1Base.getTime() - now.getTime()) / 60_000,
            is_live: false
          })
        }
      }
      return valid.sort((a, b) => b.minutes - a.minutes).slice(0, 3).sort((a, b) => a.minutes - b.minutes)
    } else {
      const allLeg1Deps: TimeEntry[] = []
      for (let i = 0; i < leg1.routeIds.length; i++) {
        const rid = leg1.routeIds[i]!
        const tid = leg1.tripIds[i]!
        const shape = shapesMap[String(rid)]
        if (!shape) continue
        allLeg1Deps.push(...getNextDepartures(shape, tid, leg1.startStopId, now, 5, MAX_MINUTES, false)
          .filter(t => t.minutes >= walkStartMin))
      }
      const leg1Deps = allLeg1Deps.sort((a, b) => a.minutes - b.minutes).slice(0, 10)

      const valid: TimeEntry[] = []
      for (const t1 of leg1Deps) {
        const arrivalAtTransfer = new Date(now.getTime() + (t1.minutes + Math.ceil(leg1.rideSeconds / 60)) * 60_000)
        let hasConnection = false
        for (let j = 0; j < leg2.routeIds.length; j++) {
          const shape2 = shapesMap[String(leg2.routeIds[j]!)]
          const tripId2 = leg2.tripIds[j]
          if (!shape2 || !tripId2) continue
          if (getNextDepartures(shape2, tripId2, leg2.startStopId, arrivalAtTransfer, 1, 30, false).length > 0) {
            hasConnection = true
            break
          }
        }
        if (hasConnection) valid.push(t1)
      }
      return valid.sort((a, b) => a.minutes - b.minutes).slice(0, 3)
    }
  }
}

function groupPlans(rawPlans: ApiPlan[]): { legs: RichLeg[]; isDirect: boolean; walkStartMeters: number; walkEndMeters: number; walkTransferMeters: number; walkSegments: PlanWalkSeg[] }[] {
  const groups = new Map<string, { legs: RichLeg[]; isDirect: boolean; walkStartMeters: number; walkEndMeters: number; walkTransferMeters: number; walkSegments: PlanWalkSeg[] }>()

  for (const p of rawPlans) {
    const key = p.legs.map(l => `${l.start_stop_id}>${l.dest_stop_id}@${l.trip_id.endsWith('_0') ? '0' : '1'}`).join('|')

    if (groups.has(key)) {
      const existing = groups.get(key)!
      for (let i = 0; i < p.legs.length; i++) {
        const apiLeg = p.legs[i]
        const richLeg = existing.legs[i]
        if (!apiLeg || !richLeg) continue
        if (!richLeg.routeIds.includes(apiLeg.route_id)) {
          richLeg.routeIds.push(apiLeg.route_id)
          richLeg.tripIds.push(apiLeg.trip_id)
        }
      }
    } else {
      groups.set(key, {
        legs: p.legs.map(l => ({
          routeIds: [l.route_id],
          tripIds: [l.trip_id],
          startStopId: l.start_stop_id,
          destStopId: l.dest_stop_id,
          rideSeconds: l.ride_seconds,
          intermediateStopIds: l.intermediate_stop_ids ?? [],
        })),
        isDirect: p.is_direct,
        walkStartMeters: p.walk_start_meters,
        walkEndMeters: p.walk_end_meters,
        walkTransferMeters: p.walk_transfer_meters,
        walkSegments: p.walk_segments ?? [],
      })
    }
  }

  return Array.from(groups.values())
}

function enrichWithAlternativesFromShapes(
  grouped: ReturnType<typeof groupPlans>,
  shapesMap: Record<string, ShapeInfo>,
  now: Date
): ReturnType<typeof groupPlans> {
  return grouped.map(plan => ({
    ...plan,
    legs: plan.legs.map(leg => {
      const dir = leg.tripIds[0]?.endsWith('_0') ? '0' : '1'
      const mergedRouteIds = [...leg.routeIds]
      const mergedTripIds = [...leg.tripIds]

      for (const [routeIdStr, shape] of Object.entries(shapesMap)) {
        const routeId = parseInt(routeIdStr)
        if (isNaN(routeId) || mergedRouteIds.includes(routeId)) continue

        const tripId = `${routeId}_${dir}`
        const stopTimes = (shape.stop_times ?? shape.stop_time ?? []).filter(st => st.trip_id === tripId)
        if (!stopTimes.length) continue

        const startIdx = stopTimes.findIndex(st => st.stop_id === leg.startStopId)
        const destIdx = stopTimes.findIndex(st => st.stop_id === leg.destStopId)
        if (startIdx === -1 || destIdx === -1 || destIdx <= startIdx) continue

        // Reject if the route has no departure from the boarding stop within 30 min.
        // This filters out routes that don't run today or not at this hour (e.g. night buses).
        if (!getNextDepartures(shape, tripId, leg.startStopId, now, 1, 30).length) continue

        mergedRouteIds.push(routeId)
        mergedTripIds.push(tripId)
      }

      if (mergedRouteIds.length === leg.routeIds.length) return leg
      return { ...leg, routeIds: mergedRouteIds, tripIds: mergedTripIds }
    })
  }))
}

function buildRichPlan(
  plan: ReturnType<typeof groupPlans>[number],
  shapesMap: Record<string, ShapeInfo>,
  now: Date,
  arriveBy = false
): RichPlan | null {
  const makeKey = (p: typeof plan) =>
    (p.isDirect ? 'D' : `C${p.legs.length}`) + ':' + p.legs.map(l =>
      `${l.routeIds[0] ?? 'x'}/${l.tripIds[0]?.endsWith('_0') ? '0' : '1'}@${l.startStopId}>${l.destStopId}`
    ).join('|')

  const nextTimes = computeNextTimesForPlan(plan, shapesMap, now, arriveBy)
  if (!nextTimes.length) return null

  // For key generation and initial leg filtering, we still need to know which routes were valid
  // but computeNextTimesForPlan already did the heavy lifting.
  // We'll just return the plan with the calculated nextTimes.

  return {
    ...plan,
    nextTimes,
    isLive: false,
    key: makeKey(plan)
  }
}

const streamTripIds = computed(() => {
  if (!isActive.value || timeMode.value !== 'now') return []
  if (!routesWithTimes.value.length) return []
  const ids = new Set<string>()
  for (const p of routesWithTimes.value) {
    for (const leg of p.legs) {
      for (const tid of leg.tripIds) ids.add(tid)
    }
  }
  return [...ids]
})
const {vehiclesByTrip} = useVehicleStream(streamTripIds)

// Rebuild list structure only when API data changes (not on every clock tick)
watch(planData, (data) => {
  if (!data?.plans?.length) {
    routesWithTimes.value = []
    liveEtaByKey.value = new Map()
    return
  }
  const now = userTime.value || new Date()
  const isArriveBy = timeMode.value === 'arrive'
  const requestedTime = (timeMode.value !== 'now' && timeValue.value) ? new Date(timeValue.value) : now

  const grouped = enrichWithAlternativesFromShapes(groupPlans(data.plans), data.shapes, requestedTime)
  const results: RichPlan[] = []
  for (const plan of grouped) {
    const rich = buildRichPlan(plan, data.shapes, requestedTime, isArriveBy)
    if (rich) {
      const offsetMin = (requestedTime.getTime() - now.getTime()) / 60_000
      rich.nextTimes = rich.nextTimes.map(t => ({ ...t, minutes: t.minutes + offsetMin }))
      results.push(rich)
    }
  }
  results.sort((a, b) => {
    const totalA = computeTotalMinutes(a)
    const totalB = computeTotalMinutes(b)
    if (totalA !== totalB) return totalA - totalB
    if (a.walkEndMeters !== b.walkEndMeters) return a.walkEndMeters - b.walkEndMeters
    return a.legs.length - b.legs.length
  })
  routesWithTimes.value = results
  liveEtaByKey.value = new Map()
}, {immediate: true})

// Update departure times in-place on every clock tick — no re-sort, no DOM churn
watch(userTime, (uTime) => {
  const data = planData.value
  if (!data?.shapes || !routesWithTimes.value.length) return
  const now = uTime || new Date()
  const isArriveBy = timeMode.value === 'arrive'
  const requestedTime = (timeMode.value !== 'now' && timeValue.value) ? new Date(timeValue.value) : now
  const offsetMin = (requestedTime.getTime() - now.getTime()) / 60_000

  for (const plan of routesWithTimes.value) {
    const newTimes = computeNextTimesForPlan(plan, data.shapes, requestedTime, isArriveBy)
    // Adjust nextTimes to be relative to "now" (real current time)
    const adjustedTimes = newTimes.map(t => ({ ...t, minutes: t.minutes + offsetMin }))

    const liveEta = liveEtaByKey.value.get(plan.key)
    const walkStartMin = plan.walkStartMeters / WALK_SPEED
    if (liveEta !== undefined && liveEta >= walkStartMin) {
      plan.isLive = true
      plan.nextTimes = [{ minutes: liveEta, is_live: true }, ...adjustedTimes.slice(0, 2)]
    } else {
      plan.isLive = liveEta !== undefined // preserve LIVE pill if bus exists but isn't catchable yet
      plan.nextTimes = adjustedTimes
    }
  }
})

const shapeIndicesByTripId = ref<Map<string, ShapeIndex>>(new Map())
const currentVehicles = ref<IndexedVehicle[]>([])

// === SINGLE shape geometry fetch: load ALL trip shapes once when planData arrives ===
watch([planData, mapActivationKey], async ([data]) => {
  if (!data?.plans?.length) {
    shapeIndicesByTripId.value = new Map()
    mapStore.setLoadedShapes([])
    return
  }

  // Build trip_id → route_id mapping from the plans (plan_routes already gives us this)
  const tripToRouteId = new Map<string, number>()
  for (const p of data.plans) {
    for (const leg of p.legs) {
      tripToRouteId.set(leg.trip_id, leg.route_id)
    }
  }

  const toRequest: DisplayShape[] = []
  for (const [tid, routeId] of tripToRouteId) {
    const s = data.shapes[String(routeId)]
    if (!s) continue
    toRequest.push({
      trip_id: tid,
      route_short_name: s.route_short_name,
      route_long_name: s.route_long_name,
      route_color: s.route_color,
      route_type: s.route_type,
    })
  }

  if (!toRequest.length) {
    shapeIndicesByTripId.value = new Map()
    mapStore.setLoadedShapes([])
    return
  }

  try {
    const shapeData = await mapStore.requestShapes(toRequest)
    const newIndices = new Map<string, ShapeIndex>()
    for (const [info, pts] of shapeData) {
      if (pts.length) {
        newIndices.set(info.trip_id, buildShapeIndex(pts))
      }
    }
    shapeIndicesByTripId.value = newIndices
  } catch (e) {
    console.error('Failed to load shape geometry:', e)
  }
}, {immediate: true})

// === Display selected plan's shapes on map (uses cached geometry, NO extra request) ===
watch([selectedPlanSignature, shapeIndicesByTripId, mapActivationKey], ([key, indices]) => {
  const plan = selectedPlan.value
  if (!key || !plan || !indices.size) {
    mapStore.setLoadedShapes([])
    return
  }

  const loadedShapes: Array<[DisplayShape, Shape[]]> = []
  for (const leg of plan.legs) {
    // Only draw the first (representative) route per grouped leg to avoid overlapping polylines
    const tid = leg.tripIds[0]
    const routeId = leg.routeIds[0]
    if (!tid || routeId === undefined) continue

    const s = shapes.value[String(routeId)]
    const shapeIndex = indices.get(tid)
    if (!s || !shapeIndex) continue

    const pts = shapeIndex.shape
    const startStop = getStop(leg.startStopId)
    const destStop = getStop(leg.destStopId)
    if (!startStop || !destStop) continue

    const startIdx = findClosestShapeIdx(startStop.stop_lat, startStop.stop_lon, pts)
    const destIdx = findClosestShapeIdx(destStop.stop_lat, destStop.stop_lon, pts)
    const [realStart, realEnd] = startIdx < destIdx ? [startIdx, destIdx] : [destIdx, startIdx]
    const croppedPts = pts.slice(realStart, realEnd + 1)

    loadedShapes.push([{
      trip_id: tid,
      route_short_name: s.route_short_name,
      route_long_name: s.route_long_name,
      route_color: s.route_color,
      route_type: s.route_type,
    }, croppedPts])
  }

  mapStore.setLoadedShapes(loadedShapes)
}, {immediate: true})

// --- Walk polylines ---
watch([selectedPlanSignature, mapActivationKey], ([key]) => {
  const plan = selectedPlan.value
  if (!key || !plan || !plan.walkSegments.length) {
    mapStore.clearWalkingPolylines()
    return
  }
  mapStore.setWalkingPolylines(plan.walkSegments.map(s => decodePolyline(s.geometry)))
}, {immediate: true})

// --- Vehicle tracking: compute live ETAs for ALL plans (like StopView) ---
// Use a generation counter to discard stale async results
let vehicleTrackingGen = 0

watch([vehiclesByTrip, shapeIndicesByTripId], async ([byTrip, indices]) => {
  if (!isActive.value || timeMode.value !== 'now') return
  if (!byTrip.size && !indices.size) {
    currentVehicles.value = []
    mapStore.setVehiclesToDisplay([])
    selectedRouteIsLive.value = false
    selectedRouteLiveEtaMin.value = null
    liveEtaByKey.value = new Map()
    return
  }

  const gen = ++vehicleTrackingGen
  const newLiveEtas = new Map<string, number>()
  const planUpdates = new Map<string, { isLive: boolean; bestEta: number | null }>()

  // Compute live ETA for every plan's first leg boarding stop
  for (const plan of routesWithTimes.value) {
    const leg = plan.legs[0]
    if (!leg) continue

    let bestEta: number | null = null

    for (let i = 0; i < leg.tripIds.length; i++) {
      const tid = leg.tripIds[i]
      const routeId = leg.routeIds[i]
      if (!tid || routeId === undefined) continue

      const shape = shapes.value[String(routeId)]
      const shapeIndex = indices.get(tid)
      if (!shapeIndex || !shape) continue

      const vehicles = byTrip.get(tid) ?? []
      if (!vehicles.length) continue

      try {
        const indexed = await getIndexedVehicles(
          tid,
          shape.route_short_name,
          shape.route_color,
          shapeIndex,
          userTime.value,
          vehicles
        )
        if (gen !== vehicleTrackingGen || !isActive.value) return

        if (!indexed.length) continue

        const stopTimes = (shape.stop_times ?? shape.stop_time ?? []).filter(st => st.trip_id === tid)
        const stopShapeIdx = buildStopShapeIdxByStopId(stopTimes, shapeIndex.shape)
        const boardingIdx = stopShapeIdx.get(leg.startStopId) ?? -1
        if (boardingIdx < 0) continue

        const eta = etaForStop(boardingIdx, indexed, shapeIndex)
        if (eta && eta.etaMinutes <= MAX_MINUTES) {
          if (bestEta === null || eta.etaMinutes < bestEta) {
            bestEta = eta.etaMinutes
          }
        }
      } catch (e) {
        console.warn('Failed to compute live ETA for trip', tid, e)
      }
    }

    planUpdates.set(plan.key, { isLive: bestEta !== null, bestEta })
  }

  // Discard if a newer computation has started
  if (gen !== vehicleTrackingGen) return

  // Apply all ETA updates atomically (no flickering)
  for (const plan of routesWithTimes.value) {
    const update = planUpdates.get(plan.key)
    if (!update) continue

    if (update.bestEta !== null) {
      const walkStartMin = plan.walkStartMeters / WALK_SPEED
      const catchable = update.bestEta >= walkStartMin
      plan.isLive = true
      if (catchable) {
        newLiveEtas.set(plan.key, update.bestEta)
        plan.nextTimes = [
          { minutes: update.bestEta, is_live: true },
          ...plan.nextTimes.filter(t => !t.is_live).slice(0, 2)
        ]
      }
    } else {
      plan.isLive = false
    }
  }

  liveEtaByKey.value = newLiveEtas

  // Update selected plan live state
  const plan = selectedPlan.value
  if (plan) {
    const liveEta = newLiveEtas.get(plan.key)
    selectedRouteIsLive.value = plan.isLive
    selectedRouteLiveEtaMin.value = liveEta ?? null
  } else {
    selectedRouteIsLive.value = false
    selectedRouteLiveEtaMin.value = null
  }

  // Compute vehicles for selected plan's map display
  updateMapVehicles(byTrip, indices)
}, {deep: true})

// Immediately update map vehicles when selected plan changes
watch(selectedPlanSignature, () => {
  // Clear old vehicles immediately to avoid stale markers
  currentVehicles.value = []
  mapStore.setVehiclesToDisplay([])
  // Then recompute for new plan
  updateMapVehicles(vehiclesByTrip.value, shapeIndicesByTripId.value)
})

async function updateMapVehicles(byTrip: Map<string, Vehicle[]>, indices: Map<string, ShapeIndex>) {
  const plan = selectedPlan.value
  if (!plan || !indices.size) {
    currentVehicles.value = []
    mapStore.setVehiclesToDisplay([])
    return
  }

  const allIndexedVehicles: IndexedVehicle[] = []
  for (const leg of plan.legs) {
    for (let i = 0; i < leg.tripIds.length; i++) {
      const tid = leg.tripIds[i]
      const routeId = leg.routeIds[i]
      if (!tid || routeId === undefined) continue

      const shape = shapes.value[String(routeId)]
      const shapeIndex = indices.get(tid)
      if (!shapeIndex || !shape) continue

      const v = byTrip.get(tid) ?? []
      if (!v.length) continue

      try {
        const indexed = await getIndexedVehicles(
          tid,
          shape.route_short_name,
          shape.route_color,
          shapeIndex,
          userTime.value,
          v
        )
        if (!isActive.value) return

        const pts = shapeIndex.shape
        const startStop = getStop(leg.startStopId)
        const destStop = getStop(leg.destStopId)
        if (!startStop || !destStop) continue

        const startIdx = findClosestShapeIdx(startStop.stop_lat, startStop.stop_lon, pts)
        const destIdx = findClosestShapeIdx(destStop.stop_lat, destStop.stop_lon, pts)
        const [rStart, rEnd] = startIdx < destIdx ? [startIdx, destIdx] : [destIdx, startIdx]

        const filtered = indexed.filter(iv => iv.shapeIdx >= Math.max(0, rStart - 10) && iv.shapeIdx <= rEnd + 1)
        allIndexedVehicles.push(...filtered)
      } catch (e) {
        console.warn('Failed to index vehicles for trip', tid, e)
      }
    }
  }

  const seenIds = new Set<number>()
  const deduped: IndexedVehicle[] = []
  for (const v of allIndexedVehicles) {
    if (!seenIds.has(v.id)) {
      seenIds.add(v.id)
      deduped.push(v)
    }
  }
  currentVehicles.value = deduped
  mapStore.setVehiclesToDisplay(deduped)
}

function computeTotalMinutes(plan: RichPlan): number {
  if (!plan.nextTimes.length) return Infinity
  const rideMin = plan.legs.reduce((s, l) => s + l.rideSeconds / 60, 0)
  const transferPenalty = (plan.legs.length - 1) * 5
  const walkEndMin = plan.walkEndMeters / WALK_SPEED
  return Math.round((plan.nextTimes[0]?.minutes ?? 0) + rideMin + transferPenalty + walkEndMin)
}

function getJourneyDuration(plan: RichPlan): number {
  const walkTime = (plan.walkStartMeters + plan.walkTransferMeters + plan.walkEndMeters) / WALK_SPEED
  const rideTime = plan.legs.reduce((acc, l) => acc + l.rideSeconds / 60, 0)
  // Approximate transfer wait time if not direct
  const transferWait = plan.isDirect ? 0 : (plan.legs.length - 1) * 2
  return walkTime + rideTime + transferWait
}

function getRelativeDepartureFormatted(plan: RichPlan, entry: TimeEntry) {
  const walkStartMin = plan.walkStartMeters / WALK_SPEED
  const approx = entry.is_live ? '' : '~'

  const now = userTime.value || new Date()
  const departureAtStop = new Date(now.getTime() + entry.minutes * 60_000)
  const departureFromOrigin = new Date(departureAtStop.getTime() - walkStartMin * 60_000)

  if (timeMode.value !== 'now') {
    const timeStr = departureFromOrigin.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
    return t('planAtTime', { time: approx + timeStr })
  }

  const waitMin = (departureFromOrigin.getTime() - now.getTime()) / 60_000
  const rounded = Math.round(waitMin)
  if (rounded <= 0) return t('now')

  const timePart = formatMinutesFromNow(waitMin, now, '')
  if (rounded < 60) {
    return t('planInTime', { time: approx + timePart })
  } else {
    return t('planAtTime', { time: approx + timePart })
  }
}

function getArrivalTimeFormatted(plan: RichPlan, entry: TimeEntry) {
  const approx = entry.is_live ? '' : '~'
  const now = userTime.value || new Date()
  const departureAtStop = new Date(now.getTime() + entry.minutes * 60_000)
  const walkStartMin = plan.walkStartMeters / WALK_SPEED
  const duration = getJourneyDuration(plan)
  const arrivalDate = new Date(departureAtStop.getTime() + (duration - walkStartMin) * 60_000)

  const timeStr = arrivalDate.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
  return t('planArrivalAt', { time: approx + timeStr })
}

const totalTripMinutes = computed(() => {
  const plan = selectedPlan.value
  if (!plan) return 0
  const val = computeTotalMinutes(plan)
  return Number.isFinite(val) ? val : 0
})

const formatMinutes = (m: number): string => {
  const rounded = Math.round(m)
  if (rounded < 60) return `${rounded}m`
  const h = Math.floor(rounded / 60)
  const rem = rounded % 60
  return rem === 0 ? `${h}h` : `${h}h ${rem}m`
}

const getTransferWalkMeters = (legIdx: number): number => {
  const plan = selectedPlan.value
  if (!plan) return 0
  const a = plan.legs[legIdx]
  const b = plan.legs[legIdx + 1]
  if (!a || !b) return 0
  if (a.destStopId === b.startStopId) return 0
  const aStop = getStop(a.destStopId)
  const bStop = getStop(b.startStopId)
  if (!aStop || !bStop) return 0
  return haversineMeters(aStop.stop_lat, aStop.stop_lon, bStop.stop_lat, bStop.stop_lon)
}

const selectedPlanLegsData = computed(() => {
  const plan = selectedPlan.value
  if (!plan) return []
  return plan.legs.map(leg => {
    const shape = shapes.value[String(leg.routeIds[0])]
    const intermediates = (leg.intermediateStopIds ?? []).map(id => ({
      stop_id: id,
      stop_name: getStopName(id),
    }))
    return { leg, shape, intermediates }
  })
})

watch([selectedPlanSignature, selectedPlanLegsData, mapActivationKey], ([, legsData]) => {
  const plan = selectedPlan.value
  if (!plan) {
    mapStore.setHighlightedStops([])
    return
  }

  const highlighted: HighlightedStop[] = []
  legsData.forEach((ld, idx) => {
    highlighted.push({
      stopId: String(ld.leg.startStopId),
      color: idx === 0 ? 'green' : 'purple'
    })
    ld.intermediates.forEach(s => {
      highlighted.push({ stopId: String(s.stop_id), color: 'gray' })
    })
    if (idx === legsData.length - 1) {
      highlighted.push({ stopId: String(ld.leg.destStopId), color: 'red' })
    } else if (ld.leg.destStopId !== legsData[idx + 1]?.leg.startStopId) {
      highlighted.push({ stopId: String(ld.leg.destStopId), color: 'amber' })
    }
  })

  mapStore.setHighlightedStops(highlighted)
}, {immediate: true})

const hasValidDest = computed(() => destName.value.length > 0)
const hasValidCoords = computed(() => !isNaN(destLat.value) && !isNaN(destLon.value))

const originLabel = computed(() => {
  if (customOrigin.value) return customOrigin.value.name
  if (userStore.userLocation) return t('planOriginCurrentLocation')
  return t('planOriginUnknown')
})

function getQueryKey() {
  const lat = destLat.value
  const lon = destLon.value
  const ul = customOrigin.value
    ? {latitude: customOrigin.value.lat, longitude: customOrigin.value.lon}
    : (hasLocationPermission.value ? userLocation.value : null)
  if (!ul || isNaN(lat) || isNaN(lon)) return null
  const tk = timeMode.value === 'now' ? 'now' : `${timeMode.value}:${timeValue.value}`
  return `plan_routes?from_lat=${ul.latitude}&from_lng=${ul.longitude}&to_lat=${lat}&to_lng=${lon}&t=${tk}`
}

watch(selectedPlanKey, (newKey) => {
  const qk = getQueryKey()
  if (qk && newKey) {
    plannerStore.setSelectedRouteKey(qk, newKey)
  }
})

const {pinnedLocationDragged, customOriginLocationDragged} = storeToRefs(mapStore)

async function reverseGeocode(lat: number, lon: number): Promise<string> {
  try {
    const params = new URLSearchParams({
      lat: lat.toString(),
      lon: lon.toString(),
      format: 'json',
      'accept-language': 'ro',
    })
    const resp = await fetch(`https://nominatim.openstreetmap.org/reverse?${params}`)
    if (resp.ok) {
      const data = await resp.json()
      return (data.display_name as string | undefined) ?? `${lat.toFixed(5)}, ${lon.toFixed(5)}`
    }
  } catch { /* ignore */ }
  return `${lat.toFixed(5)}, ${lon.toFixed(5)}`
}

let destDragGen = 0
watch(pinnedLocationDragged, async (dragged) => {
  if (!dragged) return
  const gen = ++destDragGen
  const {lat, lng} = dragged
  mapStore.clearPinnedLocationDragged()
  const name = await reverseGeocode(lat, lng)
  if (gen !== destDragGen) return

  void router.replace({query: {...route.query, lat: lat.toString(), lon: lng.toString(), name}})
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  void calculateRoutes()
})

let originDragGen = 0
watch(customOriginLocationDragged, async (dragged) => {
  if (!dragged) return
  const gen = ++originDragGen
  const {lat, lng} = dragged
  mapStore.clearCustomOriginLocationDragged()
  const name = await reverseGeocode(lat, lng)
  if (gen !== originDragGen) return

  customOrigin.value = {name, lat, lon: lng}
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  void calculateRoutes()
})

async function performSearch() {
  const q = searchQuery.value.trim()
  if (q.length < 3) {
    searchResults.value = []
    return
  }
  isSearching.value = true
  try {
    const params = new URLSearchParams({
      q,
      format: 'json',
      countrycodes: 'ro',
      viewbox: '22.75,47.50,24.27,46.38',
      bounded: '1',
      limit: '5',
      'accept-language': 'ro',
    })
    const resp = await fetch(`https://nominatim.openstreetmap.org/search?${params}`)
    searchResults.value = resp.ok ? await resp.json() : []
  } finally {
    isSearching.value = false
  }
}

watch(searchQuery, () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(performSearch, 500)
})

function selectOrigin(res: NominatimResult) {
  customOrigin.value = {
    name: res.display_name,
    lat: parseFloat(res.lat),
    lon: parseFloat(res.lon)
  }
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  void calculateRoutes()
}

function selectDestination(res: NominatimResult) {
  const newQuery = {
    ...route.query,
    lat: res.lat,
    lon: res.lon,
    name: res.display_name
  }
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  void router.replace({query: newQuery})
  void calculateRoutes()
}

function useCurrentLocation() {
  customOrigin.value = null
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  void calculateRoutes()
}

watch(customOrigin, (val) => {
  const newQuery = {...route.query}
  let changed = false
  if (val) {
    if (newQuery.originLat !== val.lat.toString() || newQuery.originLon !== val.lon.toString() || newQuery.originName !== val.name) {
      newQuery.originLat = val.lat.toString()
      newQuery.originLon = val.lon.toString()
      newQuery.originName = val.name
      changed = true
    }
  } else {
    if (newQuery.originLat || newQuery.originLon || newQuery.originName) {
      delete newQuery.originLat
      delete newQuery.originLon
      delete newQuery.originName
      changed = true
    }
  }
  if (changed) {
    router.replace({query: newQuery})
  }
})

watch(() => route.query, (newQuery) => {
  const oLat = parseFloat(newQuery.originLat as string)
  const oLon = parseFloat(newQuery.originLon as string)
  const oName = newQuery.originName as string
  if (!isNaN(oLat) && !isNaN(oLon) && oName) {
    if (!customOrigin.value || customOrigin.value.lat !== oLat || customOrigin.value.lon !== oLon || customOrigin.value.name !== oName) {
      customOrigin.value = {name: oName, lat: oLat, lon: oLon}
    }
  } else {
    if (customOrigin.value) {
      customOrigin.value = null
    }
  }
}, {immediate: true})

onMounted(async () => {
  isActive.value = true
  if (hasValidCoords.value) {
    mapStore.setPinnedLocation(destLat.value, destLon.value, destName.value)
    allStops.value = await apiRequest('stops', 60 * 60 * 1000) as Stop[]

    if (!hasLocationPermission.value && !customOrigin.value) {
      mapStore.setFlyToLocation(destLat.value, destLon.value)
    }
  } else {
    activeSearchField.value = 'destination'
  }
})

onActivated(() => {
  isActive.value = true
  if (hasValidCoords.value) {
    mapStore.setPinnedLocation(destLat.value, destLon.value, destName.value)
  }
  if (customOrigin.value) {
    mapStore.setCustomOriginLocation(customOrigin.value.lat, customOrigin.value.lon, customOrigin.value.name)
  }
  mapActivationKey.value++
})

watch([destLat, destLon, destName], ([lat, lon, name]) => {
  if (!isNaN(lat) && !isNaN(lon)) {
    mapStore.setPinnedLocation(lat, lon, name)
  }
})

watch(customOrigin, (co) => {
  if (co) {
    mapStore.setCustomOriginLocation(co.lat, co.lon, co.name)
  } else {
    mapStore.clearCustomOriginLocation()
  }
}, {immediate: true})

const navigateAwayFunctions = () => {
  isActive.value = false
  mapStore.clearPinnedLocation()
  mapStore.clearCustomOriginLocation()
  mapStore.setVehiclesToDisplay([])
  mapStore.setLoadedShapes([])
  mapStore.setHighlightedStops([])
  mapStore.clearWalkingPolylines()
}

onDeactivated(navigateAwayFunctions)
onUnmounted(navigateAwayFunctions)

watch(routesWithTimes, (newRoutes) => {
  if (!newRoutes.length) {
    selectedPlanIndex.value = 0
    selectedPlanKey.value = null
    return
  }
  if (selectedPlanKey.value) {
    const idx = newRoutes.findIndex(r => r.key === selectedPlanKey.value)
    if (idx >= 0) {
      selectedPlanIndex.value = idx
      return
    }
  }
  if (selectedPlanIndex.value >= newRoutes.length) {
    selectedPlanIndex.value = 0
  }
  selectedPlanKey.value = newRoutes[selectedPlanIndex.value]?.key ?? null
})

watch([isCalculating, routesWithTimes], ([calculating, routes]) => {
  if (calculating || routes.length > 0) {
    hasFlownToFallback.value = false
    return
  }
  if (routes.length === 0 && !isNaN(destLat.value) && !isNaN(destLon.value) && !hasFlownToFallback.value) {
    mapStore.setFlyToLocation(destLat.value, destLon.value)
    hasFlownToFallback.value = true
  }
})

async function calculateRoutes() {
  if (isCalculating.value) return
  const lat = destLat.value
  const lon = destLon.value
  const origin = customOrigin.value ?? (hasLocationPermission.value ? userLocation.value : null)
  if (!origin || isNaN(lat) || isNaN(lon)) return

  const qk = getQueryKey()
  if (qk && qk === currentQueryKey.value && planData.value?.plans.length) return

  isCalculating.value = true
  try {
    if (qk) selectedPlanKey.value = plannerStore.getSelectedRouteKey(qk)

    const fromLat = 'latitude' in origin ? origin.latitude : (origin as {lat: number}).lat
    const fromLng = 'longitude' in origin ? origin.longitude : (origin as {lon: number}).lon
    const params = new URLSearchParams({
      from_lat: fromLat.toFixed(5),
      from_lng: fromLng.toFixed(5),
      to_lat: lat.toFixed(5),
      to_lng: lon.toFixed(5),
    })
    if (timeMode.value !== 'now' && timeValue.value) {
      params.set('time', timeValue.value)
      params.set('arrive_by', timeMode.value === 'arrive' ? 'true' : 'false')
    }
    const resp = await fetch(`/api/plan_routes?${params.toString()}`)
    if (!resp.ok) throw new Error('fetch failed')
    const data = await resp.json() as ApiResp

    routesWithTimes.value = []
    planData.value = data
    currentQueryKey.value = qk

    if (data.plans.length) {
      mapStore.fitWalkingPolylines = true
      favoritesStore.addRecentPlan({
        name: destName.value || t('planTitleGeneric'),
        lat,
        lon,
        originName: customOrigin.value?.name,
        originLat: customOrigin.value?.lat,
        originLon: customOrigin.value?.lon
      })
    }
  } finally {
    isCalculating.value = false
  }
}

// Recalculate when destination changes
watch([destLat, destLon], async ([newLat, newLon], [oldLat, oldLon] = [NaN, NaN]) => {
  if (newLat !== oldLat || newLon !== oldLon) await calculateRoutes()
}, {immediate: true})

// Recalculate when custom origin changes
watch(customOrigin, async (newCO, oldCO) => {
  if (newCO?.lat !== oldCO?.lat || newCO?.lon !== oldCO?.lon) await calculateRoutes()
})

// Recalculate when GPS location first arrives (no custom origin, no plan yet)
watch(userLocation, async (newLoc, oldLoc) => {
  if (!newLoc || oldLoc || customOrigin.value) return
  await calculateRoutes()
})

function swapOriginDestination() {
  if (isNaN(destLat.value) || isNaN(destLon.value)) return
  const newDestLat = customOrigin.value?.lat ?? userLocation.value?.latitude
  const newDestLon = customOrigin.value?.lon ?? userLocation.value?.longitude
  const newDestName = customOrigin.value?.name ?? t('planOriginCurrentLocation')
  if (newDestLat === undefined || newDestLon === undefined) return

  void router.replace({
    query: {
      lat: String(newDestLat),
      lon: String(newDestLon),
      name: newDestName,
      originLat: String(destLat.value),
      originLon: String(destLon.value),
      originName: destName.value,
    }
  })
}

async function refreshRoutes() {
  if (isCalculating.value) return
  selectedPlanIndex.value = 0
  selectedPlanKey.value = null
  routesWithTimes.value = []
  planData.value = null
  currentQueryKey.value = null
  mapStore.clearWalkingPolylines()
  await calculateRoutes()
}

// Auto-populate `timeValue` with "now" when switching out of `now`.
// NOTE: we intentionally do NOT auto-refresh on time-filter changes — the
// user must click the explicit "Search" button in `.plan-time-filter-actions`
// (or switch back to "Leave now") to trigger a recalculation.
const formatLocalDateTime = (d: Date) => {
  const pad = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
// Minimum selectable datetime (current local time) — recomputed on clock tick
// so users cannot pick a moment in the past.
const minDateTime = computed(() => formatLocalDateTime(userTime.value || new Date()))
watch(timeMode, (mode) => {
  if (mode !== 'now' && !timeValue.value) {
    timeValue.value = formatLocalDateTime(new Date())
  }
  if (mode === 'now') {
    void refreshRoutes()
  }
})
// Clamp `timeValue` to the present if the user picks a past moment.
watch(timeValue, (val) => {
  if (!val) return
  if (val < minDateTime.value) {
    timeValue.value = minDateTime.value
  }
})

</script>

<template>
  <div v-if="!hasValidCoords"
       class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col">
    <ViewErrorState/>
  </div>
  <div v-else
       class="plan-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-6">
    <div class="flex items-center -mb-2">
      <HeaderNavigation/>
    </div>
    <header class="flex items-center gap-3">
      <div
        class="w-12 h-12 shrink-0 rounded-2xl bg-linear-to-br from-sky-400 to-sky-600 flex items-center justify-center shadow-lg shadow-sky-500/20">
        <span v-if="settings.traditionalActive" class="emoji-icon-xl" aria-hidden="true">🗺️</span>
        <svg v-else class="w-6 h-6 text-white" viewBox="0 0 24 24" fill="none"
             stroke="currentColor"
             stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
                d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <span
          class="text-[10px] font-semibold text-sky-600 dark:text-sky-400 tracking-wide uppercase">{{
            t('planTitle')
          }}</span>
        <h1
          class="text-xl font-black tracking-tight text-slate-900 dark:text-white leading-tight truncate"
          :title="hasValidDest ? destName : t('planTitleGeneric')">
          {{ hasValidDest ? destName : t('planTitleGeneric') }}
        </h1>
      </div>
      <button
        v-if="hasValidCoords"
        type="button"
        class="fav-btn mt-1 shrink-0"
        :class="{ 'is-fav': isFavorite }"
        :title="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-label="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-pressed="isFavorite"
        @click="toggleFavorite"
      >
        <IconHeartFilled v-if="isFavorite" class="w-5 h-5"/>
        <IconHeartOutline v-else class="w-5 h-5"/>
      </button>
    </header>

    <div v-if="hasLocationPermission || customOrigin">
      <section v-if="!isCalculating && selectedPlan" class="trip-summary">
        <div class="trip-summary-stat" v-if="timeMode === 'now'">
          <span class="trip-summary-stat-value">{{ formatMinutes(totalTripMinutes) }}</span>
          <span class="trip-summary-stat-label">{{ t('planTripTotal') }}</span>
        </div>
        <div v-if="selectedPlan.nextTimes[0]" class="trip-summary-stat">
          <span class="trip-summary-stat-value" :class="selectedRouteIsLive ? 'is-live' : ''">
            <span v-if="selectedRouteIsLive" class="live-dot"></span>{{ (selectedRouteLiveEtaMin !== null ? true : selectedPlan.nextTimes[0].is_live) ? '' : '~ ' }}{{ formatMinutesFromNow(selectedRouteLiveEtaMin ?? selectedPlan.nextTimes[0].minutes, userTime || new Date(), t('now')) }}
          </span>
          <span class="trip-summary-stat-label">{{ t('planNextDepartures') }}</span>
        </div>
        <div v-if="!selectedPlan.isDirect" class="trip-summary-stat">
          <span class="trip-summary-stat-value">{{ selectedPlan.legs.length - 1 }}</span>
          <span class="trip-summary-stat-label">{{ selectedPlan.legs.length - 1 === 1 ? t('planChange') : t('planChanges') }}</span>
        </div>
      </section>
      <section v-if="isCalculating || routesWithTimes.length > 0" class="route-legs-card">
        <div class="leg-row">
          <div class="leg-icon-col">
            <div class="leg-dot leg-dot-origin">
              <span v-if="settings.traditionalActive" class="text-sm">📍</span>
              <svg v-else class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
                <circle cx="12" cy="12" r="6"/>
              </svg>
            </div>
            <div class="leg-line"></div>
          </div>
          <div class="leg-label-col" v-if="activeSearchField !== 'origin'">
            <div class="origin-clickable" @click="activeSearchField = 'origin'; searchResults = []; searchQuery = ''">
              <span class="leg-type-badge">{{ t('planFrom') }}</span>
              <div class="leg-name-wrap">
                <span class="leg-name">{{ originLabel }}</span>
                <svg class="edit-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                  <path d="m14.304 4.844 2.852 2.852M7 7H4a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h11a1 1 0 0 0 1-1v-4.5m2.409-9.91a2.017 2.017 0 0 1 0 2.853l-6.844 6.844L8 14l.713-3.565 6.844-6.844a2.015 2.015 0 0 1 2.852 0Z"/>
                </svg>
              </div>
            </div>
          </div>
          <div class="leg-label-col search-active-col" v-else>
            <div class="search-wrap">
              <input
                type="text"
                v-model="searchQuery"
                class="search-input"
                :placeholder="t('planSearchPlaceholder')"
                @keyup.enter="performSearch"
                @keyup.esc="activeSearchField = null"
                v-focus
              />
              <button class="search-cancel" @click="activeSearchField = null">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <path d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="search-results" v-if="searchResults.length > 0 || isSearching || hasLocationPermission">
              <div v-if="isSearching" class="search-loading">
                <div class="mini-spinner"></div>
                {{ t('planSearching') }}
              </div>
              <template v-else>
                <div
                  v-if="hasLocationPermission"
                  class="search-result-item current-loc-option"
                  @click="useCurrentLocation"
                >
                  <span class="res-main">{{ t('planOriginCurrentLocation') }}</span>
                </div>
                <div
                  v-for="res in searchResults"
                  :key="res.place_id"
                  class="search-result-item"
                  @click="selectOrigin(res)"
                >
                  <span class="res-main">{{ res.display_name.split(',')[0] }}</span>
                  <span class="res-sub">{{ res.display_name.split(',').slice(1).join(',').trim() }}</span>
                </div>
              </template>
            </div>
          </div>
        </div>

        <template v-if="selectedPlan">
          <div v-for="(ld, legIdx) in selectedPlanLegsData" :key="legIdx">
            <div class="leg-row">
              <div class="leg-icon-col">
                <div class="leg-dot boarding-dot" :style="{ borderColor: ld.shape?.route_color }">
                  <div class="dot-inner" :style="{ backgroundColor: ld.shape?.route_color }"></div>
                </div>
                <div class="leg-line" :style="{ backgroundColor: ld.shape?.route_color, backgroundImage: 'none' }"></div>
              </div>
              <div class="leg-label-col">
                <span class="leg-type-badge" :style="{ color: ld.shape?.route_color }">
                  {{ t('planBoarding') }}
                  <template v-for="(routeId, rIdx) in (expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? ld.leg.routeIds : ld.leg.routeIds.slice(0, 4))" :key="routeId">
                    <RouterLink
                      class="leg-route-link"
                      :to="{ name: 'route', params: { routeId: routeId, direction: ld.leg.tripIds[rIdx]?.endsWith('_1') ? '1' : '0' } }"
                      @click.stop
                    >{{ shapes[String(routeId)]?.route_short_name }}</RouterLink><span v-if="rIdx < (expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? ld.leg.routeIds.length : Math.min(ld.leg.routeIds.length, 4)) - 1" class="leg-route-sep"> / </span>
                  </template><span v-if="ld.leg.routeIds.length > 4" class="leg-route-overflow cursor-pointer" @click.stop="toggleLegExpansion('detail-' + selectedPlan.key + '-' + legIdx)"> {{ expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? '«' : '+' + (ld.leg.routeIds.length - 4) }}</span>
                  <span v-if="Math.max(0, Math.round(ld.leg.rideSeconds / 60)) > 0" class="leg-ride-time">
                    · {{ Math.max(0, Math.round(ld.leg.rideSeconds / 60)) }}&nbsp;min&nbsp;{{ t('planRideTime') }}
                  </span>
                </span>
                <RouterLink class="leg-name leg-name-link" :to="{ name: 'stop', params: { stopId: ld.leg.startStopId } }">{{ getStopName(ld.leg.startStopId) }}</RouterLink>
              </div>
            </div>

            <div v-for="stop in ld.intermediates" :key="stop.stop_id" class="leg-row intermediate-leg">
              <div class="leg-icon-col">
                <div class="leg-dot intermediate-dot">
                  <div class="w-1 h-1 rounded-full bg-slate-300 dark:bg-slate-600"></div>
                </div>
                <div class="leg-line" :style="{ backgroundColor: ld.shape?.route_color, backgroundImage: 'none' }"></div>
              </div>
              <div class="leg-label-col">
                <RouterLink class="leg-name intermediate-name leg-name-link" :to="{ name: 'stop', params: { stopId: stop.stop_id } }">{{ stop.stop_name }}</RouterLink>
              </div>
            </div>

            <div class="leg-row" :class="{ 'leg-row-alight-transfer': legIdx < selectedPlanLegsData.length - 1 }">
              <div class="leg-icon-col">
                <div class="leg-dot boarding-dot" :style="{ borderColor: ld.shape?.route_color }">
                  <div class="dot-inner" :style="{ backgroundColor: ld.shape?.route_color }"></div>
                </div>
                <div v-if="legIdx === selectedPlanLegsData.length - 1" class="leg-line leg-line-dashed"></div>
              </div>
              <div class="leg-label-col">
                <span class="leg-type-badge" :style="{ color: ld.shape?.route_color }">{{ t('planAlighting') }}</span>
                <RouterLink class="leg-name leg-name-link" :to="{ name: 'stop', params: { stopId: ld.leg.destStopId } }">{{ getStopName(ld.leg.destStopId) }}</RouterLink>
              </div>
            </div>

            <div v-if="legIdx < selectedPlanLegsData.length - 1" class="transfer-block">
              <div class="transfer-block-rail">
                <span class="transfer-block-icon" aria-hidden="true">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="13" cy="4" r="2"/>
                    <path d="M5 22l4-9 4 4 4 5"/>
                    <path d="M9 13l-2-4 4-2 3 4 3 1"/>
                  </svg>
                </span>
              </div>
              <div class="transfer-block-content">
                <span class="transfer-block-label">{{ t('planTransfer') }}</span>
                <span class="transfer-block-detail">
                  <template v-if="getTransferWalkMeters(legIdx) > 25">
                    {{ Math.round(getTransferWalkMeters(legIdx)) }}&nbsp;m {{ t('planTransferWalk') }}
                  </template>
                  <template v-else>
                    {{ t('planTransferSameStop') }}
                  </template>
                </span>
              </div>
            </div>
          </div>
        </template>

        <div class="leg-row">
          <div class="leg-icon-col">
            <div class="leg-dot leg-dot-dest">
              <span v-if="settings.traditionalActive" class="text-sm">🏁</span>
              <svg v-else class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
              </svg>
            </div>
          </div>
          <div class="leg-label-col" v-if="activeSearchField !== 'destination'">
            <div class="origin-clickable" @click="activeSearchField = 'destination'; searchResults = []; searchQuery = ''">
              <span class="leg-type-badge leg-type-badge-dest">{{ t('planTo') }}</span>
              <div class="leg-name-wrap">
                <span class="leg-name" :title="hasValidDest ? destName : '—'">{{ hasValidDest ? destName : '—' }}</span>
                <svg class="edit-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                  <path d="m14.304 4.844 2.852 2.852M7 7H4a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h11a1 1 0 0 0 1-1v-4.5m2.409-9.91a2.017 2.017 0 0 1 0 2.853l-6.844 6.844L8 14l.713-3.565 6.844-6.844a2.015 2.015 0 0 1 2.852 0Z"/>
                </svg>
              </div>
            </div>
          </div>
          <div class="leg-label-col search-active-col" v-else>
            <div class="search-wrap">
              <input
                type="text"
                v-model="searchQuery"
                class="search-input"
                :placeholder="t('planDestSearchPlaceholder')"
                @keyup.enter="performSearch"
                @keyup.esc="activeSearchField = null"
                v-focus
              />
              <button class="search-cancel" @click="activeSearchField = null">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <path d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="search-results" v-if="searchResults.length > 0 || isSearching">
              <div v-if="isSearching" class="search-loading">
                <div class="mini-spinner"></div>
                {{ t('planSearching') }}
              </div>
              <template v-else>
                <div
                  v-for="res in searchResults"
                  :key="res.place_id"
                  class="search-result-item"
                  @click="selectDestination(res)"
                >
                  <span class="res-main">{{ res.display_name.split(',')[0] }}</span>
                  <span class="res-sub">{{ res.display_name.split(',').slice(1).join(',').trim() }}</span>
                </div>
              </template>
            </div>
          </div>
        </div>
      </section>
      <section class="flex flex-col gap-3 pb-8!">
        <div class="plan-section-head">
          <h2 class="section-label">
            <span class="w-2 h-2 rounded-full bg-sky-500 shrink-0"></span>
            {{ t('planRoutesLabel') }}
          </h2>
        </div>

        <div class="plan-time-filter" role="group" :aria-label="t('planTimeFilterLabel')">
          <div class="plan-time-filter-inputs">
            <label class="plan-time-mode">
              <select
                v-model="timeMode"
                class="plan-time-select"
                :aria-label="t('planTimeFilterLabel')"
              >
                <option value="now">{{ t('planTimeLeaveNow') }}</option>
                <option value="leave">{{ t('planTimeLeaveAt') }}</option>
                <option value="arrive">{{ t('planTimeArriveBy') }}</option>
              </select>
              <svg class="plan-time-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6"/>
              </svg>
            </label>
            <input
              v-if="timeMode !== 'now'"
              type="datetime-local"
              v-model="timeValue"
              :min="minDateTime"
              class="plan-time-datetime"
              :aria-label="timeMode === 'arrive' ? t('planTimeArriveBy') : t('planTimeLeaveAt')"
            />
          </div>
          <div class="plan-time-filter-actions">
            <button
              v-if="timeMode !== 'now'"
              type="button"
              class="plan-time-search-btn shrink-0"
              :title="t('planTimeSearchAria')"
              :aria-label="t('planTimeSearchAria')"
              :disabled="isCalculating || !timeValue || (!hasLocationPermission && !customOrigin)"
              @click="refreshRoutes"
            >
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
                <circle cx="11" cy="11" r="7" />
                <path stroke-linecap="round" stroke-linejoin="round" d="m20 20-3.5-3.5" />
              </svg>
              <span class="plan-time-search-label">{{ t('planTimeSearch') }}</span>
            </button>
            <button
              v-if="hasValidCoords && (hasLocationPermission || customOrigin)"
              type="button"
              class="swap-btn shrink-0"
              :title="t('planSwap')"
              :aria-label="t('planSwap')"
              :disabled="isCalculating"
              @click="swapOriginDestination"
            >
              <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M7 16V4m0 0L3 8m4-4l4 4M17 8v12m0 0l4-4m-4 4l-4-4"/>
              </svg>
            </button>
            <button
              v-if="hasValidCoords"
              type="button"
              class="refresh-btn shrink-0"
              :class="{ 'is-busy': isCalculating }"
              :title="t('planRefresh')"
              :aria-label="t('planRefresh')"
              :disabled="isCalculating || (!hasLocationPermission && !customOrigin)"
              @click="refreshRoutes"
            >
              <svg class="w-5 h-5" :class="{ 'animate-spin': isCalculating }" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v6h6M20 20v-6h-6M20 9A8 8 0 0 0 6 5l-2 2m0 8a8 8 0 0 0 14 4l2-2"/>
              </svg>
            </button>
          </div>
        </div>

        <div v-if="isCalculating" class="plan-loading">
          <div class="bus-loader-container">
            <span v-if="settings.traditionalActive" class="emoji-icon-xl animate-bus-run" aria-hidden="true">🚌</span>
            <span v-else-if="settings.easterEggActive" class="emoji-icon-xl animate-bus-run" aria-hidden="true">🍔</span>
            <svg v-else class="w-12 h-12 text-sky-500 animate-bus-run" viewBox="0 0 24 24"
                 fill="none" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round"
                    d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 00-10.026 0 1.106 1.106 0 00-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12"/>
            </svg>
          </div>
          <p class="loading-text animate-pulse">{{ t('planCalculating') }}</p>
        </div>

        <div v-else-if="routesWithTimes.length > 0" class="flex flex-col gap-2.5">
          <div
            v-for="(plan, index) in routesWithTimes"
            :key="plan.key"
            class="departure-card group"
            :class="{ 'is-selected': selectedPlanIndex === index }"
            @click="selectPlanAt(index)"
          >
            <div class="card-rail" :class="{ 'is-active': selectedPlanIndex === index }"></div>

            <div class="card-body">
              <div class="card-row-primary">
                <div class="bus-chain">
                  <template v-for="(leg, lIdx) in plan.legs" :key="lIdx">
                    <span
                      class="bus-chip"
                      :class="{ 'is-expanded': expandedLegs['list-' + plan.key + '-' + lIdx] }"
                      :style="{ backgroundColor: shapes[String(leg.routeIds[0])]?.route_color }"
                      :title="leg.routeIds.map(id => shapes[String(id)]?.route_short_name).join(' / ')"
                    ><template v-for="(routeId, rIdx) in (expandedLegs['list-' + plan.key + '-' + lIdx] ? leg.routeIds : leg.routeIds.slice(0, 3))" :key="rIdx"><span class="bus-chip-name">{{ shapes[String(routeId)]?.route_short_name }}</span><span v-if="rIdx < (expandedLegs['list-' + plan.key + '-' + lIdx] ? leg.routeIds.length : Math.min(leg.routeIds.length, 3)) - 1" class="bus-chip-sep">/</span></template></span><span v-if="leg.routeIds.length > 3" class="bus-chip-overflow cursor-pointer" @click.stop="toggleLegExpansion('list-' + plan.key + '-' + lIdx)">{{ expandedLegs['list-' + plan.key + '-' + lIdx] ? '«' : '+' + (leg.routeIds.length - 3) }}</span>
                    <svg v-if="lIdx < plan.legs.length - 1" class="bus-chain-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M13 6l6 6-6 6"/>
                    </svg>
                  </template>
                </div>

                <div v-if="plan.nextTimes[0]"
                  class="card-primary-time"
                  :class="plan.nextTimes[0].is_live ? 'card-primary-time-live' : 'card-primary-time-sched'">
                  <div class="card-relative-time">
                    <span v-if="plan.nextTimes[0].is_live" class="live-dot"></span>
                    {{ getRelativeDepartureFormatted(plan, plan.nextTimes[0]) }}
                  </div>
                  <div class="card-arrival-time">
                    {{ getArrivalTimeFormatted(plan, plan.nextTimes[0]) }}
                  </div>
                </div>

                <svg class="card-chevron" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
                </svg>
              </div>

              <div class="card-row-meta">
                <span class="card-arrow">→</span>
                <span class="card-dest" :title="getStopName(plan.legs[plan.legs.length-1]?.destStopId)">{{ getStopName(plan.legs[plan.legs.length-1]?.destStopId) }}</span>
              </div>

              <div class="card-row-stats">
                <span class="stat-chip stat-chip-duration">
                  <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
                  </svg>
                  {{ formatMinutes(Math.round(getJourneyDuration(plan))) }}
                </span>
                <span v-if="!plan.isDirect" class="stat-chip stat-chip-transfer">
                  <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M7 4v12m-3-3l3 3 3-3m7 9V4m-3 3l3-3 3 3"/>
                  </svg>
                  {{ plan.legs.length - 1 }}&nbsp;{{ plan.legs.length - 1 === 1 ? t('planChange') : t('planChanges') }}
                </span>
                <span v-if="plan.isLive" class="stat-chip stat-chip-live">
                  <span class="live-dot"></span>{{ t('live') }}
                </span>
                <span v-if="plan.nextTimes.length > 1" class="stat-chip-next">
                  <span class="stat-chip-label">{{ t('planThen') }}</span>
                  <template v-for="(entry, i) in plan.nextTimes.slice(1, 3)" :key="i"><span class="stat-chip-time">{{ entry.is_live ? '' : '~ ' }}{{ formatMinutesFromNow(entry.minutes, userTime || new Date(), t('now')) }}</span></template>
                </span>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="flex flex-col gap-4">
          <div class="plan-placeholder">
            <template v-if="planData && planData.plans.length > 0">
              <svg class="w-10 h-10 text-slate-300 dark:text-slate-700 mb-3" viewBox="0 0 24 24"
                   fill="none" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round"
                      d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
              </svg>
              <p class="text-sm font-medium text-slate-400 dark:text-slate-500 text-center">
                {{ t('planNoBusesNextHour') }}</p>
            </template>
            <template v-else>
              <svg class="w-10 h-10 text-slate-300 dark:text-slate-700 mb-3" viewBox="0 0 24 24"
                   fill="none" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round"
                      d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 00-10.026 0 1.106 1.106 0 00-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12"/>
              </svg>
              <p class="text-sm font-medium text-slate-400 dark:text-slate-500 text-center">
                {{ t('planNoResults') }}</p>
            </template>
          </div>

          <ClosestStopsList :stops="allStops" :center-lat="destLat" :center-lon="destLon"/>
        </div>
      </section>
    </div>

    <div v-else>
      <section class="route-legs-card mb-4">
        <div class="leg-row">
          <div class="leg-icon-col">
            <div class="leg-dot leg-dot-origin">
              <span v-if="settings.traditionalActive" class="text-sm">📍</span>
              <svg v-else class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
                <circle cx="12" cy="12" r="6"/>
              </svg>
            </div>
            <div class="leg-line"></div>
          </div>
          <div class="leg-label-col" v-if="activeSearchField !== 'origin' && (activeSearchField !== null || routesWithTimes.length > 0)">
            <div class="origin-clickable" @click="activeSearchField = 'origin'; searchResults = []; searchQuery = ''">
              <span class="leg-type-badge">{{ t('planFrom') }}</span>
              <div class="leg-name-wrap">
                <span class="leg-name">{{ originLabel }}</span>
                <svg class="edit-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                  <path d="m14.304 4.844 2.852 2.852M7 7H4a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h11a1 1 0 0 0 1-1v-4.5m2.409-9.91a2.017 2.017 0 0 1 0 2.853l-6.844 6.844L8 14l.713-3.565 6.844-6.844a2.015 2.015 0 0 1 2.852 0Z"/>
                </svg>
              </div>
            </div>
          </div>
          <div class="leg-label-col search-active-col" v-else>
            <div class="search-wrap">
              <input
                type="text"
                v-model="searchQuery"
                class="search-input"
                :placeholder="t('planSearchPlaceholder')"
                @keyup.enter="performSearch"
                @keyup.esc="activeSearchField = null"
                v-focus
              />
              <button class="search-cancel" @click="activeSearchField = null" v-if="activeSearchField === 'origin'">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <path d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="search-results" v-if="searchResults.length > 0 || isSearching || hasLocationPermission">
              <div v-if="isSearching" class="search-loading">
                <div class="mini-spinner"></div>
                {{ t('planSearching') }}
              </div>
              <template v-else>
                <div
                  v-if="hasLocationPermission"
                  class="search-result-item current-loc-option"
                  @click="useCurrentLocation"
                >
                  <span class="res-main">{{ t('planOriginCurrentLocation') }}</span>
                </div>
                <div
                  v-for="res in searchResults"
                  :key="res.place_id"
                  class="search-result-item"
                  @click="selectOrigin(res)"
                >
                  <span class="res-main">{{ res.display_name.split(',')[0] }}</span>
                  <span class="res-sub">{{ res.display_name.split(',').slice(1).join(',').trim() }}</span>
                </div>
              </template>
            </div>
          </div>
        </div>

        <div class="leg-row">
          <div class="leg-icon-col">
            <div class="leg-dot leg-dot-dest">
              <span v-if="settings.traditionalActive" class="text-sm">🏁</span>
              <svg v-else class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
              </svg>
            </div>
          </div>
          <div class="leg-label-col" v-if="activeSearchField !== 'destination'">
            <div class="origin-clickable" @click="activeSearchField = 'destination'; searchResults = []; searchQuery = ''">
              <span class="leg-type-badge leg-type-badge-dest">{{ t('planTo') }}</span>
              <div class="leg-name-wrap">
                <span class="leg-name" :title="hasValidDest ? destName : '—'">{{ hasValidDest ? destName : '—' }}</span>
                <svg class="edit-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                  <path d="m14.304 4.844 2.852 2.852M7 7H4a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h11a1 1 0 0 0 1-1v-4.5m2.409-9.91a2.017 2.017 0 0 1 0 2.853l-6.844 6.844L8 14l.713-3.565 6.844-6.844a2.015 2.015 0 0 1 2.852 0Z"/>
                </svg>
              </div>
            </div>
          </div>
          <div class="leg-label-col search-active-col" v-else>
            <div class="search-wrap">
              <input
                type="text"
                v-model="searchQuery"
                class="search-input"
                :placeholder="t('planDestSearchPlaceholder')"
                @keyup.enter="performSearch"
                @keyup.esc="activeSearchField = null"
                v-focus
              />
              <button class="search-cancel" @click="activeSearchField = null">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <path d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="search-results" v-if="searchResults.length > 0 || isSearching">
              <div v-if="isSearching" class="search-loading">
                <div class="mini-spinner"></div>
                {{ t('planSearching') }}
              </div>
              <template v-else>
                <div
                  v-for="res in searchResults"
                  :key="res.place_id"
                  class="search-result-item"
                  @click="selectDestination(res)"
                >
                  <span class="res-main">{{ res.display_name.split(',')[0] }}</span>
                  <span class="res-sub">{{ res.display_name.split(',').slice(1).join(',').trim() }}</span>
                </div>
              </template>
            </div>
          </div>
        </div>
      </section>

      <section class="flex flex-col gap-3 pb-8">
        <ClosestStopsList :stops="allStops" :center-lat="destLat" :center-lon="destLon"/>
      </section>
    </div>
  </div>
</template>

<style scoped>
.plan-view-container {
  padding: 1.25rem 1.5rem 0;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

html[data-hungry] .section-label {
  color: #92400E;
}

html.dark[data-hungry] .section-label {
  color: #fde68a;
}

html[data-hungry] .route-legs-card {
  background: #fffbeb;
  border: 2px solid #fde68a;
}

html.dark[data-hungry] .route-legs-card {
  background: #1c1608;
  border-color: #78350f;
}

.route-legs-card {
  display: flex;
  flex-direction: column;
  background: #f8fafc;
  border: 1.5px solid #e2e8f0;
  border-radius: 1rem;
  padding: 0.875rem 1rem;
  gap: 0;
}

.trip-summary {
  display: flex;
  align-items: stretch;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
  padding: 0.75rem;
  background: linear-gradient(135deg, #f0f9ff 0%, #e0f2fe 100%);
  border: 1px solid #bae6fd;
  border-radius: 1rem;
}

/* Dark mode trip-summary styling lives in src/styles/dark.css alongside the
   other route-legs/route-card overrides. */

.trip-summary-stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  min-width: 0;
  gap: 0.15rem;
  padding: 0.25rem;
}

.trip-summary-stat:not(:last-child) {
  border-right: 1px solid #bae6fd;
}


.trip-summary-stat-value {
  font-size: 1.05rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: #0c4a6e;
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
}

.trip-summary-stat-value.is-live {
  color: #047857;
}


.trip-summary-stat-label {
  font-size: 0.62rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: #0369a1;
}


html[data-hungry] .trip-summary-stat-label {
  color: #92400e;
  font-family: inherit;
}

html.dark[data-hungry] .trip-summary {
  background: linear-gradient(135deg, #1c1608 0%, #211a05 100%);
  border-color: #78350f;
}

html.dark[data-hungry] .trip-summary-stat:not(:last-child) {
  border-right-color: #78350f;
}

html.dark[data-hungry] .trip-summary-stat-value {
  color: #fde68a;
}

html.dark[data-hungry] .trip-summary-stat-value.is-live {
  color: #34d399;
}

html.dark[data-hungry] .trip-summary-stat-label {
  color: #d97706;
}

html[data-traditional] .trip-summary {
  background: #ECE9D8;
  border: 2px solid #919B9C;
  border-radius: 0;
  box-shadow: inset -1px -1px 1px #ffffff, inset 1px 1px 1px #000000;
  padding: 0.5rem;
}

html[data-traditional] .trip-summary-stat:not(:last-child) {
  border-right: 1px solid #919B9C;
}

html[data-traditional] .trip-summary-stat-value {
  color: #000000;
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
}

html[data-traditional] .trip-summary-stat-value.is-live {
  color: #006400;
}

html[data-traditional] .trip-summary-stat-label {
  color: #404040;
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
}

html[data-hungry] .trip-summary {
  background: linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%);
  border-color: #fde68a;
}

html[data-hungry] .trip-summary-stat-value {
  color: #92400e;
}

html[data-hungry] .trip-summary-stat-label {
  color: #b45309;
}

html[data-hungry] .trip-summary-stat:not(:last-child) {
  border-right-color: #fde68a;
}

/* ----------- Chomper (Hungry) – departure card sub-elements ----------- */
html[data-hungry] .card-dest {
  color: #78350f !important;
}

html[data-hungry] .card-arrow {
  color: #b45309 !important;
}

html[data-hungry] .bus-chain-arrow {
  color: #92400e !important;
}

html[data-hungry] .bus-chip-overflow {
  background: #fde68a !important;
  color: #92400e !important;
  border-color: #f59e0b !important;
}

html[data-hungry] .card-arrival-time {
  color: #b45309 !important;
}

html[data-hungry] .card-primary-time-sched {
  color: #78350f !important;
}

html[data-hungry] .card-primary-time-live {
  color: #047857 !important;
}

html[data-hungry] .departure-card.is-selected .card-primary-time-live {
  color: #047857 !important;
}

html[data-hungry] .departure-card.is-selected .live-dot {
  background: #10b981 !important;
}

html[data-hungry] .card-chevron {
  color: #d97706 !important;
}

html[data-hungry] .departure-card:hover .card-chevron {
  color: #92400e !important;
}

html[data-hungry] .departure-card.is-selected .card-chevron {
  color: #78350f !important;
}

html[data-hungry] .card-rail.is-active {
  background: #f59e0b !important;
}

html[data-hungry] .stat-chip {
  background: #fef3c7 !important;
  color: #92400e !important;
}

html[data-hungry] .stat-chip-transfer {
  background: #fde68a !important;
  color: #78350f !important;
}

html[data-hungry] .stat-chip-live {
  background: #ecfdf5 !important;
  color: #047857 !important;
}

html[data-hungry] .stat-chip-duration {
  background: #fef3c7 !important;
  color: #92400e !important;
}

html[data-hungry] .stat-chip-next {
  color: #b45309 !important;
}

html[data-hungry] .stat-chip-time {
  color: #78350f !important;
}

html[data-hungry] .stat-chip-label {
  color: #b45309 !important;
}

/* Chomper dark – departure card sub-elements */
html.dark[data-hungry] .card-dest {
  color: #fde68a !important;
}

html.dark[data-hungry] .card-arrow {
  color: #d97706 !important;
}

html.dark[data-hungry] .bus-chain-arrow {
  color: #fbbf24 !important;
}

html.dark[data-hungry] .bus-chip-overflow {
  background: #451a03 !important;
  color: #fde68a !important;
  border-color: #d97706 !important;
}

html.dark[data-hungry] .card-arrival-time {
  color: #d97706 !important;
}

html.dark[data-hungry] .card-primary-time-sched {
  color: #fde68a !important;
}

html.dark[data-hungry] .card-primary-time-live {
  color: #34d399 !important;
}

html.dark[data-hungry] .departure-card.is-selected .card-primary-time-live {
  color: #34d399 !important;
}

html.dark[data-hungry] .departure-card.is-selected .live-dot {
  background: #34d399 !important;
}

html.dark[data-hungry] .card-chevron {
  color: #78350f !important;
}

html.dark[data-hungry] .departure-card:hover .card-chevron {
  color: #d97706 !important;
}

html.dark[data-hungry] .departure-card.is-selected .card-chevron {
  color: #fde68a !important;
}

html.dark[data-hungry] .card-rail.is-active {
  background: #d97706 !important;
}

html.dark[data-hungry] .stat-chip {
  background: #451a03 !important;
  color: #fde68a !important;
}

html.dark[data-hungry] .stat-chip-transfer {
  background: #78350f !important;
  color: #fde68a !important;
}

html.dark[data-hungry] .stat-chip-live {
  background: rgba(16, 185, 129, 0.15) !important;
  color: #34d399 !important;
}

html.dark[data-hungry] .stat-chip-duration {
  background: #451a03 !important;
  color: #fde68a !important;
}

html.dark[data-hungry] .stat-chip-next {
  color: #d97706 !important;
}

html.dark[data-hungry] .stat-chip-time {
  color: #fde68a !important;
}

html.dark[data-hungry] .stat-chip-label {
  color: #d97706 !important;
}

.leg-ride-time {
  font-weight: 500;
  color: #94a3b8;
  text-transform: none;
  letter-spacing: 0;
  font-size: 0.62rem;
  margin-left: 0.15rem;
}

.dark .leg-ride-time {
  color: #64748b;
}

.leg-row {
  display: flex;
  align-items: stretch;
  gap: 0.875rem;
  min-height: 2.5rem;
}

.leg-icon-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 1.5rem;
  flex-shrink: 0;
}

.leg-dot {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 0.125rem;
}

.boarding-dot {
  background: white;
  border-width: 2px;
}

.dot-inner {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
}

.dark .boarding-dot {
  background: #0f172a;
}

.dark .leg-dot-origin {
  box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.2);
}

.dark .leg-dot-dest {
  box-shadow: 0 0 0 3px rgba(2, 132, 199, 0.2);
}

/* Hungry Theme Overrides */
html[data-hungry] .leg-dot-origin {
  background: #f59e0b;
  box-shadow: 0 0 0 3px #fef3c7;
}

html[data-hungry] .leg-dot-dest {
  background: #b45309;
  box-shadow: 0 0 0 3px #fde68a;
}

html[data-hungry] .leg-type-badge {
  color: #d97706;
}

/* Traditional Theme Overrides */
html[data-traditional] .leg-dot-origin,
html[data-traditional] .leg-dot-dest {
  background: transparent !important;
  box-shadow: none !important;
}

html[data-traditional] .boarding-dot {
  border-radius: 0;
}

html[data-traditional] .intermediate-dot {
  border-radius: 0;
}

html[data-traditional] .intermediate-dot div {
  border-radius: 0;
}

html[data-traditional] .dot-inner {
  border-radius: 0;
}

.leg-dot-origin {
  background: #0ea5e9;
  box-shadow: 0 0 0 3px #e0f2fe;
}

.leg-dot-dest {
  background: #0284c7;
  box-shadow: 0 0 0 3px #bae6fd;
}

.leg-line {
  flex: 1;
  width: 2px;
  background: repeating-linear-gradient(to bottom, #7dd3fc 0, #7dd3fc 4px, transparent 4px, transparent 8px);
  margin: 0.25rem 0;
  min-height: 0.75rem;
}

.leg-label-col {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.125rem;
  flex: 1;
  min-width: 0;
  padding-bottom: 0.625rem;
}

.leg-row:last-child .leg-label-col {
  padding-bottom: 0;
}

.leg-type-badge {
  font-size: 0.625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #0ea5e9;
}

.leg-type-badge-dest {
  color: #0284c7;
}

.leg-name {
  font-size: 0.9375rem;
  font-weight: 600;
  color: #0f172a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 90%;
}

.intermediate-leg {
  min-height: 1.5rem;
}

.intermediate-dot {
  width: 0.75rem;
  height: 0.75rem;
  background: white;
  border: 1px solid #e2e8f0;
}

.intermediate-name {
  font-size: 0.75rem;
  font-weight: 500;
  color: #64748b;
}

.dark .intermediate-name {
  color: #94a3b8;
}

.dark .intermediate-dot {
  background: #1e293b;
  border-color: #334155;
}

.plan-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2.5rem 1rem;
  border: 1.5px dashed #e2e8f0;
  border-radius: 1rem;
  background: #fafafa;
}

.leg-line-dashed {
  background: repeating-linear-gradient(to bottom, #cbd5e1 0, #cbd5e1 4px, transparent 4px, transparent 8px) !important;
}

.leg-row-alight-transfer .leg-label-col {
  padding-bottom: 0;
}

.transfer-block {
  display: flex;
  align-items: stretch;
  gap: 0.875rem;
  margin: 0.5rem 0 0.6rem;
}

.transfer-block-rail {
  width: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  position: relative;
}

.transfer-block-rail::before,
.transfer-block-rail::after {
  content: "";
  position: absolute;
  left: 50%;
  width: 2px;
  margin-left: -1px;
  background: repeating-linear-gradient(to bottom, #cbd5e1 0, #cbd5e1 3px, transparent 3px, transparent 6px);
}

.transfer-block-rail::before {
  top: -0.5rem;
  height: calc(50% - 0.65rem);
}

.transfer-block-rail::after {
  bottom: -0.6rem;
  height: calc(50% - 0.65rem);
}

.dark .transfer-block-rail::before,
.dark .transfer-block-rail::after {
  background: repeating-linear-gradient(to bottom, #475569 0, #475569 3px, transparent 3px, transparent 6px);
}

.transfer-block-icon {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 9999px;
  background: #fef3c7;
  color: #b45309;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  z-index: 1;
}

.transfer-block-icon svg {
  width: 0.85rem;
  height: 0.85rem;
}

.dark .transfer-block-icon {
  background: rgba(245, 158, 11, 0.18);
  color: #fbbf24;
}

.transfer-block-content {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1;
  min-width: 0;
  padding: 0.35rem 0.65rem;
  border-radius: 0.55rem;
  background: #fffbeb;
  border: 1px dashed #fde68a;
}

.dark .transfer-block-content {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(251, 191, 36, 0.3);
}

.transfer-block-label {
  font-size: 0.65rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: #b45309;
}

.dark .transfer-block-label {
  color: #fbbf24;
}

.transfer-block-detail {
  font-size: 0.75rem;
  font-weight: 600;
  color: #92400e;
}

.dark .transfer-block-detail {
  color: #fcd34d;
}

html[data-traditional] .transfer-block-icon,
html[data-traditional] .transfer-block-content {
  border-radius: 0;
}

/* ----------- Traditional – departure card sub-elements ----------- */
html[data-traditional] .departure-card {
  border-radius: 0;
}

html[data-traditional] .stat-chip {
  border-radius: 0 !important;
  background: var(--xp-btn) !important;
  border: 1px solid var(--xp-border) !important;
  color: #000000 !important;
  font-family: var(--xp-font) !important;
}

html[data-traditional] .stat-chip-transfer {
  background: var(--xp-btn) !important;
  color: #000000 !important;
}

html[data-traditional] .stat-chip-live {
  background: var(--xp-live) !important;
  color: #FFFFFF !important;
  border: 1px solid #3D7E22 !important;
}

html[data-traditional] .stat-chip-duration {
  background: var(--xp-btn) !important;
  color: #000000 !important;
}

html[data-traditional] .stat-chip-next {
  color: #404040 !important;
  font-family: var(--xp-font) !important;
}

html[data-traditional] .stat-chip-time {
  color: #000000 !important;
  font-family: var(--xp-font) !important;
}

html[data-traditional] .stat-chip-label {
  font-family: var(--xp-font) !important;
}

html[data-traditional] .card-dest {
  color: #000000 !important;
  font-family: var(--xp-font) !important;
}

html[data-traditional] .card-arrow {
  color: #404040 !important;
}

html[data-traditional] .bus-chain-arrow {
  color: #000000 !important;
}

html[data-traditional] .card-arrival-time {
  color: #404040 !important;
  font-family: var(--xp-font) !important;
}

html[data-traditional] .card-primary-time-sched {
  color: #000000 !important;
  font-family: var(--xp-font) !important;
}

html[data-traditional] .card-primary-time-live {
  color: #006400 !important;
  font-family: var(--xp-font) !important;
}

html[data-traditional] .departure-card.is-selected .card-primary-time-live {
  color: #90EE90 !important;
}

html[data-traditional] .departure-card.is-selected .live-dot {
  background: #90EE90 !important;
}

html[data-traditional] .card-chevron {
  color: var(--xp-blue) !important;
}

html[data-traditional] .card-rail {
  border-radius: 0 !important;
}

html[data-traditional] .card-rail.is-active {
  background: var(--xp-blue) !important;
}

html[data-traditional] .bus-chip {
  border: 1px solid rgba(0, 0, 0, 0.35) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.35) !important;
  border-radius: 0 !important;
  font-family: var(--xp-font) !important;
}

html[data-traditional] .bus-chip-overflow {
  background: var(--xp-btn) !important;
  border: 1px solid var(--xp-border) !important;
  border-radius: 0 !important;
  color: var(--xp-text) !important;
  box-shadow: none !important;
  font-family: var(--xp-font) !important;
  padding: 0 0.35rem !important;
}

html[data-traditional] .bus-chip-overflow:hover {
  background: var(--xp-btn-hover) !important;
}

html[data-traditional] .bus-chip-overflow:active {
  background: var(--xp-btn-active) !important;
  color: white !important;
}

html[data-traditional] .live-dot {
  border-radius: 0 !important;
  background: #3D7E22 !important;
  box-shadow: none !important;
}

html[data-traditional] .time-pill {
  border-radius: 0 !important;
}

html[data-traditional] .time-pill-live {
  background: #3D7E22 !important;
}

html[data-traditional] .time-pill-sched {
  background: var(--xp-btn) !important;
  color: #000000 !important;
  border: 1px solid var(--xp-border) !important;
}

/* Traditional dark – departure card sub-elements */
html.dark[data-traditional] .stat-chip {
  background: var(--xp-btn) !important;
  border: 1px solid var(--xp-border) !important;
  color: var(--xp-text) !important;
}

html.dark[data-traditional] .stat-chip-transfer {
  background: var(--xp-btn) !important;
  color: var(--xp-text) !important;
}

html.dark[data-traditional] .stat-chip-live {
  background: linear-gradient(to bottom, #7EC860 0%, #6FB452 50%, #5BAA38 100%) !important;
  color: #FFFFFF !important;
  border: 1px solid #5BAA38 !important;
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.4) !important;
}

html.dark[data-traditional] .stat-chip-duration {
  background: var(--xp-btn) !important;
  color: var(--xp-text) !important;
}

html.dark[data-traditional] .stat-chip-next {
  color: #8898B0 !important;
}

html.dark[data-traditional] .stat-chip-time {
  color: var(--xp-text) !important;
}

html.dark[data-traditional] .card-dest {
  color: var(--xp-text) !important;
}

html.dark[data-traditional] .card-arrow {
  color: #8898B0 !important;
}

html.dark[data-traditional] .bus-chain-arrow {
  color: var(--xp-text) !important;
}

html.dark[data-traditional] .card-arrival-time {
  color: #8898B0 !important;
}

html.dark[data-traditional] .card-primary-time-sched {
  color: var(--xp-text) !important;
}

html.dark[data-traditional] .card-primary-time-live {
  color: #5BAA38 !important;
}

html.dark[data-traditional] .departure-card.is-selected .card-primary-time-live {
  color: #90EE90 !important;
}

html.dark[data-traditional] .departure-card.is-selected .live-dot {
  background: #90EE90 !important;
}

html.dark[data-traditional] .card-chevron {
  color: var(--xp-blue) !important;
}

html.dark[data-traditional] .card-rail.is-active {
  background: var(--xp-blue) !important;
}

html.dark[data-traditional] .bus-chip {
  border: 1px solid rgba(0, 0, 0, 0.5) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.15) !important;
}

html.dark[data-traditional] .bus-chip-overflow {
  background: var(--xp-btn) !important;
  border: 1px solid var(--xp-border) !important;
  color: var(--xp-text) !important;
}

html.dark[data-traditional] .live-dot {
  background: #5BAA38 !important;
  box-shadow: none !important;
}

html.dark[data-traditional] .time-pill-live {
  background: #5BAA38 !important;
}

html.dark[data-traditional] .time-pill-sched {
  background: var(--xp-btn) !important;
  color: var(--xp-text) !important;
  border: 1px solid var(--xp-border) !important;
}

.departure-card {
  display: flex;
  align-items: stretch;
  gap: 0.625rem;
  padding: 0.875rem 0.875rem 0.875rem 0;
  border-radius: 1rem;
  border: 1px solid #f1f5f9;
  background: #f8fafc;
  container-type: inline-size;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
}

.departure-card:hover {
  background: white;
  border-color: #e2e8f0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.departure-card.is-selected {
  border-color: #0ea5e9;
  background: #f0f9ff;
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.12);
}

.dark .departure-card {
  background: #1e293b;
  border-color: #334155;
}

.dark .departure-card:hover {
  background: #1e293b;
  border-color: #475569;
}

.dark .departure-card.is-selected {
  background: #0f172a;
  border-color: #38bdf8;
}

.card-rail {
  width: 4px;
  flex-shrink: 0;
  border-radius: 0 4px 4px 0;
  background: transparent;
  transition: background 0.15s;
}

.card-rail.is-active {
  background: #0ea5e9;
}

.dark .card-rail.is-active {
  background: #38bdf8;
}

.card-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  padding-left: 0.5rem;
}

.card-row-primary {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  min-width: 0;
}

.bus-chain {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
}

.bus-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.25rem 0.55rem;
  min-width: 2.25rem;
  border-radius: 0.5rem;
  font-weight: 800;
  font-size: 0.78rem;
  letter-spacing: 0.01em;
  color: white;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
  white-space: nowrap;

  flex-wrap: wrap;
  flex-shrink: 0;
  max-width: 100%;
  height: auto;
}

.bus-chip.is-expanded {
  white-space: normal;
  height: auto;
  min-height: 1.75rem;
  flex-wrap: wrap;
  padding: 0.35rem 0.55rem;
  flex-shrink: 0;
  max-width: 100%;
}

.bus-chip-name {
  line-height: 1;
}

.bus-chip-sep {
  opacity: 0.65;
  margin: 0 0.18rem;
  font-weight: 600;
}

.bus-chip-overflow {
  display: inline-flex;
  align-items: center;
  height: 1.75rem;
  padding: 0 0.45rem;
  font-size: 0.72rem;
  font-weight: 700;
  white-space: nowrap;
  border-radius: 0.45rem;
  background: rgba(148, 163, 184, 0.12);
  color: #64748b;
  border: 1px solid rgba(148, 163, 184, 0.2);
  cursor: pointer;
  transition: all 0.15s;
}

.bus-chip-overflow:hover {
  background: rgba(148, 163, 184, 0.2);
  color: #475569;
}

.dark .bus-chip-overflow {
  background: rgba(148, 163, 184, 0.15);
  color: rgba(255,255,255,0.7);
  border: 1px solid rgba(148, 163, 184, 0.25);
}

.dark .bus-chip-overflow:hover {
  background: rgba(148, 163, 184, 0.25);
  color: #fff;
}

.bus-chain-arrow {
  width: 0.95rem;
  height: 0.95rem;
  color: #64748b;
  flex-shrink: 0;
}

.dark .bus-chain-arrow {
  color: #94a3b8;
}

.card-primary-time {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  font-variant-numeric: tabular-nums;
  margin-left: auto;
  line-height: 1.2;
}

.card-relative-time {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.95rem;
  font-weight: 800;
  white-space: nowrap;
}

.card-arrival-time {
  font-size: 0.72rem;
  font-weight: 600;
  color: #64748b;
  white-space: nowrap;
}

.dark .card-arrival-time {
  color: #94a3b8;
}

.card-primary-time-sched {
  color: #334155;
}

.dark .card-primary-time-sched {
  color: #e2e8f0;
}

.card-primary-time-live {
  color: #059669;
}

.dark .card-primary-time-live {
  color: #34d399;
}

.live-dot {
  display: inline-block;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
  background: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.18);
  animation: live-pulse 1.4s ease-in-out infinite;
}

@keyframes live-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.45; }
}

.card-chevron {
  width: 1rem;
  height: 1rem;
  color: #cbd5e1;
  flex-shrink: 0;
  transition: color 0.15s, transform 0.15s;
}

.departure-card:hover .card-chevron {
  color: #64748b;
  transform: translateX(2px);
}

.departure-card.is-selected .card-chevron {
  color: #0ea5e9;
}

.dark .card-chevron {
  color: #475569;
}

.dark .departure-card:hover .card-chevron {
  color: #94a3b8;
}

.dark .departure-card.is-selected .card-chevron {
  color: #38bdf8;
}

.card-row-meta {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  min-width: 0;
}

.card-arrow {
  color: #94a3b8;
  font-weight: 600;
  font-size: 0.85rem;
  flex-shrink: 0;
}

.card-dest {
  font-size: 0.85rem;
  font-weight: 700;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.dark .card-dest {
  color: #f1f5f9;
}

.card-row-stats {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.4rem;
  font-size: 0.7rem;
}

.stat-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.15rem 0.45rem;
  border-radius: 9999px;
  background: #f1f5f9;
  color: #475569;
  font-weight: 600;
  white-space: nowrap;
}

.dark .stat-chip {
  background: #334155;
  color: #cbd5e1;
}

.stat-chip-transfer {
  color: #475569;
  background: #e2e8f0;
}

.dark .stat-chip-transfer {
  color: #cbd5e1;
  background: #334155;
}

.stat-chip-live {
  background: #ecfdf5;
  color: #059669;
}

.stat-chip-duration {
  background: #f1f5f9;
  color: #475569;
}

.dark .stat-chip-duration {
  background: #1e293b;
  color: #94a3b8;
}

.dark .stat-chip-live {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
}

.stat-chip-next {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: #94a3b8;
  font-weight: 500;
}

.stat-chip-label {
  text-transform: lowercase;
  font-size: 0.7rem;
}

.stat-chip-time {
  font-weight: 700;
  color: #64748b;
  font-variant-numeric: tabular-nums;
}

.dark .stat-chip-time {
  color: #cbd5e1;
}

.dark .stat-chip-next {
  color: #64748b;
}

.time-pill {
  font-size: 0.7rem;
  font-weight: 700;
  padding: 0.2rem 0.45rem;
  border-radius: 0.5rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.time-pill-live {
  background: #10b981;
  color: white;
}

.time-pill-sched {
  background: #f1f5f9;
  color: #475569;
}

.dark .time-pill-sched {
  background: #334155;
  color: #cbd5e1;
}

.refresh-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 9999px;
  color: #64748b;
  background: transparent;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, transform 0.15s;
}

.refresh-btn:hover {
  background: #f1f5f9;
  color: #0ea5e9;
}

.refresh-btn:active {
  transform: rotate(-30deg);
}

.refresh-btn:disabled {
  cursor: wait;
}

.dark .refresh-btn {
  color: #94a3b8;
}

.dark .refresh-btn:hover {
  background: #334155;
  color: #38bdf8;
}

.swap-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 9999px;
  color: #64748b;
  background: transparent;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, transform 0.2s;
}

.swap-btn:hover {
  background: #f1f5f9;
  color: #0ea5e9;
}

.swap-btn:active {
  transform: rotate(180deg);
}

.swap-btn:disabled {
  cursor: wait;
  opacity: 0.4;
}

.dark .swap-btn {
  color: #94a3b8;
}

.dark .swap-btn:hover {
  background: #334155;
  color: #38bdf8;
}

.animate-spin {
  animation: spin 0.9s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.leg-name-link {
  text-decoration: none;
  color: inherit;
  transition: color 0.15s;
  display: inline-block;
  padding: 0.15rem 0;
  margin: -0.15rem 0;
}

.leg-name-link:hover,
.leg-name-link:active {
  color: #0ea5e9;
  text-decoration: underline;
  text-underline-offset: 3px;
}

.dark .leg-name-link:hover,
.dark .leg-name-link:active {
  color: #38bdf8;
}

.leg-route-link {
  color: inherit;
  text-decoration: none;
  border-bottom: 1px dashed currentColor;
  padding-bottom: 1px;
}

.leg-route-link:hover,
.leg-route-link:active {
  border-bottom-style: solid;
}

.leg-route-sep {
  opacity: 0.6;
}

.leg-route-overflow {
  font-size: 0.75rem;
  font-weight: 700;
  opacity: 0.6;
  margin-left: 0.1rem;
  cursor: pointer;
  transition: opacity 0.15s;
}

.leg-route-overflow:hover {
  opacity: 1;
  text-decoration: underline;
}

.fav-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 9999px;
  color: #94a3b8;
  background: transparent;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, transform 0.15s;
}

.fav-btn:hover {
  background: #fef2f2;
  color: #f43f5e;
}

.fav-btn:active {
  transform: scale(0.92);
}

.fav-btn.is-fav {
  color: #f43f5e;
}

.fav-btn.is-fav:hover {
  background: #fee2e2;
}

.plan-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem 1rem;
  gap: 1rem;
  background: #f8fafc;
  border-radius: 1rem;
  border: 1.5px solid #e2e8f0;
}

.dark .plan-loading {
  background: #1e293b;
  border-color: #334155;
}

.bus-loader-container {
  overflow: hidden;
  width: 100px;
  display: flex;
  justify-content: center;
}

html[data-traditional] .bus-loader-container {
  width: auto;
  overflow: visible;
}

.animate-bus-run {
  animation: bus-run 1.2s infinite linear;
}

.loading-text {
  font-size: 0.875rem;
  font-weight: 600;
  color: #64748b;
}

.dark .loading-text {
  color: #94a3b8;
}

/* Hungry Theme */
html[data-hungry] .plan-loading {
  background: #fffbeb;
  border-color: #fde68a;
}

html[data-hungry] .loading-text {
  color: #d97706;
}

html.dark[data-hungry] .plan-loading {
  background: #1c1608;
  border-color: #78350f;
}

html.dark[data-hungry] .loading-text {
  color: #fde68a;
}

/* Traditional Theme */
html[data-traditional] .plan-loading {
  background: var(--xp-tan, #ECE9D8);
  border: 2px solid var(--xp-border, #919B9C);
  border-radius: 0;
  box-shadow: inset -1px -1px 1px #ffffff, inset 1px 1px 1px #000000;
}

html.dark[data-traditional] .plan-loading {
  box-shadow: inset -1px -1px 1px #444a5c, inset 1px 1px 1px #000000;
}

html[data-traditional] .loading-text {
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
  color: var(--xp-text, #000000);
  animation: none !important;
}

.dark .plan-placeholder {
  background: #1e293b;
  border-color: #334155;
}

html[data-hungry] .plan-placeholder {
  background: #fffbeb;
  border-color: #fde68a;
  border-style: solid;
}

html[data-traditional] .plan-placeholder {
  background: var(--xp-tan, #ECE9D8);
  border: 2px solid var(--xp-border, #919B9C);
  border-radius: 0;
  box-shadow: inset -1px -1px 1px #ffffff, inset 1px 1px 1px #000000;
  border-style: solid;
}

html.dark[data-traditional] .plan-placeholder {
  box-shadow: inset -1px -1px 1px #444a5c, inset 1px 1px 1px #000000;
}

.origin-clickable {
  cursor: pointer;
  width: 100%;
}

.leg-name-wrap {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.edit-icon {
  width: 0.875rem;
  height: 0.875rem;
  color: #94a3b8;
  opacity: 0.6;
}

.origin-clickable:hover .edit-icon {
  opacity: 1;
  color: #0ea5e9;
}

.search-active-col {
  width: 100%;
  padding-bottom: 0.5rem;
  position: relative;
}

.search-wrap {
  display: flex;
  align-items: center;
  background: #ffffff;
  border: 1.5px solid #e2e8f0;
  border-radius: 0.5rem;
  padding: 0.25rem 0.5rem;
  gap: 0.5rem;
}

.search-wrap:focus-within {
  border-color: #0ea5e9;
  box-shadow: 0 0 0 2px rgba(14, 165, 233, 0.1);
}

.search-input {
  flex: 1;
  border: none;
  background: transparent;
  font-size: 1rem;
  color: #1e293b;
  padding: 0.25rem 0;
  outline: none;
  min-width: 0;
}

.search-cancel {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  color: #94a3b8;
  border-radius: 50%;
  flex-shrink: 0;
}

.search-cancel:hover {
  background: #f1f5f9;
  color: #475569;
}

.search-results {
  position: absolute;
  z-index: 100;
  margin-top: 0.25rem;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
  width: 100%;
  max-height: 200px;
  overflow-y: auto;
  left: 0;
  top: 100%;
}

.search-loading {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem;
  font-size: 0.875rem;
  color: #64748b;
}

.search-result-item {
  padding: 0.625rem 0.75rem;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  border-bottom: 1px solid #f1f5f9;
}

.search-result-item:last-child {
  border-bottom: none;
}

.search-result-item:hover {
  background: #f8fafc;
}

.res-main {
  font-size: 0.875rem;
  font-weight: 600;
  color: #1e293b;
}

.res-sub {
  font-size: 0.75rem;
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.current-loc-option {
  background: #f0f9ff;
  border-bottom: 1px solid #e0f2fe;
}

.current-loc-option:hover {
  background: #e0f2fe;
}

.mini-spinner {
  width: 0.75rem;
  height: 0.75rem;
  border: 2px solid #e2e8f0;
  border-top-color: #0ea5e9;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

html.dark .search-wrap {
  background: #1e293b;
  border-color: #334155;
}

html.dark .search-input {
  color: #f1f5f9;
}

html.dark .search-results {
  background: #1e293b;
  border-color: #334155;
}

html.dark .search-result-item {
  border-color: #334155;
}

html.dark .search-result-item:hover {
  background: #334155;
}

html.dark .res-main {
  color: #f1f5f9;
}

html.dark .res-sub {
  color: #94a3b8;
}

html.dark .current-loc-option {
  background: #1e293b;
  border-bottom: 1px solid #334155;
}

html.dark .current-loc-option:hover {
  background: #334155;
}

html[data-traditional] .search-results {
  background: #ECE9D8;
  border: 1px solid #919B9C;
  border-radius: 0;
  box-shadow: 2px 2px 0 rgba(0,0,0,0.5);
}

html[data-traditional] .search-result-item {
  border-bottom: 1px solid #919B9C;
  font-family: 'Tahoma', sans-serif;
}

html[data-traditional] .search-result-item:hover {
  background: #316AC5;
}

html[data-traditional] .search-result-item:hover .res-main,
html[data-traditional] .search-result-item:hover .res-sub {
  color: #FFFFFF !important;
}

html[data-traditional] .res-main {
  color: #000000;
}

html[data-traditional] .res-sub {
  color: #444444;
}

html[data-traditional] .current-loc-option {
  background: #ECE9D8;
  font-style: italic;
}

html[data-traditional] .current-loc-option:hover {
  background: #316AC5;
}

html[data-traditional] .mini-spinner {
  border-top-color: #316AC5;
}

/* Traditional Dark */
html.dark[data-traditional] .search-results {
  background: #2A2D38;
  border-color: #444A5C;
}

html.dark[data-traditional] .search-result-item {
  border-bottom-color: #444A5C;
}

html.dark[data-traditional] .res-main {
  color: #E0E6F2;
}

html.dark[data-traditional] .res-sub {
  color: #94A3B8;
}

html.dark[data-traditional] .current-loc-option {
  background: #2A2D38;
  border-bottom-color: #444A5C;
}

html.dark[data-traditional] .current-loc-option:hover {
  background: #316AC5;
}

html[data-hungry] .origin-clickable:hover .edit-icon {
  color: #F59E0B;
}

html[data-hungry] .search-wrap {
  background: #FFFBEB;
  border-color: #F59E0B;
}

html[data-hungry] .search-wrap:focus-within {
  border-color: #D97706;
}

html[data-hungry] .search-input {
  color: #92400E;
}

html[data-hungry] .search-cancel:hover {
  background: #FEF3C7;
  color: #92400E;
}

html[data-hungry] .search-results {
  background: #FFFBEB;
  border: 2px solid #F59E0B;
  border-radius: 0.5rem;
}

html[data-hungry] .search-result-item {
  border-bottom-color: #FEF3C7;
}

html[data-hungry] .search-result-item:hover {
  background: #FEF3C7;
}

html[data-hungry] .res-main {
  color: #92400E;
}

html[data-hungry] .res-sub {
  color: #B45309;
}

html[data-hungry] .current-loc-option {
  background: #FFFBEB;
  border-bottom-color: #FEF3C7;
}

html[data-hungry] .current-loc-option:hover {
  background: #FEF3C7;
}

html[data-hungry] .mini-spinner {
  border-top-color: #F59E0B;
}

/* Chomper Dark */
html.dark[data-hungry] .origin-clickable:hover .edit-icon {
  color: #d97706;
}

html.dark[data-hungry] .search-wrap {
  background: #211a05;
  border-color: #78350f;
}

html.dark[data-hungry] .search-wrap:focus-within {
  border-color: #d97706;
}

html.dark[data-hungry] .search-input {
  color: #fde68a;
}

html.dark[data-hungry] .search-cancel:hover {
  background: #2a2006;
  color: #fde68a;
}

html.dark[data-hungry] .search-results {
  background: #1c1608;
  border-color: #78350f;
}

html.dark[data-hungry] .search-result-item {
  border-bottom-color: #451a03;
}

html.dark[data-hungry] .search-result-item:hover {
  background: #2a2006;
}

html.dark[data-hungry] .res-main {
  color: #fde68a;
}

html.dark[data-hungry] .res-sub {
  color: #d97706;
}

html.dark[data-hungry] .current-loc-option {
  background: #211a05;
  border-bottom-color: #451a03;
}

html.dark[data-hungry] .current-loc-option:hover {
  background: #2a2006;
}

html.dark[data-hungry] .mini-spinner {
  border-top-color: #d97706;
}

/* ============================================================
   Plan time filter (Google-style "Leave / Arrive" + actions)
   ============================================================ */
.plan-time-filter {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.625rem;
  background: #f8fafc;
  border: 1.5px solid #e2e8f0;
  border-radius: 0.875rem;
  flex-wrap: wrap;
}

.plan-time-filter-inputs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1 1 auto;
  min-width: 0;
  flex-wrap: wrap;
}

.plan-time-filter-actions {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin-left: auto;
}

.plan-time-mode {
  position: relative;
  display: inline-flex;
  align-items: center;
}

.plan-time-select {
  appearance: none;
  -webkit-appearance: none;
  background: white;
  border: 1.5px solid #e2e8f0;
  border-radius: 9999px;
  color: #0f172a;
  font-size: 0.85rem;
  font-weight: 600;
  padding: 0.4rem 1.85rem 0.4rem 0.95rem;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}

.plan-time-select:hover,
.plan-time-select:focus {
  border-color: #0ea5e9;
  color: #0ea5e9;
  outline: none;
}

.plan-time-chevron {
  position: absolute;
  right: 0.55rem;
  top: 50%;
  width: 0.85rem;
  height: 0.85rem;
  transform: translateY(-50%);
  color: #64748b;
  pointer-events: none;
}

.plan-time-datetime {
  background: white;
  border: 1.5px solid #e2e8f0;
  border-radius: 0.625rem;
  color: #0f172a;
  font-size: 1rem;
  font-weight: 500;
  padding: 0.4rem 0.6rem;
  min-width: 0;
  flex: 1 1 9rem;
  cursor: pointer;
  transition: border-color 0.15s;
}

.plan-time-datetime:hover,
.plan-time-datetime:focus {
  border-color: #0ea5e9;
  outline: none;
}

/* ----------- Default Dark ----------- */
.dark .plan-time-filter {
  background: #1e293b;
  border-color: #334155;
}

.dark .plan-time-select {
  background: #0f172a;
  border-color: #334155;
  color: #e2e8f0;
}

.dark .plan-time-select:hover,
.dark .plan-time-select:focus {
  border-color: #38bdf8;
  color: #38bdf8;
}

.dark .plan-time-chevron {
  color: #94a3b8;
}

.dark .plan-time-datetime {
  background: #0f172a;
  border-color: #334155;
  color: #e2e8f0;
  color-scheme: dark;
}

.dark .plan-time-datetime:hover,
.dark .plan-time-datetime:focus {
  border-color: #38bdf8;
}

/* ----------- Hungry (Chomper) Theme ----------- */
html[data-hungry] .plan-time-filter {
  background: #FFFBEB;
  border: 2px solid #F59E0B;
  border-radius: 0.5rem;
}

html[data-hungry] .plan-time-select,
html[data-hungry] .plan-time-datetime {
  background: white;
  border: 2px solid #F59E0B;
  border-radius: 0.5rem;
  color: #92400E;
}

html[data-hungry] .plan-time-select:hover,
html[data-hungry] .plan-time-select:focus,
html[data-hungry] .plan-time-datetime:hover,
html[data-hungry] .plan-time-datetime:focus {
  border-color: #D97706;
  color: #92400E;
}

html[data-hungry] .plan-time-chevron {
  color: #B45309;
}

html.dark[data-hungry] .plan-time-filter {
  background: #1c1608;
  border-color: #78350f;
}

html.dark[data-hungry] .plan-time-select,
html.dark[data-hungry] .plan-time-datetime {
  background: #211a05;
  border-color: #78350f;
  color: #fde68a;
  color-scheme: dark;
}

html.dark[data-hungry] .plan-time-select:hover,
html.dark[data-hungry] .plan-time-select:focus,
html.dark[data-hungry] .plan-time-datetime:hover,
html.dark[data-hungry] .plan-time-datetime:focus {
  border-color: #d97706;
  color: #fde68a;
}

html.dark[data-hungry] .plan-time-chevron {
  color: #d97706;
}

html[data-hungry] .refresh-btn,
html[data-hungry] .swap-btn {
  color: #B45309;
}

html[data-hungry] .refresh-btn:hover,
html[data-hungry] .swap-btn:hover {
  background: #FEF3C7;
  color: #92400E;
}

html.dark[data-hungry] .refresh-btn,
html.dark[data-hungry] .swap-btn {
  color: #d97706;
}

html.dark[data-hungry] .refresh-btn:hover,
html.dark[data-hungry] .swap-btn:hover {
  background: #211a05;
  color: #fde68a;
}

/* ----------- Traditional (Windows XP Luna) Theme ----------- */
html[data-traditional] .plan-time-filter {
  background: #ECE9D8;
  border: 1px solid #7F9DB9;
  border-radius: 0;
  padding: 0.4rem 0.5rem;
}

html[data-traditional] .plan-time-select,
html[data-traditional] .plan-time-datetime {
  background: white;
  border: 1px solid #7F9DB9;
  border-radius: 0 !important;
  color: #000;
  font-family: Tahoma, Geneva, sans-serif;
  font-size: 0.8rem;
  padding: 0.25rem 0.5rem;
}

html[data-traditional] .plan-time-select {
  padding-right: 1.6rem;
}

html[data-traditional] .plan-time-select:hover,
html[data-traditional] .plan-time-select:focus,
html[data-traditional] .plan-time-datetime:hover,
html[data-traditional] .plan-time-datetime:focus {
  border-color: #245EDC;
  color: #000;
}

html[data-traditional] .plan-time-chevron {
  color: #245EDC;
}

html.dark[data-traditional] .plan-time-filter {
  background: #1a2540;
  border-color: #3a4f7a;
}

html.dark[data-traditional] .plan-time-select,
html.dark[data-traditional] .plan-time-datetime {
  background: #0a1228;
  border-color: #3a4f7a;
  color: #e2e8f0;
  color-scheme: dark;
}

html.dark[data-traditional] .plan-time-chevron {
  color: #8aa9d4;
}

/* ============================================================
   Plan time-filter Search button
   ============================================================ */
.plan-time-search-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  background: #0ea5e9;
  color: white;
  border: 1.5px solid #0ea5e9;
  border-radius: 9999px;
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0.4rem 0.85rem;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, color 0.15s, transform 0.05s;
}

.plan-time-search-btn:hover {
  background: #0284c7;
  border-color: #0284c7;
}

.plan-time-search-btn:active {
  transform: scale(0.97);
}

.plan-time-search-btn:disabled {
  background: #cbd5e1;
  border-color: #cbd5e1;
  color: #64748b;
  cursor: not-allowed;
}

.dark .plan-time-search-btn {
  background: #0284c7;
  border-color: #0284c7;
}

.dark .plan-time-search-btn:hover {
  background: #0369a1;
  border-color: #0369a1;
}

.dark .plan-time-search-btn:disabled {
  background: #334155;
  border-color: #334155;
  color: #94a3b8;
}

/* Hungry */
html[data-hungry] .plan-time-search-btn {
  background: #D97706;
  border: 2px solid #B45309;
  border-radius: 0.5rem;
  color: #FFFBEB;
}

html[data-hungry] .plan-time-search-btn:hover {
  background: #B45309;
  border-color: #92400E;
}

html[data-hungry] .plan-time-search-btn:disabled {
  background: #FCD34D;
  border-color: #FCD34D;
  color: #92400E;
}

html.dark[data-hungry] .plan-time-search-btn {
  background: #d97706;
  border-color: #92400E;
  color: #fde68a;
}

html.dark[data-hungry] .plan-time-search-btn:hover {
  background: #b45309;
}

/* Traditional (Windows XP Luna) */
html[data-traditional] .plan-time-search-btn {
  background: linear-gradient(to bottom, #FFFFFF 0%, #ECE9D8 50%, #D7D2BC 100%);
  border: 1px solid #003C74;
  border-radius: 0 !important;
  color: #000;
  font-family: Tahoma, Geneva, sans-serif;
  font-size: 0.8rem;
  padding: 0.25rem 0.65rem;
}

html[data-traditional] .plan-time-search-btn:hover {
  background: linear-gradient(to bottom, #FFFFFF 0%, #FFE9A0 50%, #F5C75A 100%);
  border-color: #003C74;
  color: #000;
}

html[data-traditional] .plan-time-search-btn:disabled {
  background: #ECE9D8;
  color: #7F7F7F;
  border-color: #A0A0A0;
}

html.dark[data-traditional] .plan-time-search-btn {
  background: linear-gradient(to bottom, #2a3a5c 0%, #1a2540 50%, #0a1228 100%);
  border-color: #3a4f7a;
  color: #e2e8f0;
}

html.dark[data-traditional] .plan-time-search-btn:hover {
  background: linear-gradient(to bottom, #3a4f7a 0%, #2a3a5c 50%, #1a2540 100%);
}
</style>

<style>
@keyframes bus-run {
  0% { transform: translateX(-50px); opacity: 0; }
  20% { opacity: 1; }
  80% { opacity: 1; }
  100% { transform: translateX(50px); opacity: 0; }
}
</style>
