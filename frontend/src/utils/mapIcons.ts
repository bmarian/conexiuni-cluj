import type {Vehicle} from '@/types/tranzy.ts'
import {type Mode, MODE_EMOJI} from '@/utils/modes.ts'
import {BUS_PATH as BUS_STOP_PATH, MODE_ICON} from '@/utils/modeIcons.ts'
import {escapeHtml} from '@/utils/text.ts'
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

export interface IconThemeOptions {
  arcadeActive: boolean
  legacyBlueActive: boolean
  paperActive: boolean
  showVehicleExtras: boolean
}

export type StationMark = {mode: Mode, color: string}

function paperPlate(size: number, fill: string, ink: string, iconSize: number, extra: string = '', glyph: string = MODE_ICON.bus): string {
  return `<div style="width:${size}px;height:${size}px;background:${fill};color:${ink};border:1.5px solid var(--pp-ink);display:flex;align-items:center;justify-content:center;${extra}">
    <svg viewBox="0 0 24 24" fill="currentColor" width="${iconSize}" height="${iconSize}">${glyph}</svg>
  </div>`
}

function clippySvg(wire: string, wireDark: string, w: number, h: number, extra: string = ''): string {
  return `<svg viewBox="0 0 26 32" width="${w}" height="${h}" xmlns="http://www.w3.org/2000/svg">
    <ellipse cx="13" cy="30.5" rx="6.5" ry="0.9" fill="rgba(0,0,0,0.22)"/>
    <rect x="3" y="5" width="20" height="25" rx="4.5" ry="4.5" fill="none" stroke="${wireDark}" stroke-width="3.2" stroke-opacity="0.55"/>
    <rect x="3" y="5" width="20" height="25" rx="4.5" ry="4.5" fill="none" stroke="${wire}" stroke-width="2.4"/>
    <rect x="7" y="9" width="12" height="17" rx="3" ry="3" fill="none" stroke="${wireDark}" stroke-width="3.2" stroke-opacity="0.55"/>
    <rect x="7" y="9" width="12" height="17" rx="3" ry="3" fill="none" stroke="${wire}" stroke-width="2.4"/>
    <path d="M 4.2 6.5 Q 4.2 5 5.7 5 L 19 5" fill="none" stroke="#FFFFFF" stroke-width="0.6" stroke-linecap="round" opacity="0.7"/>
    <path d="M 8 9.5 L 18 9.5" fill="none" stroke="#FFFFFF" stroke-width="0.5" stroke-linecap="round" opacity="0.55"/>
    <ellipse cx="10" cy="13" rx="3.2" ry="3.7" fill="white" stroke="#1A1A1A" stroke-width="0.6"/>
    <ellipse cx="17" cy="13" rx="3.2" ry="3.7" fill="white" stroke="#1A1A1A" stroke-width="0.6"/>
    <ellipse cx="10.6" cy="13.6" rx="1.3" ry="1.7" fill="#1A1A1A"/>
    <ellipse cx="17.6" cy="13.6" rx="1.3" ry="1.7" fill="#1A1A1A"/>
    <circle cx="10" cy="12.7" r="0.55" fill="white"/>
    <circle cx="17" cy="12.7" r="0.55" fill="white"/>
    <path d="M 5.5 8 Q 10 6 13 7.5" stroke="#1A1A1A" stroke-width="1.5" stroke-linecap="round" fill="none"/>
    <path d="M 14 7.5 Q 17 6 21.5 8" stroke="#1A1A1A" stroke-width="1.5" stroke-linecap="round" fill="none"/>
    ${extra}
  </svg>`
}

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

function stationCardIcon(mark: StationMark, opts: IconThemeOptions): string {
  if (opts.legacyBlueActive) return `<span class="station-card-emoji">${MODE_EMOJI[mark.mode]}</span>`
  if (opts.arcadeActive) return ghost(mark.color)
  return `<span class="station-card-icon" style="background:${mark.color}"><svg viewBox="0 0 24 24" fill="currentColor" width="14" height="14">${MODE_ICON[mark.mode]}</svg></span>`
}

