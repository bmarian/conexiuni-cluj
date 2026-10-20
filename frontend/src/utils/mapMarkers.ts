import {Marker, type Map as MapLibreMap} from 'maplibre-gl'
import type {MarkerIcon} from '@/utils/mapIcons.ts'

export type LatLng = { lat: number, lng: number }

// Also the map's clickTolerance, so a marker and the map agree on what a drag is.
export const CLICK_TOLERANCE_PX = 3

export interface HtmlMarkerOptions {
  interactive?: boolean
  draggable?: boolean
  zIndex?: number
  opacity?: number
}

function fillIcon(el: HTMLElement, icon: MarkerIcon) {
  el.style.width = `${icon.size[0]}px`
  el.style.height = `${icon.size[1]}px`
  if (typeof icon.html === 'string') el.innerHTML = icon.html
  else el.replaceChildren(icon.html)
}

const offsetOf = (icon: MarkerIcon): [number, number] => [-icon.anchor[0], -icon.anchor[1]]

export function htmlMarker({lat, lng}: LatLng, icon: MarkerIcon, options: HtmlMarkerOptions = {}): Marker {
  const el = document.createElement('div')
  fillIcon(el, icon)
  if (options.interactive === false) el.style.pointerEvents = 'none'
  if (options.zIndex) el.style.zIndex = String(options.zIndex)
  return new Marker({
    element: el,
    anchor: 'top-left',
    offset: offsetOf(icon),
    draggable: options.draggable ?? false,
    opacity: String(options.opacity ?? 1),
  }).setLngLat([lng, lat])
}

// The marker moves with the map while it's dragged, so a pan that starts on it still ends in a click.
export function onElementClick(el: HTMLElement, handler: () => void) {
  let down: { x: number, y: number } | null = null
  el.style.cursor = 'pointer'
  el.addEventListener('pointerdown', (e) => {
    down = {x: e.clientX, y: e.clientY}
  })
  el.addEventListener('click', (e) => {
    const moved = down !== null && Math.hypot(e.clientX - down.x, e.clientY - down.y) > CLICK_TOLERANCE_PX
    down = null
    if (!moved) handler()
  })
}

export function onMarkerClick(marker: Marker, handler: () => void) {
  onElementClick(marker.getElement(), handler)
}

export function setMarkerIcon(marker: Marker, icon: MarkerIcon) {
  fillIcon(marker.getElement(), icon)
  marker.setOffset(offsetOf(icon))
}

export function markerLatLng(marker: Marker): LatLng {
  const {lat, lng} = marker.getLngLat()
  return {lat, lng}
}

// One hover label at a time.
export function createTooltip(map: MapLibreMap) {
  const el = document.createElement('div')
  el.style.pointerEvents = 'none'
  el.style.zIndex = '20000'
  const bubble = document.createElement('div')
  el.append(bubble)
  const marker = new Marker({element: el, anchor: 'bottom'})
  let owner: object | null = null

  const hide = () => {
    owner = null
    marker.remove()
  }

  const show = (key: object, at: LatLng, offset: [number, number], html: string | (() => string), className: string, lift: number) => {
    owner = key
    bubble.className = `map-tooltip ${className}`
    bubble.innerHTML = typeof html === 'function' ? html() : html
    marker.setOffset([offset[0], offset[1] - lift]).setLngLat([at.lng, at.lat]).addTo(map)
  }

  const bindAt = (key: object, targetEl: HTMLElement, at: () => LatLng, offset: [number, number], html: string | (() => string), className: string, lift: number) => {
    targetEl.addEventListener('mouseenter', () => show(key, at(), offset, html, className, lift))
    targetEl.addEventListener('mouseleave', () => {
      if (owner === key) hide()
    })
  }

  const bind = (target: Marker, html: string | (() => string), className: string, lift: number) => {
    bindAt(target, target.getElement(), () => markerLatLng(target), [0, 0], html, className, lift)
    target.on('dragstart', hide)
  }

  const forget = (target: object) => {
    if (owner === target) hide()
  }

  return {bind, bindAt, forget, hide}
}
