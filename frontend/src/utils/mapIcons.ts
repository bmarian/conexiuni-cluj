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
    const c = isFav ? '#BB1C2A' : '#6B1212'
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div style="width:20px;height:26px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 20 26" width="18" height="24" xmlns="http://www.w3.org/2000/svg">
          <line x1="10" y1="26" x2="10" y2="6" stroke="${c}" stroke-width="1.8" stroke-linecap="round"/>
          <ellipse cx="10" cy="4" rx="2.5" ry="4" fill="${c}"/>
          <ellipse cx="6.5" cy="9" rx="3" ry="1.5" fill="${c}" transform="rotate(-40 6.5 9)"/>
          <ellipse cx="5.5" cy="14" rx="3" ry="1.5" fill="${c}" transform="rotate(-30 5.5 14)"/>
          <ellipse cx="13.5" cy="9" rx="3" ry="1.5" fill="${c}" transform="rotate(40 13.5 9)"/>
          <ellipse cx="14.5" cy="14" rx="3" ry="1.5" fill="${c}" transform="rotate(30 14.5 14)"/>
        </svg>
      </div>`,
      iconSize: [20, 26],
      iconAnchor: [10, 24],
      popupAnchor: [0, -24],
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
      html: `<div class="animate-bounce" style="width:28px;height:36px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 20 26" width="26" height="34" xmlns="http://www.w3.org/2000/svg">
          <line x1="10" y1="26" x2="10" y2="6" stroke="#2D8B2D" stroke-width="1.8" stroke-linecap="round"/>
          <ellipse cx="10" cy="4" rx="2.5" ry="4" fill="#2D8B2D"/>
          <ellipse cx="6.5" cy="9" rx="3" ry="1.5" fill="#2D8B2D" transform="rotate(-40 6.5 9)"/>
          <ellipse cx="5.5" cy="14" rx="3" ry="1.5" fill="#2D8B2D" transform="rotate(-30 5.5 14)"/>
          <ellipse cx="13.5" cy="9" rx="3" ry="1.5" fill="#2D8B2D" transform="rotate(40 13.5 9)"/>
          <ellipse cx="14.5" cy="14" rx="3" ry="1.5" fill="#2D8B2D" transform="rotate(30 14.5 14)"/>
        </svg>
      </div>`,
      iconSize: [28, 36], iconAnchor: [14, 34], popupAnchor: [0, -34],
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
      html: `<div style="width:24px;height:30px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 20 26" width="22" height="28" xmlns="http://www.w3.org/2000/svg">
          <line x1="10" y1="26" x2="10" y2="6" stroke="${bg}" stroke-width="1.8" stroke-linecap="round"/>
          <ellipse cx="10" cy="4" rx="2.5" ry="4" fill="${bg}"/>
          <ellipse cx="6.5" cy="9" rx="3" ry="1.5" fill="${bg}" transform="rotate(-40 6.5 9)"/>
          <ellipse cx="5.5" cy="14" rx="3" ry="1.5" fill="${bg}" transform="rotate(-30 5.5 14)"/>
          <ellipse cx="13.5" cy="9" rx="3" ry="1.5" fill="${bg}" transform="rotate(40 13.5 9)"/>
          <ellipse cx="14.5" cy="14" rx="3" ry="1.5" fill="${bg}" transform="rotate(30 14.5 14)"/>
        </svg>
      </div>`,
      iconSize: [24, 30],
      iconAnchor: [12, 28],
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
    const tractorW = Math.round(sz * 0.62)
    const tractorH = Math.round(tractorW * 16 / 22)

    const tractorSvg = `<svg viewBox="0 0 22 16" width="${tractorW}" height="${tractorH}" xmlns="http://www.w3.org/2000/svg">
      <circle cx="5.5" cy="11" r="3.5" fill="none" stroke="white" stroke-width="1.2"/>
      <circle cx="5.5" cy="11" r="1.4" fill="rgba(255,255,255,0.3)"/>
      <line x1="5.5" y1="7.5" x2="5.5" y2="14.5" stroke="white" stroke-width="0.6" opacity="0.65"/>
      <line x1="2" y1="11" x2="9" y2="11" stroke="white" stroke-width="0.6" opacity="0.65"/>
      <rect x="4" y="7.5" width="14" height="4.5" rx="1" fill="white"/>
      <rect x="4" y="2" width="7.5" height="8.5" rx="1.2" fill="white"/>
      <rect x="5.3" y="3.3" width="4.8" height="3.2" rx="0.4" fill="rgba(0,0,0,0.22)"/>
      <rect x="10.2" y="0.5" width="1.8" height="4" rx="0.4" fill="white"/>
      <rect x="11.5" y="8.5" width="6" height="4" rx="1" fill="rgba(255,255,255,0.85)"/>
      <circle cx="17.5" cy="11.5" r="2.8" fill="none" stroke="white" stroke-width="1.2"/>
      <circle cx="17.5" cy="11.5" r="1.1" fill="rgba(255,255,255,0.3)"/>
      <line x1="17.5" y1="8.7" x2="17.5" y2="14.3" stroke="white" stroke-width="0.6" opacity="0.65"/>
      <line x1="14.7" y1="11.5" x2="20.3" y2="11.5" stroke="white" stroke-width="0.6" opacity="0.65"/>
    </svg>`

    const circle = `<div style="position:relative;width:${sz}px;height:${sz}px;border-radius:50%;background:${resolvedColor};border:2.5px solid white;box-shadow:0 2px 8px rgba(0,0,0,0.28);display:flex;align-items:center;justify-content:center;flex-shrink:0;">
      ${tractorSvg}
      <div style="position:absolute;bottom:-2px;right:-2px;width:13px;height:13px;border-radius:50%;background:rgba(28,16,8,0.85);border:1.5px solid white;display:flex;align-items:center;justify-content:center;">
        <svg viewBox="0 0 24 24" fill="white" width="8" height="8" style="transform:rotate(${heading}deg);">
          <path d="M12 2L21 21l-9-4-9 4 9-19z" stroke="white" stroke-width="1" stroke-linejoin="round"/>
        </svg>
      </div>
    </div>`

    if (isStopView) {
      return `
        <div style="position:relative;display:flex;flex-direction:column;align-items:center;gap:1px;">
          ${circle}
          <div style="background-color:${resolvedColor};color:white;font-size:${routeFontSize}px;font-weight:900;padding:0 3px;border-radius:3px;border:1px solid rgba(255,255,255,0.8);line-height:1.5;white-space:nowrap;">${routeName}</div>
          ${showStopInfo ? `
            <div class="absolute" style="left:${sz + 8}px;top:0;background:rgba(28,16,8,0.92);color:#F0DFC0;padding:4px 10px;border-radius:6px;box-shadow:0 2px 8px rgba(0,0,0,0.3);display:flex;flex-direction:column;white-space:nowrap;z-index:20;pointer-events:none;">
              <span style="font-weight:700;font-size:14px;">${titleText}</span>
              <span style="font-size:12px;color:#C8B090;">${roundedSpeed} km/h</span>
            </div>
          ` : ''}
        </div>`
    }

    return `
      <div style="position:relative;display:flex;align-items:center;">
        ${circle}
        <div class="absolute" style="left:${sz + 8}px;background:rgba(28,16,8,0.92);color:#F0DFC0;padding:4px 10px;border-radius:6px;box-shadow:0 2px 8px rgba(0,0,0,0.3);display:flex;flex-direction:column;white-space:nowrap;z-index:20;pointer-events:none;">
          <span style="font-weight:700;font-size:14px;">${titleText}</span>
          <span style="font-size:12px;color:#C8B090;">${roundedSpeed} km/h</span>
        </div>
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
