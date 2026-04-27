<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings.ts'
import { useUserStore } from '@/stores/user.ts'

const props = defineProps<{
  routeShortName: string
  routeColor: string
}>()
const emit = defineEmits<{ exit: [] }>()

const { t } = useI18n()
const { easterEggActive, traditionalActive } = storeToRefs(useSettingsStore())
const { isDarkMode } = storeToRefs(useUserStore())

// canvasTheme and overlayTheme are computed so draw() always reads the current theme each rAF frame.
const canvasTheme = computed(() => {
  const h = easterEggActive.value
  const x = traditionalActive.value
  const d = isDarkMode.value
  if (x && d) return {
    court:     '#1A2030',
    paddle:    '#5BA1F0',
    ballBorder:'#FFFFFF',
    line:      'rgba(91,161,240,0.28)',
    score:     '#90B4E0',
    pipOn:     '#5BA1F0',
    pipOff:    'rgba(91,161,240,0.20)',
    isXp:      true,
  }
  if (x) return {
    court:     '#ECE9D8',
    paddle:    '#245EDC',
    ballBorder:'#000000',
    line:      'rgba(36,94,220,0.30)',
    score:     '#245EDC',
    pipOn:     '#245EDC',
    pipOff:    'rgba(36,94,220,0.18)',
    isXp:      true,
  }
  if (h && d) return {
    court:     '#1c1608',
    paddle:    'rgba(253,230,138,0.90)',
    ballBorder:'',
    line:      'rgba(253,230,138,0.10)',
    score:     'rgba(253,230,138,0.38)',
    pipOn:     'rgba(253,230,138,0.85)',
    pipOff:    'rgba(253,230,138,0.18)',
    isXp:      false,
  }
  if (h) return {
    court:     '#fefce8',
    paddle:    '#78350f',
    ballBorder:'',
    line:      'rgba(120,53,15,0.12)',
    score:     'rgba(120,53,15,0.42)',
    pipOn:     'rgba(120,53,15,0.78)',
    pipOff:    'rgba(120,53,15,0.16)',
    isXp:      false,
  }
  if (d) return {
    court:     '#0f172a',
    paddle:    'rgba(255,255,255,0.88)',
    ballBorder:'',
    line:      'rgba(255,255,255,0.10)',
    score:     'rgba(255,255,255,0.30)',
    pipOn:     'rgba(255,255,255,0.85)',
    pipOff:    'rgba(255,255,255,0.18)',
    isXp:      false,
  }
  return {
    court:     '#f1f5f9',
    paddle:    'rgba(15,23,42,0.72)',
    ballBorder:'',
    line:      'rgba(15,23,42,0.10)',
    score:     'rgba(15,23,42,0.33)',
    pipOn:     'rgba(15,23,42,0.72)',
    pipOff:    'rgba(15,23,42,0.16)',
    isXp:      false,
  }
})