export function stationCardHtml(name: string, opts: IconThemeOptions, mark?: StationMark & {routes: Array<{route_short_name: string, route_color: string}>}): string {
  if (!mark) return escapeHtml(name)
  const lines = mark.routes.map((r) => `<span class="station-card-line" style="background:${r.route_color}">${escapeHtml(r.route_short_name)}</span>`).join('')
  return `<div class="station-card">
    ${opts.paperActive ? '' : stationCardIcon(mark, opts)}
    <span class="station-card-body"><span class="station-card-name">${escapeHtml(name)}</span><span class="station-card-lines">${lines}</span></span>
  </div>`
}

export function makeStopIcon(opts: IconThemeOptions, mark?: StationMark): MarkerIcon {
  if (opts.arcadeActive) {
    const body = mark?.color ?? '#94a3b8'
    const pupil = '#334155'
    return {
      html: `<div style="width:20px;height:26px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 12 16" width="18" height="24" xmlns="http://www.w3.org/2000/svg">
          <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="${body}"/>
          <circle cx="3.8" cy="8" r="1.3" fill="white"/>
          <circle cx="7.8" cy="8" r="1.3" fill="white"/>
          <circle cx="4.2" cy="8.4" r="0.7" fill="${pupil}"/>
          <circle cx="8.2" cy="8.4" r="0.7" fill="${pupil}"/>
        </svg>
      </div>`,
      size: [20, 26],
      anchor: [10, 24],
    }
  }

  if (opts.legacyBlueActive) {
    const wire = mark?.color ?? '#9CA3AF'
    const wireDark = '#5C6470'
    return {
      html: `<div style="width:26px;height:32px;display:flex;align-items:flex-end;justify-content:center;filter:drop-shadow(1px 2px 1.5px rgba(0,0,0,0.35));">
        ${clippySvg(wire, wireDark, 24, 30)}
      </div>`,
      size: [26, 32],
      anchor: [13, 30],
    }
  }

  if (opts.paperActive) {
    return {
      html: paperPlate(20, 'var(--pp-paper)', 'var(--pp-ink)', 13, '', MODE_ICON[mark?.mode ?? 'bus']),
      size: [20, 20],
      anchor: [10, 10],
    }
  }

  return mark ? stationIcon(mark) : defaultStopIcon
}

const CLUSTER_LABEL_PX = 15
const CLUSTER_PAD_PX = 3
const CLUSTER_GAP_PX = 2
const CLUSTER_ROW = 3
export const CLUSTER_STATIONS_MAX = 3
const CLUSTER_NAMES_MAX = 8

const ghost = (fill: string, extra = '') => `<svg viewBox="0 0 12 16" width="18" height="24"><path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="${fill}"/>
  <circle cx="3.8" cy="8" r="1.3" fill="white"/><circle cx="7.8" cy="8" r="1.3" fill="white"/>
  <circle cx="4.2" cy="8.4" r="0.7" fill="#334155"/><circle cx="8.2" cy="8.4" r="0.7" fill="#334155"/>${extra}</svg>`

type ClusterLook = {w: number, h: number, icon: (mark: StationMark) => string, heart: () => string}

function clusterLook(opts: IconThemeOptions): ClusterLook {
  if (opts.arcadeActive) {
    return {w: 18, h: 24, icon: (mark) => ghost(mark.color), heart: () => ghost('#f43f5e', `<path d="${HEART_PATH}" fill="white" transform="translate(4.1 9.6) scale(0.16)"/>`)}
  }
  if (opts.legacyBlueActive) {
    return {w: 22, h: 22, icon: (mark) => `<span class="stop-cluster-emoji">${MODE_EMOJI[mark.mode]}</span>`, heart: () => '<span class="stop-cluster-emoji">\u2764\ufe0f</span>'}
  }
  if (opts.paperActive) {
    return {
      w: 20,
      h: 20,
      icon: (mark) => paperPlate(20, 'var(--pp-paper)', 'var(--pp-ink)', 13, '', MODE_ICON[mark.mode]),
      heart: () => paperPlate(20, 'var(--pp-red)', 'var(--pp-paper)', 13, '', `<path d="${HEART_PATH}"/>`),
    }
  }
  return {w: 24, h: 24, icon: (mark) => stationIcon(mark).html as string, heart: () => makeHighlightIcon('favorite', opts).html as string}
}

