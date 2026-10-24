import type {Vehicle} from '@/types/tranzy.ts'
import type {Mode} from '@/utils/modes.ts'
import {BUS_PATH as BUS_STOP_PATH, MODE_ICON} from '@/utils/modeIcons.ts'
import {escapeHtml} from '@/utils/text.ts'
import {displayColor} from '@/utils/lineColor.ts'
import {type SpiderLeg, spiderRadius} from '@/utils/spider.ts'

export type DisplayVehicle = Vehicle & {
  route_short_name: string;
  route_color?: string;
  heading: number
  // Drawn faded: on the other direction now, counted on coming back.
  faded?: boolean
}

export interface MarkerIcon {
  html: string | HTMLElement
  size: [number, number]
  anchor: [number, number]
}

export type StationMark = {mode: Mode, color: string}

const defaultStopIcon: MarkerIcon = {
  html: `
    <div class="flex items-center justify-center w-6 h-6 rounded-full border-2 border-white shadow-sm z-20 bg-slate-500 dark:bg-slate-600 text-white">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-3.5 h-3.5">
        <path d="${BUS_STOP_PATH}"/>
      </svg>
    </div>
  `,
  size: [24, 24],
  anchor: [12, 12],
}

const stationIcons = new Map<string, MarkerIcon>()

function stationIcon({mode, color}: StationMark): MarkerIcon {
  const key = `${mode}|${color}`
  let icon = stationIcons.get(key)
  if (!icon) {
    icon = {
      html: `<div style="width:24px;height:24px;border-radius:50%;background:${color};border:2px solid white;box-shadow:0 1px 3px rgba(0,0,0,0.25);display:flex;align-items:center;justify-content:center;color:white;">
        <svg viewBox="0 0 24 24" fill="currentColor" width="14" height="14">${MODE_ICON[mode]}</svg>
      </div>`,
      size: [24, 24],
      anchor: [12, 12],
    }
    stationIcons.set(key, icon)
  }
  return icon
}

export function stationCardHtml(name: string, mark?: StationMark & {routes: Array<{route_short_name: string, route_color: string}>}): string {
  if (!mark) return escapeHtml(name)
  const lines = mark.routes.map((r) => `<span class="station-card-line" style="background:${displayColor(r.route_short_name, r.route_color)}">${escapeHtml(r.route_short_name)}</span>`).join('')
  return `<div class="station-card">
    <span class="station-card-icon" style="background:${mark.color}"><svg viewBox="0 0 24 24" fill="currentColor" width="14" height="14">${MODE_ICON[mark.mode]}</svg></span>
    <span class="station-card-body"><span class="station-card-name">${escapeHtml(name)}</span><span class="station-card-lines">${lines}</span></span>
  </div>`
}

export function makeStopIcon(mark?: StationMark): MarkerIcon {
  return mark ? stationIcon(mark) : defaultStopIcon
}

const CLUSTER_LABEL_PX = 15
const CLUSTER_PAD_PX = 3
const CLUSTER_GAP_PX = 2
const CLUSTER_ROW = 3
export const CLUSTER_STATIONS_MAX = 3
const CLUSTER_NAMES_MAX = 8
const CLUSTER_CELL_PX = 24

function clusterPill(count: number): [number, number] {
  const rows = count > CLUSTER_ROW ? 2 : 1
  const cols = Math.ceil(count / rows)
  return [cols * CLUSTER_CELL_PX + (cols - 1) * CLUSTER_GAP_PX + 2 * CLUSTER_PAD_PX, rows * CLUSTER_CELL_PX + (rows - 1) * CLUSTER_GAP_PX + 2 * CLUSTER_PAD_PX]
}

export function clusterIconBox(count: number, labelWidth: number): Pick<MarkerIcon, 'size' | 'anchor'> {
  const [w, h] = clusterPill(count)
  const width = Math.max(w, Math.ceil(labelWidth))
  return {size: [width, h + CLUSTER_LABEL_PX], anchor: [width / 2, h / 2]}
}

export type ClusterIcon = StationMark | 'heart'

export function makeClusterIcon(icons: ClusterIcon[], name: string, more: number, labelWidth: number): MarkerIcon {
  const [w, h] = clusterPill(icons.length)
  const box = clusterIconBox(icons.length, labelWidth)
  const cells = icons.map((icon) =>
    `<div style="width:${CLUSTER_CELL_PX}px;height:${CLUSTER_CELL_PX}px;display:flex;align-items:center;justify-content:center;">${(icon === 'heart' ? makeHighlightIcon('favorite') : stationIcon(icon)).html}</div>`).join('')
  return {
    html: `<div class="stop-cluster" style="width:${box.size[0]}px;">
      <div class="stop-cluster-pill" style="width:${w}px;height:${h}px;padding:${CLUSTER_PAD_PX}px;gap:${CLUSTER_GAP_PX}px;border-radius:${CLUSTER_CELL_PX / 2 + CLUSTER_PAD_PX}px;">${cells}</div>
      <div class="stop-cluster-label"><span>${escapeHtml(name)}</span>${more ? ` <span class="stop-cluster-more">+${more}</span>` : ''}</div>
    </div>`,
    ...box,
  }
}

