<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useSettingsStore } from '@/stores/settings.ts'
import { useUserStore } from '@/stores/user.ts'

const props = defineProps<{
  routeShortName: string
  routeColor: string
}>()
const emit = defineEmits<{ exit: [] }>()

const { easterEggActive } = storeToRefs(useSettingsStore())
const { isDarkMode } = storeToRefs(useUserStore())

// ── Theme ────────────────────────────────────────────────────────────────────
// Read by draw() every rAF frame — always current.
const canvasTheme = computed(() => {
  const h = easterEggActive.value, d = isDarkMode.value
  if (h && d) return {
    court:     '#1c1608',
    paddle:    'rgba(253,230,138,0.90)',
    line:      'rgba(253,230,138,0.10)',
    score:     'rgba(253,230,138,0.38)',
    pipOn:     'rgba(253,230,138,0.85)',
    pipOff:    'rgba(253,230,138,0.18)',
  }
  if (h) return {
    court:     '#fefce8',
    paddle:    '#78350f',
    line:      'rgba(120,53,15,0.12)',
    score:     'rgba(120,53,15,0.42)',
    pipOn:     'rgba(120,53,15,0.78)',
    pipOff:    'rgba(120,53,15,0.16)',
  }
  if (d) return {
    court:     '#0f172a',
    paddle:    'rgba(255,255,255,0.88)',
    line:      'rgba(255,255,255,0.10)',
    score:     'rgba(255,255,255,0.30)',
    pipOn:     'rgba(255,255,255,0.85)',
    pipOff:    'rgba(255,255,255,0.18)',
  }
  return {
    court:     '#f1f5f9',
    paddle:    'rgba(15,23,42,0.72)',
    line:      'rgba(15,23,42,0.10)',
    score:     'rgba(15,23,42,0.33)',
    pipOn:     'rgba(15,23,42,0.72)',
    pipOff:    'rgba(15,23,42,0.16)',
  }
})

const overlayTheme = computed(() => {
  const h = easterEggActive.value, d = isDarkMode.value
  if (h && d) return {
    bg:      'rgba(28,22,8,0.82)',
    title:   '#fde68a',
    score:   'rgba(253,230,138,0.45)',
    btnBg:   '#fde68a',
    btnFg:   '#78350f',
    ghBg:    'rgba(253,230,138,0.14)',
    ghFg:    'rgba(253,230,138,0.75)',
  }
  if (h) return {
    bg:      'rgba(254,252,232,0.88)',
    title:   '#78350f',
    score:   'rgba(120,53,15,0.48)',
    btnBg:   '#78350f',
    btnFg:   '#fef9c3',
    ghBg:    'rgba(120,53,15,0.12)',
    ghFg:    'rgba(120,53,15,0.72)',
  }
  if (d) return {
    bg:      'rgba(0,0,0,0.72)',
    title:   '#ffffff',
    score:   'rgba(255,255,255,0.42)',
    btnBg:   '#ffffff',
    btnFg:   '#0f172a',
    ghBg:    'rgba(255,255,255,0.12)',
    ghFg:    'rgba(255,255,255,0.68)',
  }
  return {
    bg:      'rgba(248,250,252,0.90)',
    title:   '#0f172a',
    score:   'rgba(15,23,42,0.42)',
    btnBg:   '#0f172a',
    btnFg:   '#f8fafc',
    ghBg:    'rgba(15,23,42,0.10)',
    ghFg:    'rgba(15,23,42,0.62)',
  }
})

const exitBtnStyle = computed(() => isDarkMode.value
  ? { background: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.45)' }
  : { background: 'rgba(15,23,42,0.09)',    color: 'rgba(15,23,42,0.48)'    }
)

// ── Game constants ────────────────────────────────────────────────────────────
const canvasEl  = ref<HTMLCanvasElement | null>(null)
const gameOver  = ref<'player' | 'ai' | null>(null)

const H         = 220
const PW        = 10
const PH        = 55
const BR        = 15
const BASE      = 2.5
const WIN_SCORE = 5

// ── Mutable game state (lives outside Vue reactivity — mutated in rAF) ────────
let W           = 300
let bx = W / 2, by = H / 2, vx = BASE, vy = 1
let lpy         = H / 2 - PH / 2
let rpy         = H / 2 - PH / 2
let scoreL      = 0, scoreR = 0
let pauseFrames = 0
let raf         = 0

