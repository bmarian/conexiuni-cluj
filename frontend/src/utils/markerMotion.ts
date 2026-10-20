import type {Marker} from 'maplibre-gl'
import {type LatLng, markerLatLng} from '@/utils/mapMarkers.ts'

export const LEAVE_MS = 160

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

export function fadeOutMarker(marker: Marker) {
  marker.getElement().classList.add('marker-leave')
  setTimeout(() => marker.remove(), LEAVE_MS)
}

export function flightMs(distancePx: number, zoomDelta: number): number {
  return Math.min(1100, Math.max(350, 300 + distancePx * 0.25 + Math.abs(zoomDelta) * 180))
}