const overlayTheme = computed(() => {
  const h = easterEggActive.value
  const x = traditionalActive.value
  const d = isDarkMode.value
  if (x && d) return {
    bg:    '#1A2030',
    title: '#90B4E0',
    score: '#5BA1F0',
  }
  if (x) return {
    bg:    '#ECE9D8',
    title: '#245EDC',
    score: '#316AC5',
  }
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

const exitBtnStyle = computed(() => {
  if (traditionalActive.value) {
    return {
      background: 'linear-gradient(to bottom, #F88080, #D04040 50%, #A82020)',
      color: '#FFFFFF',
      border: '1px solid #6B1010',
    }
  }
  return isDarkMode.value
    ? { background: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.45)' }
    : { background: 'rgba(15,23,42,0.09)',    color: 'rgba(15,23,42,0.48)'    }
})

const canvasEl  = ref<HTMLCanvasElement | null>(null)
const gameOver  = ref<'player' | 'ai' | null>(null)

const H         = 220
const PW        = 10
const PH        = 55
const BR        = 15
const WIN_SCORE = 5

// Game state lives outside Vue reactivity so mutations inside the rAF loop don't trigger re-renders.
let W              = 300
let BASE           = 150
let bx = W / 2, by = H / 2, vx = BASE, vy = 0
let lpy            = H / 2 - PH / 2
let rpy            = H / 2 - PH / 2
let scoreL         = 0, scoreR = 0
let pauseRemaining = 0
let lastTime       = 0
let raf            = 0
let running        = false

function launch() {
  bx = W / 2
  by = H / 2
  const angle = (Math.random() * 0.35 - 0.175) * Math.PI
  const dir   = Math.random() < 0.5 ? 1 : -1
  vx = dir * BASE * Math.cos(angle)
  vy = BASE * Math.sin(angle)
  pauseRemaining = 1.2
}

function clampVy() {
  const MAX = BASE * 2.2
  vy = Math.max(-MAX, Math.min(MAX, vy))
}

function endGame(winner: 'player' | 'ai') {
  running = false
  gameOver.value = winner
}

function tick(dt: number) {
  const want = by - PH / 2 - lpy
  lpy += Math.max(-BASE * 0.5 * dt, Math.min(BASE * 0.5 * dt, want))
  lpy = Math.max(0, Math.min(H - PH, lpy))

  if (pauseRemaining > 0) { pauseRemaining -= dt; return }

  bx += vx * dt
  by += vy * dt

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
    const x = cx + (i - 2) * 10
    ctx.fillStyle = i < filled ? t.pipOn : t.pipOff
    if (t.isXp) {
      ctx.fillRect(x - 3, cy - 3, 6, 6)
    } else {
      ctx.beginPath()
      ctx.arc(x, cy, 3, 0, Math.PI * 2)
      ctx.fill()
    }
  }
}

function draw() {
  const ctx = canvasEl.value?.getContext('2d')
  if (!ctx) return
  const t = canvasTheme.value

  // Court
  ctx.fillStyle = t.court
  ctx.fillRect(0, 0, W, H)

  // Center divider
  ctx.save()
  ctx.setLineDash(t.isXp ? [2, 4] : [5, 7])
  ctx.strokeStyle = t.line
  ctx.lineWidth = 2
  ctx.beginPath()
  ctx.moveTo(W / 2, 0)
  ctx.lineTo(W / 2, H)
  ctx.stroke()
  ctx.restore()

  // Score
  ctx.fillStyle = t.score
  ctx.font = t.isXp
    ? 'bold 24px Tahoma, "Trebuchet MS", sans-serif'
    : 'bold 28px ui-monospace, monospace'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  ctx.fillText(`${scoreL}`, W / 2 - 34, 8)
  ctx.fillText(`${scoreR}`, W / 2 + 34, 8)

  drawPips(ctx, scoreL, W / 2 - 34, 48)
  drawPips(ctx, scoreR, W / 2 + 34, 48)

  // Paddles
  ctx.fillStyle = t.paddle
  if (t.isXp) {
    ctx.fillRect(0, lpy, PW, PH)
    ctx.fillRect(W - PW, rpy, PW, PH)
  } else {
    rr(ctx, 0, lpy, PW, PH, 3); ctx.fill()
    rr(ctx, W - PW, rpy, PW, PH, 3); ctx.fill()
  }

  // Ball badge
  ctx.font = t.isXp
    ? 'bold 12px Tahoma, "Trebuchet MS", sans-serif'
    : 'bold 13px ui-sans-serif, sans-serif'
  const tw = ctx.measureText(props.routeShortName).width
  const bw = Math.max(tw + 18, 34)
  const bh = 26

  if (t.isXp) {
    ctx.fillStyle = props.routeColor
    ctx.fillRect(bx - bw / 2, by - bh / 2, bw, bh)
    ctx.strokeStyle = t.ballBorder
    ctx.lineWidth = 1
    ctx.strokeRect(bx - bw / 2 + 0.5, by - bh / 2 + 0.5, bw - 1, bh - 1)
  } else {
    ctx.save()
    ctx.shadowColor = props.routeColor
    ctx.shadowBlur  = 18
    ctx.fillStyle   = props.routeColor
    rr(ctx, bx - bw / 2, by - bh / 2, bw, bh, 7)
    ctx.fill()
    ctx.restore()
  }

  ctx.fillStyle    = '#fff'
  ctx.textAlign    = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(props.routeShortName, bx, by + 1)
}

function loop(ts: number) {
  if (!running) return
  const dt = lastTime === 0 ? 0 : Math.min((ts - lastTime) / 1000, 0.05)
  lastTime = ts
  tick(dt)
  draw()
  if (running) raf = requestAnimationFrame(loop)
}

function restart() {
  scoreL = 0; scoreR = 0
  gameOver.value = null
  lpy = H / 2 - PH / 2
  rpy = H / 2 - PH / 2
  lastTime = 0
  running = true
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
    W    = canvasEl.value.offsetWidth || 300
    BASE = W / 1.6
    canvasEl.value.width  = W
    canvasEl.value.height = H
    lpy = H / 2 - PH / 2
    rpy = H / 2 - PH / 2
    launch()
  }
  running = true
  raf = requestAnimationFrame(loop)
  window.addEventListener('keydown', onKey)
})

