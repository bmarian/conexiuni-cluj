import type {Map as MapLibreMap, Marker} from 'maplibre-gl'
import {type LatLng, markerLatLng} from '@/utils/mapMarkers.ts'

export const LEAVE_MS = 140

export const easeOutCubic = (k: number) => 1 - (1 - k) ** 3
export const linear = (k: number) => k

export function glideMarker(marker: Marker, to: LatLng, ms: number, ease: (k: number) => number = easeOutCubic): () => void {
  const from = markerLatLng(marker)
  if (ms <= 0 || (from.lat === to.lat && from.lng === to.lng)) {
    marker.setLngLat([to.lng, to.lat])
    return () => {}
  }
  const start = performance.now()
  let frame = 0
  const step = (now: number) => {
    const k = ease(Math.min(1, (now - start) / ms))
    marker.setLngLat([from.lng + (to.lng - from.lng) * k, from.lat + (to.lat - from.lat) * k])
    if (k < 1) frame = requestAnimationFrame(step)
  }
  frame = requestAnimationFrame(step)
  return () => cancelAnimationFrame(frame)
}

export function showMarker(marker: Marker, map: MapLibreMap, enter: boolean) {
  const el = marker.getElement()
  el.classList.remove('marker-leave')
  el.classList.toggle('marker-enter', enter)
  marker.addTo(map)
}

export function fadeOutMarker(marker: Marker) {
  const el = marker.getElement()
  el.classList.add('marker-leave')
  setTimeout(() => {
    if (el.classList.contains('marker-leave')) marker.remove()
  }, LEAVE_MS)
}

export function flightMs(distancePx: number, zoomDelta: number): number {
  return Math.min(955, Math.max(305, 260 + distancePx * 0.22 + Math.abs(zoomDelta) * 155))
}