function clusterPill(count: number, look: ClusterLook): [number, number] {
  const rows = count > CLUSTER_ROW ? 2 : 1
  const cols = Math.ceil(count / rows)
  return [cols * look.w + (cols - 1) * CLUSTER_GAP_PX + 2 * CLUSTER_PAD_PX, rows * look.h + (rows - 1) * CLUSTER_GAP_PX + 2 * CLUSTER_PAD_PX]
}

export function clusterIconBox(count: number, labelWidth: number, opts: IconThemeOptions): Pick<MarkerIcon, 'size' | 'anchor'> {
  const [w, h] = clusterPill(count, clusterLook(opts))
  const width = Math.max(w, Math.ceil(labelWidth))
  return {size: [width, h + CLUSTER_LABEL_PX], anchor: [width / 2, h / 2]}
}

export type ClusterIcon = StationMark | 'heart'

export function makeClusterIcon(icons: ClusterIcon[], name: string, more: number, labelWidth: number, opts: IconThemeOptions): MarkerIcon {
  const look = clusterLook(opts)
  const [w, h] = clusterPill(icons.length, look)
  const box = clusterIconBox(icons.length, labelWidth, opts)
  const cells = icons.map((icon) =>
    `<div style="width:${look.w}px;height:${look.h}px;display:flex;align-items:center;justify-content:center;">${icon === 'heart' ? look.heart() : look.icon(icon)}</div>`).join('')
  return {
    html: `<div class="stop-cluster" style="width:${box.size[0]}px;">
      <div class="stop-cluster-pill" style="width:${w}px;height:${h}px;padding:${CLUSTER_PAD_PX}px;gap:${CLUSTER_GAP_PX}px;border-radius:${look.h / 2 + CLUSTER_PAD_PX}px;">${cells}</div>
      <div class="stop-cluster-label"><span>${escapeHtml(name)}</span>${more ? ` <span class="stop-cluster-more">+${more}</span>` : ''}</div>
    </div>`,
    ...box,
  }
}

export type ClusterStation = {name: string, mark?: Parameters<typeof stationCardHtml>[2]}

