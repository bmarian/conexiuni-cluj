import {computed, onUnmounted, ref, watch, watchEffect, type Ref} from 'vue'
import {type DisplayShape, useMapStore} from '@/stores/map.ts'
import {useLive} from '@/composables/useLive.ts'
import {network} from '@/utils/network.ts'
import type {DisplayVehicle} from '@/utils/mapIcons.ts'
import {divergingStops, type GroupLine, tripOf} from '@/utils/lineGroups.ts'

export function useLineGroup(lines: Ref<GroupLine[]>) {
  const mapStore = useMapStore()
  mapStore.linesApart = false
  mapStore.lineEnds = true
  const hidden = ref(new Set<number>())
  const shown = computed(() => lines.value.filter((l) => !hidden.value.has(l.routeId)))

  const toggleHidden = (routeId: number) => {
    const next = new Set(hidden.value)
    if (!next.delete(routeId)) next.add(routeId)
    hidden.value = next
  }

  const tripIds = computed(() => shown.value.map(tripOf))
  const {vehiclesByTrip} = useLive(ref([]), tripIds)

  const displayShapes = computed<DisplayShape[]>(() =>
    shown.value.flatMap((line) => {
      const route = network.value?.routeById.get(line.routeId)
      return route
        ? [
            {
              trip_id: tripOf(line),
              route_short_name: route.route_short_name,
              route_long_name: route.route_long_name,
              route_color: route.route_color,
              route_type: route.route_type,
            },
          ]
        : []
    }),
  )

  let drawnKey = ''
  watch(
    displayShapes,
    async (shapes) => {
      const key = shapes.map((s) => s.trip_id).join(',')
      if (key === drawnKey) return
      const first = !drawnKey
      drawnKey = key
      const loaded = await mapStore.requestShapes(shapes)
      if (key !== drawnKey) return
      mapStore.setLoadedShapes(loaded.filter(([, points]) => points.length))
      if (first) mapStore.zoomOut = true
    },
    {immediate: true},
  )

  watchEffect(() => {
    const vehicles: DisplayVehicle[] = []
    for (const shape of displayShapes.value) {
      for (const v of vehiclesByTrip.value.get(shape.trip_id) ?? []) {
        vehicles.push({...v, route_short_name: shape.route_short_name, route_color: shape.route_color, faded: v.at_stand})
      }
    }
    mapStore.setVehiclesToDisplay(vehicles)
  })

  watchEffect(() => {
    const index = network.value
    mapStore.setHighlightedStops(index ? divergingStops(index, shown.value).map((stop) => ({stopId: String(stop), color: 'gray' as const})) : [])
  })

  onUnmounted(() => {
    mapStore.linesApart = true
    mapStore.lineEnds = false
    mapStore.setHighlightedStops([])
    mapStore.setShapesToDisplay([])
    mapStore.setVehiclesToDisplay([])
  })

  return {hidden, toggleHidden}
}
