import {Marker, type Map as MapLibreMap} from 'maplibre-gl'
import type {MarkerIcon} from '@/utils/mapIcons.ts'
import {HOLD_MS} from '@/composables/useLongPress.ts'

export type LatLng = {lat: number; lng: number}

// Also the map's clickTolerance, so a marker and the map agree on what a drag is.
export const CLICK_TOLERANCE_PX = 3

export const FLAT_MAP_OPTIONS = {clickTolerance: CLICK_TOLERANCE_PX, dragRotate: false, pitchWithRotate: false, touchPitch: false}

export interface HtmlMarkerOptions {
  interactive?: boolean
  draggable?: boolean
  zIndex?: number
  opacity?: number
}

function fillIcon(el: HTMLElement, icon: MarkerIcon) {
  el.style.width = `${icon.size[0]}px`
  el.style.height = `${icon.size[1]}px`
  const body = document.createElement('div')
  body.className = 'marker-body'
  if (typeof icon.html === 'string') body.innerHTML = icon.html
  else body.append(icon.html)
  el.replaceChildren(body)
}

const offsetOf = (icon: MarkerIcon): [number, number] => [-icon.anchor[0], -icon.anchor[1]]

export function htmlMarker({lat, lng}: LatLng, icon: MarkerIcon, options: HtmlMarkerOptions = {}): Marker {
  const el = document.createElement('div')
  el.className = 'marker-enter'
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

// Elements whose last press was a hold, so its lift is not a click.
const held = new WeakSet<HTMLElement>()

// The marker moves with the map while it's dragged, so a pan that starts on it still ends in a click.
export function onElementClick(el: HTMLElement, handler: () => void) {
  let down: {x: number; y: number} | null = null
  el.style.cursor = 'pointer'
  el.classList.add('marker-clickable', 'hold-target')
  el.addEventListener('pointerdown', (e) => {
    down = {x: e.clientX, y: e.clientY}
  })
  el.addEventListener('click', (e) => {
    const moved = down !== null && Math.hypot(e.clientX - down.x, e.clientY - down.y) > CLICK_TOLERANCE_PX
    down = null
    if (!moved && !held.has(el)) handler()
  })
}

// A mouse previews on hover, a finger on hold; a hold's preview stays until the next press anywhere.
export function onPreview(el: HTMLElement, show: () => void, hide: () => void) {
  let timer: ReturnType<typeof setTimeout> | undefined
  let start = {x: 0, y: 0}
  let holding = false
  const cancel = () => {
    clearTimeout(timer)
    timer = undefined
  }
  const hold = () => {
    cancel()
    if (holding) return
    holding = true
    held.add(el)
    show()
    document.addEventListener('pointerdown', hide, {capture: true, once: true})
  }
  el.addEventListener('pointerenter', (e) => {
    if (e.pointerType === 'mouse') show()
  })
  el.addEventListener('pointerleave', (e) => {
    if (e.pointerType === 'mouse') hide()
  })
  el.addEventListener('pointerdown', (e) => {
    holding = false
    held.delete(el)
    if (e.pointerType === 'mouse') return
    start = {x: e.clientX, y: e.clientY}
    timer = setTimeout(hold, HOLD_MS)
  })
  el.addEventListener('pointermove', (e) => {
    if (timer && Math.hypot(e.clientX - start.x, e.clientY - start.y) > CLICK_TOLERANCE_PX) cancel()
  })
  el.addEventListener('pointerup', cancel)
  el.addEventListener('pointercancel', cancel)
  // Touch browsers also send this on a hold.
  el.addEventListener('contextmenu', (e) => {
    if (!timer && !holding && (e as PointerEvent).pointerType !== 'touch') return
    e.preventDefault()
    hold()
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

export type ScreenArea = {top: number; right: number; bottom: number; left: number}

const TOOLTIP_MARGIN_PX = 8

// How far to move the span [start, end] so it sits inside [min, max]; the start wins when it can't fit.
export function shiftInto(start: number, end: number, min: number, max: number): number {
  if (start < min || end - start > max - min) return min - start
  return Math.min(0, max - end)
}

export function keepSideTipInside(markerEl: HTMLElement, area: ScreenArea) {
  const tip = markerEl.querySelector<HTMLElement>('.vehicle-tip')
  if (!tip) return
  tip.classList.remove('is-left')
  tip.classList.toggle('is-left', tip.getBoundingClientRect().right > area.right - TOOLTIP_MARGIN_PX)
}

// One hover label at a time, kept inside the part of the map that isn't covered.
export function createTooltip(map: MapLibreMap, visibleArea: () => ScreenArea) {
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

  const fit = (at: LatLng, offset: [number, number], lift: number) => {
    const area = visibleArea()
    const box = el.getBoundingClientRect()
    const pointY = map.getContainer().getBoundingClientRect().top + map.project([at.lng, at.lat]).y + offset[1]
    const roomAbove = pointY - lift - area.top
    const roomBelow = area.bottom - pointY - lift
    const below = roomAbove < box.height + TOOLTIP_MARGIN_PX && roomBelow > roomAbove
    if (below) marker.setOffset([offset[0], offset[1] + lift + box.height])
    const top = below ? pointY + lift : box.top
    const dx = shiftInto(box.left, box.right, area.left + TOOLTIP_MARGIN_PX, area.right - TOOLTIP_MARGIN_PX)
    const dy = shiftInto(top, top + box.height, area.top + TOOLTIP_MARGIN_PX, area.bottom - TOOLTIP_MARGIN_PX)
    const maxTipShift = Math.max(0, box.width / 2 - 12)
    bubble.classList.toggle('map-tooltip-detached', below || dy !== 0)
    bubble.style.translate = dx || dy ? `${dx}px ${dy}px` : ''
    bubble.style.setProperty('--tip-shift', `${Math.max(-maxTipShift, Math.min(maxTipShift, -dx))}px`)
  }

  const show = (key: object, at: LatLng, offset: [number, number], html: string | (() => string), className: string, lift: number) => {
    owner = key
    bubble.className = `map-tooltip ${className}`
    bubble.innerHTML = typeof html === 'function' ? html() : html
    bubble.style.translate = ''
    marker
      .setOffset([offset[0], offset[1] - lift])
      .setLngLat([at.lng, at.lat])
      .addTo(map)
    fit(at, offset, lift)
  }

  const bindAt = (
    key: object,
    targetEl: HTMLElement,
    at: () => LatLng,
    offset: [number, number],
    html: string | (() => string),
    className: string,
    lift: number,
  ) => {
    onPreview(
      targetEl,
      () => show(key, at(), offset, html, className, lift),
      () => {
        if (owner === key) hide()
      },
    )
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