export type ClusterStation = {name: string, mark?: Parameters<typeof stationCardHtml>[1]}

export function clusterCardHtml(stations: ClusterStation[], more: (n: number) => string): string {
  if (stations.length <= CLUSTER_STATIONS_MAX) {
    return `<div class="station-cards">${stations.map((s) => stationCardHtml(s.name, s.mark)).join('')}</div>`
  }
  const names = [...new Set(stations.map((s) => s.name))].sort((a, b) => a.localeCompare(b))
  const shown = names.slice(0, CLUSTER_NAMES_MAX)
  const rest = names.length - shown.length
  return `<div class="station-names">
    ${shown.map((name) => `<div>${escapeHtml(name)}</div>`).join('')}
    ${rest ? `<div class="station-names-more">${escapeHtml(more(rest))}</div>` : ''}
  </div>`
}

export type SpiderStation = {id: string, name: string, icon: MarkerIcon, leg: SpiderLeg}

const SPIDER_PAD_PX = 16

export function makeSpiderIcon(stations: SpiderStation[]): MarkerIcon {
  const r = spiderRadius(stations.map((s) => s.leg)) + SPIDER_PAD_PX
  const size = 2 * r
  const legs = stations.map(({leg}) => `<line x1="${r}" y1="${r}" x2="${r + leg.x}" y2="${r + leg.y}"/>`).join('')
  const items = stations.map(({id, name, icon, leg}, i) => {
    const [w, h] = icon.size
    const [ax, ay] = icon.anchor
    const place = leg.side === 'right' ? `left:${r + leg.x - ax}px;` : `right:${r - leg.x - (w - ax)}px;flex-direction:row-reverse;`
    const html = typeof icon.html === 'string' ? icon.html : icon.html.outerHTML
    return `<div class="stop-spider-item" data-stop="${escapeHtml(id)}" style="${place}top:${r + leg.y - ay}px;--dx:${-leg.x}px;--dy:${-leg.y}px;--i:${i};">
      <div style="width:${w}px;height:${h}px;flex:none;">${html}</div>
      <div class="stop-cluster-label">${escapeHtml(name)}</div>
    </div>`
  }).join('')
  return {
    html: `<div class="stop-spider" style="width:${size}px;height:${size}px;">
      <svg class="stop-spider-legs" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}">${legs}<circle cx="${r}" cy="${r}" r="4"/></svg>${items}
    </div>`,
    size: [size, size],
    anchor: [r, r],
  }
}

export function makeSelectedStopIcon(mark?: StationMark): MarkerIcon {
  return {
    html: `
      <div class="flex items-center justify-center w-8 h-8 rounded-full border-2 border-white shadow-lg z-50 text-white animate-bounce ${mark ? '' : 'bg-emerald-500'}"
           style="${mark ? `background:${mark.color};` : ''}">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-4 h-4">${MODE_ICON[mark?.mode ?? 'bus']}</svg>
      </div>
    `,
    size: [32, 32], anchor: [16, 16],
  }
}

const HEART_PATH = 'M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z'

export function makeHighlightIcon(
  color: 'green' | 'purple' | 'red' | 'gray' | 'amber' | 'favorite',
): MarkerIcon {
  const bg =
    color === 'green' ? '#10b981'
      : color === 'purple' ? '#a855f7'
        : color === 'red' || color === 'favorite' ? '#f43f5e'
          : color === 'amber' ? '#f59e0b'
            : '#64748b'

  if (color === 'favorite') {
    return {
      html: `<div style="width:24px;height:24px;border-radius:50%;background:${bg};border:2px solid white;box-shadow:0 1px 3px rgba(0,0,0,0.25);display:flex;align-items:center;justify-content:center;">
        <svg viewBox="0 0 24 24" fill="white" width="13" height="13"><path d="${HEART_PATH}"/></svg>
      </div>`,
      size: [24, 24],
      anchor: [12, 12],
    }
  }

  return {
    html: `<div style="width:24px;height:24px;border-radius:50%;background:${bg};border:2px solid white;box-shadow:0 1px 4px rgba(0,0,0,0.22);display:flex;align-items:center;justify-content:center;">
      <svg viewBox="0 0 24 24" fill="white" width="14" height="14"><path d="${BUS_STOP_PATH}"/></svg>
    </div>`,
    size: [24, 24],
    anchor: [12, 12],
  }
}

export function makePinIcon(color: string = "#0ea5e9"): MarkerIcon {
  return {
    html: `<div class="planner-pin-visual" style="width:24px;height:32px;display:flex;align-items:flex-end;justify-content:center;position:relative;filter:drop-shadow(1px 2px 2px rgba(0,0,0,0.35));">
      <svg viewBox="0 0 20 28" width="20" height="28" xmlns="http://www.w3.org/2000/svg">
        <path d="M10 0C4.48 0 0 4.48 0 10C0 17.5 10 28 10 28C10 28 20 17.5 20 10C20 4.48 15.52 0 10 0Z" fill="${color}" stroke="white" stroke-width="1.5"/>
        <circle cx="10" cy="10" r="3.5" fill="white" opacity="0.9"/>
      </svg>
    </div>`,
    size: [24, 32],
    anchor: [12, 32],
  }
}

