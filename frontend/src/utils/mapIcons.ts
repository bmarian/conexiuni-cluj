import L from 'leaflet'
import type { Vehicle } from '@/types/tranzy.ts'

export type DisplayVehicle = Vehicle & { route_short_name: string; route_color?: string; heading: number }

export interface IconThemeOptions {
  easterEggActive: boolean
  traditionalActive: boolean
}

// ── Shared constants ────────────────────────────────────────────────────────

const BUS_STOP_PATH =
  'M4 16c0 .88.39 1.67 1 2.22V20c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1h8v1c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1.78c.61-.55 1-1.34 1-2.22V6c0-3.5-3.58-4-8-4s-8 .5-8 4v10zm3.5 1c-.83 0-1.5-.67-1.5-1.5S6.67 14 7.5 14s1.5.67 1.5 1.5S8.33 17 7.5 17zm9 0c-.83 0-1.5-.67-1.5-1.5s.67-1.5 1.5-1.5 1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm-10-7V6h11v4H6.5z'

// ── Clippy ──────────────────────────────────────────────────────────────────
// Recognizable Office Assistant: paperclip body in `wire` colour with a darker
// `wireDark` shadow stroke under it for depth, two big googly eyes overlapping
// the top loop, and bushy raised eyebrows. viewBox 0 0 26 32.

function clippySvg(wire: string, wireDark: string, w: number, h: number): string {
  return `<svg viewBox="0 0 26 32" width="${w}" height="${h}" xmlns="http://www.w3.org/2000/svg">
    <ellipse cx="13" cy="30.5" rx="6.5" ry="0.9" fill="rgba(0,0,0,0.22)"/>
    <path d="M 19 7 L 7 7 Q 3 7 3 11 L 3 24 Q 3 28 7 28 L 16 28" stroke="${wireDark}" stroke-width="3.2" stroke-linecap="round" stroke-linejoin="round" fill="none" opacity="0.55"/>
    <path d="M 19 11 L 12 11 Q 8 11 8 14 L 8 26" stroke="${wireDark}" stroke-width="3.2" stroke-linecap="round" stroke-linejoin="round" fill="none" opacity="0.55"/>
    <path d="M 19 7 L 7 7 Q 3 7 3 11 L 3 24 Q 3 28 7 28 L 16 28" stroke="${wire}" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" fill="none"/>
    <path d="M 19 11 L 12 11 Q 8 11 8 14 L 8 26" stroke="${wire}" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" fill="none"/>
    <path d="M 19 6.2 L 7.5 6.2 Q 4 6.2 4 9" stroke="#FFFFFF" stroke-width="0.6" stroke-linecap="round" fill="none" opacity="0.7"/>
    <path d="M 19 10.2 L 12 10.2" stroke="#FFFFFF" stroke-width="0.5" stroke-linecap="round" fill="none" opacity="0.55"/>
    <ellipse cx="10" cy="12" rx="3.2" ry="3.7" fill="white" stroke="#1A1A1A" stroke-width="0.6"/>
    <ellipse cx="17" cy="12" rx="3.2" ry="3.7" fill="white" stroke="#1A1A1A" stroke-width="0.6"/>
    <ellipse cx="10.6" cy="12.6" rx="1.3" ry="1.7" fill="#1A1A1A"/>
    <ellipse cx="17.6" cy="12.6" rx="1.3" ry="1.7" fill="#1A1A1A"/>
    <circle cx="10" cy="11.7" r="0.55" fill="white"/>
    <circle cx="17" cy="11.7" r="0.55" fill="white"/>
    <path d="M 5.5 7.5 Q 10 5.5 13 7" stroke="#1A1A1A" stroke-width="1.5" stroke-linecap="round" fill="none"/>
    <path d="M 14 7 Q 17 5.5 21.5 7.5" stroke="#1A1A1A" stroke-width="1.5" stroke-linecap="round" fill="none"/>
  </svg>`
}

// ── Default stop icon (shared, created once) ────────────────────────────────

