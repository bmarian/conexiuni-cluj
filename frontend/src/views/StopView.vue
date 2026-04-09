<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts"
import {storeToRefs} from "pinia"
import {computed, ref, watch} from "vue"
import {useStopInfoApi} from "@/composables/useStopInfoApi.ts"
import StopIcon from "../../public/stop.svg"
import type {Timetable} from "@/types/ctp.ts";
import {
  INCOMING_SUFFIX,
  OUTGOING_SUFFIX,
  type Shape,
  type ShapeInfo,
  type StopTime,
  type Vehicle,
  type VehiclesInStop
} from "@/types/tranzy.ts";
import {getMinutesFromDate, timeStringToMinutes} from "@/utils/time.ts";
import {type DisplayShape, useMapStore} from "@/stores/map.ts";
import {apiRequest, HIGH_ACCURACY_SHELF_LIFE} from "@/utils/request_cache.ts";
import {calculateBearing, haversineMeters} from "@/utils/geo.ts";

const props = defineProps<{
  stopId: string
}>()
const userStore = useUserStore()
const mapStore = useMapStore()
const {centerOnUser, zoomOut} = storeToRefs(mapStore)
const {userTime} = storeToRefs(userStore)
const {stopInfo, fetchStopData} = useStopInfoApi()
const stopName = computed(() => stopInfo.value?.stop_name)
const isLoading = ref(false)
const selectedTripId = ref<string | null>(null)
const shapesComingToTheStopBasedOnVehiclePositions = ref<VehiclesInStop[]>([])
const allFetchedVehicles = ref<Vehicle[]>([])
const CLOSE_TO_STOP_THRESHOLD = 200
const VEHICLE_GRACE_PERIOD = 10 // minutes

function formatGtfsColor(colorString?: string) {
  if (!colorString) return '#3b82f6';
  return colorString.startsWith('#') ? colorString : `#${colorString}`;
}

function getRouteTypeLabel(type: number) {
  switch (type) {
    case 0:
      return '🚋';
    case 2:
      return '🚆';
    case 3:
      return '🚌';
    case 11:
      return '🚎';
    default:
      return '🚌';
  }
}

const hasTimetable = (timetable: Timetable): boolean => {
  const hasWeekday = timetable?.weekdays?.entries?.length
  const hasSaturday = timetable?.saturday?.entries?.length
  const hasSunday = timetable?.sunday?.entries?.length
  return !!(hasWeekday || hasSaturday || hasSunday)
}

const isAvailableToday = (today: number, timetable: Timetable): boolean => {
  const hasWeekday = timetable?.weekdays?.entries?.length
  const hasSaturday = timetable?.saturday?.entries?.length
  const hasSunday = timetable?.sunday?.entries?.length
  return !!(today === 0 ? hasSunday : today === 6 ? hasSaturday : hasWeekday)
}

const busesWithAvailableTimetables = computed(() => {
  return stopInfo.value?.shapes_info.filter((shape: ShapeInfo) => hasTimetable(shape.timetable))
})

const getTimeOffsetToStop = (stopTime: StopTime[], tripId: string): number => {
  const propStopId = Number(props.stopId)

  let timeOffset = 0
  for (let j = 0; j < stopTime.length; j++) {
    const {trip_id, stop_id, offset_arrival_time} = stopTime[j]!
    if (trip_id !== tripId) continue
    if (stop_id === propStopId) break

    timeOffset += Math.ceil(offset_arrival_time / 60)
  }

  return timeOffset
}

const getTripId = (outgoingTripIds: string[], incomingTripIds: string[], routeId: string) => {
  return [...(outgoingTripIds || []), ...(incomingTripIds || [])].find((id) => id.startsWith(routeId))
}

const reverseRouteLongName = (routeLongName: string) => {
  return routeLongName.split(' - ').reverse().join(' - ')
}

const toggleAllRoutesOnMap = () => {
  if (selectedTripId.value === 'ALL') {
    selectedTripId.value = null
    centerOnUser.value = true
  } else {
    selectedTripId.value = 'ALL'
    zoomOut.value = true
  }
}

