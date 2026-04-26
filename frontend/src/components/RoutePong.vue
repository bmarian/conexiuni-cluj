<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

const props = defineProps<{
  routeShortName: string
  routeColor: string
}>()
const emit = defineEmits<{ exit: [] }>()

const canvasEl = ref<HTMLCanvasElement | null>(null)
const gameOver = ref<'player' | 'ai' | null>(null)

const H = 220
const PW = 10
const PH = 55
const BR = 15
const BASE = 2.5
const WIN_SCORE = 5

let W = 300
let bx = W / 2, by = H / 2, vx = BASE, vy = 1
let lpy = H / 2 - PH / 2
let rpy = H / 2 - PH / 2
let scoreL = 0, scoreR = 0
let pauseFrames = 0
let raf = 0

function launch() {
  bx = W / 2
  by = H / 2
  const angle = (Math.random() * 0.35 - 0.175) * Math.PI
  const dir = Math.random() < 0.5 ? 1 : -1
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
  // AI always tracks — even during pause so it recenters
  const aiTarget = by - PH / 2
  const aiStep = (aiTarget - lpy) * 0.07
  lpy += Math.max(-2.8, Math.min(2.8, aiStep))
  lpy = Math.max(0, Math.min(H - PH, lpy))

  if (pauseFrames > 0) { pauseFrames--; return }

  bx += vx
  by += vy

  // Top / bottom walls
  if (by - BR < 0)  { by = BR;     vy =  Math.abs(vy) }
  if (by + BR > H)  { by = H - BR; vy = -Math.abs(vy) }

  // Left paddle hit
  if (vx < 0 && bx - BR < PW && by > lpy && by < lpy + PH) {
    bx = PW + BR
    vx = Math.abs(vx) * 1.04
    vy += ((by - lpy) / PH - 0.5) * BASE * 2
    clampVy()
  }

  // Right paddle hit
  if (vx > 0 && bx + BR > W - PW && by > rpy && by < rpy + PH) {
    bx = W - PW - BR
    vx = -Math.abs(vx) * 1.04
    vy += ((by - rpy) / PH - 0.5) * BASE * 2
    clampVy()
  }

  // Score
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
  for (let i = 0; i < WIN_SCORE; i++) {
    ctx.beginPath()
    ctx.arc(cx + (i - 2) * 10, cy, 3, 0, Math.PI * 2)
    ctx.fillStyle = i < filled ? 'rgba(255,255,255,0.85)' : 'rgba(255,255,255,0.18)'
    ctx.fill()
  }
}

function draw() {
  const ctx = canvasEl.value?.getContext('2d')
  if (!ctx) return

  // Court
  ctx.fillStyle = '#0f172a'
  ctx.fillRect(0, 0, W, H)

  // Center dashed line
  ctx.save()
  ctx.setLineDash([5, 7])
  ctx.strokeStyle = 'rgba(255,255,255,0.10)'
  ctx.lineWidth = 2
  ctx.beginPath()
  ctx.moveTo(W / 2, 0)
  ctx.lineTo(W / 2, H)
  ctx.stroke()
  ctx.restore()

  // Score numbers
  ctx.fillStyle = 'rgba(255,255,255,0.30)'
  ctx.font = 'bold 28px ui-monospace, monospace'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  ctx.fillText(`${scoreL}`, W / 2 - 34, 8)
  ctx.fillText(`${scoreR}`, W / 2 + 34, 8)

  // Goal pips (5 dots per side)
  drawPips(ctx, scoreL, W / 2 - 34, 48)
  drawPips(ctx, scoreR, W / 2 + 34, 48)

  // Paddles
  ctx.fillStyle = 'rgba(255,255,255,0.88)'
  rr(ctx, 0, lpy, PW, PH, 3); ctx.fill()
  rr(ctx, W - PW, rpy, PW, PH, 3); ctx.fill()

  // Ball badge
  ctx.font = 'bold 13px ui-sans-serif, sans-serif'
  const tw = ctx.measureText(props.routeShortName).width
  const bw = Math.max(tw + 18, 34)
  const bh = 26

  ctx.save()
  ctx.shadowColor = props.routeColor
  ctx.shadowBlur = 18
  ctx.fillStyle = props.routeColor
  rr(ctx, bx - bw / 2, by - bh / 2, bw, bh, 7)
  ctx.fill()
  ctx.restore()

  ctx.fillStyle = '#fff'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(props.routeShortName, bx, by + 1)
}

function loop() {
  tick()
  draw()
  raf = requestAnimationFrame(loop)
}

function restart() {
  scoreL = 0
  scoreR = 0
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
    canvasEl.value.width = W
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

    <div v-if="gameOver" class="pong-over">
      <p class="pong-over-title">{{ gameOver === 'player' ? '🎉 You win!' : '😅 AI wins' }}</p>
      <p class="pong-over-score">{{ scoreR }} – {{ scoreL }}</p>
      <div class="pong-over-actions">
        <button class="pong-over-btn pong-over-btn-primary" @click="restart">Play again</button>
        <button class="pong-over-btn pong-over-btn-ghost" @click="emit('exit')">Exit</button>
      </div>
    </div>

    <button v-else class="pong-exit" @click="emit('exit')" title="Exit (Esc)">✕</button>
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
  background: rgba(255, 255, 255, 0.12);
  color: rgba(255, 255, 255, 0.45);
  font-size: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border: none;
  transition: background 0.15s, color 0.15s;
}
.pong-exit:hover {
  background: rgba(255, 255, 255, 0.22);
  color: rgba(255, 255, 255, 0.9);
}

/* Game-over overlay */
.pong-over {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  background: rgba(0, 0, 0, 0.72);
  cursor: default;
}

.pong-over-title {
  font-size: 1.35rem;
  font-weight: 900;
  color: #fff;
  letter-spacing: -0.01em;
}

.pong-over-score {
  font-size: 1rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: rgba(255, 255, 255, 0.45);
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
.pong-over-btn:hover { opacity: 0.85; }

.pong-over-btn-primary {
  background: #fff;
  color: #0f172a;
}

.pong-over-btn-ghost {
  background: rgba(255, 255, 255, 0.12);
  color: rgba(255, 255, 255, 0.7);
}
</style>