export function clusterCardHtml(stations: ClusterStation[], opts: IconThemeOptions, more: (n: number) => string): string {
  if (stations.length <= CLUSTER_STATIONS_MAX) {
    return `<div class="station-cards">${stations.map((s) => stationCardHtml(s.name, opts, s.mark)).join('')}</div>`
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

export function makeSpiderIcon(stations: SpiderStation[], opts: IconThemeOptions): MarkerIcon {
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
  const hub = opts.legacyBlueActive || opts.paperActive ? `<rect x="${r - 4}" y="${r - 4}" width="8" height="8"/>` : `<circle cx="${r}" cy="${r}" r="4"/>`
  return {
    html: `<div class="stop-spider" style="width:${size}px;height:${size}px;">
      <svg class="stop-spider-legs" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}">${legs}${hub}</svg>${items}
    </div>`,
    size: [size, size],
    anchor: [r, r],
  }
}

export function makeSelectedStopIcon(opts: IconThemeOptions, mark?: StationMark): MarkerIcon {
  if (opts.arcadeActive) {
    return {
      html: `<div class="animate-bounce" style="width:28px;height:36px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 12 16" width="26" height="34" xmlns="http://www.w3.org/2000/svg">
          <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="${mark?.color ?? '#22c55e'}"/>
          <circle cx="3.8" cy="8" r="1.3" fill="white"/>
          <circle cx="7.8" cy="8" r="1.3" fill="white"/>
          <circle cx="4.4" cy="8.5" r="0.6" fill="${mark ? 'rgba(0,0,0,0.65)' : '#15803d'}"/>
          <circle cx="8.4" cy="8.5" r="0.6" fill="${mark ? 'rgba(0,0,0,0.65)' : '#15803d'}"/>
        </svg>
      </div>`,
      size: [28, 36], anchor: [14, 34],
    }
  }

  if (opts.legacyBlueActive) {
    return {
      html: `<div class="animate-bounce" style="width:34px;height:42px;display:flex;align-items:flex-end;justify-content:center;filter:drop-shadow(1px 2px 2px rgba(0,0,0,0.45));">
        ${clippySvg(mark?.color ?? '#FFD64A', '#B07A10', 32, 40)}
      </div>`,
      size: [34, 42], anchor: [17, 40],
    }
  }

  if (opts.paperActive) {
    return {
      html: paperPlate(30, 'var(--pp-ink)', 'var(--pp-paper)', 18, 'outline:2px solid var(--pp-red);outline-offset:2px;', MODE_ICON[mark?.mode ?? 'bus']),
      size: [30, 30], anchor: [15, 15],
    }
  }

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
  opts: IconThemeOptions,
): MarkerIcon {
  const bg =
    color === 'green' ? '#10b981'
      : color === 'purple' ? '#a855f7'
        : color === 'red' || color === 'favorite' ? '#f43f5e'
          : color === 'amber' ? '#f59e0b'
            : '#64748b'

  if (opts.arcadeActive) {
    return {
      html: `<div style="width:24px;height:30px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 12 16" width="22" height="28" xmlns="http://www.w3.org/2000/svg">
          <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="${bg}"/>
          <circle cx="3.8" cy="8" r="1.3" fill="white"/>
          <circle cx="7.8" cy="8" r="1.3" fill="white"/>
          <circle cx="4.2" cy="8.4" r="0.7" fill="rgba(0,0,0,0.65)"/>
          <circle cx="8.2" cy="8.4" r="0.7" fill="rgba(0,0,0,0.65)"/>
          ${color === 'favorite' ? `<path d="${HEART_PATH}" fill="white" transform="translate(4.1 9.6) scale(0.16)"/>` : ''}
        </svg>
      </div>`,
      size: [24, 30],
      anchor: [12, 28],
    }
  }

  if (opts.legacyBlueActive) {
    return {
      html: `<div style="width:28px;height:34px;display:flex;align-items:flex-end;justify-content:center;filter:drop-shadow(1px 2px 1.5px rgba(0,0,0,0.4));">
        ${clippySvg(bg, bg, 26, 32, color === 'favorite' ? `<path d="${HEART_PATH}" fill="${bg}" stroke="white" stroke-width="2.5" transform="translate(9 20.5) scale(0.33)"/>` : '')}
      </div>`,
      size: [28, 34],
      anchor: [14, 32],
    }
  }

  if (opts.paperActive) {
    const ink =
      color === 'green' ? 'var(--pp-green)'
        : color === 'purple' ? 'var(--pp-plum)'
          : color === 'red' || color === 'favorite' ? 'var(--pp-red)'
            : color === 'amber' ? 'var(--pp-ochre)'
              : 'var(--pp-ink-2)'
    return {
      html: paperPlate(22, ink, 'var(--pp-paper)', 14, '', color === 'favorite' ? `<path d="${HEART_PATH}"/>` : MODE_ICON.bus),
      size: [22, 22],
      anchor: [11, 11],
    }
  }

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

export function makePinIcon(opts: IconThemeOptions, color: string = "#0ea5e9"): MarkerIcon {
  if (opts.arcadeActive) {
    return {
      html: `<div class="planner-pin-visual" style="width:24px;height:32px;display:flex;align-items:flex-end;justify-content:center;position:relative;filter:drop-shadow(1px 2px 2px rgba(0,0,0,0.35));">
        <svg viewBox="0 0 20 28" width="20" height="28" xmlns="http://www.w3.org/2000/svg">
          <path d="M10 0C4.48 0 0 4.48 0 10C0 17.5 10 28 10 28C10 28 20 17.5 20 10C20 4.48 15.52 0 10 0Z" fill="${color}" stroke="white" stroke-width="1.5"/>
          <path d="M10 10 L15.2 7 A6 6 0 1 0 15.2 13 Z" fill="#fff8e1"/>
        </svg>
      </div>`,
      size: [24, 32],
      anchor: [12, 32],
    }
  }

  if (opts.legacyBlueActive) {
    const emoji = color === '#0ea5e9' ? '🏁' : '📍'
    return {
      html: `<div class="planner-pin-visual" style="font-size:28px;line-height:1;position:relative;filter:drop-shadow(1px 2px 2px rgba(0,0,0,0.4));display:block;">${emoji}</div>`,
      size: [28, 28],
      anchor: [14, 28],
    }
  }

  if (opts.paperActive) {
    const ink = color === '#22c55e' ? 'var(--pp-green)' : 'var(--pp-red)'
    return {
      html: `<div class="planner-pin-visual" style="width:24px;height:32px;display:flex;flex-direction:column;align-items:center;position:relative;">
        <div style="width:18px;height:18px;background:${ink};border:1.5px solid var(--pp-ink);display:flex;align-items:center;justify-content:center;">
          <div style="width:6px;height:6px;background:var(--pp-paper);"></div>
        </div>
        <div style="width:2px;height:14px;background:var(--pp-ink);"></div>
      </div>`,
      size: [24, 32],
      anchor: [12, 32],
    }
  }

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

function accessibilityWords(vehicle: DisplayVehicle, t: (key: string) => string): string {
  const {wheelchair, bike} = accessibility(vehicle)
  const words = [
    wheelchair === undefined ? '' : t(wheelchair ? 'paperWheelchair' : 'paperNoWheelchair'),
    bike === undefined ? '' : t(bike ? 'paperBikes' : 'paperNoBikes'),
  ].filter(Boolean)
  return words.length ? `<div style="font-size:10px;font-stretch:85%;font-weight:600;letter-spacing:0.06em;text-transform:uppercase;">${words.join(' · ')}</div>` : ''
}

export function getVehicleMarkerHtml(
  vehicle: DisplayVehicle,
  resolvedColor: string,
  showStopInfo: boolean,
  opts: IconThemeOptions,
  t: (key: string) => string,
): string {
  const routeName = vehicle.route_short_name || ''
  const routeFontSize = routeName.length >= 4 ? 8 : routeName.length >= 3 ? 9 : 11
  const roundedSpeed = Math.round(vehicle.speed ?? 0)
  const label = String(vehicle.label ?? '').replace(/[^\w -]/g, '')
  const speedText = `${label ? `#${label} · ` : ''}${roundedSpeed} km/h`
  const heading = vehicle.heading || 0
  const extrasHtml = (color: string) => {
    const icons = opts.showVehicleExtras ? accessibilityIcons(vehicle, color) : ''
    return icons && `<span style="color:${color};font-weight:700;">·</span>${icons}`
  }

  if (opts.arcadeActive) {
    const rotation = heading - 90
    const arcade = `<div style="transform:rotate(${rotation}deg);flex-shrink:0;">
      <div class="arcade-chomp" style="width:36px;height:36px;background-color:${resolvedColor};border-radius:50%;border:2px solid white;box-shadow:0 2px 8px rgba(0,0,0,0.28);"></div>
    </div>`

    return `
      <div style="position:relative;display:flex;flex-direction:column;align-items:center;gap:1px;">
        ${arcade}
        <div style="background-color:${resolvedColor};color:white;font-size:${routeFontSize}px;font-weight:900;padding:0 3px;border-radius:3px;border:1px solid rgba(255,255,255,0.8);line-height:1.5;white-space:nowrap;">${routeName}</div>
        ${showStopInfo ? `
          <div class="absolute" style="left:42px;top:0;background:rgba(15,23,42,0.9);color:#f1f5f9;padding:4px 10px;border-radius:6px;box-shadow:0 2px 8px rgba(0,0,0,0.3);white-space:nowrap;z-index:20;pointer-events:none;">
            <div style="display:flex;align-items:center;gap:4px;">
              <span style="font-weight:700;font-size:13px;">${routeName}</span>
              ${extrasHtml('#94a3b8')}
            </div>
            <span style="font-size:12px;color:#94a3b8;">${speedText}</span>
          </div>
        ` : ''}
      </div>`
  }

  if (opts.legacyBlueActive) {
    const sz = 36
    const cursorRotation = heading + 45

    // XP cursor, rotated heading+45 because its tip points NW.
    const cursorSvg = `<svg viewBox="-1 -1 14 20" width="${sz}" height="${sz}" xmlns="http://www.w3.org/2000/svg" style="transform:rotate(${cursorRotation}deg);transform-origin:center;display:block;filter:drop-shadow(1px 2px 2px rgba(0,0,0,0.45));">
      <path d="M 0 0 L 0 16 L 4 12 L 6 18 L 8 17 L 6 11 L 11 11 Z" fill="${resolvedColor}" stroke="black" stroke-width="1" stroke-linejoin="miter" stroke-linecap="round"/>
      <path d="M 1 1 L 1 14" stroke="rgba(255,255,255,0.55)" stroke-width="0.7" stroke-linecap="round" fill="none"/>
    </svg>`

    const cursorBox = `<div style="width:${sz}px;height:${sz}px;display:flex;align-items:center;justify-content:center;flex-shrink:0;">${cursorSvg}</div>`

    const xpTip = (extraStyle: string = '') => `<div class="absolute" style="${extraStyle}background:#FFFFE0;color:#000000;padding:3px 7px;border:1px solid #000000;box-shadow:1px 1px 0 rgba(0,0,0,0.35);white-space:nowrap;z-index:20;pointer-events:none;font-family:'Tahoma','Trebuchet MS',sans-serif;line-height:1.3;">
      <div style="display:flex;align-items:center;gap:4px;">
        <span style="font-weight:700;font-size:11px;">${routeName}</span>
        ${extrasHtml('#404040')}
      </div>
      <span style="font-size:11px;color:#404040;">${speedText}</span>
    </div>`

    const routeBadge = `<div style="background-color:${resolvedColor};color:white;font-size:${routeFontSize}px;font-weight:700;padding:1px 5px;border-radius:0;border:1px solid #000000;line-height:1.4;white-space:nowrap;font-family:'Tahoma','Trebuchet MS',sans-serif;box-shadow:1px 1px 0 rgba(0,0,0,0.3);">${routeName}</div>`

    return `
      <div style="position:relative;display:flex;flex-direction:column;align-items:center;gap:2px;">
        ${cursorBox}
        ${routeBadge}
        ${showStopInfo ? xpTip(`left:${sz + 6}px;top:0;`) : ''}
      </div>`
  }

  if (opts.paperActive) {
    const paperTip = `<div class="absolute" style="left:44px;top:0;background:var(--pp-paper);color:var(--pp-ink);padding:3px 8px;border:1px solid var(--pp-ink);white-space:nowrap;z-index:20;pointer-events:none;font-family:var(--pp-display);line-height:1.3;">
      <div style="font-weight:600;font-size:13px;letter-spacing:0.03em;">${routeName}</div>
      <div style="font-size:12px;font-family:var(--pp-text);font-style:italic;">${speedText}</div>
      ${opts.showVehicleExtras ? accessibilityWords(vehicle, t) : ''}
    </div>`

    return `
      <div style="position:relative;display:flex;align-items:center;">
        <div class="pp-map-stamp" style="min-width:36px;height:26px;padding:0 4px;background-color:${resolvedColor};display:flex;align-items:center;justify-content:center;font-family:var(--pp-display);font-weight:600;font-size:${routeFontSize + 3}px;letter-spacing:0.02em;line-height:1;">${routeName}</div>
        <div style="position:absolute;right:-7px;bottom:-7px;width:15px;height:15px;background:var(--pp-ink);display:flex;align-items:center;justify-content:center;">
          <svg viewBox="0 0 24 24" width="10" height="10" style="fill:var(--pp-paper);transform:rotate(${heading}deg);"><path d="M12 2L21 21l-9-4-9 4 9-19z"/></svg>
        </div>
        ${showStopInfo ? paperTip : ''}
      </div>`
  }

  return `
    <div class="relative flex items-center">
      <div class="flex items-center justify-center w-9 h-9 rounded-full border-2 border-white shadow-md z-30"
           style="background-color: ${resolvedColor};">
        <span class="font-black leading-none text-white tracking-tight"
              style="font-size:${routeFontSize}px;max-width:22px;">${routeName}</span>
      </div>
      <div class="absolute -right-0.5 -bottom-0.5 w-4 h-4 rounded-full border border-white bg-slate-900/85 flex items-center justify-center shadow-sm z-40">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="white" class="w-2.5 h-2.5 shrink-0"
             style="transform: rotate(${heading}deg);">
          <path d="M12 2L21 21l-9-4-9 4 9-19z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
        </svg>
      </div>
      ${showStopInfo ? `
        <div class="absolute left-10 bg-slate-900/90 dark:bg-slate-800/90 text-slate-100 px-2.5! py-1! rounded-md shadow-md whitespace-nowrap z-20 pointer-events-none">
          <div style="display:flex;align-items:center;gap:4px;">
            <span class="font-bold text-sm tracking-wide">${routeName}</span>
            ${extrasHtml('#94a3b8')}
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