const toggleRouteOnMap = (tripId: string) => {
  if (!tripId) return
  if (selectedTripId.value === tripId) {
    selectedTripId.value = null
    centerOnUser.value = true
  } else {
    selectedTripId.value = tripId
    zoomOut.value = true
  }
}

const getShapesDisplay = (availableShapes: unknown): DisplayShape[] => {
  if (!Array.isArray(availableShapes) || availableShapes.length === 0) return []

  const routeIdsWithAvailableTimetables = availableShapes.map((shape: ShapeInfo) => shape.route_id)
  const {outgoing_trip_ids, incoming_trip_ids} = stopInfo.value

  return [...(outgoing_trip_ids || []), ...(incoming_trip_ids || [])]
    .filter((trip_id) => routeIdsWithAvailableTimetables.some((route_id: number) => trip_id.startsWith(route_id.toString())))
    .reduce((acc: DisplayShape[], trip_id: string) => {
      const routeId = Number(trip_id.replace(OUTGOING_SUFFIX, '').replace(INCOMING_SUFFIX, ''))
      const shape = availableShapes.find((shape: ShapeInfo) => shape.route_id === routeId)
      if (shape) {
        acc.push({
          trip_id,
          route_short_name: shape.route_short_name,
          route_long_name: shape.route_long_name,
          route_color: shape.route_color,
          route_type: shape.route_type,
        })
      }
      return acc
    }, [])
}

const getVehiclesOnRoute = async (tripId: string, routeShortName: string, trip: Shape[]): Promise<Vehicle[]> => {
  if (!trip || !Array.isArray(trip) || trip.length === 0) return []
  const vehiclesOnRoute = await apiRequest(`vehicles?trip_id=${tripId}`, HIGH_ACCURACY_SHELF_LIFE) as Vehicle[] || []
  const fistStop = trip[0]!
  const lastStop = trip[trip.length - 1]!

  const returnVehicles = []
  for (let i = 0; i < vehiclesOnRoute.length; i++) {
    const vehicle = vehiclesOnRoute[i]!
    const isCloseToFirstStop = haversineMeters(vehicle.latitude, vehicle.longitude, fistStop.shape_pt_lat, fistStop.shape_pt_lon) <= CLOSE_TO_STOP_THRESHOLD
    const isCloseToLastStop = haversineMeters(vehicle.latitude, vehicle.longitude, lastStop.shape_pt_lat, lastStop.shape_pt_lon) <= CLOSE_TO_STOP_THRESHOLD

    if ((isCloseToFirstStop || isCloseToLastStop) && vehicle.speed < 1) continue

    const vehicleTimestamp = new Date(vehicle.timestamp).getTime()
    if (isNaN(vehicleTimestamp)) continue

    const ut = userTime.value?.getTime() || new Date().getTime()
    const isReadingFresh = ut - vehicleTimestamp <= VEHICLE_GRACE_PERIOD * 60 * 1000
    if (!isReadingFresh) continue

    let heading = 0
    const closestNode = getClosestNodeToPoint({lat: vehicle.latitude, lon: vehicle.longitude}, trip)
    if (closestNode) {
      const closestIdx = trip.findIndex(t => t.shape_pt_sequence === closestNode.shape_pt_sequence)
      const lookAheadIdx = Math.min(closestIdx + 3, trip.length - 1)
      const targetPt = trip[lookAheadIdx]!
      heading = calculateBearing(vehicle.latitude, vehicle.longitude, targetPt.shape_pt_lat, targetPt.shape_pt_lon)
    }

    returnVehicles.push({...vehicle, route_short_name: routeShortName, heading})
  }

  return returnVehicles
}

const getClosestNodeToPoint = ({lat, lon}: {
  lat: number,
  lon: number
}, trip: Shape[]): Shape | undefined => {
  let closestDistance = Infinity
  let closestTrip: Shape | undefined

  for (let i = 0; i < trip.length; i++) {
    const {shape_pt_lat, shape_pt_lon} = trip[i]!
    const distance = haversineMeters(shape_pt_lat, shape_pt_lon, lat, lon)
    if (distance < closestDistance) {
      closestDistance = distance
      closestTrip = trip[i]
    }
  }

  return closestTrip
}