export const defaultStopIcon = L.divIcon({
  className: 'bg-transparent border-none',
  html: `
    <div class="flex items-center justify-center w-6 h-6 rounded-full border-2 border-white shadow-sm z-20 bg-slate-500 dark:bg-slate-600 text-white">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-3.5 h-3.5">
        <path d="${BUS_STOP_PATH}"/>
      </svg>
    </div>
  `,
  iconSize: [24, 24],
  iconAnchor: [12, 12],
  popupAnchor: [0, -12],
})

// ── Stop icon ────────────────────────────────────────────────────────────────

export function makeStopIcon(isFav: boolean, opts: IconThemeOptions): L.DivIcon {
  if (opts.easterEggActive) {
    const body = isFav ? '#f43f5e' : '#94a3b8'
    const pupil = isFav ? '#881337' : '#334155'
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div style="width:20px;height:26px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 12 16" width="18" height="24" xmlns="http://www.w3.org/2000/svg">
          <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="${body}"/>
          <circle cx="3.8" cy="8" r="1.3" fill="white"/>
          <circle cx="7.8" cy="8" r="1.3" fill="white"/>
          <circle cx="4.2" cy="8.4" r="0.7" fill="${pupil}"/>
          <circle cx="8.2" cy="8.4" r="0.7" fill="${pupil}"/>
        </svg>
      </div>`,
      iconSize: [20, 26],
      iconAnchor: [10, 24],
      popupAnchor: [0, -24],
    })
  }

  if (opts.traditionalActive) {
    const wire = isFav ? '#E89C2A' : '#9CA3AF'
    const wireDark = isFav ? '#A06010' : '#5C6470'
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div style="width:26px;height:32px;display:flex;align-items:flex-end;justify-content:center;filter:drop-shadow(1px 2px 1.5px rgba(0,0,0,0.35));">
        ${clippySvg(wire, wireDark, 24, 30)}
      </div>`,
      iconSize: [26, 32],
      iconAnchor: [13, 30],
      popupAnchor: [0, -30],
    })
  }

  return defaultStopIcon
}

// ── Selected stop icon ───────────────────────────────────────────────────────

export function makeSelectedStopIcon(opts: IconThemeOptions): L.DivIcon {
  if (opts.easterEggActive) {
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div class="animate-bounce" style="width:28px;height:36px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 12 16" width="26" height="34" xmlns="http://www.w3.org/2000/svg">
          <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="#22c55e"/>
          <circle cx="3.8" cy="8" r="1.3" fill="white"/>
          <circle cx="7.8" cy="8" r="1.3" fill="white"/>
          <circle cx="4.4" cy="8.5" r="0.6" fill="#15803d"/>
          <circle cx="8.4" cy="8.5" r="0.6" fill="#15803d"/>
        </svg>
      </div>`,
      iconSize: [28, 36], iconAnchor: [14, 34], popupAnchor: [0, -34],
    })
  }

  if (opts.traditionalActive) {
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div class="animate-bounce" style="width:34px;height:42px;display:flex;align-items:flex-end;justify-content:center;filter:drop-shadow(1px 2px 2px rgba(0,0,0,0.45));">
        ${clippySvg('#FFD64A', '#B07A10', 32, 40)}
      </div>`,
      iconSize: [34, 42], iconAnchor: [17, 40], popupAnchor: [0, -40],
    })
  }

  return L.divIcon({
    className: 'bg-transparent border-none',
    html: `
      <div class="flex items-center justify-center w-8 h-8 rounded-full border-2 border-white shadow-lg z-50 bg-emerald-500 text-white animate-bounce">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-4 h-4">
          <path d="${BUS_STOP_PATH}"/>
        </svg>
      </div>
    `,
    iconSize: [32, 32], iconAnchor: [16, 16], popupAnchor: [0, -16],
  })
}

// ── Highlight icon (route-highlighted stops) ─────────────────────────────────

