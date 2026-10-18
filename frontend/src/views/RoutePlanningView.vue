<script lang="ts">
export default {
  name: 'RoutePlanningView'
}
</script>

<script setup lang="ts">
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
import {useKbdEscape, useKbdShortcuts, useKeyboardNav} from "@/composables/useKeyboardNav.ts"

import {useLive} from "@/composables/useLive.ts"
import type {DisplayShape} from "@/stores/map.ts"
import type {DisplayVehicle} from "@/utils/mapIcons.ts"
import {formatMinutesFromNow} from "@/utils/time.ts"
import {decodePolyline} from "@/utils/geo.ts"
import {journeyFromOption, legTrips, planQuery, rideLegs, type PlanOption, type PlanResponse, type PlanRide} from "@/utils/plan.ts"
import {reversePlace, searchPlaces, type Place} from "@/utils/places.ts"
import { VueDatePicker } from '@vuepic/vue-datepicker'
import '@vuepic/vue-datepicker/dist/main.css'
import {siteUrl} from '@/utils/site.ts'
import {cityInfo, cityPath} from '@/utils/city.ts'
import {cityParams} from '@/utils/city.ts'

const {t, locale} = useI18n()
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
const activeSearchField = ref<'origin' | 'destination' | null>(null)
const searchQuery = ref('')
const searchResults = ref<Place[]>([])
let searchRequest: AbortController | null = null
const isSearching = ref(false)
let searchTimeout: ReturnType<typeof setTimeout> | null = null

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
        {property: 'og:url', content: siteUrl('/plan')},
        {name: 'twitter:title', content: t('headPlanTitle')},
        {name: 'twitter:description', content: t('headPlanDesc', cityParams())},
      ],
      link: [{rel: 'canonical', href: siteUrl('/plan')}],
    }
  }

  const shortDest = shortPlaceName(destName.value)
  const originName = customOrigin.value?.name ?? ''
  const params: Record<string, string> = originName
    ? {origin: shortPlaceName(originName), dest: shortDest}
    : {dest: shortDest}
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
  const stored = favoritesStore.favoritePlans.find(p =>
    p.lat === destLat.value && p.lon === destLon.value &&
    p.originLat === customOrigin.value?.lat && p.originLon === customOrigin.value?.lon
  )
  renameValue.value = stored?.label ?? ''
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
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
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
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  void calculateRoutes()
})

async function performSearch() {
  if (!isOnline.value && activeSearchField.value === 'destination') {
    searchResults.value = []
    isSearching.value = false
    return
  }
  const q = searchQuery.value.trim()
  searchRequest?.abort()
  searchRequest = null
  if (q.length < 3) {
    searchResults.value = []
    isSearching.value = false
    return
  }
  const request = new AbortController()
  searchRequest = request
  isSearching.value = true
  try {
    const results = await searchPlaces(q, 5, request.signal)
    if (!request.signal.aborted) searchResults.value = results
  } finally {
    if (searchRequest === request) {
      searchRequest = null
      isSearching.value = false
    }
  }
}

watch(searchQuery, () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(performSearch, 150)
})

function selectOrigin(res: Place) {
  customOrigin.value = {
    name: res.label,
    lat: parseFloat(res.lat),
    lon: parseFloat(res.lon)
  }
  clearPlanSelection()
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
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
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  void router.replace({query: newQuery})
  void calculateRoutes()
}

function useCurrentLocation() {
  customOrigin.value = null
  clearPlanSelection()
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  void calculateRoutes()
}

function closeActiveSearch() {
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
  if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
}