const getClosestVehicleBeforeStop = (vehicles: Vehicle[], closestNodeToStop: Shape, trip: Shape[]): {
  closestVehicle: Vehicle | undefined,
  closestNode: Shape | undefined
} => {
  let closestDistance = Infinity
  let bestVehicle: Vehicle | undefined
  let bestNode: Shape | undefined

  for (let i = 0; i < vehicles.length; i++) {
    const vehicle = vehicles[i]!

    const currentNode = getClosestNodeToPoint({lat: vehicle.latitude, lon: vehicle.longitude}, trip)
    if (!currentNode || currentNode.shape_pt_sequence > closestNodeToStop.shape_pt_sequence) continue

    const distanceToStop = haversineMeters(
      currentNode.shape_pt_lat, currentNode.shape_pt_lon,
      closestNodeToStop.shape_pt_lat, closestNodeToStop.shape_pt_lon
    )

    if (distanceToStop < closestDistance) {
      closestDistance = distanceToStop
      bestVehicle = vehicle
      bestNode = currentNode
    }
  }

  return {closestVehicle: bestVehicle, closestNode: bestNode}
}

const computeETA = (stopShape: Shape, busShape: Shape, vehicle: Vehicle, trip: Shape[]): number => {
  const busIndex = trip.findIndex(t => t.shape_pt_sequence === busShape.shape_pt_sequence)
  const stopIndex = trip.findIndex(t => t.shape_pt_sequence === stopShape.shape_pt_sequence)

  if (busIndex === -1 || stopIndex === -1) return -1
  if (busIndex > stopIndex) return -2

  let totalDistance = 0

  if (busIndex === stopIndex) {
    totalDistance = haversineMeters(vehicle.latitude, vehicle.longitude, stopShape.shape_pt_lat, stopShape.shape_pt_lon)
  } else {
    for (let i = busIndex; i < stopIndex; i++) {
      const cur = trip[i]
      const next = trip[i + 1]
      if (cur && next) {
        totalDistance += haversineMeters(
          cur.shape_pt_lat, cur.shape_pt_lon,
          next.shape_pt_lat, next.shape_pt_lon
        )
      }
    }
  }

  const speedKmh = Math.max(vehicle.speed, 12)
  const time = ((totalDistance / 1000) / speedKmh) * 60

  return Math.ceil(time)
}

const shapesComingToTheStopBasedOnTimetable = computed(() => {
  if (!stopInfo.value) return []

  const today = userTime.value?.getDay() || new Date().getDay()
  const currentTimeInMinutes = getMinutesFromDate(userTime.value || new Date())
  const {outgoing_trip_ids, incoming_trip_ids, shapes_info} = stopInfo.value

  const results = []
  for (let i = 0; i < shapes_info.length; i++) {
    const {
      route_short_name,
      route_type,
      route_color,
      route_id,
      stop_time,
      timetable
    } = shapes_info[i]
    if (!hasTimetable(timetable) || !isAvailableToday(today, timetable)) continue

    const tripId = getTripId(outgoing_trip_ids, incoming_trip_ids, route_id)
    if (!tripId) continue

    const timeOffsetToStop = getTimeOffsetToStop(stop_time, tripId)
    const isOutgoing = tripId.endsWith(OUTGOING_SUFFIX)

    const todayTimetable = (today === 0 ? timetable.sunday : today === 6 ? timetable.saturday : timetable.weekdays)
    const todayTimes = todayTimetable
      .entries
      .map((entry: { departure_in: string; departure_out: string }) => {
        const timetableTime = timeStringToMinutes(isOutgoing ? entry.departure_in : entry.departure_out)
        return timetableTime === null ? timetableTime : timetableTime + timeOffsetToStop
      }).reduce((acc: number[], curr: number) => {
        const minutesLeft = ((curr - currentTimeInMinutes) + 1440) % 1440
        if (minutesLeft <= 30) acc.push(minutesLeft)
        return acc
      }, [])
    if (!todayTimes.length) continue

    results.push({
      minutes_left: Math.min(...todayTimes),
      route_short_name,
      route_type,
      route_color,
      trip_id: tripId,
      route_id,
      route_long_name: isOutgoing
        ? `${timetable.route_long_name}`
        : `${reverseRouteLongName(timetable.route_long_name)}`,
      static_time_approximation: true,
    } as VehiclesInStop)
  }
  return results.sort((a, b) => a.minutes_left - b.minutes_left)
})