function launch() {
  bx = W / 2
  by = H / 2
  const angle = (Math.random() * 0.35 - 0.175) * Math.PI
  const dir   = Math.random() < 0.5 ? 1 : -1
  vx = dir * BASE * Math.cos(angle)
  vy = BASE * Math.sin(angle)
  pauseFrames = 70
}

function clampVy() {
  const MAX = BASE * 2.2
  vy = Math.max(-MAX, Math.min(MAX, vy))
}

function endGame(winner: 'player' | 'ai') {
  cancelAnimationFrame(raf)
  raf = 0
  draw()
  gameOver.value = winner
}

function tick() {
  // AI tracks ball even during pause so it recenters
  const aiStep = (by - PH / 2 - lpy) * 0.07
  lpy += Math.max(-2.8, Math.min(2.8, aiStep))
  lpy = Math.max(0, Math.min(H - PH, lpy))

  if (pauseFrames > 0) { pauseFrames--; return }

  bx += vx
  by += vy

  if (by - BR < 0)  { by = BR;     vy =  Math.abs(vy) }
  if (by + BR > H)  { by = H - BR; vy = -Math.abs(vy) }

  if (vx < 0 && bx - BR < PW && by > lpy && by < lpy + PH) {
    bx = PW + BR
    vx = Math.abs(vx) * 1.04
    vy += ((by - lpy) / PH - 0.5) * BASE * 2
    clampVy()
  }

  if (vx > 0 && bx + BR > W - PW && by > rpy && by < rpy + PH) {
    bx = W - PW - BR
    vx = -Math.abs(vx) * 1.04
    vy += ((by - rpy) / PH - 0.5) * BASE * 2
    clampVy()
  }

  if (bx + BR < 0) {
    scoreR++
    if (scoreR >= WIN_SCORE) { endGame('player'); return }
    launch()
  }
  if (bx - BR > W) {
    scoreL++
    if (scoreL >= WIN_SCORE) { endGame('ai'); return }
    launch()
  }
}

function rr(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.lineTo(x + w - r, y)
  ctx.arcTo(x + w, y,     x + w, y + r,     r)
  ctx.lineTo(x + w, y + h - r)
  ctx.arcTo(x + w, y + h, x + w - r, y + h, r)
  ctx.lineTo(x + r, y + h)
  ctx.arcTo(x,     y + h, x,       y + h - r, r)
  ctx.lineTo(x, y + r)
  ctx.arcTo(x,     y,     x + r,   y,         r)
  ctx.closePath()
}

function drawPips(ctx: CanvasRenderingContext2D, filled: number, cx: number, cy: number) {
  const t = canvasTheme.value
  for (let i = 0; i < WIN_SCORE; i++) {
    ctx.beginPath()
    ctx.arc(cx + (i - 2) * 10, cy, 3, 0, Math.PI * 2)
    ctx.fillStyle = i < filled ? t.pipOn : t.pipOff
    ctx.fill()
  }
}

function draw() {
  const ctx = canvasEl.value?.getContext('2d')
  if (!ctx) return
  const t = canvasTheme.value

  ctx.fillStyle = t.court
  ctx.fillRect(0, 0, W, H)

  ctx.save()
  ctx.setLineDash([5, 7])
  ctx.strokeStyle = t.line
  ctx.lineWidth = 2
  ctx.beginPath()
  ctx.moveTo(W / 2, 0)
  ctx.lineTo(W / 2, H)
  ctx.stroke()
  ctx.restore()

  ctx.fillStyle = t.score
  ctx.font = 'bold 28px ui-monospace, monospace'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  ctx.fillText(`${scoreL}`, W / 2 - 34, 8)
  ctx.fillText(`${scoreR}`, W / 2 + 34, 8)

  drawPips(ctx, scoreL, W / 2 - 34, 48)
  drawPips(ctx, scoreR, W / 2 + 34, 48)

  ctx.fillStyle = t.paddle
  rr(ctx, 0, lpy, PW, PH, 3); ctx.fill()
  rr(ctx, W - PW, rpy, PW, PH, 3); ctx.fill()

  ctx.font = 'bold 13px ui-sans-serif, sans-serif'
  const tw = ctx.measureText(props.routeShortName).width
  const bw = Math.max(tw + 18, 34)
  const bh = 26

  ctx.save()
  ctx.shadowColor = props.routeColor
  ctx.shadowBlur  = 18
  ctx.fillStyle   = props.routeColor
  rr(ctx, bx - bw / 2, by - bh / 2, bw, bh, 7)
  ctx.fill()
  ctx.restore()

  ctx.fillStyle    = '#fff'
  ctx.textAlign    = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(props.routeShortName, bx, by + 1)
}

