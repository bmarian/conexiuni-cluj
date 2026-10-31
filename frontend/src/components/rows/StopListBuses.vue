<script setup lang="ts">
import {onMounted, onUnmounted, ref, watch} from 'vue'
import type {LiveVehicle} from '@/types/tranzy.ts'
import {clamp} from '@/utils/geo.ts'
import {easeInOut, SWEEP_MS} from '@/utils/lineSweep.ts'
import {MODE_ICON} from '@/utils/modeIcons.ts'
import type {Mode} from '@/utils/modes.ts'
import {reducedMotion, squash} from '@/utils/motion.ts'
import {rowAlong} from '@/utils/stopListBuses.ts'
import {locate, VehicleTrack, type Path} from '@/utils/vehicleMotion.ts'

const props = defineProps<{path: Path | null; stopsAt: (number | null)[]; vehicles: LiveVehicle[]; target: number; color: string; mode: Mode}>()

const root = ref<HTMLElement>()
const rowPx = ref(0)
const railPx = ref(0)
const arrived = ref(0)
const mark = ref('')
const ids = ref<number[]>([])
const wraps = new Map<number, HTMLElement>()
const bodies = new Map<number, HTMLElement>()

type Shown = {track: VehicleTrack | null; s: number; row: number | null}
const shown = new Map<number, Shown>()
let frame = 0
let stretchSince: number | null = null

const rows = () => [...(root.value?.parentElement?.querySelectorAll<HTMLElement>('.stop-line') ?? [])]
const y = (row: number) => row * rowPx.value + rowPx.value / 2

function lightTarget() {
  const row = rows()[props.target]
  if (!row || reducedMotion()) return
  mark.value = getComputedStyle(row).getPropertyValue('--mark')
  arrived.value++
}

// A new stretch fills down from the bus in the time a line grows on the map.
function paint(lead: number | null, nowMs: number) {
  const target = props.target
  const on = lead !== null && target >= 0
  if (!on) stretchSince = null
  else stretchSince ??= nowMs
  const grow = !on || reducedMotion() ? 1 : easeInOut(clamp((nowMs - stretchSince!) / SWEEP_MS, 0, 1))
  const from = lead ?? 0
  const to = from + (target - from) * grow
  rows().forEach((row, i) => {
    for (const half of row.querySelectorAll<HTMLElement>('.paint')) {
      const start = half.classList.contains('is-top') ? i - 0.5 : i
      const cutTop = clamp((from - start) * 2, 0, 1)
      const cutBottom = clamp((start + 0.5 - to) * 2, 0, 1)
      half.style.clipPath = on && cutTop + cutBottom < 1 ? `inset(${cutTop * 100}% 0 ${cutBottom * 100}% 0)` : ''
    }
  })
}

function tick() {
  frame = 0
  if (!rowPx.value) return
  const nowMs = Date.now()
  let lead: number | null = null
  for (const [id, bus] of shown) {
    if (bus.track) {
      bus.track.step(nowMs)
      bus.s = bus.track.s
    }
    const row = rowAlong(props.stopsAt, bus.s)
    const was = bus.row
    bus.row = row
    const wrap = wraps.get(id)
    if (wrap) wrap.style.translate = `0 ${y(row)}px`
    if (was !== null && Math.floor(row) > Math.floor(was)) {
      squash(bodies.get(id))
      if (was < props.target && row >= props.target) lightTarget()
    }
    if (row <= props.target && (lead === null || row > lead)) lead = row
  }
  paint(lead, nowMs)
  if (shown.size) frame = requestAnimationFrame(tick)
}

watch(
  () => [props.vehicles, props.path] as const,
  ([vehicles, path]) => {
    const nowMs = Date.now()
    const seen = new Set<number>()
    for (const v of path ? vehicles : []) {
      const at = {lat: v.latitude, lng: v.longitude}
      const fixMs = Date.parse(v.timestamp)
      let bus = shown.get(v.id)
      if (!bus) shown.set(v.id, (bus = {track: null, s: 0, row: null}))
      if (bus.track?.path !== path) bus.track = new VehicleTrack(path!)
      if (Number.isNaN(fixMs) || !bus.track.fix(at, fixMs, v.speed ?? 0, v.heading, nowMs)) {
        bus.track = null
        const spot = locate(path!, at)
        if (!spot) continue
        bus.s = spot.s
      }
      seen.add(v.id)
    }
    for (const id of shown.keys()) if (!seen.has(id)) shown.delete(id)
    ids.value = [...shown.keys()]
    if (!shown.size) paint(null, nowMs)
    else if (!frame) frame = requestAnimationFrame(tick)
  },
  {immediate: true},
)

onMounted(() => {
  const row = rows()[0]
  const rail = row?.querySelector<HTMLElement>('.rail')
  if (!row || !rail) return
  rowPx.value = row.offsetHeight
  railPx.value = rail.offsetLeft + rail.offsetWidth / 2
  if (shown.size && !frame) frame = requestAnimationFrame(tick)
})

onUnmounted(() => cancelAnimationFrame(frame))
</script>

<template>
  <div ref="root" class="stop-list-buses" :style="{'--rail': `${railPx}px`, '--line': color}" aria-hidden="true">
    <template v-if="rowPx">
      <span v-if="arrived" :key="arrived" class="arrived" :style="{top: `${target * rowPx}px`, height: `${rowPx}px`, '--mark': mark}"></span>
      <span v-for="id in ids" :key="id" :ref="(el) => (el ? wraps.set(id, el as HTMLElement) : wraps.delete(id))" class="bus">
        <span :ref="(el) => (el ? bodies.set(id, el as HTMLElement) : bodies.delete(id))" class="bus-body">
          <svg viewBox="0 0 24 24" v-html="MODE_ICON[mode]"></svg>
        </span>
      </span>
    </template>
  </div>
</template>

<style scoped>
.stop-list-buses {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.bus {
  position: absolute;
  top: -11px;
  left: calc(var(--rail) - 11px);
}

.bus-body {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: 2px solid #fff;
  border-radius: 50%;
  background: var(--line);
  color: #fff;
  box-shadow: 0 1px 3px rgb(0 0 0 / 0.3);
}

.bus-body svg {
  width: 12px;
  height: 12px;
  fill: currentColor;
}

.arrived {
  position: absolute;
  left: 0;
  right: 0;
  animation: arrived calc(955ms * var(--pace-long)) ease-out forwards;
}

@keyframes arrived {
  from {
    background: color-mix(in srgb, var(--mark) 30%, transparent);
  }
  to {
    background: transparent;
  }
}
</style>