function dropPinAsDestination() {
  if (!isOnline.value) return
  const label = t('droppedPin')
  const newQuery = { ...route.query, lat: String(cityInfo()?.lat ?? 46.7712), lon: String(cityInfo()?.lon ?? 23.6236), name: label }
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
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
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
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

function openSearchField(field: 'origin' | 'destination') {
  if (field === 'destination' && !isOnline.value) return
  activeSearchField.value = field
  searchResults.value = []
  searchQuery.value = ''
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
  if (searchTimeout) clearTimeout(searchTimeout)
  await performSearch()
  if (!kbd.keyboardMode.value || !activeSearchField.value || query.trim().length < 3) return
  if (searchQuery.value !== query || document.activeElement !== input) return
  await nextTick()
  kbd.focusSection(activeSearchField.value === 'origin' ? 'from-results' : 'to-results')
}

function closeSearchField() {
  activeSearchField.value = null
  searchQuery.value = ''
  searchResults.value = []
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
const formatLocalDateTime = (d: Date) => {
  const pad = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
const minDateTime = computed(() => formatLocalDateTime(userTime.value || new Date()))
const minDateObj = computed(() => userTime.value || new Date())
const isDarkTheme = computed(() => settings.isDark)
watch(timeMode, (mode) => {
  if (mode !== 'now' && mode !== 'last' && !timeValue.value) {
    timeValue.value = formatLocalDateTime(new Date())
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
            @keydown.enter="confirmRename"
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

<style scoped>
.plan-view-container {
  padding: 1.25rem 1.5rem 0;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

html[data-arcade] .section-label {
  color: #92400E;
}

html.dark[data-arcade] .section-label {
  color: #fde68a;
}

html[data-arcade] .route-legs-card {
  background: #fffbeb;
  border: 2px solid #fde68a;
}

html.dark[data-arcade] .route-legs-card {
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
  padding: 0.75rem;
  background: linear-gradient(135deg, #f0f9ff 0%, #e0f2fe 100%);
  border: 1px solid #bae6fd;
  border-radius: 1rem;
}

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


html[data-arcade] .trip-summary-stat-label {
  color: #92400e;
  font-family: inherit;
}

html.dark[data-arcade] .trip-summary {
  background: linear-gradient(135deg, #1c1608 0%, #211a05 100%);
  border-color: #78350f;
}

html.dark[data-arcade] .trip-summary-stat:not(:last-child) {
  border-right-color: #78350f;
}

html.dark[data-arcade] .trip-summary-stat-value {
  color: #fde68a;
}

html.dark[data-arcade] .trip-summary-stat-value.is-live {
  color: #34d399;
}

html.dark[data-arcade] .trip-summary-stat-label {
  color: #d97706;
}

html[data-legacy-blue] .trip-summary {
  background: #ECE9D8;
  border: 2px solid #919B9C;
  border-radius: 0;
  box-shadow: inset -1px -1px 1px #ffffff, inset 1px 1px 1px #000000;
  padding: 0.5rem;
}

html[data-legacy-blue] .trip-summary-stat:not(:last-child) {
  border-right: 1px solid #919B9C;
}

html[data-legacy-blue] .trip-summary-stat-value {
  color: #000000;
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
}

html[data-legacy-blue] .trip-summary-stat-value.is-live {
  color: #006400;
}

html[data-legacy-blue] .trip-summary-stat-label {
  color: #404040;
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
}

html[data-arcade] .trip-summary {
  background: linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%);
  border-color: #fde68a;
}

html[data-arcade] .trip-summary-stat-value {
  color: #92400e;
}

html[data-arcade] .trip-summary-stat-label {
  color: #b45309;
}

html[data-arcade] .trip-summary-stat:not(:last-child) {
  border-right-color: #fde68a;
}

html[data-arcade] .card-dest {
  color: #78350f !important;
}

html[data-arcade] .card-arrow {
  color: #b45309 !important;
}

html[data-arcade] .bus-chain-arrow {
  color: #92400e !important;
}

html[data-arcade] .bus-chip-overflow {
  background: #fde68a !important;
  color: #92400e !important;
  border-color: #f59e0b !important;
}

html[data-arcade] .card-arrival-time {
  color: #b45309 !important;
}

html[data-arcade] .card-primary-time-sched {
  color: #78350f !important;
}

html[data-arcade] .card-primary-time-live {
  color: #047857 !important;
}

html[data-arcade] .departure-card.is-selected .card-primary-time-live {
  color: #047857 !important;
}

html[data-arcade] .departure-card.is-selected .live-dot {
  background: #10b981 !important;
}

html[data-arcade] .card-chevron {
  color: #d97706 !important;
}

html[data-arcade] .departure-card:hover .card-chevron {
  color: #92400e !important;
}

html[data-arcade] .departure-card.is-selected .card-chevron {
  color: #78350f !important;
}

html[data-arcade] .card-rail.is-active {
  background: #f59e0b !important;
}

html[data-arcade] .stat-chip {
  background: #fef3c7 !important;
  color: #92400e !important;
}

html[data-arcade] .stat-chip-transfer {
  background: #fde68a !important;
  color: #78350f !important;
}

html[data-arcade] .stat-chip-live {
  background: #ecfdf5 !important;
  color: #047857 !important;
}

html[data-arcade] .stat-chip-duration {
  background: #fef3c7 !important;
  color: #92400e !important;
}

html[data-arcade] .stat-chip-next {
  color: #b45309 !important;
}

html[data-arcade] .stat-chip-time {
  color: #78350f !important;
}

html[data-arcade] .stat-chip-label {
  color: #b45309 !important;
}

html.dark[data-arcade] .card-dest {
  color: #fde68a !important;
}

html.dark[data-arcade] .card-arrow {
  color: #d97706 !important;
}

html.dark[data-arcade] .bus-chain-arrow {
  color: #fbbf24 !important;
}

html.dark[data-arcade] .bus-chip-overflow {
  background: #451a03 !important;
  color: #fde68a !important;
  border-color: #d97706 !important;
}

html.dark[data-arcade] .card-arrival-time {
  color: #d97706 !important;
}

html.dark[data-arcade] .card-primary-time-sched {
  color: #fde68a !important;
}

html.dark[data-arcade] .card-primary-time-live {
  color: #34d399 !important;
}

html.dark[data-arcade] .departure-card.is-selected .card-primary-time-live {
  color: #34d399 !important;
}

html.dark[data-arcade] .departure-card.is-selected .live-dot {
  background: #34d399 !important;
}

html.dark[data-arcade] .card-chevron {
  color: #78350f !important;
}

html.dark[data-arcade] .departure-card:hover .card-chevron {
  color: #d97706 !important;
}

html.dark[data-arcade] .departure-card.is-selected .card-chevron {
  color: #fde68a !important;
}

html.dark[data-arcade] .card-rail.is-active {
  background: #d97706 !important;
}

html.dark[data-arcade] .stat-chip {
  background: #451a03 !important;
  color: #fde68a !important;
}

html.dark[data-arcade] .stat-chip-transfer {
  background: #78350f !important;
  color: #fde68a !important;
}

html.dark[data-arcade] .stat-chip-live {
  background: rgba(16, 185, 129, 0.15) !important;
  color: #34d399 !important;
}

html.dark[data-arcade] .stat-chip-duration {
  background: #451a03 !important;
  color: #fde68a !important;
}

html.dark[data-arcade] .stat-chip-next {
  color: #d97706 !important;
}

html.dark[data-arcade] .stat-chip-time {
  color: #fde68a !important;
}

html.dark[data-arcade] .stat-chip-label {
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

html[data-arcade] .leg-dot-origin {
  background: #f59e0b;
  box-shadow: 0 0 0 3px #fef3c7;
}

html[data-arcade] .leg-dot-dest {
  background: #b45309;
  box-shadow: 0 0 0 3px #fde68a;
}

html[data-arcade] .leg-type-badge {
  color: #d97706;
}

html[data-legacy-blue] .leg-dot-origin,
html[data-legacy-blue] .leg-dot-dest {
  background: transparent !important;
  box-shadow: none !important;
}

html[data-legacy-blue] .boarding-dot {
  border-radius: 0;
}

html[data-legacy-blue] .intermediate-dot {
  border-radius: 0;
}

html[data-legacy-blue] .intermediate-dot div {
  border-radius: 0;
}

html[data-legacy-blue] .dot-inner {
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

html[data-legacy-blue] .transfer-block-icon,
html[data-legacy-blue] .transfer-block-content {
  border-radius: 0;
}

html[data-legacy-blue] .departure-card {
  border-radius: 0;
}

html[data-legacy-blue] .stat-chip {
  border-radius: 0 !important;
  background: var(--xp-btn) !important;
  border: 1px solid var(--xp-border) !important;
  color: #000000 !important;
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .stat-chip-transfer {
  background: var(--xp-btn) !important;
  color: #000000 !important;
}

html[data-legacy-blue] .stat-chip-live {
  background: var(--xp-live) !important;
  color: #FFFFFF !important;
  border: 1px solid #3D7E22 !important;
  text-shadow: 0 1px 0 rgba(0, 0, 0, 0.35) !important;
}

html[data-legacy-blue] .stat-chip-duration {
  background: var(--xp-btn) !important;
  color: #000000 !important;
}

html[data-legacy-blue] .stat-chip-next {
  color: #404040 !important;
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .stat-chip-time {
  color: #000000 !important;
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .stat-chip-label {
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .card-dest {
  color: #000000 !important;
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .card-arrow {
  color: #404040 !important;
}

html[data-legacy-blue] .bus-chain-arrow {
  color: #000000 !important;
}

html[data-legacy-blue] .card-arrival-time {
  color: #404040 !important;
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .card-primary-time-sched {
  color: #000000 !important;
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .card-primary-time-live {
  color: #006400 !important;
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .departure-card.is-selected .card-primary-time-live {
  color: #90EE90 !important;
}

html[data-legacy-blue] .departure-card.is-selected .live-dot {
  background: #90EE90 !important;
}

html[data-legacy-blue] .departure-card.is-selected .card-dest,
html[data-legacy-blue] .departure-card.is-selected .card-arrow,
html[data-legacy-blue] .departure-card.is-selected .bus-chain-arrow,
html[data-legacy-blue] .departure-card.is-selected .card-arrival-time,
html[data-legacy-blue] .departure-card.is-selected .card-primary-time-sched,
html[data-legacy-blue] .departure-card.is-selected .stat-chip-next,
html[data-legacy-blue] .departure-card.is-selected .stat-chip-label,
html[data-legacy-blue] .departure-card.is-selected .stat-chip-time,
html[data-legacy-blue] .departure-card.is-selected .card-chevron {
  color: #FFFFFF !important;
}

html[data-legacy-blue] .departure-card.is-selected .stat-chip,
html[data-legacy-blue] .departure-card.is-selected .stat-chip-transfer,
html[data-legacy-blue] .departure-card.is-selected .stat-chip-duration {
  background: rgba(255, 255, 255, 0.2) !important;
  border-color: rgba(255, 255, 255, 0.45) !important;
  color: #FFFFFF !important;
}

html[data-legacy-blue] .card-chevron {
  color: var(--xp-blue) !important;
}

html[data-legacy-blue] .card-rail {
  border-radius: 0 !important;
}

html[data-legacy-blue] .card-rail.is-active {
  background: var(--xp-blue) !important;
}

html[data-legacy-blue] .bus-chip {
  border: 1px solid rgba(0, 0, 0, 0.35) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.35) !important;
  border-radius: 0 !important;
  font-family: var(--xp-font) !important;
}

html[data-legacy-blue] .bus-chip-overflow {
  background: var(--xp-btn) !important;
  border: 1px solid var(--xp-border) !important;
  border-radius: 0 !important;
  color: var(--xp-text) !important;
  box-shadow: none !important;
  font-family: var(--xp-font) !important;
  padding: 0 0.35rem !important;
}

html[data-legacy-blue] .bus-chip-overflow:hover {
  background: var(--xp-btn-hover) !important;
}

html[data-legacy-blue] .bus-chip-overflow:active {
  background: var(--xp-btn-active) !important;
  color: white !important;
}

html[data-legacy-blue] .live-dot {
  border-radius: 0 !important;
  background: #6FD75A !important;
  box-shadow: none !important;
}

html[data-legacy-blue] .departure-card:not(.is-selected) .stat-chip-live {
  background: linear-gradient(to bottom, #7DCB5E 0%, #5BAA38 52%, #46892C 100%) !important;
  border-color: #3D7E22 !important;
  color: #FFFFFF !important;
}

html[data-legacy-blue] .departure-card:not(.is-selected) .stat-chip-live .live-dot {
  background: #5AAE42 !important;
}

html[data-legacy-blue] .time-pill {
  border-radius: 0 !important;
}

html[data-legacy-blue] .time-pill-live {
  background: #3D7E22 !important;
}

html[data-legacy-blue] .time-pill-sched {
  background: var(--xp-btn) !important;
  color: #000000 !important;
  border: 1px solid var(--xp-border) !important;
}

html.dark[data-legacy-blue] .stat-chip {
  background: var(--xp-btn) !important;
  border: 1px solid var(--xp-border) !important;
  color: var(--xp-text) !important;
}

html.dark[data-legacy-blue] .stat-chip-transfer {
  background: var(--xp-btn) !important;
  color: var(--xp-text) !important;
}

html.dark[data-legacy-blue] .stat-chip-live {
  background: linear-gradient(to bottom, #7EC860 0%, #6FB452 50%, #5BAA38 100%) !important;
  color: #FFFFFF !important;
  border: 1px solid #5BAA38 !important;
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.4) !important;
}

html.dark[data-legacy-blue] .departure-card:not(.is-selected) .stat-chip-live {
  background: linear-gradient(to bottom, #78C65C 0%, #58A737 55%, #3F8228 100%) !important;
  border-color: #3A7A23 !important;
}

html.dark[data-legacy-blue] .departure-card:not(.is-selected) .stat-chip-live .live-dot {
  background: #6ACF58 !important;
}

html.dark[data-legacy-blue] .stat-chip-duration {
  background: var(--xp-btn) !important;
  color: var(--xp-text) !important;
}

html.dark[data-legacy-blue] .stat-chip-next {
  color: #8898B0 !important;
}

html.dark[data-legacy-blue] .stat-chip-time {
  color: var(--xp-text) !important;
}

html.dark[data-legacy-blue] .card-dest {
  color: var(--xp-text) !important;
}

html.dark[data-legacy-blue] .card-arrow {
  color: #8898B0 !important;
}

html.dark[data-legacy-blue] .bus-chain-arrow {
  color: var(--xp-text) !important;
}

html.dark[data-legacy-blue] .card-arrival-time {
  color: #8898B0 !important;
}

html.dark[data-legacy-blue] .card-primary-time-sched {
  color: var(--xp-text) !important;
}

html.dark[data-legacy-blue] .card-primary-time-live {
  color: #5BAA38 !important;
}

html.dark[data-legacy-blue] .departure-card.is-selected .card-primary-time-live {
  color: #90EE90 !important;
}

html.dark[data-legacy-blue] .departure-card.is-selected .live-dot {
  background: #90EE90 !important;
}

html.dark[data-legacy-blue] .card-chevron {
  color: var(--xp-blue) !important;
}

html.dark[data-legacy-blue] .card-rail.is-active {
  background: var(--xp-blue) !important;
}

html.dark[data-legacy-blue] .bus-chip {
  border: 1px solid rgba(0, 0, 0, 0.5) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.15) !important;
}

html.dark[data-legacy-blue] .bus-chip-overflow {
  background: var(--xp-btn) !important;
  border: 1px solid var(--xp-border) !important;
  color: var(--xp-text) !important;
}

html.dark[data-legacy-blue] .live-dot {
  background: #5BAA38 !important;
  box-shadow: none !important;
}

html.dark[data-legacy-blue] .time-pill-live {
  background: #5BAA38 !important;
}

html.dark[data-legacy-blue] .time-pill-sched {
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
  padding: 0 0.55rem;
  min-width: 2.25rem;
  min-height: 1.9rem;
  height: 1.9rem;
  border-radius: 0.5rem;
  font-weight: 800;
  font-size: 0.78rem;
  line-height: 1;
  letter-spacing: 0.01em;
  color: white;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
  white-space: nowrap;
  flex-wrap: nowrap;
  flex-shrink: 0;
  max-width: 100%;
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
  min-height: 1.9rem;
  height: 1.9rem;
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

.route-results-list {
  padding-bottom: max(0.75rem, env(safe-area-inset-bottom));
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

.start-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 9999px;
  color: var(--color-sky-600, #0284c7);
  background: transparent;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, transform 0.15s;
}

.start-btn:hover {
  background: var(--color-sky-100, #e0f2fe);
}

.start-btn:active {
  transform: scale(0.92);
}

:root.dark .start-btn {
  color: var(--color-sky-400, #38bdf8);
}

:root.dark .start-btn:hover {
  background: rgb(14 165 233 / 0.15);
}

.rename-btn {
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

.rename-btn:hover {
  background: #eff6ff;
  color: #3b82f6;
}

.rename-btn:active {
  transform: scale(0.92);
}

.rename-input {
  flex: 1;
  min-width: 0;
  font-size: 1.05rem;
  font-weight: 700;
  color: #0f172a;
  background: #f1f5f9;
  border: 1.5px solid #94a3b8;
  border-radius: 0.5rem;
  padding: 0.2rem 0.5rem;
  outline: none;
}

.rename-input:focus {
  border-color: #3b82f6;
  background: #fff;
}

.rename-confirm-btn,
.rename-cancel-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 9999px;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.12s, color 0.12s;
}

.rename-confirm-btn {
  color: #16a34a;
  background: transparent;
}

.rename-confirm-btn:hover {
  background: #dcfce7;
}

.rename-cancel-btn {
  color: #64748b;
  background: transparent;
}

.rename-cancel-btn:hover {
  background: #f1f5f9;
}

.dark .plan-placeholder {
  background: #1e293b;
  border-color: #334155;
}

html[data-arcade] .plan-placeholder {
  background: #fffbeb;
  border-color: #fde68a;
  border-style: solid;
}

html[data-legacy-blue] .plan-placeholder {
  background: var(--xp-tan, #ECE9D8);
  border: 2px solid var(--xp-border, #919B9C);
  border-radius: 0;
  box-shadow: inset -1px -1px 1px #ffffff, inset 1px 1px 1px #000000;
  border-style: solid;
}

html.dark[data-legacy-blue] .plan-placeholder {
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

.drop-pin-option {
  flex-direction: row;
  align-items: center;
  gap: 0.5rem;
  color: #64748b;
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

html[data-legacy-blue] .search-results {
  background: #ECE9D8;
  border: 1px solid #919B9C;
  border-radius: 0;
  box-shadow: 2px 2px 0 rgba(0,0,0,0.5);
}

html[data-legacy-blue] .search-result-item {
  border-bottom: 1px solid #919B9C;
  font-family: 'Tahoma', sans-serif;
}

html[data-legacy-blue] .search-result-item:hover {
  background: #316AC5;
}

html[data-legacy-blue] .search-result-item:hover .res-main,
html[data-legacy-blue] .search-result-item:hover .res-sub {
  color: #FFFFFF !important;
}

html[data-legacy-blue] .res-main {
  color: #000000;
}

html[data-legacy-blue] .res-sub {
  color: #444444;
}

html[data-legacy-blue] .current-loc-option {
  background: #ECE9D8;
  font-style: italic;
}

html[data-legacy-blue] .current-loc-option:hover {
  background: #316AC5;
}

html[data-legacy-blue] .mini-spinner {
  border-top-color: #316AC5;
}

html.dark[data-legacy-blue] .search-results {
  background: #2A2D38;
  border-color: #444A5C;
}

html.dark[data-legacy-blue] .search-result-item {
  border-bottom-color: #444A5C;
}

html.dark[data-legacy-blue] .res-main {
  color: #E0E6F2;
}

html.dark[data-legacy-blue] .res-sub {
  color: #94A3B8;
}

html.dark[data-legacy-blue] .current-loc-option {
  background: #2A2D38;
  border-bottom-color: #444A5C;
}

html.dark[data-legacy-blue] .current-loc-option:hover {
  background: #316AC5;
}

html[data-arcade] .origin-clickable:hover .edit-icon {
  color: #F59E0B;
}

html[data-arcade] .search-wrap {
  background: #FFFBEB;
  border-color: #F59E0B;
}

html[data-arcade] .search-wrap:focus-within {
  border-color: #D97706;
}

html[data-arcade] .search-input {
  color: #92400E;
}

html[data-arcade] .search-cancel:hover {
  background: #FEF3C7;
  color: #92400E;
}

html[data-arcade] .search-results {
  background: #FFFBEB;
  border: 2px solid #F59E0B;
  border-radius: 0.5rem;
}

html[data-arcade] .search-result-item {
  border-bottom-color: #FEF3C7;
}

html[data-arcade] .search-result-item:hover {
  background: #FEF3C7;
}

html[data-arcade] .res-main {
  color: #92400E;
}

html[data-arcade] .res-sub {
  color: #B45309;
}

html[data-arcade] .current-loc-option {
  background: #FFFBEB;
  border-bottom-color: #FEF3C7;
}

html[data-arcade] .current-loc-option:hover {
  background: #FEF3C7;
}

html[data-arcade] .mini-spinner {
  border-top-color: #F59E0B;
}

html.dark[data-arcade] .origin-clickable:hover .edit-icon {
  color: #d97706;
}

html.dark[data-arcade] .search-wrap {
  background: #211a05;
  border-color: #78350f;
}

html.dark[data-arcade] .search-wrap:focus-within {
  border-color: #d97706;
}

html.dark[data-arcade] .search-input {
  color: #fde68a;
}

html.dark[data-arcade] .search-cancel:hover {
  background: #2a2006;
  color: #fde68a;
}

html.dark[data-arcade] .search-results {
  background: #1c1608;
  border-color: #78350f;
}

html.dark[data-arcade] .search-result-item {
  border-bottom-color: #451a03;
}

html.dark[data-arcade] .search-result-item:hover {
  background: #2a2006;
}

html.dark[data-arcade] .res-main {
  color: #fde68a;
}

html.dark[data-arcade] .res-sub {
  color: #d97706;
}

html.dark[data-arcade] .current-loc-option {
  background: #211a05;
  border-bottom-color: #451a03;
}

html.dark[data-arcade] .current-loc-option:hover {
  background: #2a2006;
}

html.dark[data-arcade] .mini-spinner {
  border-top-color: #d97706;
}

.plan-time-filter {
  --pt-font-family: inherit;
  --pt-font-size: 1rem;
  --pt-font-weight: 500;
  --pt-filter-radius: 0.875rem;
  --pt-input-radius: 0.5rem;
  --pt-cell-radius: 0.5rem;
  --pt-border-width: 1.5px;
  --pt-input-border-width: 1px;
  --pt-input-height: 2.375rem;
  --pt-padding: 0.5rem 0.625rem;
  --pt-surface: #f8fafc;
  --pt-border: #e2e8f0;
  --pt-border-hover: #94a3b8;
  --pt-text: #334155;
  --pt-muted: #94a3b8;
  --pt-input-bg: transparent;
  --pt-input-bg-hover: #f8fafc;
  --pt-option-bg: #ffffff;
  --pt-option-text: #334155;
  --pt-option-selected-bg: #f1f5f9;
  --pt-option-selected-text: #0f172a;
  --pt-color-scheme: light;
  --pt-dp-bg: #ffffff;
  --pt-dp-text: #0f172a;
  --pt-dp-primary: #0ea5e9;
  --pt-dp-primary-text: #ffffff;
  --pt-dp-hover: #f1f5f9;
  --pt-dp-hover-text: #0f172a;
  --pt-dp-secondary: #94a3b8;
  --pt-dp-disabled: #f1f5f9;
  --pt-dp-icon: #94a3b8;
  --pt-dp-scroll-bg: #f1f5f9;
  --pt-dp-scroll: #cbd5e1;
  --pt-dp-preview-size: 0.85rem;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  grid-template-areas:
    "mode actions"
    "datetime datetime";
  align-items: center;
  gap: 0.5rem;
  padding: var(--pt-padding);
  background: var(--pt-surface);
  border: var(--pt-border-width) solid var(--pt-border);
  border-radius: var(--pt-filter-radius);
}

.plan-time-filter-actions {
  grid-area: actions;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: nowrap;
  gap: 0.25rem;
}

.plan-time-mode {
  grid-area: mode;
  position: relative;
  display: inline-flex;
  align-items: center;
  width: 100%;
  min-width: 0;
}

.plan-time-select {
  appearance: none;
  -webkit-appearance: none;
  color-scheme: var(--pt-color-scheme);
  box-sizing: border-box;
  height: var(--pt-input-height);
  background: var(--pt-input-bg);
  border: var(--pt-input-border-width) solid var(--pt-border);
  border-radius: var(--pt-input-radius);
  color: var(--pt-text);
  font-family: var(--pt-font-family);
  font-size: var(--pt-font-size);
  font-weight: var(--pt-font-weight);
  line-height: 1.5;
  padding: 0 1.85rem 0 0.95rem;
  cursor: pointer;
  outline: none;
  transition: border-color 120ms ease, background 120ms ease, color 120ms ease;
  width: 100%;
}

.plan-time-select option {
  background: var(--pt-option-bg);
  color: var(--pt-option-text);
}

.plan-time-select option:checked {
  background: var(--pt-option-selected-bg);
  color: var(--pt-option-selected-text);
}

.plan-time-select:hover,
.plan-time-select:focus,
.plan-time-select:focus-visible {
  border-color: var(--pt-border-hover);
  background: var(--pt-input-bg-hover);
}

.plan-time-chevron {
  position: absolute;
  right: 0.55rem;
  top: 50%;
  width: 0.85rem;
  height: 0.85rem;
  transform: translateY(-50%);
  color: var(--pt-muted);
  pointer-events: none;
}

.plan-time-datetime {
  grid-area: datetime;
  min-width: 0;
  max-width: 80%;
  font-family: var(--pt-font-family);
  color-scheme: var(--pt-color-scheme);
}

.plan-time-datetime :deep(.dp--main),
.plan-time-datetime :deep(.dp--theme-light),
.plan-time-datetime :deep(.dp--theme-dark) {
  --dp-font-family: var(--pt-font-family);
  --dp-font-size: var(--pt-font-size);
  --dp-border-radius: var(--pt-input-radius);
  --dp-cell-border-radius: var(--pt-cell-radius);
  --dp-input-padding: 0.4rem 0.625rem;
  --dp-input-icon-padding: 2rem;
  --dp-menu-min-width: 260px;
  --dp-action-button-height: 28px;
  --dp-action-buttons-padding: 4px 12px;
  --dp-preview-font-size: var(--pt-dp-preview-size);
  --dp-background-color: var(--pt-dp-bg);
  --dp-text-color: var(--pt-dp-text);
  --dp-border-color: var(--pt-border);
  --dp-menu-border-color: var(--pt-border);
  --dp-border-color-hover: var(--pt-border-hover);
  --dp-border-color-focus: var(--pt-border-hover);
  --dp-icon-color: var(--pt-dp-icon);
  --dp-primary-color: var(--pt-dp-primary);
  --dp-primary-text-color: var(--pt-dp-primary-text);
  --dp-hover-color: var(--pt-dp-hover);
  --dp-hover-text-color: var(--pt-dp-hover-text);
  --dp-secondary-color: var(--pt-dp-secondary);
  --dp-success-color: var(--pt-dp-primary);
  --dp-disabled-color: var(--pt-dp-disabled);
  --dp-scroll-bar-background: var(--pt-dp-scroll-bg);
  --dp-scroll-bar-color: var(--pt-dp-scroll);
}

.plan-time-datetime :deep(.dp--input) {
  box-sizing: border-box;
  height: var(--pt-input-height);
  font-family: var(--pt-font-family);
  font-size: var(--pt-font-size);
  font-weight: var(--pt-font-weight);
  border-width: var(--pt-input-border-width);
  border-color: var(--pt-border);
  background: var(--pt-input-bg);
  color: var(--pt-text);
  padding: 0 0.625rem 0 var(--dp-input-icon-padding);
}

.plan-time-datetime :deep(.dp--input:hover),
.plan-time-datetime :deep(.dp--input:focus),
.plan-time-datetime :deep(.dp--input:focus-visible),
.plan-time-datetime :deep(.dp--input-focus),
.plan-time-datetime :deep(.dp--input-wrap:focus-within .dp--input) {
  border-color: var(--pt-border-hover) !important;
}

.plan-time-datetime :deep(.dp--input-wrap .dp--input) {
  border-color: var(--pt-border) !important;
}

.plan-time-datetime :deep(.dp--menu) {
  border-width: var(--pt-input-border-width);
  border-radius: var(--pt-input-radius);
}

.plan-time-datetime :deep(.dp--menu-inner),
.plan-time-datetime :deep(.dp--calendar),
.plan-time-datetime :deep(.dp--action-row),
.plan-time-datetime :deep(.dp--action-extra),
.plan-time-datetime :deep(.dp--input-wrap),
.plan-time-datetime :deep(.dp--inner-nav),
.plan-time-datetime :deep(.dp--action-button),
.plan-time-datetime :deep(.dp--cell-inner) {
  border-radius: var(--pt-cell-radius);
}

.plan-time-datetime :deep(.dp--month-year-wrap),
.plan-time-datetime :deep(.dp--calendar-header),
.plan-time-datetime :deep(.dp--calendar-header-item),
.plan-time-datetime :deep(.dp--inner-nav),
.plan-time-datetime :deep(.dp--action-row) {
  color: var(--pt-dp-text);
}

.plan-time-datetime :deep(.dp--calendar-header-separator) {
  border-color: var(--pt-border);
}

.plan-time-datetime :deep(.dp--arrow-top) {
  border-left-color: var(--pt-border);
  border-top-color: var(--pt-border);
}

.plan-time-datetime :deep(.dp--clear-btn),
.plan-time-datetime :deep(.dp--input-icons) {
  color: var(--pt-dp-icon);
}

.plan-time-datetime :deep(.dp--action-select) {
  background: var(--pt-dp-primary);
  color: var(--pt-dp-primary-text);
  font-weight: 600;
}

.plan-time-datetime :deep(.dp--action-select:hover) {
  filter: brightness(1.08);
}

.plan-time-datetime :deep(.dp--action-cancel) {
  color: var(--pt-dp-text);
  border: 1px solid var(--pt-border);
  background: transparent;
}

.plan-time-datetime :deep(.dp--action-cancel:hover) {
  border-color: var(--pt-border-hover);
  background: var(--pt-dp-hover);
}

.dark .plan-time-filter {
  --pt-surface: #1e293b;
  --pt-border: #334155;
  --pt-border-hover: #475569;
  --pt-text: #cbd5e1;
  --pt-muted: #64748b;
  --pt-input-bg-hover: #1e293b;
  --pt-option-bg: #1e293b;
  --pt-option-text: #cbd5e1;
  --pt-option-selected-bg: #475569;
  --pt-option-selected-text: #f1f5f9;
  --pt-color-scheme: dark;
  --pt-dp-bg: #0f172a;
  --pt-dp-text: #e2e8f0;
  --pt-dp-primary: #38bdf8;
  --pt-dp-primary-text: #0f172a;
  --pt-dp-hover: #1e293b;
  --pt-dp-hover-text: #f1f5f9;
  --pt-dp-secondary: #64748b;
  --pt-dp-disabled: #1e293b;
  --pt-dp-icon: #94a3b8;
  --pt-dp-scroll-bg: #1e293b;
  --pt-dp-scroll: #475569;
}

html[data-arcade] .plan-time-filter {
  --pt-filter-radius: 0.5rem;
  --pt-input-radius: 0.5rem;
  --pt-border-width: 2px;
  --pt-input-border-width: 2px;
  --pt-surface: #fffbeb;
  --pt-border: #f59e0b;
  --pt-border-hover: #d97706;
  --pt-text: #92400e;
  --pt-muted: #b45309;
  --pt-input-bg: #ffffff;
  --pt-input-bg-hover: #ffffff;
  --pt-option-bg: #ffffff;
  --pt-option-text: #92400e;
  --pt-option-selected-bg: #fef3c7;
  --pt-option-selected-text: #92400e;
  --pt-dp-bg: #fffbeb;
  --pt-dp-text: #92400e;
  --pt-dp-primary: #d97706;
  --pt-dp-primary-text: #ffffff;
  --pt-dp-hover: #fef3c7;
  --pt-dp-hover-text: #92400e;
  --pt-dp-secondary: #b45309;
  --pt-dp-disabled: #fef3c7;
  --pt-dp-icon: #b45309;
}

html.dark[data-arcade] .plan-time-filter {
  --pt-surface: #1c1608;
  --pt-border: #78350f;
  --pt-border-hover: #d97706;
  --pt-text: #fde68a;
  --pt-muted: #d97706;
  --pt-input-bg: #211a05;
  --pt-input-bg-hover: #211a05;
  --pt-option-bg: #1c1608;
  --pt-option-text: #fde68a;
  --pt-option-selected-bg: #2a2006;
  --pt-option-selected-text: #fde68a;
  --pt-color-scheme: dark;
  --pt-dp-bg: #1c1608;
  --pt-dp-text: #fde68a;
  --pt-dp-primary: #d97706;
  --pt-dp-primary-text: #1c1608;
  --pt-dp-hover: #2a2006;
  --pt-dp-hover-text: #fde68a;
  --pt-dp-secondary: #d97706;
  --pt-dp-disabled: #211a05;
  --pt-dp-icon: #d97706;
}

html[data-arcade] .refresh-btn,
html[data-arcade] .swap-btn {
  color: #B45309;
}

html[data-arcade] .refresh-btn:hover,
html[data-arcade] .swap-btn:hover {
  background: #FEF3C7;
  color: #92400E;
}

html.dark[data-arcade] .refresh-btn,
html.dark[data-arcade] .swap-btn {
  color: #d97706;
}

html.dark[data-arcade] .refresh-btn:hover,
html.dark[data-arcade] .swap-btn:hover {
  background: #211a05;
  color: #fde68a;
}

html[data-legacy-blue] .plan-time-filter {
  --pt-font-family: Tahoma, Geneva, sans-serif;
  --pt-font-size: 0.8rem;
  --pt-filter-radius: 0;
  --pt-input-radius: 0;
  --pt-cell-radius: 0;
  --pt-border-width: 1px;
  --pt-input-border-width: 1px;
  --pt-padding: 0.4rem 0.5rem;
  --pt-surface: #ece9d8;
  --pt-border: #7f9db9;
  --pt-border-hover: #245edc;
  --pt-text: #000000;
  --pt-muted: #245edc;
  --pt-input-bg: #ffffff;
  --pt-input-bg-hover: #ffffff;
  --pt-option-bg: #ffffff;
  --pt-option-text: #000000;
  --pt-option-selected-bg: #dce9f8;
  --pt-option-selected-text: #000000;
  --pt-dp-bg: #ece9d8;
  --pt-dp-text: #000000;
  --pt-dp-primary: #245edc;
  --pt-dp-primary-text: #ffffff;
  --pt-dp-hover: #dce9f8;
  --pt-dp-hover-text: #000000;
  --pt-dp-secondary: #245edc;
  --pt-dp-disabled: #d4d0c8;
  --pt-dp-icon: #245edc;
}

html.dark[data-legacy-blue] .plan-time-filter {
  --pt-surface: #1a2540;
  --pt-border: #3a4f7a;
  --pt-border-hover: #5a78b8;
  --pt-text: #e2e8f0;
  --pt-muted: #8aa9d4;
  --pt-input-bg: #0a1228;
  --pt-input-bg-hover: #0a1228;
  --pt-option-bg: #0a1228;
  --pt-option-text: #e2e8f0;
  --pt-option-selected-bg: #243254;
  --pt-option-selected-text: #ffffff;
  --pt-color-scheme: dark;
  --pt-dp-bg: #1a2540;
  --pt-dp-text: #e2e8f0;
  --pt-dp-primary: #5a78b8;
  --pt-dp-primary-text: #0a1228;
  --pt-dp-hover: #243254;
  --pt-dp-hover-text: #ffffff;
  --pt-dp-secondary: #8aa9d4;
  --pt-dp-disabled: #0a1228;
  --pt-dp-icon: #8aa9d4;
}

:global(.plan-time-menu) {
  --dp-font-family: inherit;
  --dp-font-size: 1rem;
  --dp-border-radius: 0.5rem;
  --dp-cell-border-radius: 0.5rem;
  --dp-menu-min-width: 260px;
  --dp-action-button-height: 28px;
  --dp-action-buttons-padding: 4px 12px;
  --dp-preview-font-size: 0.85rem;
  --dp-background-color: #ffffff;
  --dp-text-color: #0f172a;
  --dp-border-color: #e2e8f0;
  --dp-menu-border-color: #e2e8f0;
  --dp-border-color-hover: #94a3b8;
  --dp-border-color-focus: #94a3b8;
  --dp-icon-color: #94a3b8;
  --dp-primary-color: #0ea5e9;
  --dp-primary-text-color: #ffffff;
  --dp-hover-color: #f1f5f9;
  --dp-hover-text-color: #0f172a;
  --dp-secondary-color: #94a3b8;
  --dp-success-color: #0ea5e9;
  --dp-disabled-color: #f1f5f9;
  --dp-scroll-bar-background: #f1f5f9;
  --dp-scroll-bar-color: #cbd5e1;
  --plan-time-menu-border-width: 1px;
  border-width: var(--plan-time-menu-border-width);
  border-radius: var(--dp-border-radius);
}

:global(html.dark .plan-time-menu) {
  --dp-background-color: #0f172a;
  --dp-text-color: #e2e8f0;
  --dp-border-color: #334155;
  --dp-menu-border-color: #334155;
  --dp-border-color-hover: #475569;
  --dp-border-color-focus: #475569;
  --dp-icon-color: #94a3b8;
  --dp-primary-color: #38bdf8;
  --dp-primary-text-color: #0f172a;
  --dp-hover-color: #1e293b;
  --dp-hover-text-color: #f1f5f9;
  --dp-secondary-color: #64748b;
  --dp-disabled-color: #1e293b;
  --dp-scroll-bar-background: #1e293b;
  --dp-scroll-bar-color: #475569;
}

:global(html[data-arcade] .plan-time-menu) {
  --dp-background-color: #fffbeb;
  --dp-text-color: #92400e;
  --dp-border-color: #f59e0b;
  --dp-menu-border-color: #f59e0b;
  --dp-border-color-hover: #d97706;
  --dp-border-color-focus: #d97706;
  --dp-icon-color: #b45309;
  --dp-primary-color: #d97706;
  --dp-primary-text-color: #ffffff;
  --dp-hover-color: #fef3c7;
  --dp-hover-text-color: #92400e;
  --dp-secondary-color: #b45309;
  --dp-disabled-color: #fef3c7;
  --plan-time-menu-border-width: 2px;
}

:global(html.dark[data-arcade] .plan-time-menu) {
  --dp-background-color: #1c1608;
  --dp-text-color: #fde68a;
  --dp-border-color: #78350f;
  --dp-menu-border-color: #78350f;
  --dp-border-color-hover: #d97706;
  --dp-border-color-focus: #d97706;
  --dp-icon-color: #d97706;
  --dp-primary-color: #d97706;
  --dp-primary-text-color: #1c1608;
  --dp-hover-color: #2a2006;
  --dp-hover-text-color: #fde68a;
  --dp-secondary-color: #d97706;
  --dp-disabled-color: #211a05;
}

:global(html[data-legacy-blue] .plan-time-menu) {
  --dp-font-family: Tahoma, Geneva, sans-serif;
  --dp-font-size: 0.8rem;
  --dp-border-radius: 0;
  --dp-cell-border-radius: 0;
  --dp-background-color: #ece9d8;
  --dp-text-color: #000000;
  --dp-border-color: #7f9db9;
  --dp-menu-border-color: #7f9db9;
  --dp-border-color-hover: #245edc;
  --dp-border-color-focus: #245edc;
  --dp-icon-color: #245edc;
  --dp-primary-color: #245edc;
  --dp-primary-text-color: #ffffff;
  --dp-hover-color: #dce9f8;
  --dp-hover-text-color: #000000;
  --dp-secondary-color: #245edc;
  --dp-disabled-color: #d4d0c8;
  --plan-time-menu-border-width: 1px;
}

:global(html.dark[data-legacy-blue] .plan-time-menu) {
  --dp-background-color: #1a2540;
  --dp-text-color: #e2e8f0;
  --dp-border-color: #3a4f7a;
  --dp-menu-border-color: #3a4f7a;
  --dp-border-color-hover: #5a78b8;
  --dp-border-color-focus: #5a78b8;
  --dp-icon-color: #8aa9d4;
  --dp-primary-color: #5a78b8;
  --dp-primary-text-color: #0a1228;
  --dp-hover-color: #243254;
  --dp-hover-text-color: #ffffff;
  --dp-secondary-color: #8aa9d4;
  --dp-disabled-color: #0a1228;
}

:global(.plan-time-menu .dp--menu-inner),
:global(.plan-time-menu .dp--calendar),
:global(.plan-time-menu .dp--action-row),
:global(.plan-time-menu .dp--action-extra),
:global(.plan-time-menu .dp--inner-nav),
:global(.plan-time-menu .dp--action-button),
:global(.plan-time-menu .dp--cell-inner) {
  border-radius: var(--dp-cell-border-radius);
}

:global(.plan-time-menu .dp--month-year-wrap),
:global(.plan-time-menu .dp--calendar-header),
:global(.plan-time-menu .dp--calendar-header-item),
:global(.plan-time-menu .dp--inner-nav),
:global(.plan-time-menu .dp--action-row) {
  color: var(--dp-text-color);
}

:global(.plan-time-menu .dp--calendar-header-separator) {
  border-color: var(--dp-border-color);
}

:global(.plan-time-menu .dp--arrow-top) {
  border-left-color: var(--dp-menu-border-color);
  border-top-color: var(--dp-menu-border-color);
}

:global(.plan-time-menu .dp--action-select) {
  background: var(--dp-primary-color);
  color: var(--dp-primary-text-color);
  font-weight: 600;
}

:global(.plan-time-menu .dp--action-select:hover) {
  filter: brightness(1.08);
}

:global(.plan-time-menu .dp--action-cancel) {
  color: var(--dp-text-color);
  border: 1px solid var(--dp-border-color);
  background: transparent;
}

:global(.plan-time-menu .dp--action-cancel:hover) {
  border-color: var(--dp-border-color-hover);
  background: var(--dp-hover-color);
}

html[data-legacy-blue] .plan-time-datetime :deep(.dp--menu),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--menu-inner),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--calendar),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--action-row),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--month-year-wrap),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--inner-nav),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--action-button),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--cell-inner),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--input),
html[data-legacy-blue] .plan-time-datetime :deep(.dp--input-wrap),
:global(html[data-legacy-blue] .plan-time-menu),
:global(html[data-legacy-blue] .plan-time-menu .dp--menu-inner),
:global(html[data-legacy-blue] .plan-time-menu .dp--calendar),
:global(html[data-legacy-blue] .plan-time-menu .dp--action-row),
:global(html[data-legacy-blue] .plan-time-menu .dp--month-year-wrap),
:global(html[data-legacy-blue] .plan-time-menu .dp--inner-nav),
:global(html[data-legacy-blue] .plan-time-menu .dp--action-button),
:global(html[data-legacy-blue] .plan-time-menu .dp--cell-inner),
:global(html[data-legacy-blue] .plan-time-menu .dp--input),
:global(html[data-legacy-blue] .plan-time-menu .dp--input-wrap) {
  border-radius: 0 !important;
}

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

html[data-arcade] .plan-time-search-btn {
  background: #D97706;
  border: 2px solid #B45309;
  border-radius: 0.5rem;
  color: #FFFBEB;
}

html[data-arcade] .plan-time-search-btn:hover {
  background: #B45309;
  border-color: #92400E;
}

html[data-arcade] .plan-time-search-btn:disabled {
  background: #FCD34D;
  border-color: #FCD34D;
  color: #92400E;
}

html.dark[data-arcade] .plan-time-search-btn {
  background: #d97706;
  border-color: #92400E;
  color: #fde68a;
}

html.dark[data-arcade] .plan-time-search-btn:hover {
  background: #b45309;
}

html[data-legacy-blue] .plan-time-search-btn {
  background: linear-gradient(to bottom, #FFFFFF 0%, #ECE9D8 50%, #D7D2BC 100%);
  border: 1px solid #003C74;
  border-radius: 0 !important;
  color: #000;
  font-family: Tahoma, Geneva, sans-serif;
  font-size: 0.8rem;
  padding: 0.25rem 0.65rem;
}

html[data-legacy-blue] .plan-time-search-btn:hover {
  background: linear-gradient(to bottom, #FFFFFF 0%, #FFE9A0 50%, #F5C75A 100%);
  border-color: #003C74;
  color: #000;
}

html[data-legacy-blue] .plan-time-search-btn:disabled {
  background: #ECE9D8;
  color: #7F7F7F;
  border-color: #A0A0A0;
}

html.dark[data-legacy-blue] .plan-time-search-btn {
  background: linear-gradient(to bottom, #2a3a5c 0%, #1a2540 50%, #0a1228 100%);
  border-color: #3a4f7a;
  color: #e2e8f0;
}

html.dark[data-legacy-blue] .plan-time-search-btn:hover {
  background: linear-gradient(to bottom, #3a4f7a 0%, #2a3a5c 50%, #1a2540 100%);
}
</style>