onUnmounted(() => {
  running = false
  cancelAnimationFrame(raf)
  window.removeEventListener('keydown', onKey)
})
</script>

<template>
  <div
    class="pong-wrap"
    :class="{ 'is-xp': traditionalActive }"
    @pointermove="onPointerMove"
    @touchmove.prevent="onTouchMove"
  >
    <div v-if="traditionalActive" class="pong-titlebar">
      <span class="pong-titlebar-text">🎮 Pong — {{ props.routeShortName }}</span>
    </div>

    <canvas ref="canvasEl" :height="H" class="pong-canvas" />

    <div v-if="gameOver" class="pong-over" :style="{ background: overlayTheme.bg }">
      <p class="pong-over-title" :style="{ color: overlayTheme.title }">
        {{ gameOver === 'player' ? t('pongYouWin') : t('pongAiWins') }}
      </p>
      <p class="pong-over-score" :style="{ color: overlayTheme.score }">
        {{ scoreR }} – {{ scoreL }}
      </p>
      <div class="pong-over-actions">
        <button
          class="pong-over-btn"
          :class="{ 'pong-over-btn-primary': true, 'is-xp': traditionalActive }"
          :style="!traditionalActive && overlayTheme.btnBg ? { background: overlayTheme.btnBg, color: overlayTheme.btnFg } : undefined"
          @click="restart"
        >{{ t('pongPlayAgain') }}</button>
        <button
          class="pong-over-btn"
          :class="{ 'is-xp': traditionalActive }"
          :style="!traditionalActive && overlayTheme.ghBg ? { background: overlayTheme.ghBg, color: overlayTheme.ghFg } : undefined"
          @click="emit('exit')"
        >{{ t('pongExit') }}</button>
      </div>
    </div>

    <button
      v-else
      class="pong-exit"
      :class="{ 'is-xp': traditionalActive }"
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

/* ── XP window chrome ─────────────────────────────────────────────────── */
.pong-wrap.is-xp {
  border-radius: 0;
  border: 1px solid #003C9C;
  box-shadow: 1px 1px 0 #FFFFFF inset, 0 2px 6px rgba(0,0,0,0.25);
  margin: 0.75rem 0 1.25rem;
}
:global(html.dark[data-traditional]) .pong-wrap.is-xp {
  border-color: #001E5C;
  box-shadow: 1px 1px 0 rgba(255,255,255,0.05) inset, 0 2px 8px rgba(0,0,0,0.5);
}