export function makeHighlightIcon(
  color: 'green' | 'purple' | 'red' | 'gray',
  opts: IconThemeOptions,
): L.DivIcon {
  const bg =
    color === 'green' ? '#10b981'
    : color === 'purple' ? '#a855f7'
    : color === 'red' ? '#f43f5e'
    : '#64748b'

  if (opts.easterEggActive) {
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div style="width:24px;height:30px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 12 16" width="22" height="28" xmlns="http://www.w3.org/2000/svg">
          <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="${bg}"/>
          <circle cx="3.8" cy="8" r="1.3" fill="white"/>
          <circle cx="7.8" cy="8" r="1.3" fill="white"/>
          <circle cx="4.2" cy="8.4" r="0.7" fill="rgba(0,0,0,0.65)"/>
          <circle cx="8.2" cy="8.4" r="0.7" fill="rgba(0,0,0,0.65)"/>
        </svg>
      </div>`,
      iconSize: [24, 30],
      iconAnchor: [12, 28],
    })
  }

  if (opts.traditionalActive) {
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div style="width:28px;height:34px;display:flex;align-items:flex-end;justify-content:center;filter:drop-shadow(1px 2px 1.5px rgba(0,0,0,0.4));">
        ${clippySvg(bg, bg, 26, 32)}
      </div>`,
      iconSize: [28, 34],
      iconAnchor: [14, 32],
    })
  }

  return L.divIcon({
    className: 'bg-transparent border-none !overflow-visible',
    html: `<div style="width:24px;height:24px;border-radius:50%;background:${bg};border:2px solid white;box-shadow:0 1px 4px rgba(0,0,0,0.22);display:flex;align-items:center;justify-content:center;">
      <svg viewBox="0 0 24 24" fill="white" width="14" height="14"><path d="${BUS_STOP_PATH}"/></svg>
    </div>`,
    iconSize: [24, 24],
    iconAnchor: [12, 12],
  })
}

// ── Vehicle marker HTML ──────────────────────────────────────────────────────