// What the feed says about each, if anything; a feed that does not say gets no icon rather than a "no".
function accessibility(vehicle: DisplayVehicle): {wheelchair?: boolean, bike?: boolean} {
  const told = (value?: string) => value !== undefined && value !== '' && value !== 'UNKNOWN'
  return {
    wheelchair: told(vehicle.wheelchair_accessible) ? vehicle.wheelchair_accessible === 'WHEELCHAIR_ACCESSIBLE' : undefined,
    bike: told(vehicle.bike_accessible) ? vehicle.bike_accessible === 'BIKE_ACCESSIBLE' : undefined,
  }
}

function accessibilityIcons(vehicle: DisplayVehicle, color: string): string {
  const {wheelchair, bike} = accessibility(vehicle)
  const sz = 12
  const strike = `<line x1="2" y1="2" x2="22" y2="22" stroke="#ef4444" stroke-width="4" stroke-linecap="round"/>`
  const icon = (ok: boolean, shape: string) =>
    `<svg width="${sz}" height="${sz}" viewBox="0 0 24 24" fill="none" stroke-linecap="round" stroke-linejoin="round"><g stroke="${color}" stroke-width="2.5"${!ok ? ' opacity="0.4"' : ''}>${shape}</g>${!ok ? strike : ''}</svg>`
  const icons = [
    wheelchair === undefined ? '' : icon(wheelchair, `<circle cx="12" cy="3.5" r="2.5" fill="${color}" stroke="none"/><path d="M10 8v6h5l2 5"/><circle cx="9" cy="19.5" r="3.5"/>`),
    bike === undefined ? '' : icon(bike, `<circle cx="18.5" cy="17.5" r="3.5"/><circle cx="5.5" cy="17.5" r="3.5"/><circle cx="14" cy="4.5" r="2" fill="${color}" stroke="none"/><path d="M12 17.5V13l-3-3 4-3 2 3h3"/>`),
  ].join('')
  return icons && `<span style="display:flex;gap:4px;align-items:center;">${icons}</span>`
}

export function getVehicleMarkerHtml(
  vehicle: DisplayVehicle,
  resolvedColor: string,
  showStopInfo: boolean,
): string {
  const routeName = vehicle.route_short_name || ''
  const routeFontSize = routeName.length >= 4 ? 8 : routeName.length >= 3 ? 9 : 11
  const roundedSpeed = Math.round(vehicle.speed ?? 0)
  const label = String(vehicle.label ?? '').replace(/[^\w -]/g, '')
  const speedText = `${label ? `#${label} · ` : ''}${roundedSpeed} km/h`
  const heading = vehicle.heading || 0
  const extras = accessibilityIcons(vehicle, '#94a3b8')

  return `
    <div class="relative flex items-center">
      <div class="flex items-center justify-center w-9 h-9 rounded-full border-2 border-white shadow-md z-30"
           style="background-color: ${resolvedColor};">
        <span class="font-black leading-none text-white tracking-tight"
              style="font-size:${routeFontSize}px;max-width:22px;">${routeName}</span>
      </div>
      <div class="absolute -right-0.5 -bottom-0.5 w-4 h-4 rounded-full border border-white bg-slate-900/85 flex items-center justify-center shadow-sm z-40">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="white" class="vehicle-heading w-2.5 h-2.5 shrink-0"
             style="transform: rotate(${heading}deg);">
          <path d="M12 2L21 21l-9-4-9 4 9-19z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
        </svg>
      </div>
      ${showStopInfo ? `
        <div class="absolute left-10 bg-slate-900/90 dark:bg-slate-800/90 text-slate-100 px-2.5! py-1! rounded-md shadow-md whitespace-nowrap z-20 pointer-events-none">
          <div style="display:flex;align-items:center;gap:4px;">
            <span class="font-bold text-sm tracking-wide">${routeName}</span>
            ${extras && `<span style="color:#94a3b8;font-weight:700;">·</span>${extras}`}
          </div>
          <span class="text-xs text-slate-400">${speedText}</span>
        </div>
      ` : ''}
    </div>
  `
}

// A planned place's label, "Name, area", as its pin's tooltip; it can come from a shared link.
export function placeTooltipHtml(label: string): string {
  const parts = label.split(',').map((p) => escapeHtml(p.trim()))
  const main = parts[0] ?? escapeHtml(label)
  const sub = parts.slice(1).join(', ')
  return sub
    ? `<div class="pin-tip-main">${main}</div><div class="pin-tip-sub">${sub}</div>`
    : `<div class="pin-tip-main">${main}</div>`
}
