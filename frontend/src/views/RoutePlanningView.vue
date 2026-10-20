<script lang="ts">
export default {
  name: 'RoutePlanningView'
}
</script>

<script setup lang="ts">
import {cityDateTime} from '@/utils/cityClock.ts'
import {computed, nextTick, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {RouterLink, useRoute, useRouter, type LocationQueryRaw} from 'vue-router'
import HeaderNavigation from "@/components/HeaderNavigation.vue"
import {useI18n} from 'vue-i18n'
import {useMapStore, type HighlightedStop} from '@/stores/map.ts'
import {useUserStore} from '@/stores/user.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {usePlannerStore} from '@/stores/planner.ts'
import IconHeartFilled from "@/components/icons/IconHeartFilled.vue"
import IconHeartOutline from "@/components/icons/IconHeartOutline.vue"
import ShareButton from "@/components/ShareButton.vue"
import {useJourneyStore} from "@/stores/journey.ts"
import {apiRequest} from "@/utils/api.ts"
import type {RouteType, Shape} from "@/types/tranzy.ts"
import {storeToRefs} from "pinia"
import ViewErrorState from "@/components/ViewErrorState.vue"
import LoadingIndicator from "@/components/LoadingIndicator.vue"
import {useOnline} from "@/composables/useOnline.ts"
import {usePlaceSearch} from "@/composables/usePlaceSearch.ts"
import {useKbdEscape, useKbdShortcuts, useKeyboardNav} from "@/composables/useKeyboardNav.ts"

import {useLive} from "@/composables/useLive.ts"
import type {DisplayShape} from "@/stores/map.ts"
import type {DisplayVehicle} from "@/utils/mapIcons.ts"
import {formatMinutesFromNow} from "@/utils/time.ts"
import {decodePolyline} from "@/utils/geo.ts"
import {journeyFromOption, legTrips, planQuery, rideLegs, type PlanOption, type PlanResponse, type PlanRide} from "@/utils/plan.ts"
import {reversePlace, type Place} from "@/utils/places.ts"
import { VueDatePicker } from '@vuepic/vue-datepicker'
import '@vuepic/vue-datepicker/dist/main.css'
import {siteUrl} from '@/utils/site.ts'
import {cityCenter, cityParams, cityPath} from '@/utils/city.ts'

const {t} = useI18n()
const route = useRoute()
const router = useRouter()

const mapStore = useMapStore()
const userStore = useUserStore()
const settings = useSettingsStore()
const favoritesStore = useFavoritesStore()
const plannerStore = usePlannerStore()
const {isOnline} = useOnline()
const {userLocation, hasLocationPermission, isLocating, userTime} = storeToRefs(userStore)
const {timeMode, timeValue} = storeToRefs(plannerStore)

const customOrigin = ref<{ name: string, lat: number, lon: number } | null>(null)
const {activeSearchField, searchQuery, searchResults, isSearching, searchNow, openSearchField, closeSearchField} = usePlaceSearch(isOnline)

const vFocus = {
  mounted: (el: HTMLElement) => el.focus()
}

const destName = computed(() => (route.query.name as string | undefined) ?? '')
const destLat = computed(() => parseFloat((route.query.lat as string) ?? 'NaN'))
const destLon = computed(() => parseFloat((route.query.lon as string) ?? 'NaN'))

function shortPlaceName(name: string): string {
  const i = name.indexOf(',')
  return i > 0 ? name.slice(0, i).trim() : name
}

useHead(() => {
  if (!destName.value) {
    return {
      title: t('headPlanTitle'),
      meta: [
        {name: 'description', content: t('headPlanDesc', cityParams())},
        {property: 'og:title', content: t('headPlanTitle')},
        {property: 'og:description', content: t('headPlanDesc', cityParams())},
        {property: 'og:url', content: siteUrl(cityPath('/plan'))},
        {name: 'twitter:title', content: t('headPlanTitle')},
        {name: 'twitter:description', content: t('headPlanDesc', cityParams())},
      ],
      link: [{rel: 'canonical', href: siteUrl(cityPath('/plan'))}],
    }
  }

  const shortDest = shortPlaceName(destName.value)
  const originName = customOrigin.value?.name ?? ''
  const params: Record<string, string> = {...cityParams(), dest: shortDest}
  if (originName) params.origin = shortPlaceName(originName)
  const titleKey = originName ? 'headPlanFromToTitle' : 'headPlanToTitle'
  const descKey = originName ? 'headPlanFromToDesc' : 'headPlanToDesc'
  const title = t(titleKey, params)
  const desc = t(descKey, params)
  const url = siteUrl(route.fullPath)

  return {
    title,
    meta: [
      {name: 'description', content: desc},
      {property: 'og:title', content: title},
      {property: 'og:description', content: desc},
      {property: 'og:url', content: url},
      {name: 'twitter:title', content: title},
      {name: 'twitter:description', content: desc},
    ],
    link: [{rel: 'canonical', href: url}],
  }
})

const isFavorite = computed(() => favoritesStore.isPlanFavorite(
  destLat.value,
  destLon.value,
  customOrigin.value?.lat,
  customOrigin.value?.lon
))

const favoriteLabel = computed(() => favoritesStore.favoritePlans.find(p =>
  p.lat === destLat.value && p.lon === destLon.value &&
  p.originLat === customOrigin.value?.lat && p.originLon === customOrigin.value?.lon
)?.label)

const journeyStore = useJourneyStore()

function startJourney() {
  const plan = selectedPlan.value
  if (!plan || !planData.value || !rideLegs(plan).length || !hasValidCoords.value) return
  const last = rideLegs(plan).at(-1)!
  const journey = journeyFromOption(plan, planData.value, {
    id: -1,
    name: destName.value || stopName(last.to),
    lat: destLat.value,
    lon: destLon.value,
  }, route.fullPath)
  if (!journey) return
  const location = userLocation.value
  void journeyStore.start(journey, location ? {lat: location.latitude, lon: location.longitude, accuracy: 50, speed: null} : null)
  void router.push(cityPath('/nav'))
}

function toggleFavorite() {
  if (!isOnline.value) return
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

const isRenaming = ref(false)
const renameValue = ref('')

function startRename() {
  if (!isOnline.value) return
  renameValue.value = favoriteLabel.value ?? ''
  isRenaming.value = true
}

function confirmRename() {
  if (!isOnline.value) return
  favoritesStore.renamePlanFavorite(
    destLat.value,
    destLon.value,
    customOrigin.value?.lat,
    customOrigin.value?.lon,
    renameValue.value.trim() || undefined
  )
  isRenaming.value = false
}

function cancelRename() {
  isRenaming.value = false
}

const planData = ref<PlanResponse | null>(null)
const currentQueryKey = ref<string | null>(null)
const selectedPlanIndex = ref(0)
const selectedPlanKey = ref<string | null>(null)
const isCalculating = ref(false)
const expandedLegs = ref<Record<string, boolean>>({})

function toggleLegExpansion(id: string) {
  expandedLegs.value[id] = !expandedLegs.value[id]
}

const mapActivationKey = ref(0)
const isActive = ref(false)

const options = computed(() => planData.value?.options ?? [])
const selectedPlan = computed(() => options.value[selectedPlanIndex.value])
const selectedPlanSignature = computed(() => selectedPlan.value?.key ?? null)

function selectPlanAt(idx: number) {
  selectedPlanIndex.value = idx
  const p = options.value[idx]
  selectedPlanKey.value = p ? p.key : null
  mapStore.fitWalkingPolylines = true
  const newQuery = {...route.query}
  if (idx === 0) {
    delete newQuery.plan
  } else {
    newQuery.plan = String(idx)
  }
  void router.replace({query: newQuery})
}

const stopName = (id: number | undefined) => (id ? planData.value?.stops[String(id)]?.name : undefined) ?? ''
const line = (id: number | undefined) => id === undefined ? undefined : planData.value?.lines[String(id)]
const lineColor = (id: number | undefined) => line(id)?.color ?? '#64748b'

const now = computed(() => userTime.value || new Date())
const minutesFrom = (at: number) => (at * 1000 - now.value.getTime()) / 60_000
const clock = (at: number) => new Date(at * 1000).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit', hour12: false})
const firstRide = (plan: PlanOption): PlanRide | undefined => plan.runs[0]?.rides[0]
const laterRides = (plan: PlanOption) => plan.runs.slice(1).map((r) => r.rides[0]).filter((r): r is PlanRide => !!r)
const journeyMinutes = (plan: PlanOption) => {
  const run = plan.runs[0]
  return run ? (run.arrive - run.depart) / 60 : 0
}
const fromNow = (ride: PlanRide) => (ride.live ? '' : '~ ') + (timeMode.value === 'now' ? formatMinutesFromNow(minutesFrom(ride.depart), now.value, t('now')) : clock(ride.depart))

function departureLabel(ride: PlanRide) {
  const approx = ride.live ? '' : '~'
  if (timeMode.value !== 'now') return t('planAtTime', {time: approx + clock(ride.depart)})
  const minutes = minutesFrom(ride.depart)
  const rounded = Math.round(minutes)
  if (rounded <= 0) return t('now')
  const timePart = formatMinutesFromNow(minutes, now.value, '')
  return rounded < 60 ? t('planInTime', {time: approx + timePart}) : t('planAtTime', {time: approx + timePart})
}

function arrivalLabel(plan: PlanOption) {
  const run = plan.runs[0]
  if (!run) return ''
  return t('planArrivalAt', {time: (firstRide(plan)?.live ? '' : '~') + clock(run.arrive)})
}

const totalTripMinutes = computed(() => {
  const run = selectedPlan.value?.runs[0]
  return run ? Math.max(0, Math.round(minutesFrom(run.arrive))) : 0
})

const formatMinutes = (m: number): string => {
  const rounded = Math.round(m)
  if (rounded < 60) return `${rounded} min`
  const h = Math.floor(rounded / 60)
  const rem = rounded % 60
  return rem === 0 ? `${h} h` : `${h} h ${rem} min`
}

const selectedPlanLegsData = computed(() => {
  const plan = selectedPlan.value
  if (!plan) return []
  const out = []
  let ride = 0
  for (const [i, leg] of plan.legs.entries()) {
    if (leg.kind !== 'ride') continue
    const lines = leg.lines ?? []
    const walk = plan.legs[i + 1]
    out.push({
      leg,
      lines,
      trips: legTrips(plan, ride),
      color: lineColor(lines[0]),
      intermediates: (leg.passed ?? []).map((id) => ({stop_id: id, stop_name: stopName(id)})),
      transferMeters: walk?.kind === 'walk' && plan.legs[i + 2]?.kind === 'ride' ? walk.meters ?? 0 : 0,
    })
    ride++
  }
  return out
})

// The selected option on the map: its rides along the line's own shape, its walks, its stops.
watch([selectedPlanSignature, planData, mapActivationKey], () => {
  const plan = selectedPlan.value
  if (!plan || !isActive.value) {
    mapStore.setLoadedShapes([])
    mapStore.setHighlightedStops([])
    mapStore.clearWalkingPolylines()
    return
  }
  const rides = rideLegs(plan)
  const shapes: Array<[DisplayShape, Shape[]]> = []
  const highlighted: HighlightedStop[] = []
  rides.forEach((leg, i) => {
    const id = leg.lines?.[0]
    const tripId = (id !== undefined ? legTrips(plan, i).get(id) : undefined) ?? ''
    const l = line(id)
    shapes.push([
      {trip_id: tripId, route_short_name: l?.short_name ?? '', route_long_name: l?.long_name ?? '', route_color: lineColor(id), route_type: (l?.type ?? 3) as RouteType},
      decodePolyline(leg.path).map(([lat, lon], k) => ({shape_id: tripId, shape_pt_lat: lat, shape_pt_lon: lon, shape_pt_sequence: k, shape_dist_traveled: -1})),
    ])
    highlighted.push({stopId: String(leg.from), color: i === 0 ? 'green' : 'purple'})
    for (const stopId of leg.passed ?? []) highlighted.push({stopId: String(stopId), color: 'gray'})
    if (i === rides.length - 1) highlighted.push({stopId: String(leg.to), color: 'red'})
    else if (leg.to !== rides[i + 1]!.from) highlighted.push({stopId: String(leg.to), color: 'amber'})
  })
  mapStore.setLoadedShapes(shapes)
  mapStore.setHighlightedStops(highlighted)
  const walks = plan.legs.filter((l) => l.kind === 'walk').map((l) => decodePolyline(l.path))
  if (walks.length) mapStore.setWalkingPolylines(walks)
  else mapStore.clearWalkingPolylines()
}, {immediate: true})

// Only the buses the selected option catches, nothing else on the map.
const caughtTrips = computed(() => {
  if (!isActive.value || timeMode.value !== 'now') return []
  return [...new Set((selectedPlan.value?.runs[0]?.rides ?? []).filter((r) => r.vehicle_id).map((r) => r.trip_id))]
})
const {vehiclesByTrip} = useLive(ref<number[]>([]), caughtTrips)

watch([vehiclesByTrip, selectedPlanSignature, isActive], () => {
  const vehicles: DisplayVehicle[] = []
  for (const ride of (isActive.value ? selectedPlan.value?.runs[0]?.rides : undefined) ?? []) {
    const vehicle = ride.vehicle_id ? vehiclesByTrip.value.get(ride.trip_id)?.find((v) => v.id === ride.vehicle_id) : undefined
    const l = line(ride.route_id)
    if (vehicle && l) vehicles.push({...vehicle, route_short_name: l.short_name, route_color: l.color})
  }
  mapStore.setVehiclesToDisplay(vehicles)
}, {deep: true})

let refreshTimer: ReturnType<typeof setTimeout> | null = null
watch([planData, isActive], ([data, active]) => {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = null
  if (!data?.refresh_in || !active || timeMode.value !== 'now') return
  refreshTimer = setTimeout(() => void calculateRoutes(true), data.refresh_in * 1000)
})
onUnmounted(() => {
  if (refreshTimer) clearTimeout(refreshTimer)
})

const hasValidDest = computed(() => destName.value.length > 0)
const hasValidCoords = computed(() => !isNaN(destLat.value) && !isNaN(destLon.value))
const isGpsResolving = computed(() => isLocating.value && !userLocation.value && !customOrigin.value)

const originLabel = computed(() => {
  if (customOrigin.value) return customOrigin.value.name
  if (userStore.userLocation) return t('planOriginCurrentLocation')
  return t('planOriginUnknown')
})

function planOrigin(): {lat: number, lon: number} | null {
  if (customOrigin.value) return {lat: customOrigin.value.lat, lon: customOrigin.value.lon}
  const location = hasLocationPermission.value ? userLocation.value : null
  return location ? {lat: location.latitude, lon: location.longitude} : null
}

function getQueryKey() {
  const origin = planOrigin()
  if (!origin || isNaN(destLat.value) || isNaN(destLon.value)) return null
  return planQuery(origin, {lat: destLat.value, lon: destLon.value}, timeMode.value, timeValue.value)
}

function clearPlanSelection() {
  if (route.query.plan === undefined) return
  const newQuery = {...route.query}
  delete newQuery.plan
  void router.replace({query: newQuery})
}

const {pinnedLocationDragged, customOriginLocationDragged} = storeToRefs(mapStore)

async function reverseGeocode(lat: number, lon: number): Promise<string> {
  try {
    const result = await reversePlace(lat, lon)
    return result?.label ?? `${lat.toFixed(4)}, ${lon.toFixed(4)}`
  } catch { /* ignore */ }
  return `${lat.toFixed(4)}, ${lon.toFixed(4)}`
}

function clampCoord4(v: number): number {
  return Number(v.toFixed(4))
}

function coordQueryString(v: number): string {
  return clampCoord4(v).toFixed(4)
}

function singleQueryString(value: unknown): string | undefined {
  if (typeof value === 'string') return value
  if (Array.isArray(value) && typeof value[0] === 'string') return value[0]
  return undefined
}

let destDragGen = 0
watch(pinnedLocationDragged, async (dragged) => {
  if (!dragged) return
  const gen = ++destDragGen
  const {lat, lng} = dragged
  mapStore.clearPinnedLocationDragged()
  const name = await reverseGeocode(lat, lng)
  if (gen !== destDragGen) return

  const newQuery: LocationQueryRaw = {
    ...route.query,
    lat: coordQueryString(lat),
    lon: coordQueryString(lng),
    name
  }
  delete newQuery.plan
  await router.replace({query: newQuery})
  closeSearchField()
})

let originDragGen = 0
watch(customOriginLocationDragged, async (dragged) => {
  if (!dragged) return
  const gen = ++originDragGen
  const {lat, lng} = dragged
  mapStore.clearCustomOriginLocationDragged()
  const name = await reverseGeocode(lat, lng)
  if (gen !== originDragGen) return

  customOrigin.value = {name, lat: clampCoord4(lat), lon: clampCoord4(lng)}
  clearPlanSelection()
  closeSearchField()
  void calculateRoutes()
})

function selectOrigin(res: Place) {
  customOrigin.value = {
    name: res.label,
    lat: parseFloat(res.lat),
    lon: parseFloat(res.lon)
  }
  clearPlanSelection()
  closeSearchField()
  void calculateRoutes()
}

function selectDestination(res: Place) {
  if (!isOnline.value) return
  const newQuery = {
    ...route.query,
    lat: res.lat,
    lon: res.lon,
    name: res.label
  }
  closeSearchField()
  void router.replace({query: newQuery})
  void calculateRoutes()
}

function useCurrentLocation() {
  customOrigin.value = null
  clearPlanSelection()
  closeSearchField()
  void calculateRoutes()
}

function closeActiveSearch() {
  closeSearchField()
  if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
}

function dropPinAsDestination() {
  if (!isOnline.value) return
  const label = t('droppedPin')
  const newQuery = { ...route.query, lat: String(cityCenter().lat), lon: String(cityCenter().lon), name: label }
  closeSearchField()
  void router.replace({ query: newQuery })
  void calculateRoutes()
}

function useCurrentLocationAsDestination() {
  if (!isOnline.value) return
  if (!userLocation.value) return
  const newQuery = {
    ...route.query,
    lat: userLocation.value.latitude.toString(),
    lon: userLocation.value.longitude.toString(),
    name: t('planOriginCurrentLocation')
  }
  closeSearchField()
  void router.replace({query: newQuery})
}

watch(customOrigin, (val) => {
  const newQuery = {...route.query}
  let changed = false
  if (val) {
    const originLat = coordQueryString(val.lat)
    const originLon = coordQueryString(val.lon)
    if (newQuery.originLat !== originLat || newQuery.originLon !== originLon || newQuery.originName !== val.name) {
      newQuery.originLat = originLat
      newQuery.originLon = originLon
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

  const queryTime = singleQueryString(newQuery.time)
  const queryArriveBy = singleQueryString(newQuery.arrive_by) === 'true'
  const queryLast = singleQueryString(newQuery.last) === 'true'
  if (queryLast) {
    if (timeMode.value !== 'last') timeMode.value = 'last'
  } else if (queryTime) {
    const nextMode = queryArriveBy ? 'arrive' : 'leave'
    if (timeValue.value !== queryTime) {
      timeValue.value = queryTime
    }
    if (timeMode.value !== nextMode) {
      timeMode.value = nextMode
    }
  } else if (timeMode.value !== 'now') {
    timeMode.value = 'now'
  }
}, {immediate: true})

watch([timeMode, timeValue], ([mode, value]) => {
  const newQuery = {...route.query}
  let changed = false

  if (mode === 'now') {
    if (newQuery.time !== undefined || newQuery.arrive_by !== undefined || newQuery.last !== undefined) {
      delete newQuery.time
      delete newQuery.arrive_by
      delete newQuery.last
      changed = true
    }
  } else if (mode === 'last') {
    if (newQuery.last !== 'true' || newQuery.time !== undefined || newQuery.arrive_by !== undefined) {
      delete newQuery.time
      delete newQuery.arrive_by
      newQuery.last = 'true'
      changed = true
    }
  } else if (value) {
    const currentTime = singleQueryString(newQuery.time)
    const currentArriveBy = singleQueryString(newQuery.arrive_by)
    const nextArriveBy = mode === 'arrive' ? 'true' : 'false'
    if (currentTime !== value) {
      newQuery.time = value
      changed = true
    }
    if (currentArriveBy !== nextArriveBy) {
      newQuery.arrive_by = nextArriveBy
      changed = true
    }
    if (newQuery.last !== undefined) {
      delete newQuery.last
      changed = true
    }
  }

  if (changed) {
    void router.replace({query: newQuery})
  }
})

onMounted(async () => {
  isActive.value = true
  if (hasValidCoords.value) {
    mapStore.setPinnedLocation(destLat.value, destLon.value, destName.value)
    if (!hasLocationPermission.value && !customOrigin.value) {
      activeSearchField.value = 'origin'
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

watch(options, (opts) => {
  if (!opts.length) {
    selectedPlanIndex.value = 0
    selectedPlanKey.value = null
    return
  }
  const kept = selectedPlanKey.value ? opts.findIndex((o) => o.key === selectedPlanKey.value) : -1
  const urlPlan = parseInt(route.query.plan as string)
  const targetIdx = kept >= 0 ? kept : !isNaN(urlPlan) && urlPlan >= 0 && urlPlan < opts.length ? urlPlan : 0
  selectedPlanIndex.value = targetIdx
  selectedPlanKey.value = opts[targetIdx]?.key ?? null
})

// A silent call only refreshes the timings of what is on screen.
async function calculateRoutes(silent = false) {
  if (isCalculating.value) return
  const origin = planOrigin()
  const qk = getQueryKey()
  if (!origin || !qk) return
  if (!silent && qk === currentQueryKey.value && planData.value?.options.length) return

  if (!silent) {
    planData.value = null
    currentQueryKey.value = null
    mapStore.clearWalkingPolylines()
    isCalculating.value = true
  }
  try {
    const data = await apiRequest<PlanResponse>(qk)
    planData.value = data
    currentQueryKey.value = qk

    if (!silent && data.options.length) {
      mapStore.fitWalkingPolylines = true
      if (isOnline.value) {
        favoritesStore.addRecentPlan({
          name: destName.value || t('planTitleGeneric'),
          lat: destLat.value,
          lon: destLon.value,
          originName: customOrigin.value?.name,
          originLat: customOrigin.value?.lat,
          originLon: customOrigin.value?.lon
        })
      }
    }
  } finally {
    if (!silent) isCalculating.value = false
  }
}

watch([destLat, destLon], async ([newLat, newLon], [oldLat, oldLon] = [NaN, NaN]) => {
  if (newLat === oldLat && newLon === oldLon) return
  if (!isNaN(oldLat!)) clearPlanSelection()
  planData.value = null
  currentQueryKey.value = null
  mapStore.clearWalkingPolylines()
  await calculateRoutes()
}, {immediate: true})

watch(customOrigin, async (newCO, oldCO) => {
  if (newCO?.lat !== oldCO?.lat || newCO?.lon !== oldCO?.lon) {
    clearPlanSelection()
    await calculateRoutes()
  }
})

watch(userLocation, async (newLoc, oldLoc) => {
  if (!newLoc || oldLoc || customOrigin.value) return
  if (activeSearchField.value === 'origin') closeActiveSearch()
  await calculateRoutes()
})

function isCurrentLocationName(name: string) {
  return name === t('planOriginCurrentLocation')
}

async function swapOriginDestination() {
  if (isNaN(destLat.value) || isNaN(destLon.value)) return
  const newDestLat = customOrigin.value?.lat ?? userLocation.value?.latitude
  const newDestLon = customOrigin.value?.lon ?? userLocation.value?.longitude
  if (newDestLat === undefined || newDestLon === undefined) return

  const newDestName = customOrigin.value?.name ?? await reverseGeocode(newDestLat, newDestLon)
  const newOriginName = isCurrentLocationName(destName.value)
    ? await reverseGeocode(destLat.value, destLon.value)
    : destName.value

  const newQuery: LocationQueryRaw = {
    ...route.query,
    lat: String(newDestLat),
    lon: String(newDestLon),
    name: newDestName,
    originLat: String(destLat.value),
    originLon: String(destLon.value),
    originName: newOriginName,
  }
  delete newQuery.plan
  void router.replace({query: newQuery})
}

watch(() => route.query.q, (q) => {
  if (typeof q !== 'string' || !q.trim() || !isOnline.value) return
  activeSearchField.value = 'destination'
  searchQuery.value = q
}, {immediate: true})

async function refreshRoutes() {
  if (isCalculating.value) return
  clearPlanSelection()
  selectedPlanIndex.value = 0
  selectedPlanKey.value = null
  planData.value = null
  currentQueryKey.value = null
  mapStore.clearWalkingPolylines()
  await calculateRoutes()
}

const kbd = useKeyboardNav()
let calcNavSeq = 0

watch(isCalculating, (calculating) => {
  if (calculating) {
    calcNavSeq = kbd.navSeq()
    return
  }
  if (isActive.value && kbd.navSeq() === calcNavSeq) kbd.focusSectionSoon('results')
})

async function onSearchEnter(e: KeyboardEvent) {
  const input = e.target
  const query = searchQuery.value
  await searchNow()
  if (!kbd.keyboardMode.value || !activeSearchField.value || query.trim().length < 3) return
  if (searchQuery.value !== query || document.activeElement !== input) return
  await nextTick()
  kbd.focusSection(activeSearchField.value === 'origin' ? 'from-results' : 'to-results')
}

watch(activeSearchField, (field, prev) => {
  if (field || !prev || !kbd.keyboardMode.value) return
  void nextTick(() => kbd.focusKey(prev === 'origin' ? 'from' : 'to'))
})

watch(isRenaming, (renaming) => {
  if (!renaming && kbd.keyboardMode.value) void nextTick(() => kbd.focusKey('rename') || kbd.focusKey('fav'))
})

function onResultEnter(e: KeyboardEvent, index: number) {
  if (e.repeat) return
  if (selectedPlanIndex.value === index) kbd.focusSection('legs')
  else selectPlanAt(index)
}

const canRecalculate = computed(() =>
  hasValidCoords.value && !isCalculating.value && (hasLocationPermission.value || !!customOrigin.value)
)

useKbdEscape(() => isRenaming.value, cancelRename)
useKbdEscape(() => activeSearchField.value !== null, closeSearchField)

useKbdShortcuts({
  f: () => {
    if (hasValidCoords.value) toggleFavorite()
  },
  r: () => {
    if (canRecalculate.value) void refreshRoutes()
  },
  v: () => {
    if (canRecalculate.value) void swapOriginDestination()
  },
})

// Changing the time doesn't recalculate; Search does.
const minDateTime = computed(() => cityDateTime(userTime.value || new Date()))
const minDateObj = computed(() => userTime.value || new Date())
const isDarkTheme = computed(() => settings.isDark)
watch(timeMode, (mode) => {
  if (mode !== 'now' && mode !== 'last' && !timeValue.value) {
    timeValue.value = cityDateTime(new Date())
  }
  if (mode === 'now' || mode === 'last') {
    void refreshRoutes()
  }
})
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
    <header class="flex items-center gap-3" data-kbd-section="actions" data-kbd-axis="x">
      <div
        class="w-12 h-12 shrink-0 rounded-2xl bg-linear-to-br from-sky-400 to-sky-600 flex items-center justify-center shadow-lg shadow-sky-500/20">
        <span v-if="settings.legacyBlueActive" class="emoji-icon-xl" aria-hidden="true">🗺️</span>
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
        <div v-if="isRenaming" class="flex items-center gap-1.5 mt-0.5">
          <input
            v-model="renameValue"
            class="rename-input"
            :placeholder="t('renameFavoritePlaceholder')"
            data-kbd-item="rename-input"
            @keydown.enter.prevent="confirmRename"
            v-focus
          />
          <button class="rename-confirm-btn" type="button" data-kbd-item="rename-confirm" :aria-label="t('renameFavorite')" @click="confirmRename">
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </button>
          <button class="rename-cancel-btn" type="button" data-kbd-item="rename-cancel" aria-label="Cancel" @click="cancelRename">
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>
        <h1
          v-else
          class="text-lg font-bold text-slate-900 dark:text-white leading-snug truncate"
          :title="favoriteLabel ?? (hasValidDest ? destName : t('planTitleGeneric'))">
          {{ favoriteLabel ?? (hasValidDest ? destName : t('planTitleGeneric')) }}
        </h1>
      </div>
      <button
        v-if="selectedPlan && rideLegs(selectedPlan).length && !isCalculating && hasValidCoords && !isRenaming"
        type="button"
        class="start-btn mt-1 shrink-0"
        :title="t('journeyStart')"
        :aria-label="t('journeyStart')"
        data-kbd-item="start"
        @click="startJourney"
      >
        <span v-if="settings.legacyBlueActive" class="emoji-icon" aria-hidden="true">▶️</span>
        <template v-else-if="settings.paperActive">{{ t('journeyStart') }}</template>
        <svg v-else class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2L4.5 20.3l.7.7L12 18l6.8 3 .7-.7z"/></svg>
      </button>
      <button
        v-if="isOnline && hasValidCoords && isFavorite && !isRenaming"
        type="button"
        class="rename-btn mt-1 shrink-0"
        :title="t('renameFavorite')"
        :aria-label="t('renameFavorite')"
        data-kbd-item="rename"
        @click="startRename"
      >
        <span v-if="settings.legacyBlueActive" class="emoji-icon" aria-hidden="true">✏️</span>
        <template v-else-if="settings.paperActive">{{ t('paperRename') }}</template>
        <svg v-else class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10"/>
        </svg>
      </button>
      <ShareButton v-if="hasValidCoords && !isRenaming" class="mt-1"/>
      <button
        v-if="hasValidCoords"
        type="button"
        class="fav-btn mt-1 shrink-0"
        :class="{ 'is-fav': isFavorite }"
        :title="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-label="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-pressed="isFavorite"
        :disabled="!isOnline"
        data-kbd-item="fav"
        @click="toggleFavorite"
      >
        <template v-if="settings.paperActive">{{ isFavorite ? '♥' : '♡' }} {{ t('paperFavoriteStamp') }}</template>
        <IconHeartFilled v-else-if="isFavorite" class="w-5 h-5"/>
        <IconHeartOutline v-else class="w-5 h-5"/>
      </button>
    </header>

    <div>
      <section v-if="selectedPlan && !isCalculating" class="trip-summary">
        <div class="trip-summary-stat" v-if="timeMode === 'now'">
          <span class="trip-summary-stat-value">{{ formatMinutes(totalTripMinutes) }}</span>
          <span class="trip-summary-stat-label">{{ t('planTripTotal') }}</span>
        </div>
        <div v-if="firstRide(selectedPlan)" class="trip-summary-stat">
          <span class="trip-summary-stat-value" :class="firstRide(selectedPlan)!.live ? 'is-live' : ''">
            <span v-if="firstRide(selectedPlan)!.live" class="live-dot"></span>{{ fromNow(firstRide(selectedPlan)!) }}
          </span>
          <span class="trip-summary-stat-label">{{ t(timeMode === 'last' ? 'planLastDeparture' : 'planNextDepartures') }}</span>
        </div>
        <div v-if="selectedPlan.transfers > 0" class="trip-summary-stat">
          <span class="trip-summary-stat-value">{{ selectedPlan.transfers }}</span>
          <span class="trip-summary-stat-label">{{ selectedPlan.transfers === 1 ? t('planChange') : t('planChanges') }}</span>
        </div>
      </section>
      <!-- From/To card — always shown -->
      <section class="route-legs-card">
        <div class="leg-row" data-kbd-section="from" data-kbd-entry="2">
          <div class="leg-icon-col">
            <div class="leg-dot leg-dot-origin">
              <span v-if="settings.legacyBlueActive" class="text-sm">📍</span>
              <svg v-else class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
                <circle cx="12" cy="12" r="6"/>
              </svg>
            </div>
            <div class="leg-line"></div>
          </div>
          <div class="leg-label-col" v-if="activeSearchField !== 'origin'">
            <div class="origin-clickable" data-kbd-item="from" @click="openSearchField('origin')">
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
                data-kbd-item="from-input"
                @keydown.enter="onSearchEnter"
                v-focus
              />
              <button class="search-cancel" @click="activeSearchField = null">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <path d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="search-results" v-if="searchResults.length > 0 || isSearching || hasLocationPermission"
                 data-kbd-section="from-results">
              <div v-if="isSearching" class="search-loading">
                <div class="mini-spinner"></div>
                {{ t('planSearching') }}
              </div>
              <template v-else>
                <div
                  v-if="hasLocationPermission"
                  class="search-result-item current-loc-option"
                  data-kbd-item="from-current"
                  @click="useCurrentLocation"
                >
                  <span class="res-main">{{ t('planOriginCurrentLocation') }}</span>
                </div>
                <div
                  v-for="res in searchResults"
                  :key="res.id"
                  class="search-result-item"
                  :data-kbd-item="`from-res-${res.id}`"
                  @click="selectOrigin(res)"
                >
                  <span class="res-main">{{ res.label }}</span>
                </div>
              </template>
            </div>
          </div>
        </div>

        <template v-if="selectedPlan">
          <div v-if="!rideLegs(selectedPlan).length" class="leg-row">
            <div class="leg-icon-col">
              <div class="leg-dot intermediate-dot">
                <div class="w-1 h-1 rounded-full bg-slate-300 dark:bg-slate-600"></div>
              </div>
              <div class="leg-line leg-line-dashed"></div>
            </div>
            <div class="leg-label-col">
              <span class="leg-type-badge">{{ t('planWalk') }}</span>
              <span class="leg-name">{{ formatMinutes(journeyMinutes(selectedPlan)) }}</span>
            </div>
          </div>
          <div v-for="(ld, legIdx) in selectedPlanLegsData" :key="legIdx" data-kbd-section="legs">
            <div class="leg-row">
              <div class="leg-icon-col">
                <div class="leg-dot boarding-dot" :style="{ borderColor: ld.color }">
                  <div class="dot-inner" :style="{ backgroundColor: ld.color }"></div>
                </div>
                <div class="leg-line" :style="{ backgroundColor: ld.color, backgroundImage: 'none' }"></div>
              </div>
              <div class="leg-label-col">
                <span class="leg-type-badge" :style="{ color: ld.color }">
                  {{ t('planBoarding') }}
                  <template v-for="(routeId, rIdx) in (expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? ld.lines : ld.lines.slice(0, 4))" :key="routeId">
                    <RouterLink
                      class="leg-route-link"
                      :data-kbd-item="`leg-${legIdx}-route-${routeId}`"
                      :to="{ name: 'route', params: { routeId: routeId, direction: ld.trips.get(routeId)?.endsWith('_1') ? '1' : '0' } }"
                      @click.stop
                    >{{ line(routeId)?.short_name }}</RouterLink><span v-if="rIdx < (expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? ld.lines.length : Math.min(ld.lines.length, 4)) - 1" class="leg-route-sep"> / </span>
                  </template><span v-if="ld.lines.length > 4" class="leg-route-overflow cursor-pointer" :data-kbd-item="`leg-${legIdx}-more`" @click.stop="toggleLegExpansion('detail-' + selectedPlan.key + '-' + legIdx)"> {{ expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? '«' : '+' + (ld.lines.length - 4) }}</span>
                  <span v-if="Math.max(0, Math.round(ld.leg.seconds / 60)) > 0" class="leg-ride-time">
                    · {{ Math.max(0, Math.round(ld.leg.seconds / 60)) }}&nbsp;min&nbsp;{{ t('planRideTime') }}
                  </span>
                </span>
                <RouterLink class="leg-name leg-name-link" :data-kbd-item="`leg-${legIdx}-start`" :to="{ name: 'stop', params: { stopId: ld.leg.from } }">{{ stopName(ld.leg.from) }}</RouterLink>
              </div>
            </div>

            <div v-for="stop in ld.intermediates" :key="stop.stop_id" class="leg-row intermediate-leg">
              <div class="leg-icon-col">
                <div class="leg-dot intermediate-dot">
                  <div class="w-1 h-1 rounded-full bg-slate-300 dark:bg-slate-600"></div>
                </div>
                <div class="leg-line" :style="{ backgroundColor: ld.color, backgroundImage: 'none' }"></div>
              </div>
              <div class="leg-label-col">
                <RouterLink class="leg-name intermediate-name leg-name-link" :data-kbd-item="`leg-${legIdx}-mid-${stop.stop_id}`" :to="{ name: 'stop', params: { stopId: stop.stop_id } }">{{ stop.stop_name }}</RouterLink>
              </div>
            </div>

            <div class="leg-row" :class="{ 'leg-row-alight-transfer': legIdx < selectedPlanLegsData.length - 1 }">
              <div class="leg-icon-col">
                <div class="leg-dot boarding-dot" :style="{ borderColor: ld.color }">
                  <div class="dot-inner" :style="{ backgroundColor: ld.color }"></div>
                </div>
                <div v-if="legIdx === selectedPlanLegsData.length - 1" class="leg-line leg-line-dashed"></div>
              </div>
              <div class="leg-label-col">
                <span class="leg-type-badge" :style="{ color: ld.color }">{{ t('planAlighting') }}</span>
                <RouterLink class="leg-name leg-name-link" :data-kbd-item="`leg-${legIdx}-end`" :to="{ name: 'stop', params: { stopId: ld.leg.to } }">{{ stopName(ld.leg.to) }}</RouterLink>
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
                  <template v-if="ld.transferMeters > 0">
                    {{ Math.round(ld.transferMeters) }}&nbsp;m {{ t('planTransferWalk') }}
                  </template>
                  <template v-else>
                    {{ t('planTransferSameStop') }}
                  </template>
                </span>
              </div>
            </div>
          </div>
        </template>

        <div class="leg-row" data-kbd-section="to" :data-kbd-entry="hasValidDest ? undefined : 1">
          <div class="leg-icon-col">
            <div class="leg-dot leg-dot-dest">
              <span v-if="settings.legacyBlueActive" class="text-sm">🏁</span>
              <svg v-else class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
              </svg>
            </div>
          </div>
          <div class="leg-label-col" v-if="activeSearchField !== 'destination'">
            <div
              class="origin-clickable"
              :class="{ 'opacity-60 pointer-events-none': !isOnline }"
              data-kbd-item="to"
              @click="openSearchField('destination')"
            >
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
                :disabled="!isOnline"
                data-kbd-item="to-input"
                @keydown.enter="onSearchEnter"
                v-focus
              />
              <button class="search-cancel" @click="activeSearchField = null">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <path d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="search-results" v-if="isOnline && (searchResults.length > 0 || isSearching || (hasLocationPermission && userLocation) || (!isSearching && searchQuery.trim().length >= 3))"
                 data-kbd-section="to-results">
              <div v-if="isSearching" class="search-loading">
                <div class="mini-spinner"></div>
                {{ t('planSearching') }}
              </div>
              <template v-else>
                <div
                  v-if="hasLocationPermission && userLocation"
                  class="search-result-item current-loc-option"
                  data-kbd-item="to-current"
                  @click="useCurrentLocationAsDestination"
                >
                  <span class="res-main">{{ t('planOriginCurrentLocation') }}</span>
                </div>
                <div
                  v-for="res in searchResults"
                  :key="res.id"
                  class="search-result-item"
                  :data-kbd-item="`to-res-${res.id}`"
                  @click="selectDestination(res)"
                >
                  <span class="res-main">{{ res.label }}</span>
                </div>
                <div
                  v-if="!searchResults.length && searchQuery.trim().length >= 3"
                  class="search-result-item drop-pin-option"
                  data-kbd-item="to-drop-pin"
                  @click="dropPinAsDestination"
                >
                  <svg class="w-4 h-4 shrink-0 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 21c0 0-7-6.686-7-11a7 7 0 1 1 14 0c0 4.314-7 11-7 11z"/>
                    <circle cx="12" cy="10" r="2.5" fill="currentColor" stroke="none"/>
                  </svg>
                  <span class="res-main">{{ t('dropPin') }}</span>
                </div>
              </template>
            </div>
          </div>
        </div>
      </section>
      <!-- Routes section — always shown -->
      <section class="flex flex-col gap-3 pb-8">
        <div class="plan-section-head">
          <h2 class="section-label">
            <span class="w-2 h-2 rounded-full bg-sky-500 shrink-0"></span>
            {{ t('planRoutesLabel') }}
          </h2>
        </div>

        <div class="plan-time-filter" role="group" :aria-label="t('planTimeFilterLabel')"
             data-kbd-section="time" data-kbd-axis="x">
          <label class="plan-time-mode">
            <select
              v-model="timeMode"
              class="plan-time-select"
              data-kbd-item="time-mode"
              :aria-label="t('planTimeFilterLabel')"
            >
              <option value="now">{{ t('planTimeLeaveNow') }}</option>
              <option value="leave">{{ t('planTimeLeaveAt') }}</option>
              <option value="arrive">{{ t('planTimeArriveBy') }}</option>
              <option value="last">{{ t('planTimeLast') }}</option>
            </select>
            <svg class="plan-time-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6"/>
            </svg>
          </label>
          <div class="plan-time-filter-actions">
            <button
              v-if="timeMode !== 'now' && timeMode !== 'last'"
              type="button"
              class="plan-time-search-btn shrink-0"
              data-kbd-item="time-search"
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
              data-kbd-item="swap"
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
              data-kbd-item="refresh"
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
          <VueDatePicker
            v-if="timeMode !== 'now' && timeMode !== 'last'"
            v-model="timeValue"
            :min-date="minDateObj"
            model-type="yyyy-MM-dd'T'HH:mm"
            format="dd/MM/yyyy HH:mm"
            :is24="true"
            :enable-seconds="false"
            :minutes-increment="1"
            :week-start="1"
            :auto-apply="false"
            :teleport="true"
            :floating="{ arrow: false, strategy: 'fixed', offset: 0, placement: 'bottom-start' }"
            :ui="{ menu: 'plan-time-menu' }"
            :input-attrs="{ clearable: true, alwaysClearable: true, inputmode: 'none' }"
            :dark="isDarkTheme"
            text-input
            class="plan-time-datetime"
            data-kbd-item="time-date"
            data-kbd-inner
            :aria-label="timeMode === 'arrive' ? t('planTimeArriveBy') : t('planTimeLeaveAt')"
          />
        </div>

        <!-- GPS resolving: waiting for device location -->
        <LoadingIndicator v-if="isGpsResolving" :text="t('planLocating')"/>

        <!-- Route calculation in progress -->
        <LoadingIndicator v-else-if="isCalculating" :text="t('planCalculating')"/>

        <!-- Results -->
        <div v-else-if="options.length > 0" class="route-results-list flex flex-col gap-2.5"
             data-kbd-section="results" data-kbd-entry="1">
          <div
            v-for="(plan, index) in options"
            :key="plan.key"
            class="departure-card group"
            :class="{ 'is-selected': selectedPlanIndex === index }"
            :data-kbd-item="`plan-${plan.key}`"
            :data-kbd-active="selectedPlanIndex === index"
            @click="selectPlanAt(index)"
            @keydown.enter.prevent="onResultEnter($event, index)"
          >
            <div class="card-rail" :class="{ 'is-active': selectedPlanIndex === index }"></div>

            <div class="card-body">
              <div class="card-row-primary">
                <div class="bus-chain">
                  <span v-if="!rideLegs(plan).length" class="bus-chip">
                    <span class="bus-chip-name">{{ t('planWalk') }}</span>
                  </span>
                  <template v-for="(leg, lIdx) in rideLegs(plan)" :key="lIdx">
                    <span
                      class="bus-chip"
                      :class="{ 'is-expanded': expandedLegs['list-' + plan.key + '-' + lIdx] }"
                      :style="{ backgroundColor: lineColor(leg.lines?.[0]) }"
                      :title="(leg.lines ?? []).map(id => line(id)?.short_name).join(' / ')"
                    ><template v-for="(routeId, rIdx) in (expandedLegs['list-' + plan.key + '-' + lIdx] ? leg.lines ?? [] : (leg.lines ?? []).slice(0, 3))" :key="rIdx"><span class="bus-chip-name">{{ line(routeId)?.short_name }}</span><span v-if="rIdx < (expandedLegs['list-' + plan.key + '-' + lIdx] ? (leg.lines ?? []).length : Math.min((leg.lines ?? []).length, 3)) - 1" class="bus-chip-sep">/</span></template></span><span v-if="(leg.lines ?? []).length > 3" class="bus-chip-overflow cursor-pointer" @click.stop="toggleLegExpansion('list-' + plan.key + '-' + lIdx)">{{ expandedLegs['list-' + plan.key + '-' + lIdx] ? '«' : '+' + ((leg.lines ?? []).length - 3) }}</span>
                    <svg v-if="lIdx < rideLegs(plan).length - 1" class="bus-chain-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M13 6l6 6-6 6"/>
                    </svg>
                  </template>
                </div>

                <div v-if="plan.runs[0]"
                  class="card-primary-time"
                  :class="firstRide(plan)?.live ? 'card-primary-time-live' : 'card-primary-time-sched'">
                  <div v-if="firstRide(plan)" class="card-relative-time">
                    <span v-if="firstRide(plan)!.live" class="live-dot"></span>
                    {{ departureLabel(firstRide(plan)!) }}
                  </div>
                  <div class="card-arrival-time">
                    {{ arrivalLabel(plan) }}
                  </div>
                </div>

                <svg class="card-chevron" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
                </svg>
              </div>

              <div class="card-row-meta">
                <span class="card-arrow">→</span>
                <span class="card-dest" :title="rideLegs(plan).length ? stopName(rideLegs(plan).at(-1)?.to) : destName">
                  {{ rideLegs(plan).length ? stopName(rideLegs(plan).at(-1)?.to) : destName }}
                </span>
              </div>

              <div class="card-row-stats">
                <span class="stat-chip stat-chip-duration">
                  <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
                  </svg>
                  {{ formatMinutes(Math.round(journeyMinutes(plan))) }}
                </span>
                <span v-if="plan.transfers > 0" class="stat-chip stat-chip-transfer">
                  <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M7 4v12m-3-3l3 3 3-3m7 9V4m-3 3l3-3 3 3"/>
                  </svg>
                  {{ plan.transfers }}&nbsp;{{ plan.transfers === 1 ? t('planChange') : t('planChanges') }}
                </span>
                <span v-if="firstRide(plan)?.live" class="stat-chip stat-chip-live">
                  <span class="live-dot"></span>{{ t('live') }}
                </span>
                <span v-if="laterRides(plan).length" class="stat-chip-next">
                  <span class="stat-chip-label">{{ t(timeMode === 'last' ? 'planEarlier' : 'planThen') }}</span>
                  <template v-for="(ride, i) in laterRides(plan)" :key="i"><span class="stat-chip-time">{{ fromNow(ride) }}</span></template>
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- No results -->
        <div v-else class="plan-placeholder">
            <svg class="w-10 h-10 text-slate-300 dark:text-slate-700 mb-3" viewBox="0 0 24 24"
                 fill="none" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round"
                    d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 00-10.026 0 1.106 1.106 0 00-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12"/>
            </svg>
            <p class="text-sm font-medium text-slate-400 dark:text-slate-500 text-center">
              {{ t('planNoResults') }}</p>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped src="./RoutePlanningView.css"></style>
