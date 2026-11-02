<script setup lang="ts">
import {LngLatBounds, type ExpressionSpecification, type GeoJSONSource, type MapLayerMouseEvent} from 'maplibre-gl'
import {ref, watch} from 'vue'
import type {AdminLine} from '@/components/admin/types.ts'
import {ADMIN_MAP_PADDING_PX, useAdminMap} from '@/composables/useAdminMap.ts'
import {decodePolyline} from '@/utils/geo.ts'

const SOURCE = 'admin-lines'
const LAYER = 'admin-lines'
const FOCUSED_WIDTH = 6
const LINE_WIDTH = 3
const FADED_OPACITY = 0.3

const props = defineProps<{lines: AdminLine[]; focused: string | null}>()
const emit = defineEmits<{pick: [name: string]}>()

const container = ref<HTMLElement | null>(null)
const map = useAdminMap(container, (m) => {
  m.on('style.load', addLayer)
  m.on('click', LAYER, (e: MapLayerMouseEvent) => {
    const name = e.features?.[0]?.properties?.name
    if (name) emit('pick', name)
  })
  m.on('mouseenter', LAYER, () => (m.getCanvas().style.cursor = 'pointer'))
  m.on('mouseleave', LAYER, () => (m.getCanvas().style.cursor = ''))
  fit([])
})

function collection(): GeoJSON.FeatureCollection {
  const uncoloured = getComputedStyle(container.value!).getPropertyValue('--md-outline').trim()
  return {
    type: 'FeatureCollection',
    features: props.lines.flatMap((line) =>
      line.shapes.map((shape) => ({
        type: 'Feature' as const,
        properties: {name: line.name, color: line.color || uncoloured},
        geometry: {type: 'LineString' as const, coordinates: decodePolyline(shape).map(([lat, lng]) => [lng, lat])},
      })),
    ),
  }
}

function addLayer() {
  const m = map()
  if (!m || m.getSource(SOURCE)) return
  m.addSource(SOURCE, {type: 'geojson', data: collection()})
  m.addLayer({id: LAYER, type: 'line', source: SOURCE, layout: {'line-cap': 'round', 'line-join': 'round'}, paint: {'line-color': ['get', 'color']}})
  paintFocus()
}

function paintFocus() {
  const m = map()
  if (!m?.getLayer(LAYER)) return
  const focused: ExpressionSpecification = ['==', ['get', 'name'], props.focused ?? '']
  m.setPaintProperty(LAYER, 'line-width', ['case', focused, FOCUSED_WIDTH, LINE_WIDTH])
  m.setPaintProperty(LAYER, 'line-opacity', props.focused ? ['case', focused, 1, FADED_OPACITY] : 1)
}

function fit(names: string[]) {
  const bounds = new LngLatBounds()
  for (const line of props.lines) {
    if (names.length && !names.includes(line.name)) continue
    for (const shape of line.shapes) decodePolyline(shape).forEach(([lat, lng]) => bounds.extend([lng, lat]))
  }
  if (!bounds.isEmpty()) map()?.fitBounds(bounds, {padding: ADMIN_MAP_PADDING_PX, duration: 0})
}

watch(
  () => props.lines,
  () => {
    ;(map()?.getSource(SOURCE) as GeoJSONSource | undefined)?.setData(collection())
    fit([])
  },
)

watch(
  () => props.focused,
  (name) => {
    paintFocus()
    if (name) fit([name])
  },
)
</script>

<template>
  <div ref="container" class="admin-map"></div>
</template>
