import {computed, onUnmounted, shallowRef, watch, type Ref} from 'vue'
import {type DisplayShape, useMapStore} from '@/stores/map.ts'
import type {Shape} from '@/types/tranzy.ts'
import {network, type NetworkIndex} from '@/utils/network.ts'
import {type GroupLine, groupKey, tripOf} from '@/utils/lineGroups.ts'
import {followedPart, type Span, stretchSpan} from '@/utils/lineStretch.ts'

export function displayShapeOf(index: NetworkIndex | null, line: GroupLine): DisplayShape | undefined {
  const route = index?.routeById.get(line.routeId)
  return (
    route && {
      trip_id: tripOf(line),
      route_short_name: route.route_short_name,
      route_long_name: route.route_long_name,
      route_color: route.route_color,
      route_type: route.route_type,
    }
  )
}

let pagesUsing = 0

// A page that takes over instantly is set up before the one it replaces lets go.
export function useLinesTogether() {
  const mapStore = useMapStore()
  pagesUsing++
  mapStore.linesApart = false
  mapStore.lineEnds = true
  onUnmounted(() => {
    if (--pagesUsing) return
    mapStore.linesApart = true
    mapStore.lineEnds = false
    mapStore.setShapesToDisplay([])
  })
  return mapStore
}

export function useGroupShapes(lines: Ref<GroupLine[]>) {
  const mapStore = useLinesTogether()
  const shapes = shallowRef(new Map<string, Shape[]>())

  const spans = computed(() => {
    const index = network.value
    const out = new Map<number, Span>()
    for (const line of lines.value) {
      const shape = shapes.value.get(tripOf(line))
      const span = index && shape ? stretchSpan(index, line, shape) : null
      if (span) out.set(line.routeId, span)
    }
    return out
  })

  let drawnKey = ''
  watch(
    [lines, network],
    async ([drawn, index]) => {
      const key = groupKey(drawn)
      if (!index || key === drawnKey) return
      const first = !drawnKey
      drawnKey = key
      const metas = drawn.flatMap((line) => displayShapeOf(index, line) ?? [])
      const loaded = await mapStore.requestShapes(metas)
      if (key !== drawnKey) return
      shapes.value = new Map(loaded.filter(([, points]) => points.length).map(([meta, points]) => [meta.trip_id, points]))
      const rest: Array<[DisplayShape, Shape[]]> = []
      const followed: Array<[DisplayShape, Shape[]]> = []
      for (const line of drawn) {
        const meta = displayShapeOf(index, line)
        const shape = shapes.value.get(tripOf(line))
        if (!meta || !shape) continue
        const part = followedPart(index, line, shape)
        if (part) rest.push([{...meta, faint: true, skipFit: true}, shape])
        followed.push([meta, part ?? shape])
      }
      mapStore.setLoadedShapes([...rest, ...followed])
      if (first) mapStore.zoomOut = true
    },
    {immediate: true},
  )

  return {spans}
}