export function getVehicleMarkerHtml(
  vehicle: DisplayVehicle,
  resolvedColor: string,
  isStopView: boolean,
  showStopInfo: boolean,
  opts: IconThemeOptions,
): string {
  const routeName = vehicle.route_short_name || ''
  const routeFontSize = routeName.length >= 4 ? 8 : routeName.length >= 3 ? 9 : 11
  const roundedSpeed = Math.round(vehicle.speed)
  const titleText = routeName ? `${routeName} • ${vehicle.label}` : vehicle.label
  const heading = vehicle.heading || 0

  if (opts.easterEggActive) {
    const rotation = heading - 90
    const chomper = `<div style="transform:rotate(${rotation}deg);flex-shrink:0;">
      <div class="hungry-chomp" style="width:${isStopView ? 36 : 32}px;height:${isStopView ? 36 : 32}px;background-color:${resolvedColor};border-radius:50%;border:2px solid white;box-shadow:0 2px 8px rgba(0,0,0,0.28);"></div>
    </div>`

    if (isStopView) {
      return `
        <div style="position:relative;display:flex;flex-direction:column;align-items:center;gap:1px;">
          ${chomper}
          <div style="background-color:${resolvedColor};color:white;font-size:${routeFontSize}px;font-weight:900;padding:0 3px;border-radius:3px;border:1px solid rgba(255,255,255,0.8);line-height:1.5;white-space:nowrap;">${routeName}</div>
          ${showStopInfo ? `
            <div class="absolute" style="left:42px;top:0;background:rgba(15,23,42,0.9);color:#f1f5f9;padding:4px 10px;border-radius:6px;box-shadow:0 2px 8px rgba(0,0,0,0.3);display:flex;flex-direction:column;white-space:nowrap;z-index:20;pointer-events:none;">
              <span style="font-weight:700;font-size:14px;">${titleText}</span>
              <span style="font-size:12px;color:#94a3b8;">${roundedSpeed} km/h</span>
            </div>
          ` : ''}
        </div>`
    }

    return `
      <div style="position:relative;display:flex;align-items:center;">
        ${chomper}
        <div class="absolute" style="left:40px;background:rgba(15,23,42,0.9);color:#f1f5f9;padding:4px 10px;border-radius:6px;box-shadow:0 2px 8px rgba(0,0,0,0.3);display:flex;flex-direction:column;white-space:nowrap;z-index:20;pointer-events:none;">
          <span style="font-weight:700;font-size:14px;">${titleText}</span>
          <span style="font-size:12px;color:#94a3b8;">${roundedSpeed} km/h</span>
        </div>
      </div>`
  }

  if (opts.traditionalActive) {
    const sz = isStopView ? 36 : 32
    const cursorRotation = heading + 45

    // Classic XP mouse arrow — fill = route color, black outline (the cursor IS the bus)
    const cursorSvg = `<svg viewBox="-1 -1 14 20" width="${sz}" height="${sz}" xmlns="http://www.w3.org/2000/svg" style="transform:rotate(${cursorRotation}deg);transform-origin:center;display:block;filter:drop-shadow(1px 2px 2px rgba(0,0,0,0.45));">
      <path d="M 0 0 L 0 16 L 4 12 L 6 18 L 8 17 L 6 11 L 11 11 Z" fill="${resolvedColor}" stroke="black" stroke-width="1" stroke-linejoin="miter" stroke-linecap="round"/>
      <path d="M 1 1 L 1 14" stroke="rgba(255,255,255,0.55)" stroke-width="0.7" stroke-linecap="round" fill="none"/>
    </svg>`

    const cursorBox = `<div style="width:${sz}px;height:${sz}px;display:flex;align-items:center;justify-content:center;flex-shrink:0;">${cursorSvg}</div>`

    // XP tooltip styling: pale-yellow infobox with 1px black border, Tahoma 11px
    const xpTip = (extraStyle: string = '') => `<div class="absolute" style="${extraStyle}background:#FFFFE0;color:#000000;padding:3px 7px;border:1px solid #000000;box-shadow:1px 1px 0 rgba(0,0,0,0.35);display:flex;flex-direction:column;white-space:nowrap;z-index:20;pointer-events:none;font-family:'Tahoma','Trebuchet MS',sans-serif;line-height:1.3;">
      <span style="font-weight:700;font-size:11px;">${titleText}</span>
      <span style="font-size:11px;color:#404040;">${roundedSpeed} km/h</span>
    </div>`

    // XP-square route badge for stopView
    const routeBadge = `<div style="background-color:${resolvedColor};color:white;font-size:${routeFontSize}px;font-weight:700;padding:1px 5px;border-radius:0;border:1px solid #000000;line-height:1.4;white-space:nowrap;font-family:'Tahoma','Trebuchet MS',sans-serif;box-shadow:1px 1px 0 rgba(0,0,0,0.3);">${routeName}</div>`

    if (isStopView) {
      return `
        <div style="position:relative;display:flex;flex-direction:column;align-items:center;gap:2px;">
          ${cursorBox}
          ${routeBadge}
          ${showStopInfo ? xpTip(`left:${sz + 6}px;top:0;`) : ''}
        </div>`
    }

    return `
      <div style="position:relative;display:flex;align-items:center;">
        ${cursorBox}
        ${xpTip(`left:${sz + 4}px;`)}
      </div>`
  }

  return isStopView
    ? `
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
          <div class="absolute left-10 bg-slate-900/90 dark:bg-slate-800/90 text-slate-100 px-2.5! py-1! rounded-md shadow-md flex flex-col whitespace-nowrap z-20 pointer-events-none">
            <span class="font-bold text-sm tracking-wide">${titleText}</span>
            <span class="text-xs text-slate-400">${roundedSpeed} km/h</span>
          </div>
        ` : ''}
      </div>
    `
    : `
      <div class="relative flex items-center">
        <div class="flex items-center justify-center w-8 h-8 rounded-full border-2 border-white shadow-md z-30 shrink-0"
             style="background-color: ${resolvedColor};">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="white" class="w-4 h-4"
               style="transform: rotate(${heading}deg);">
            <path d="M12 2L21 21l-9-4-9 4 9-19z" stroke="currentColor" stroke-width="1" stroke-linejoin="round"/>
          </svg>
        </div>
        <div class="absolute left-10 bg-slate-900/90 dark:bg-slate-800/90 text-slate-100 px-2.5! py-1! rounded-md shadow-md flex flex-col whitespace-nowrap z-20 pointer-events-none">
          <span class="font-bold text-sm tracking-wide">${titleText}</span>
          <span class="text-xs text-slate-400">${roundedSpeed} km/h</span>
        </div>
      </div>
    `
}