watch(() => props.stopId, async (newValue) => {
  isLoading.value = true
  mapStore.setVehiclesToDisplay([])
  await mapStore.setShapesToDisplay([])
  shapesComingToTheStopBasedOnVehiclePositions.value = []
  selectedTripId.value = null
  await fetchStopData(newValue)
  isLoading.value = false
}, {immediate: true})

watch([busesWithAvailableTimetables, selectedTripId], ([newVal, selectedId]) => {
  if (!selectedId || !stopInfo.value || !Array.isArray(newVal) || newVal.length === 0) {
    mapStore.setShapesToDisplay([])
    return
  }

  const allShapes = getShapesDisplay(newVal)

  if (selectedId === 'ALL') {
    mapStore.setShapesToDisplay(allShapes)
  } else {
    const filteredShapes = allShapes.filter(s => s.trip_id === selectedId)
    mapStore.setShapesToDisplay(filteredShapes)
  }
})

watch(shapesComingToTheStopBasedOnTimetable, async (shapesComingNext) => {
  if (!Array.isArray(shapesComingNext) || shapesComingNext.length === 0) {
    shapesComingToTheStopBasedOnVehiclePositions.value = []
    return
  }

  const displayShapes = getShapesDisplay(busesWithAvailableTimetables.value)
  const displayShapesWithTrip = await mapStore.requestShapes(displayShapes)
  if (!Array.isArray(displayShapesWithTrip) || displayShapesWithTrip.length === 0) {
    shapesComingToTheStopBasedOnVehiclePositions.value = shapesComingNext
    return
  }

  const resultsWithImprovedAccuracy: VehiclesInStop[] = []
  const vehiclesToDisplay: Vehicle[] = []
  for (let i = 0; i < shapesComingNext.length; i++) {
    const shape = shapesComingNext[i]!
    const trip = displayShapesWithTrip.find(([s]) => s.trip_id === shape.trip_id)?.[1] as Shape[]
    const vehiclesOnRoute = await getVehiclesOnRoute(shape.trip_id, shape.route_short_name, trip)
    vehiclesToDisplay.push(...vehiclesOnRoute)

    // If there is no bus on the route, we just use static approximation `static_time_approximation: true`
    if (!Array.isArray(vehiclesOnRoute) || vehiclesOnRoute.length === 0) {
      resultsWithImprovedAccuracy.push(shape)
      continue
    }

    const closestNodeToStop = getClosestNodeToPoint({
      lat: stopInfo.value!.stop_lat,
      lon: stopInfo.value!.stop_lon
    }, trip)
    if (!closestNodeToStop) {
      resultsWithImprovedAccuracy.push(shape)
      continue
    }

    const {
      closestVehicle: closestVehicleBeforeStop,
      closestNode: closestNodeToVehicle
    } = getClosestVehicleBeforeStop(vehiclesOnRoute, closestNodeToStop, trip)
    if (!closestVehicleBeforeStop || !closestNodeToVehicle) {
      resultsWithImprovedAccuracy.push(shape)
      continue
    }

    const vehicleETA = computeETA(closestNodeToStop, closestNodeToVehicle, closestVehicleBeforeStop, trip)
    if (vehicleETA === -1) {
      resultsWithImprovedAccuracy.push(shape)
      continue
    }

    // If the bus already passed the stop in the second round of calculations
    if (vehicleETA === -2) {
      continue
    }

    resultsWithImprovedAccuracy.push({
      ...shape,
      minutes_left: vehicleETA,
      static_time_approximation: false
    })
  }
  shapesComingToTheStopBasedOnVehiclePositions.value = resultsWithImprovedAccuracy.sort((a, b) => a.minutes_left - b.minutes_left)

// Cache all fetched vehicles locally so we can filter them dynamically
  allFetchedVehicles.value = vehiclesToDisplay

  if (selectedTripId.value === 'ALL') {
    mapStore.setVehiclesToDisplay(vehiclesToDisplay)
  } else if (selectedTripId.value) {
    mapStore.setVehiclesToDisplay(vehiclesToDisplay.filter(v => v.trip_id === selectedTripId.value))
  } else {
    mapStore.setVehiclesToDisplay([])
  }
})