function loop() {
  tick()
  draw()
  raf = requestAnimationFrame(loop)
}

function restart() {
  scoreL = 0; scoreR = 0
  gameOver.value = null
  lpy = H / 2 - PH / 2
  rpy = H / 2 - PH / 2
  launch()
  raf = requestAnimationFrame(loop)
}

function moveRightPaddle(clientY: number, rect: DOMRect) {
  rpy = Math.max(0, Math.min(H - PH, (clientY - rect.top) / rect.height * H - PH / 2))
}

function onPointerMove(e: PointerEvent) {
  if (canvasEl.value) moveRightPaddle(e.clientY, canvasEl.value.getBoundingClientRect())
}

function onTouchMove(e: TouchEvent) {
  e.preventDefault()
  const t = e.touches[0]
  if (t && canvasEl.value) moveRightPaddle(t.clientY, canvasEl.value.getBoundingClientRect())
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') { emit('exit'); return }
  const step = 18
  if (e.key === 'ArrowUp')   rpy = Math.max(0, rpy - step)
  if (e.key === 'ArrowDown') rpy = Math.min(H - PH, rpy + step)
}

onMounted(() => {
  if (canvasEl.value) {
    W = canvasEl.value.offsetWidth || 300
    canvasEl.value.width  = W
    canvasEl.value.height = H
    lpy = H / 2 - PH / 2
    rpy = H / 2 - PH / 2
    launch()
  }
  raf = requestAnimationFrame(loop)
  window.addEventListener('keydown', onKey)
})

onUnmounted(() => {
  cancelAnimationFrame(raf)
  window.removeEventListener('keydown', onKey)
})
</script>

<template>
  <div
    class="pong-wrap"
    @pointermove="onPointerMove"
    @touchmove.prevent="onTouchMove"
  >
    <canvas ref="canvasEl" :height="H" class="pong-canvas" />

    <div v-if="gameOver" class="pong-over" :style="{ background: overlayTheme.bg }">
      <p class="pong-over-title" :style="{ color: overlayTheme.title }">
        {{ gameOver === 'player' ? '🎉 You win!' : '😅 AI wins' }}
      </p>
      <p class="pong-over-score" :style="{ color: overlayTheme.score }">
        {{ scoreR }} – {{ scoreL }}
      </p>
      <div class="pong-over-actions">
        <button
          class="pong-over-btn"
          :style="{ background: overlayTheme.btnBg, color: overlayTheme.btnFg }"
          @click="restart"
        >Play again</button>
        <button
          class="pong-over-btn"
          :style="{ background: overlayTheme.ghBg, color: overlayTheme.ghFg }"
          @click="emit('exit')"
        >Exit</button>
      </div>
    </div>

    <button
      v-else
      class="pong-exit"
      :style="exitBtnStyle"
      @click="emit('exit')"
      title="Exit (Esc)"
    >✕</button>
  </div>
</template>

<style scoped>
.pong-wrap {
  position: relative;
  border-radius: 0.875rem;
  overflow: hidden;
  margin: 1rem 0 1.5rem;
  cursor: none;
  user-select: none;
}

.pong-canvas {
  display: block;
  width: 100%;
  touch-action: none;
}

.pong-exit {
  position: absolute;
  top: 6px;
  right: 8px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  font-size: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border: none;
  transition: opacity 0.15s;
}
.pong-exit:hover { opacity: 0.7; }

.pong-over {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  cursor: default;
}

.pong-over-title {
  font-size: 1.35rem;
  font-weight: 900;
  letter-spacing: -0.01em;
}

.pong-over-score {
  font-size: 1rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  margin-bottom: 0.5rem;
}

.pong-over-actions {
  display: flex;
  gap: 0.5rem;
}

.pong-over-btn {
  padding: 0.4rem 1.1rem;
  border-radius: 0.625rem;
  font-size: 0.8rem;
  font-weight: 700;
  border: none;
  cursor: pointer;
  transition: opacity 0.15s;
}
.pong-over-btn:hover { opacity: 0.82; }
</style>