.pong-titlebar {
  height: 24px;
  background: linear-gradient(
    to bottom,
    #0058DA 0%,
    #2E84E8 6%,
    #1A6CD0 14%,
    #1056C0 50%,
    #0E54BE 51%,
    #1A66D0 95%,
    #0E4DAC 100%
  );
  display: flex;
  align-items: center;
  padding: 0 8px;
  border-bottom: 1px solid #003C9C;
}
:global(html.dark[data-traditional]) .pong-titlebar {
  background: linear-gradient(
    to bottom,
    #003478 0%,
    #1A6CD0 8%,
    #0F4FA8 50%,
    #0A3E90 51%,
    #1656B8 95%,
    #062A6C 100%
  );
  border-bottom-color: #001E5C;
}
.pong-titlebar-text {
  color: #FFFFFF;
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
  font-size: 11px;
  font-weight: 700;
  text-shadow: 1px 1px 1px rgba(0,0,0,0.45);
  letter-spacing: 0.02em;
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

/* XP close button — repositioned into the title bar's right edge */
.pong-exit.is-xp {
  top: 4px;
  right: 4px;
  width: 22px;
  height: 17px;
  border-radius: 0;
  font-size: 11px;
  font-weight: 700;
  font-family: 'Tahoma', sans-serif;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.4);
}
.pong-exit.is-xp:hover {
  opacity: 1;
  filter: brightness(1.15);
}

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
/* When XP titlebar is present, the overlay sits below it */
.pong-wrap.is-xp .pong-over {
  inset: 25px 0 0 0;
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

/* XP-style buttons in the message dialog */
.pong-over-btn.is-xp {
  border-radius: 0;
  background: linear-gradient(to bottom, #FDFDFB 0%, #ECE9D8 50%, #D6D2C0 100%);
  border: 1px solid #ACA899;
  color: #000000;
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
  font-size: 0.75rem;
  padding: 0.35rem 1.1rem;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.7);
  min-width: 80px;
}
.pong-over-btn.is-xp:hover {
  opacity: 1;
  background: linear-gradient(to bottom, #FFF5C8 0%, #FFE07A 50%, #F3C94E 100%);
  border-color: #D08020;
}
.pong-over-btn.is-xp.pong-over-btn-primary {
  background: linear-gradient(to bottom, #4A90E0 0%, #2470D4 50%, #1A52B8 100%);
  color: #FFFFFF;
  border-color: #003C9C;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.4);
}
.pong-over-btn.is-xp.pong-over-btn-primary:hover {
  background: linear-gradient(to bottom, #5BA1F0 0%, #316AC5 50%, #2558B0 100%);
  border-color: #003C9C;
}
:global(html.dark[data-traditional]) .pong-over-btn.is-xp {
  background: linear-gradient(to bottom, #2A2F40 0%, #1F2230 50%, #14182A 100%);
  color: #E0E6F2;
  border-color: #444A5C;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.07);
}
:global(html.dark[data-traditional]) .pong-over-btn.is-xp:hover {
  background: linear-gradient(to bottom, #3A4055 0%, #2A2F40 50%, #1F2230 100%);
  border-color: #4E88D8;
}
:global(html.dark[data-traditional]) .pong-over-btn.is-xp.pong-over-btn-primary {
  background: linear-gradient(to bottom, #4A88D8 0%, #2A66B8 50%, #1B4F90 100%);
  color: #FFFFFF;
  border-color: #1B3E78;
}
:global(html.dark[data-traditional]) .pong-over-btn.is-xp.pong-over-btn-primary:hover {
  background: linear-gradient(to bottom, #5BA1F0 0%, #3A7EC8 50%, #2A5FA0 100%);
}

/* XP overlay — opaque XP "message dialog" surface */
.pong-wrap.is-xp .pong-over {
  background: #ECE9D8 !important;
}
.pong-wrap.is-xp .pong-over-title {
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
  font-size: 1.05rem;
  letter-spacing: 0;
}
.pong-wrap.is-xp .pong-over-score {
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
}
:global(html.dark[data-traditional]) .pong-wrap.is-xp .pong-over {
  background: #1A2030 !important;
}
</style>
