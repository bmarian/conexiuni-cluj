<script lang="ts">
export default {
  name: 'RoutePlanningView',
}
</script>

<script setup lang="ts">
import {cityDateTime} from '@/utils/cityClock.ts'
import {computed, nextTick, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {RouterLink, useRouter, type LocationQueryRaw} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {useMapStore, type HighlightedStop} from '@/stores/map.ts'
import {useUserStore} from '@/stores/user.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {usePlannerStore, type PlanTimeMode} from '@/stores/planner.ts'
import {apiRequest} from '@/utils/api.ts'
import type {RouteType, Shape} from '@/types/tranzy.ts'
import {storeToRefs} from 'pinia'
import ViewErrorState from '@/components/ViewErrorState.vue'
import LoadingIndicator from '@/components/LoadingIndicator.vue'
import {useOnline} from '@/composables/useOnline.ts'
import {usePlaceSearch} from '@/composables/usePlaceSearch.ts'
import {useKbdEscape, useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'

import {useLive} from '@/composables/useLive.ts'
import {useSwapMotion} from '@/composables/useSwapMotion.ts'
import {drawTimeline} from '@/utils/drawTimeline.ts'
import type {DisplayShape} from '@/stores/map.ts'
import type {DisplayVehicle} from '@/utils/mapIcons.ts'
import {formatMinutesFromNow} from '@/utils/time.ts'
import {decodePolyline} from '@/utils/geo.ts'
import {legTrips, planQuery, rideLegs, type PlanOption, type PlanResponse, type PlanRide} from '@/utils/plan.ts'
import {reversePlace, type Place} from '@/utils/places.ts'
import {canonicalUrl, siteUrl} from '@/utils/site.ts'
import {cityCenter, cityParams, cityPath} from '@/utils/city.ts'
import {useShare} from '@/composables/useShare.ts'
import {useOwnRoute} from '@/composables/useOwnRoute.ts'
import {usePlacePicks} from '@/composables/usePlacePicks.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ActionChips from '@/components/ui/ActionChips.vue'
import SelectField from '@/components/ui/SelectField.vue'
import PlanTimeFields from '@/components/plan/PlanTimeFields.vue'
import TimelineRow from '@/components/rows/TimelineRow.vue'
import type {ChipAction, Choice} from '@/components/ui/types.ts'
import {
  CheckIcon,
  CloseIcon,
  DirectionsBusIcon,
  DirectionsWalkIcon,
  EditIcon,
  FavoriteBorderIcon,
  FavoriteIcon,
  KeyboardArrowRightIcon,
  MapIcon,
  MyLocationIcon,
  PlaceIcon,
  PushPinIcon,
  RefreshIcon,
  ScheduleIcon,
  SearchIcon,
  ShareIcon,
  SwapVertIcon,
} from '@/utils/icons.ts'
import {displayColor, manyLines} from '@/utils/lineColor.ts'
import {linePath, stopLocation} from '@/utils/pagePaths.ts'

const {t} = useI18n()
const route = useOwnRoute('plan')
const router = useRouter()

const mapStore = useMapStore()
const userStore = useUserStore()
const favoritesStore = useFavoritesStore()
const plannerStore = usePlannerStore()
const {isOnline} = useOnline()
const {userLocation, hasLocationPermission, isLocating, userTime} = storeToRefs(userStore)
const {timeMode, timeValue} = storeToRefs(plannerStore)

const customOrigin = ref<{name: string; lat: number; lon: number} | null>(null)
const {activeSearchField, searchQuery, searchResults, isSearching, searchNow, openSearchField, closeSearchField} = usePlaceSearch(isOnline)
const picks = usePlacePicks(searchQuery)

const vFocus = {
  mounted: (el: HTMLElement) => el.focus(),
}

const destName = computed(() => (route.value.query.name as string | undefined) ?? '')
const destLat = computed(() => parseFloat((route.value.query.lat as string) ?? 'NaN'))
const destLon = computed(() => parseFloat((route.value.query.lon as string) ?? 'NaN'))

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
        {property: 'og:url', content: canonicalUrl(cityPath('/plan'))},
        {name: 'twitter:title', content: t('headPlanTitle')},
        {name: 'twitter:description', content: t('headPlanDesc', cityParams())},
      ],
      link: [{rel: 'canonical', href: canonicalUrl(cityPath('/plan'))}],
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
  const url = siteUrl(route.value.fullPath)

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

const isFavorite = computed(() => favoritesStore.isPlanFavorite(destLat.value, destLon.value, customOrigin.value?.lat, customOrigin.value?.lon))

const favoriteLabel = computed(() => favoritesStore.findPlanFavorite(destLat.value, destLon.value, customOrigin.value?.lat, customOrigin.value?.lon)?.label)

function toggleFavorite() {
  if (!isOnline.value) return
  if (isNaN(destLat.value) || isNaN(destLon.value)) return
  favoritesStore.togglePlanFavorite({
    name: destName.value || t('planTitleGeneric'),
    lat: destLat.value,
    lon: destLon.value,
    originName: customOrigin.value?.name,
    originLat: customOrigin.value?.lat,
    originLon: customOrigin.value?.lon,
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
  favoritesStore.renamePlanFavorite(destLat.value, destLon.value, customOrigin.value?.lat, customOrigin.value?.lon, renameValue.value.trim() || undefined)
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
const view = ref<HTMLElement>()

const options = computed(() => planData.value?.options ?? [])
const selectedPlan = computed(() => options.value[selectedPlanIndex.value])
const selectedPlanSignature = computed(() => selectedPlan.value?.key ?? null)

function selectPlanAt(idx: number) {
  selectedPlanIndex.value = idx
  const p = options.value[idx]
  selectedPlanKey.value = p ? p.key : null
  mapStore.fitWalkingPolylines = true
  const newQuery = {...route.value.query}
  if (idx === 0) {
    delete newQuery.plan
  } else {
    newQuery.plan = String(idx)
  }
  void router.replace({query: newQuery})
}

const stopName = (id: number | undefined) => (id ? planData.value?.stops[String(id)]?.name : undefined) ?? ''
const line = (id: number | undefined) => (id === undefined ? undefined : planData.value?.lines[String(id)])
const lineColor = (id: number | undefined, apart: boolean) => {
  const l = line(id)
  return l ? displayColor(l.short_name, l.color, apart) : '#64748b'
}
const linesApart = (plan: PlanOption) => manyLines(rideLegs(plan).map((leg) => line(leg.lines?.[0])?.short_name ?? ''))

const now = computed(() => userTime.value || new Date())
const minutesFrom = (at: number) => (at * 1000 - now.value.getTime()) / 60_000
const clock = (at: number) => new Date(at * 1000).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit', hour12: false})
const firstRide = (plan: PlanOption): PlanRide | undefined => plan.runs[0]?.rides[0]
const laterRides = (plan: PlanOption) =>
  plan.runs
    .slice(1)
    .map((r) => r.rides[0])
    .filter((r): r is PlanRide => !!r)
const journeyMinutes = (plan: PlanOption) => {
  const run = plan.runs[0]
  return run ? (run.arrive - run.depart) / 60 : 0
}
const fromNow = (ride: PlanRide) =>
  (ride.live ? '' : '~ ') + (timeMode.value === 'now' ? formatMinutesFromNow(minutesFrom(ride.depart), now.value, t('now')) : clock(ride.depart))

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
  const apart = linesApart(plan)
  let ride = 0
  for (const [i, leg] of plan.legs.entries()) {
    if (leg.kind !== 'ride') continue
    const lines = leg.lines ?? []
    const walk = plan.legs[i + 1]
    out.push({
      leg,
      lines,
      trips: legTrips(plan, ride),
      color: lineColor(lines[0], apart),
      intermediates: (leg.passed ?? []).map((id) => ({stop_id: id, stop_name: stopName(id)})),
      transferMeters: walk?.kind === 'walk' && plan.legs[i + 2]?.kind === 'ride' ? (walk.meters ?? 0) : 0,
    })
    ride++
  }
  return out
})

// The selected option on the map: its rides along the line's own shape, its walks, its stops.
watch(
  [selectedPlanSignature, planData, mapActivationKey],
  () => {
    const plan = selectedPlan.value
    if (!plan || !isActive.value) {
      mapStore.setLoadedShapes([])
      mapStore.setHighlightedStops([])
      mapStore.clearWalkingPolylines()
      return
    }
    const rides = rideLegs(plan)
    const apart = linesApart(plan)
    const shapes: Array<[DisplayShape, Shape[]]> = []
    const highlighted: HighlightedStop[] = []
    rides.forEach((leg, i) => {
      const id = leg.lines?.[0]
      const tripId = (id !== undefined ? legTrips(plan, i).get(id) : undefined) ?? ''
      const l = line(id)
      shapes.push([
        {
          trip_id: tripId,
          route_short_name: l?.short_name ?? '',
          route_long_name: l?.long_name ?? '',
          route_color: lineColor(id, apart),
          route_type: (l?.type ?? 3) as RouteType,
        },
        decodePolyline(leg.path).map(([lat, lon], k) => ({
          shape_id: tripId,
          shape_pt_lat: lat,
          shape_pt_lon: lon,
          shape_pt_sequence: k,
          shape_dist_traveled: -1,
        })),
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
  },
  {immediate: true},
)

// Only the buses the selected option catches, nothing else on the map.
const caughtTrips = computed(() => {
  if (!isActive.value || timeMode.value !== 'now') return []
  return [...new Set((selectedPlan.value?.runs[0]?.rides ?? []).filter((r) => r.vehicle_id).map((r) => r.trip_id))]
})
const {vehiclesByTrip} = useLive(ref<number[]>([]), caughtTrips)

watch(
  [vehiclesByTrip, selectedPlanSignature, isActive],
  () => {
    const vehicles: DisplayVehicle[] = []
    for (const ride of (isActive.value ? selectedPlan.value?.runs[0]?.rides : undefined) ?? []) {
      const vehicle = ride.vehicle_id ? vehiclesByTrip.value.get(ride.trip_id)?.find((v) => v.id === ride.vehicle_id) : undefined
      const l = line(ride.route_id)
      if (vehicle && l) vehicles.push({...vehicle, route_short_name: l.short_name, route_color: l.color})
    }
    mapStore.setVehiclesToDisplay(vehicles)
  },
  {deep: true},
)

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

function planOrigin(): {lat: number; lon: number} | null {
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
  if (route.value.query.plan === undefined) return
  const newQuery = {...route.value.query}
  delete newQuery.plan
  void router.replace({query: newQuery})
}

const {pinnedLocationDragged, customOriginLocationDragged} = storeToRefs(mapStore)

async function reverseGeocode(lat: number, lon: number): Promise<string> {
  try {
    const result = await reversePlace(lat, lon)
    return result?.label ?? `${lat.toFixed(4)}, ${lon.toFixed(4)}`
  } catch {
    /* ignore */
  }
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
    ...route.value.query,
    lat: coordQueryString(lat),
    lon: coordQueryString(lng),
    name,
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
    lon: parseFloat(res.lon),
  }
  clearPlanSelection()
  closeSearchField()
  void calculateRoutes()
}

function selectDestination(res: Place) {
  if (!isOnline.value) return
  const newQuery = {
    ...route.value.query,
    lat: res.lat,
    lon: res.lon,
    name: res.label,
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
  const newQuery = {...route.value.query, lat: String(cityCenter().lat), lon: String(cityCenter().lon), name: label}
  closeSearchField()
  void router.replace({query: newQuery})
  void calculateRoutes()
}

function useCurrentLocationAsDestination() {
  if (!isOnline.value) return
  if (!userLocation.value) return
  const newQuery = {
    ...route.value.query,
    lat: userLocation.value.latitude.toString(),
    lon: userLocation.value.longitude.toString(),
    name: t('planOriginCurrentLocation'),
  }
  closeSearchField()
  void router.replace({query: newQuery})
}

watch(customOrigin, (val) => {
  const newQuery = {...route.value.query}
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

watch(
  () => route.value.query,
  (newQuery) => {
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
  },
  {immediate: true},
)

watch([timeMode, timeValue], ([mode, value]) => {
  const newQuery = {...route.value.query}
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
  // It comes back as tall as it left, so the shared scroller would keep the previous page's offset.
  const scroller = view.value?.closest('.drawer-view')
  if (scroller) scroller.scrollTop = 0
  if (hasValidCoords.value) {
    mapStore.setPinnedLocation(destLat.value, destLon.value, destName.value)
  }
  if (customOrigin.value) {
    mapStore.setCustomOriginLocation(customOrigin.value.lat, customOrigin.value.lon, customOrigin.value.name)
  }
  mapActivationKey.value++
  if (currentQueryKey.value === null) void calculateRoutes()
})

watch([destLat, destLon, destName], ([lat, lon, name]) => {
  if (!isNaN(lat) && !isNaN(lon)) {
    mapStore.setPinnedLocation(lat, lon, name)
  }
})

watch(
  customOrigin,
  (co) => {
    if (co) {
      mapStore.setCustomOriginLocation(co.lat, co.lon, co.name)
    } else {
      mapStore.clearCustomOriginLocation()
    }
  },
  {immediate: true},
)

const navigateAwayFunctions = () => {
  isActive.value = false
  mapStore.clearPinnedLocation()
  mapStore.clearCustomOriginLocation()
  mapStore.setVehiclesToDisplay([])
  mapStore.setLoadedShapes([])
  mapStore.setHighlightedStops([])
  mapStore.clearWalkingPolylines()
}

onDeactivated(() => {
  navigateAwayFunctions()
  // Kept for the leave animation, then fetched again on the next visit.
  currentQueryKey.value = null
})
onUnmounted(navigateAwayFunctions)

watch(options, (opts) => {
  if (!opts.length) {
    selectedPlanIndex.value = 0
    selectedPlanKey.value = null
    return
  }
  const kept = selectedPlanKey.value ? opts.findIndex((o) => o.key === selectedPlanKey.value) : -1
  const urlPlan = parseInt(route.value.query.plan as string)
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
        favoritesStore.addRecent({
          name: destName.value || t('planTitleGeneric'),
          lat: destLat.value,
          lon: destLon.value,
          originName: customOrigin.value?.name,
          originLat: customOrigin.value?.lat,
          originLon: customOrigin.value?.lon,
        })
      }
    }
  } finally {
    if (!silent) isCalculating.value = false
  }
}

watch(
  [destLat, destLon],
  async ([newLat, newLon], [oldLat, oldLon] = [NaN, NaN]) => {
    if (newLat === oldLat && newLon === oldLon) return
    if (!isNaN(oldLat!)) clearPlanSelection()
    planData.value = null
    currentQueryKey.value = null
    mapStore.clearWalkingPolylines()
    await calculateRoutes()
  },
  {immediate: true},
)

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

const swapButton = ref<HTMLElement>()
const originLabelEl = ref<HTMLElement>()
const destLabelEl = ref<HTMLElement>()
const swapMotion = useSwapMotion(swapButton, originLabelEl, destLabelEl)

async function swapOriginDestination() {
  if (isNaN(destLat.value) || isNaN(destLon.value)) return
  const newDestLat = customOrigin.value?.lat ?? userLocation.value?.latitude
  const newDestLon = customOrigin.value?.lon ?? userLocation.value?.longitude
  if (newDestLat === undefined || newDestLon === undefined) return
  swapMotion.spin()

  const newDestName = customOrigin.value?.name ?? (await reverseGeocode(newDestLat, newDestLon))
  const newOriginName = isCurrentLocationName(destName.value) ? await reverseGeocode(destLat.value, destLon.value) : destName.value

  const newQuery: LocationQueryRaw = {
    ...route.value.query,
    lat: String(newDestLat),
    lon: String(newDestLon),
    name: newDestName,
    originLat: String(destLat.value),
    originLon: String(destLon.value),
    originName: newOriginName,
  }
  delete newQuery.plan
  await swapMotion.swap(() => router.replace({query: newQuery}))
}

watch(
  () => route.value.query.q,
  (q) => {
    if (typeof q !== 'string' || !q.trim() || !isOnline.value) return
    activeSearchField.value = 'destination'
    searchQuery.value = q
  },
  {immediate: true},
)

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

const legsCard = ref<HTMLElement>()
let stopDrawing = () => {}
// Kept alive, so a plan can arrive while the view is still detached and the draw would finish unseen.
watch(
  [selectedPlanSignature, isCalculating, isActive],
  ([plan, calculating, active]) => {
    stopDrawing()
    if (plan && !calculating && active && legsCard.value) stopDrawing = drawTimeline(legsCard.value)
  },
  {flush: 'post'},
)
onUnmounted(() => stopDrawing())

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

const canRecalculate = computed(() => hasValidCoords.value && !isCalculating.value && (hasLocationPermission.value || !!customOrigin.value))

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
const timeModes = computed<Choice<PlanTimeMode>[]>(() => [
  {value: 'now', label: t('planTimeLeaveNow')},
  {value: 'leave', label: t('planTimeLeaveAt')},
  {value: 'arrive', label: t('planTimeArriveBy')},
  {value: 'last', label: t('planTimeLast')},
])
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

const {share} = useShare()

const chips = computed<ChipAction[]>(() => [
  {
    label: t(isFavorite.value ? 'saved' : 'save'),
    icon: isFavorite.value ? FavoriteIcon : FavoriteBorderIcon,
    selected: isFavorite.value,
    tint: isFavorite.value ? 'var(--md-rose)' : undefined,
    disabled: !isOnline.value,
    kbd: 'fav',
    onClick: toggleFavorite,
  },
  {label: t('share'), icon: ShareIcon, kbd: 'share', onClick: () => void share()},
  ...(isOnline.value && isFavorite.value ? [{label: t('renameFavorite'), icon: EditIcon, kbd: 'rename', onClick: startRename}] : []),
])
</script>
<template>
  <div ref="view" class="plan-view flex flex-col">
    <SheetTopBar :title="favoriteLabel ?? (hasValidDest ? destName : null)" />

    <ViewErrorState v-if="!hasValidCoords" />

    <template v-else>
      <header class="header" data-kbd-section="rename" data-kbd-axis="x">
        <span class="tile"><MapIcon class="m-icon" /></span>
        <div class="flex-1 min-w-0">
          <p class="text-label-medium text-tertiary">{{ t('planTitle') }}</p>
          <input
            v-if="isRenaming"
            v-model="renameValue"
            v-focus
            class="rename-field text-title-large"
            :placeholder="t('renameFavoritePlaceholder')"
            data-kbd-item="rename-input"
            @keydown.enter.prevent="confirmRename"
          />
          <h1 v-else class="text-title-large font-bold truncate" :title="favoriteLabel ?? (hasValidDest ? destName : t('planTitleGeneric'))">
            {{ favoriteLabel ?? (hasValidDest ? destName : t('planTitleGeneric')) }}
          </h1>
        </div>
        <template v-if="isRenaming">
          <button
            type="button"
            class="icon-btn"
            data-kbd-item="rename-confirm"
            :aria-label="t('renameFavorite')"
            :title="t('renameFavorite')"
            @click="confirmRename"
          >
            <CheckIcon class="m-icon" />
          </button>
          <button type="button" class="icon-btn" data-kbd-item="rename-cancel" :aria-label="t('cancel')" :title="t('cancel')" @click="cancelRename">
            <CloseIcon class="m-icon" />
          </button>
        </template>
      </header>
      <ActionChips v-if="!isRenaming" :actions="chips" class="pb-2" />

      <section v-if="selectedPlan && !isCalculating" class="summary">
        <div v-if="timeMode === 'now'" class="stat">
          <span class="stat-value">{{ formatMinutes(totalTripMinutes) }}</span>
          <span class="stat-label">{{ t('planTripTotal') }}</span>
        </div>
        <div v-if="firstRide(selectedPlan)" class="stat">
          <span class="stat-value" :class="{'is-live': firstRide(selectedPlan)!.live}">
            <span v-if="firstRide(selectedPlan)!.live" class="live-dot"></span>{{ fromNow(firstRide(selectedPlan)!) }}
          </span>
          <span class="stat-label">{{ t(timeMode === 'last' ? 'planLastDeparture' : 'planNextDepartures') }}</span>
        </div>
        <div v-if="selectedPlan.transfers > 0" class="stat">
          <span class="stat-value">{{ selectedPlan.transfers }}</span>
          <span class="stat-label">{{ selectedPlan.transfers === 1 ? t('planChange') : t('planChanges') }}</span>
        </div>
      </section>

      <section ref="legsCard" class="legs-card">
        <div data-kbd-section="from" data-kbd-entry="2">
          <TimelineRow
            v-if="activeSearchField !== 'origin'"
            :bottom="selectedPlan ? 'walk' : null"
            dot="station"
            color="var(--md-tertiary)"
            class="endpoint state squish"
            role="button"
            tabindex="0"
            data-kbd-item="from"
            @click="openSearchField('origin')"
            @keydown.enter.prevent="openSearchField('origin')"
          >
            <span class="kicker">{{ t('planFrom') }}</span>
            <span ref="originLabelEl" class="text-body-large truncate">{{ originLabel }}</span>
            <template #trailing><EditIcon class="m-icon size-4 text-outline self-center m-2" /></template>
          </TimelineRow>
          <div v-else class="place-search" data-kbd-section="from-results">
            <label class="search-field">
              <SearchIcon class="m-icon text-on-surface-variant" />
              <input
                v-model="searchQuery"
                v-focus
                type="text"
                :placeholder="t('planSearchPlaceholder')"
                data-kbd-item="from-input"
                @keydown.enter="onSearchEnter"
              />
              <button type="button" class="icon-btn" :aria-label="t('cancel')" @click="activeSearchField = null"><CloseIcon class="m-icon" /></button>
            </label>
            <button v-if="hasLocationPermission" type="button" class="result squish" data-kbd-item="from-current" @click="useCurrentLocation">
              <MyLocationIcon class="m-icon size-5 text-primary" />{{ t('planOriginCurrentLocation') }}
            </button>
            <button v-for="res in picks" :key="res.id" type="button" class="result squish" :data-kbd-item="`from-pick-${res.id}`" @click="selectOrigin(res)">
              <ScheduleIcon v-if="res.recent" class="m-icon size-5 text-on-surface-variant" />
              <PlaceIcon v-else class="m-icon size-5 text-tertiary" />{{ res.label }}
            </button>
            <p v-if="isSearching" class="searching"><span class="spinner"></span>{{ t('planSearching') }}</p>
            <template v-else>
              <button
                v-for="res in searchResults"
                :key="res.id"
                type="button"
                class="result squish"
                :data-kbd-item="`from-res-${res.id}`"
                @click="selectOrigin(res)"
              >
                <PlaceIcon class="m-icon size-5 text-tertiary" />{{ res.label }}
              </button>
            </template>
          </div>
        </div>

        <template v-if="selectedPlan">
          <TimelineRow v-if="!rideLegs(selectedPlan).length" top="walk" bottom="walk" dot="none" color="var(--md-outline)">
            <span class="kicker">{{ t('planWalk') }}</span>
            <span class="text-body-medium">{{ formatMinutes(journeyMinutes(selectedPlan)) }}</span>
          </TimelineRow>
          <div v-for="(ld, legIdx) in selectedPlanLegsData" :key="legIdx" data-kbd-section="legs">
            <TimelineRow top="walk" :bottom="ld.color" dot="station" :color="ld.color">
              <span class="kicker leg-lines" :style="{color: ld.color}">
                <span class="font-bold">{{ t('planBoarding') }}&nbsp;</span>
                <template v-for="(routeId, rIdx) in expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? ld.lines : ld.lines.slice(0, 4)" :key="routeId">
                  <RouterLink
                    class="font-black"
                    :data-kbd-item="`leg-${legIdx}-route-${routeId}`"
                    :to="linePath(routeId, ld.trips.get(routeId)?.endsWith('_1') ? '1' : '0')"
                    @click.stop
                    >{{ line(routeId)?.short_name }}</RouterLink
                  ><span v-if="rIdx < (expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? ld.lines.length : Math.min(ld.lines.length, 4)) - 1"
                    >&nbsp;/&nbsp;</span
                  >
                </template>
                <button
                  v-if="ld.lines.length > 4"
                  type="button"
                  class="more"
                  :data-kbd-item="`leg-${legIdx}-more`"
                  @click.stop="toggleLegExpansion('detail-' + selectedPlan.key + '-' + legIdx)"
                >
                  {{ expandedLegs['detail-' + selectedPlan.key + '-' + legIdx] ? '«' : '+' + (ld.lines.length - 4) }}
                </button>
                <span v-if="Math.max(0, Math.round(ld.leg.seconds / 60)) > 0" class="text-on-surface-variant">
                  &nbsp;· {{ Math.max(0, Math.round(ld.leg.seconds / 60)) }}&nbsp;min&nbsp;{{ t('planRideTime') }}
                </span>
              </span>
              <RouterLink class="stop-link is-bold" :data-kbd-item="`leg-${legIdx}-start`" :to="stopLocation(ld.leg.from)">{{
                stopName(ld.leg.from)
              }}</RouterLink>
            </TimelineRow>
            <TimelineRow v-for="stop in ld.intermediates" :key="stop.stop_id" :top="ld.color" :bottom="ld.color" dot="stop" :color="ld.color" compact>
              <RouterLink class="stop-link" :data-kbd-item="`leg-${legIdx}-mid-${stop.stop_id}`" :to="stopLocation(stop.stop_id)">{{
                stop.stop_name
              }}</RouterLink>
            </TimelineRow>
            <TimelineRow :top="ld.color" bottom="walk" dot="station" :color="ld.color">
              <span class="kicker font-bold" :style="{color: ld.color}">{{ t('planAlighting') }}</span>
              <RouterLink class="stop-link is-bold" :data-kbd-item="`leg-${legIdx}-end`" :to="stopLocation(ld.leg.to)">{{ stopName(ld.leg.to) }}</RouterLink>
            </TimelineRow>
            <TimelineRow v-if="legIdx < selectedPlanLegsData.length - 1" top="walk" bottom="walk" dot="none" :color="ld.color" compact>
              <div class="transfer">
                <DirectionsWalkIcon class="m-icon size-[18px] text-amber" />
                <span class="text-label-large font-bold">{{ t('planTransfer') }}</span>
                <span class="text-body-small text-on-surface-variant">
                  {{ ld.transferMeters > 0 ? `${Math.round(ld.transferMeters)} m ${t('planTransferWalk')}` : t('planTransferSameStop') }}
                </span>
              </div>
            </TimelineRow>
          </div>
        </template>

        <div data-kbd-section="to" :data-kbd-entry="hasValidDest ? undefined : 1">
          <TimelineRow
            v-if="activeSearchField !== 'destination'"
            :top="selectedPlan ? 'walk' : null"
            dot="station"
            color="var(--md-primary)"
            class="endpoint state squish"
            :class="{'is-disabled': !isOnline}"
            role="button"
            tabindex="0"
            data-kbd-item="to"
            @click="openSearchField('destination')"
            @keydown.enter.prevent="openSearchField('destination')"
          >
            <span class="kicker">{{ t('planTo') }}</span>
            <span ref="destLabelEl" class="text-body-large truncate" :title="hasValidDest ? destName : '—'">{{ hasValidDest ? destName : '—' }}</span>
            <template #trailing><EditIcon class="m-icon size-4 text-outline self-center m-2" /></template>
          </TimelineRow>
          <div v-else class="place-search" data-kbd-section="to-results">
            <label class="search-field">
              <SearchIcon class="m-icon text-on-surface-variant" />
              <input
                v-model="searchQuery"
                v-focus
                type="text"
                :placeholder="t('planDestSearchPlaceholder')"
                :disabled="!isOnline"
                data-kbd-item="to-input"
                @keydown.enter="onSearchEnter"
              />
              <button type="button" class="icon-btn" :aria-label="t('cancel')" @click="activeSearchField = null"><CloseIcon class="m-icon" /></button>
            </label>
            <template v-if="isOnline">
              <button
                v-if="hasLocationPermission && userLocation"
                type="button"
                class="result squish"
                data-kbd-item="to-current"
                @click="useCurrentLocationAsDestination"
              >
                <MyLocationIcon class="m-icon size-5 text-primary" />{{ t('planOriginCurrentLocation') }}
              </button>
              <button
                v-for="res in picks"
                :key="res.id"
                type="button"
                class="result squish"
                :data-kbd-item="`to-pick-${res.id}`"
                @click="selectDestination(res)"
              >
                <ScheduleIcon v-if="res.recent" class="m-icon size-5 text-on-surface-variant" />
                <PlaceIcon v-else class="m-icon size-5 text-tertiary" />{{ res.label }}
              </button>
              <p v-if="isSearching" class="searching"><span class="spinner"></span>{{ t('planSearching') }}</p>
              <template v-else>
                <button
                  v-for="res in searchResults"
                  :key="res.id"
                  type="button"
                  class="result squish"
                  :data-kbd-item="`to-res-${res.id}`"
                  @click="selectDestination(res)"
                >
                  <PlaceIcon class="m-icon size-5 text-tertiary" />{{ res.label }}
                </button>
                <button
                  v-if="!searchResults.length && !picks.length && searchQuery.trim().length >= 3"
                  type="button"
                  class="result squish"
                  data-kbd-item="to-drop-pin"
                  @click="dropPinAsDestination"
                >
                  <PushPinIcon class="m-icon size-5 text-on-surface-variant" />{{ t('dropPin') }}
                </button>
              </template>
            </template>
          </div>
        </div>
      </section>

      <div class="routes-label">
        <span class="size-2 rounded-full bg-tertiary"></span>
        <h2 class="text-title-small text-on-surface-variant">{{ t('planRoutesLabel') }}</h2>
      </div>

      <div class="time-filter" role="group" :aria-label="t('planTimeFilterLabel')" data-kbd-section="time" data-kbd-axis="x">
        <div class="flex items-center gap-2">
          <SelectField v-model="timeMode" class="flex-1" :label="t('planTimeFilterLabel')" hide-label :options="timeModes" kbd="time-mode" />
          <button
            v-if="timeMode === 'leave' || timeMode === 'arrive'"
            type="button"
            class="btn btn-tonal"
            data-kbd-item="time-search"
            :title="t('planTimeSearchAria')"
            :aria-label="t('planTimeSearchAria')"
            :disabled="isCalculating || !timeValue || (!hasLocationPermission && !customOrigin)"
            @click="refreshRoutes"
          >
            <SearchIcon class="m-icon" />{{ t('planTimeSearch') }}
          </button>
          <button
            v-if="hasValidCoords && (hasLocationPermission || customOrigin)"
            ref="swapButton"
            type="button"
            class="tonal-icon squish"
            data-kbd-item="swap"
            :title="t('planSwap')"
            :aria-label="t('planSwap')"
            :disabled="isCalculating"
            @click="swapOriginDestination"
          >
            <SwapVertIcon class="m-icon" />
          </button>
          <button
            type="button"
            class="tonal-icon squish"
            data-kbd-item="refresh"
            :title="t('planRefresh')"
            :aria-label="t('planRefresh')"
            :disabled="isCalculating || (!hasLocationPermission && !customOrigin)"
            @click="refreshRoutes"
          >
            <span v-if="isCalculating" class="spinner"></span>
            <RefreshIcon v-else class="m-icon" />
          </button>
        </div>
        <PlanTimeFields v-if="(timeMode === 'leave' || timeMode === 'arrive') && timeValue" v-model="timeValue" :now="now" />
      </div>

      <LoadingIndicator v-if="isGpsResolving" :text="t('planLocating')" />
      <LoadingIndicator v-else-if="isCalculating" :text="t('planCalculating')" />

      <div v-else-if="options.length" class="flex flex-col" data-kbd-section="results" data-kbd-entry="1">
        <div
          v-for="(plan, index) in options"
          :key="plan.key"
          class="plan-card squish"
          :class="{'is-selected': selectedPlanIndex === index, cascade: index < 12}"
          :style="{'--i': index}"
          role="button"
          tabindex="0"
          :data-kbd-item="`plan-${plan.key}`"
          :data-kbd-active="selectedPlanIndex === index"
          @click="selectPlanAt(index)"
          @keydown.enter.prevent="onResultEnter($event, index)"
        >
          <div class="flex items-center gap-1">
            <div class="flex-1 min-w-0 flex flex-wrap items-center gap-1">
              <span v-if="!rideLegs(plan).length" class="line-chip is-walk">{{ t('planWalk') }}</span>
              <template v-for="(leg, lIdx) in rideLegs(plan)" :key="lIdx">
                <span
                  class="line-chip"
                  :style="{backgroundColor: lineColor(leg.lines?.[0], linesApart(plan))}"
                  :title="(leg.lines ?? []).map((id) => line(id)?.short_name).join(' / ')"
                >
                  {{
                    (expandedLegs['list-' + plan.key + '-' + lIdx] ? (leg.lines ?? []) : (leg.lines ?? []).slice(0, 3))
                      .map((id) => line(id)?.short_name)
                      .join('/')
                  }}
                </span>
                <button v-if="(leg.lines ?? []).length > 3" type="button" class="more" @click.stop="toggleLegExpansion('list-' + plan.key + '-' + lIdx)">
                  {{ expandedLegs['list-' + plan.key + '-' + lIdx] ? '«' : '+' + ((leg.lines ?? []).length - 3) }}
                </button>
                <KeyboardArrowRightIcon v-if="lIdx < rideLegs(plan).length - 1" class="m-icon size-4 text-outline" />
              </template>
            </div>
            <div v-if="plan.runs[0]" class="flex flex-col items-end">
              <span v-if="firstRide(plan)" class="departure" :class="{'is-live': firstRide(plan)!.live}">
                <span v-if="firstRide(plan)!.live" class="live-dot"></span>{{ departureLabel(firstRide(plan)!) }}
              </span>
              <span class="text-body-small text-on-surface-variant">{{ arrivalLabel(plan) }}</span>
            </div>
            <KeyboardArrowRightIcon class="m-icon size-5 text-outline" />
          </div>
          <p class="text-body-medium text-on-surface-variant truncate">→ {{ rideLegs(plan).length ? stopName(rideLegs(plan).at(-1)?.to) : destName }}</p>
          <div class="flex flex-wrap items-center gap-x-1.5 gap-y-1">
            <span class="stat-chip"><ScheduleIcon class="m-icon size-3" />{{ formatMinutes(Math.round(journeyMinutes(plan))) }}</span>
            <span v-if="plan.transfers > 0" class="stat-chip">
              <SwapVertIcon class="m-icon size-3" />{{ plan.transfers }} {{ plan.transfers === 1 ? t('planChange') : t('planChanges') }}
            </span>
            <span v-if="firstRide(plan)?.live" class="stat-chip is-live"><span class="live-dot"></span>{{ t('live') }}</span>
            <span v-if="laterRides(plan).length" class="text-label-medium text-on-surface-variant">
              {{ t(timeMode === 'last' ? 'planEarlier' : 'planThen') }} {{ laterRides(plan).map(fromNow).join('  ') }}
            </span>
          </div>
        </div>
      </div>

      <div v-else class="empty">
        <DirectionsBusIcon class="m-icon size-10 text-outline-variant" />
        <p class="text-body-medium text-on-surface-variant text-center">{{ t('planNoResults') }}</p>
      </div>
      <div class="h-8"></div>
    </template>
  </div>
</template>

<style scoped>
.header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 4px 8px 16px;
}

.tile {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  background: var(--md-tertiary);
  color: var(--md-on-tertiary);
}

.rename-field {
  width: 100%;
  padding: 4px 0;
  border: 0;
  border-bottom: 2px solid var(--md-primary);
  outline: 0;
  background: transparent;
  color: var(--md-on-surface);
}

.summary {
  display: flex;
  margin: 4px 12px;
  padding: 12px 0;
  border-radius: 12px;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
}

.stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stat + .stat {
  border-left: 1px solid var(--md-outline-variant);
}

.stat-value {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 22px;
  line-height: 28px;
  font-weight: 800;
}

.stat-value.is-live {
  color: var(--md-live);
}

.stat-label {
  font-size: 11px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  opacity: 0.75;
}

.live-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--md-live);
  animation: live-pulse 900ms ease-in-out infinite alternate;
}

@keyframes live-pulse {
  to {
    opacity: 0.35;
  }
}

.legs-card {
  margin: 8px 12px;
  padding: 8px 0;
  border-radius: 12px;
  background: var(--md-surface-container);
}

.endpoint {
  cursor: pointer;
}

.endpoint.is-disabled {
  opacity: 0.5;
  pointer-events: none;
}

.kicker {
  font-size: 11px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  text-transform: uppercase;
  color: var(--md-on-surface-variant);
}

.leg-lines {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
}

.leg-lines a {
  color: inherit;
  text-decoration: none;
}

.more {
  padding: 0 2px;
  border: 0;
  background: transparent;
  color: var(--md-primary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.stop-link {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  line-height: 16px;
  color: var(--md-on-surface-variant);
  text-decoration: none;
}

.stop-link.is-bold {
  font-size: 14px;
  line-height: 20px;
  font-weight: 600;
  color: var(--md-on-surface);
}

.transfer {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--md-amber) 12%, transparent);
}

.place-search {
  padding: 4px 12px;
}

.search-field {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 56px;
  padding: 0 4px 0 12px;
  border-radius: 14px;
  box-shadow: inset 0 0 0 1px var(--md-outline);
}

.search-field:focus-within {
  box-shadow: inset 0 0 0 2px var(--md-primary);
}

.search-field input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0 8px;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--md-on-surface);
  font-size: 16px;
  caret-color: var(--md-primary);
}

.search-field input::placeholder {
  color: var(--md-on-surface-variant);
}

.searching {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  font-size: 12px;
}

.result {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 8px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--md-on-surface);
  font-size: 14px;
  line-height: 20px;
  text-align: left;
  cursor: pointer;
}