// Listen for clicks to immediately update the drawn buses
watch(selectedTripId, (newId) => {
  if (newId === 'ALL') {
    mapStore.setVehiclesToDisplay(allFetchedVehicles.value)
  } else if (newId) {
    mapStore.setVehiclesToDisplay(allFetchedVehicles.value.filter(v => v.trip_id === newId))
  } else {
    mapStore.setVehiclesToDisplay([])
  }
})

</script>

<template>
  <div v-if="isLoading"
       class="stop-view-container bg-white dark:bg-[#0f172a] p-5 h-full flex flex-col gap-10 animate-pulse">

    <header class="flex items-center gap-4">
      <div class="w-12 h-12 shrink-0 rounded-full bg-slate-200 dark:bg-slate-800"></div>
      <div class="h-8 w-48 bg-slate-200 dark:bg-slate-800 rounded-lg"></div>
    </header>

    <section>
      <div class="h-3.5 w-36 bg-slate-200 dark:bg-slate-800 rounded mb-5!"></div>

      <div class="flex flex-col gap-4">
        <div v-for="i in 3" :key="'skeleton-next-'+i"
             class="bg-slate-50 dark:bg-slate-800/40 rounded-2xl p-4 pr-5 flex items-center gap-4 border border-slate-100 dark:border-slate-800/50">
          <div class="shrink-0 w-14 h-12 rounded-xl bg-slate-200 dark:bg-slate-700/50"></div>

          <div class="flex-1 flex flex-col gap-2 justify-center py-1">
            <div class="h-2.5 w-24 bg-slate-200 dark:bg-slate-700/50 rounded"></div>
            <div class="h-4 w-40 bg-slate-200 dark:bg-slate-700/50 rounded"></div>
          </div>

          <div class="shrink-0 w-10 h-8 bg-slate-200 dark:bg-slate-700/50 rounded-lg mr-1"></div>
        </div>
      </div>
    </section>

    <section>
      <div class="h-3.5 w-48 bg-slate-200 dark:bg-slate-800 rounded mb-5!"></div>

      <div class="grid grid-cols-1 xl:grid-cols-2 gap-x-4 gap-y-3">
        <div v-for="i in 6" :key="'skeleton-all-'+i" class="flex items-center gap-3 p-2 -mx-2">
          <div class="shrink-0 w-10 h-8 rounded-md bg-slate-200 dark:bg-slate-800"></div>
          <div class="h-3.5 w-32 bg-slate-200 dark:bg-slate-800 rounded"></div>
        </div>
      </div>
    </section>

  </div>
  <div v-else
       class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 h-full overflow-y-auto flex flex-col gap-10 font-sans shadow-2xl transition-colors duration-300 relative">

    <header class="flex items-center gap-4">
      <div
        class="w-12 h-12 shrink-0 rounded-full bg-emerald-100 dark:bg-emerald-500/20 flex items-center justify-center text-emerald-600 dark:text-emerald-400 shadow-sm border border-emerald-200 dark:border-emerald-500/30 transition-colors">
        <StopIcon class="w-6 h-6"/>
      </div>
      <h1
        class="text-2xl sm:text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white leading-tight transition-colors">
        {{ stopName }}
      </h1>
    </header>

    <section>
      <h2
        class="text-sm font-bold text-slate-500 dark:text-slate-400 uppercase tracking-[0.2em] mb-5 flex items-center gap-3 transition-colors">
        <span
          class="w-2.5 h-2.5 rounded-full bg-emerald-500 animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.5)]"></span>
        Next Departures
      </h2>

      <div class="flex flex-col gap-4">
        <div v-for="shape in shapesComingToTheStopBasedOnVehiclePositions"
             :key="shape.route_short_name"
             @click="toggleRouteOnMap(shape.trip_id)"
             :class="['transition-all rounded-2xl p-4 pr-5 flex items-center gap-4 border shadow-sm dark:shadow-md relative overflow-hidden group cursor-pointer',
                      selectedTripId === shape.trip_id ? 'bg-emerald-50 dark:bg-emerald-500/10 border-emerald-300 dark:border-emerald-500/50 scale-[1.02]' : 'bg-slate-50 hover:bg-slate-100 dark:bg-slate-800/60 dark:hover:bg-slate-800/80 border-slate-200 dark:border-slate-700/50']">

          <div
            class="flex items-center justify-center shrink-0 w-14 h-12 rounded-xl font-black text-lg text-white shadow-sm z-10"
            :style="{ backgroundColor: formatGtfsColor(shape.route_color) }">
            {{ shape.route_short_name }}
          </div>

          <div class="flex-1 min-w-0 flex flex-col justify-center z-10">
            <span
              class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-0.5 flex items-center gap-1.5 transition-colors">
              {{ getRouteTypeLabel(shape.route_type) }} Scheduled
            </span>
            <span
              class="text-base font-bold text-slate-800 dark:text-slate-200 truncate group-hover:text-slate-900 dark:group-hover:text-white transition-colors">
              {{ shape.route_long_name }}
            </span>
          </div>

          <div class="flex flex-col items-end justify-center shrink-0 min-w-16 z-10 mr-3!">
            <div class="flex items-baseline gap-1">
              <span v-if="shape.static_time_approximation"
                    class="text-slate-400 dark:text-slate-500 font-semibold text-xl transition-colors">~</span>
              <span
                class="text-3xl font-black tracking-tighter transition-colors"
                :class="shape.minutes_left <= 5 ? 'text-emerald-600 dark:text-emerald-400 animate-pulse' : 'text-rose-500 dark:text-rose-400'">
                {{ shape.minutes_left === 0 ? 'NOW' : shape.minutes_left }}
              </span>
            </div>
            <span v-if="shape.minutes_left > 0"
                  class="text-[10px] text-slate-500 dark:text-slate-400 font-bold uppercase tracking-widest mt-0.5 transition-colors">
              min
            </span>
          </div>

        </div>
      </div>
    </section>

    <section>
      <div @click="toggleAllRoutesOnMap"
           class="mb-5 border-b border-slate-200 dark:border-slate-700/50 pb-3 transition-colors cursor-pointer group flex items-center justify-between">
        <h2
          class="text-sm font-bold uppercase tracking-[0.2em] m-0 border-none pb-0 transition-colors"
          :class="selectedTripId === 'ALL' ? 'text-emerald-600 dark:text-emerald-400' : 'text-slate-500 dark:text-slate-400 group-hover:text-slate-800 dark:group-hover:text-slate-200'">
          All Routes at this Stop
        </h2>
      </div>

      <div class="grid grid-cols-1 xl:grid-cols-2 gap-x-4 gap-y-3">
        <div v-for="shape in busesWithAvailableTimetables" :key="shape.timetable.route_short_name"
             @click="toggleRouteOnMap(getTripId(stopInfo?.outgoing_trip_ids || [], stopInfo?.incoming_trip_ids || [], shape.route_id.toString()) || '')"
             :class="['flex items-center gap-3 group cursor-pointer p-2 -mx-2 rounded-lg transition-colors border',
                      selectedTripId === getTripId(stopInfo?.outgoing_trip_ids || [], stopInfo?.incoming_trip_ids || [], shape.route_id.toString()) ? 'bg-emerald-50 dark:bg-emerald-500/10 border-emerald-300 dark:border-emerald-500/50' : 'hover:bg-slate-50 dark:hover:bg-slate-800/40 border-transparent hover:border-slate-200 dark:hover:border-slate-700/50']">

          <div
            class="flex items-center justify-center shrink-0 w-10 h-8 rounded-md text-xs font-black text-white shadow-sm opacity-90 group-hover:opacity-100 transition-opacity"
            :style="{ backgroundColor: formatGtfsColor(shape.route_color) }">
            {{ shape.timetable.route_short_name }}
          </div>

          <span
            class="text-sm font-semibold text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
              {{ shape.timetable.route_long_name }}
            </span>
        </div>
      </div>
    </section>

  </div>
</template>

<style scoped>
.stop-view-container {
  padding-left: 2rem;
  padding-right: 2rem;
  padding-top: 0.5rem;
}
</style>
