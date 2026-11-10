import {onUnmounted, watch, watchEffect, type Ref} from 'vue'
import type {DisplayShape, HighlightedStop} from '@/stores/map.ts'
import {displayShapeOf, useLinesTogether} from '@/composables/useGroupShapes.ts'
import type {Shape} from '@/types/tranzy.ts'
import {network} from '@/utils/network.ts'
import {type GroupLine, stretchOf, tripOf} from '@/utils/lineGroups.ts'
import {followedPart} from '@/utils/lineStretch.ts'

export function useStretchMap(line: Ref<GroupLine | null>, others: Ref<GroupLine[]>, nearest: Ref<number | undefined>, pick: (stopId: number) => void) {
  const mapStore = useLinesTogether()
  mapStore.stopPick = pick
  mapStore.setVehiclesToDisplay([])

  let fitted = ''
  watch(
    [line, others, network],
    async ([edited, rest, index]) => {
      if (!index || !edited) return
      const drawn = [edited, ...rest].flatMap((l) => {
        const meta = displayShapeOf(index, l)
        return meta ? [{line: l, meta}] : []
      })
      const loaded = await mapStore.requestShapes(drawn.map((d) => d.meta))
      if (line.value !== edited) return
      const beside: Array<[DisplayShape, Shape[]]> = []
      const own: Array<[DisplayShape, Shape[]]> = []
      drawn.forEach(({line: l, meta}, i) => {
        const shape = loaded[i]?.[1] ?? []
        if (!shape.length) return
        const part = followedPart(index, l, shape)
        if (l !== edited) {
          beside.push([{...meta, faint: true, skipFit: true}, part ?? shape])
          return
        }
        if (part) own.push([{...meta, faint: true}, shape])
        own.push([meta, part ?? shape])
      })
      // Taps on stops leave the view alone; a new line or direction fits it.
      mapStore.keepView = fitted === tripOf(edited)
      fitted = tripOf(edited)
      mapStore.setLoadedShapes([...beside, ...own])
    },
    {immediate: true},
  )

  watchEffect(() => {
    const index = network.value
    if (!index || !line.value) return mapStore.setHighlightedStops([])
    const {stops, from} = stretchOf(index, line.value)
    const marks = new Map<number, HighlightedStop['color']>()
    stops.forEach((id, i) => marks.set(id, i === from ? 'green' : id === nearest.value ? 'purple' : (marks.get(id) ?? 'gray')))
    mapStore.setHighlightedStops([...marks].map(([id, color]) => ({stopId: String(id), color})))
  })

  onUnmounted(() => {
    mapStore.stopPick = null
    mapStore.keepView = false
    mapStore.setHighlightedStops([])
  })
}