@media (hover: hover) {
  .result:hover {
    background: color-mix(in srgb, var(--md-on-surface) 8%, transparent);
  }
}

.routes-label {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px 4px;
}

.time-filter {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 4px 12px;
}

.tonal-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  margin: 4px;
  border: 0;
  border-radius: 20px;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
  cursor: pointer;
}

.tonal-icon:disabled {
  background: color-mix(in srgb, var(--md-on-surface) 12%, transparent);
  color: color-mix(in srgb, var(--md-on-surface) 38%, transparent);
  pointer-events: none;
}

.spinner {
  width: 18px;
  height: 18px;
  border: 2px solid var(--md-primary);
  border-right-color: transparent;
  border-radius: 50%;
  animation: spin 800ms linear infinite;
}

@keyframes spin {
  to {
    rotate: 360deg;
  }
}

.plan-card {
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 4px 12px;
  padding: 12px;
  border-radius: 12px;
  background: var(--md-surface-container);
  color: var(--md-on-surface);
  cursor: pointer;
}

.plan-card.is-selected {
  background: var(--md-secondary-container);
  box-shadow: inset 0 0 0 1.5px var(--md-primary);
}

.line-chip {
  padding: 4px 8px;
  border-radius: 8px;
  color: #fff;
  font-size: 14px;
  line-height: 20px;
  font-weight: 900;
}

.line-chip.is-walk {
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface);
}

.departure {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 14px;
  line-height: 20px;
  font-weight: 700;
}

.departure.is-live {
  color: var(--md-live);
}

.stat-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border-radius: 8px;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
  font-size: 11px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
}

.stat-chip.is-live {
  background: var(--md-live-container);
  color: var(--md-on-live-container);
  font-weight: 700;
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 32px;
}
</style>
