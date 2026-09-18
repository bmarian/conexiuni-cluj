<script setup lang="ts">
import {ref, watch} from 'vue'
import {type ToastIcon, useSettingsStore} from '@/stores/settings'
import IconBellFilled from '@/components/icons/IconBellFilled.vue'
import ArcadeGhostIcon from '@/components/icons/ArcadeGhostIcon.vue'

defineProps<{ landscapeOpen?: boolean }>()

const settings = useSettingsStore()

const LEGACY_EMOJI: Record<ToastIcon, string> = {
  info: 'ℹ️',
  bell: '🔔',
  'bell-off': '🔕',
  joystick: '🕹️',
  ghost: '👻',
}

const SWIPE_DISMISS_PX = 40

const dragY = ref(0)
const dragging = ref(false)
let pointerId = -1
let startY = 0
let moved = false

watch(() => settings.toast?.id, () => {
  dragY.value = 0
  dragging.value = false
})

function onPointerDown(e: PointerEvent) {
  pointerId = e.pointerId
  startY = e.clientY
  moved = false
  dragging.value = true
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}

function onPointerMove(e: PointerEvent) {
  if (!dragging.value || e.pointerId !== pointerId) return
  const dy = e.clientY - startY
  if (Math.abs(dy) > 5) moved = true
  // It came from below, so dragging up only gives a little.
  dragY.value = dy > 0 ? dy : dy / 4
}

function onPointerUp(e: PointerEvent) {
  if (e.pointerId !== pointerId) return
  pointerId = -1
  dragging.value = false
  if (!moved || dragY.value > SWIPE_DISMISS_PX) settings.dismissToast()
  else dragY.value = 0
}

function onPointerCancel() {
  pointerId = -1
  dragging.value = false
  dragY.value = 0
}
</script>

<template>
  <div class="app-toast-host" :class="{ 'landscape-open': landscapeOpen }" role="status" aria-live="polite">
    <Transition name="app-toast" mode="out-in">
      <div
        v-if="settings.toast"
        :key="settings.toast.id"
        class="app-toast"
        :class="[`is-${settings.toast.icon}`, { 'is-dragging': dragging }]"
        :style="{ '--drag': `${dragY}px`, '--toast-duration': `${settings.toast.duration}ms` }"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerCancel"
      >
        <span class="app-toast-icon" aria-hidden="true">
          <span v-if="settings.legacyBlueActive" class="app-toast-emoji">{{ LEGACY_EMOJI[settings.toast.icon] }}</span>
          <IconBellFilled v-else-if="settings.toast.icon === 'bell'"/>
          <svg v-else-if="settings.toast.icon === 'bell-off'" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M13.73 21a2 2 0 0 1-3.46 0"/>
            <path d="M18.63 13A17.9 17.9 0 0 1 18 8"/>
            <path d="M6.26 6.26A5.9 5.9 0 0 0 6 8c0 7-3 9-3 9h14"/>
            <path d="M18 8a6 6 0 0 0-9.33-5"/>
            <path d="M2 2l20 20"/>
          </svg>
          <svg v-else-if="settings.toast.icon === 'joystick'" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="5.5" r="3"/>
            <path d="M12 8.5V15"/>
            <rect x="3" y="15" width="18" height="6" rx="2"/>
            <path d="M17 18h.01"/>
          </svg>
          <ArcadeGhostIcon v-else-if="settings.toast.icon === 'ghost'" color="#ef4444" pupil-color="#1e3a8a"/>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
               stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="9"/>
            <path d="M12 11v5M12 8h.01"/>
          </svg>
        </span>
        <span class="app-toast-text">
          <span class="app-toast-title">{{ settings.toast.title }}</span>
          <span v-if="settings.toast.body" class="app-toast-body">{{ settings.toast.body }}</span>
        </span>
        <span class="app-toast-progress" aria-hidden="true"></span>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.app-toast-host {
  position: fixed;
  left: 0;
  right: 0;
  bottom: calc(1rem + env(safe-area-inset-bottom));
  z-index: 9999;
  display: flex;
  justify-content: center;
  padding: 0 0.75rem;
  pointer-events: none;
  transition: right 250ms cubic-bezier(0.32, 0.72, 0, 1);
}

@media (max-width: 1023px) and (orientation: landscape) {
  .app-toast-host.landscape-open {
    right: var(--landscape-drawer-width);
  }
}

@media (min-width: 1024px) {
  .app-toast-host {
    right: 30vw;
    bottom: 1.5rem;
  }
}

.app-toast {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: min(24rem, 100%);
  padding: 0.75rem 1rem 0.8rem 0.75rem;
  border: 1px solid #e2e8f0;
  border-radius: 1rem;
  background: rgb(255 255 255 / 0.97);
  box-shadow: 0 16px 36px -10px rgb(15 23 42 / 0.35), 0 2px 8px rgb(15 23 42 / 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  overflow: hidden;
  pointer-events: auto;
  cursor: pointer;
  touch-action: none;
  user-select: none;
  transform: translateY(var(--drag, 0px));
  transition: transform 220ms cubic-bezier(0.32, 0.72, 0, 1);
}

.app-toast.is-dragging {
  transition: none;
}

.app-toast-icon {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.375rem;
  height: 2.375rem;
  border-radius: 0.75rem;
  background: #f1f5f9;
  color: #475569;
}

.app-toast-icon svg {
  width: 1.25rem;
  height: 1.25rem;
}

.is-ghost .app-toast-icon svg {
  width: 1rem;
  height: 1.35rem;
}

.is-bell .app-toast-icon {
  background: #fef9c3;
  color: #eab308;
}

.is-joystick .app-toast-icon,
.is-ghost .app-toast-icon {
  background: #fef3c7;
  color: #b45309;
}

.app-toast-emoji {
  font-size: 1.125rem;
  line-height: 1;
}

.app-toast-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}

.app-toast-title {
  font-size: 0.875rem;
  font-weight: 700;
  line-height: 1.3;
  color: #0f172a;
}

.app-toast-body {
  font-size: 0.78rem;
  font-weight: 500;
  line-height: 1.35;
  color: #64748b;
}

.app-toast-progress {
  position: absolute;
  left: 0;
  bottom: 0;
  width: 100%;
  height: 3px;
  background: #cbd5e1;
  transform-origin: left;
  animation: app-toast-countdown var(--toast-duration, 3000ms) linear forwards;
}

.is-bell .app-toast-progress {
  background: #facc15;
}

@keyframes app-toast-countdown {
  from {
    transform: scaleX(1);
  }
  to {
    transform: scaleX(0);
  }
}

:root.dark .app-toast {
  border-color: #334155;
  background: rgb(30 41 59 / 0.97);
  box-shadow: 0 16px 36px -10px rgb(0 0 0 / 0.6), 0 2px 8px rgb(0 0 0 / 0.3);
}

:root.dark .app-toast-title {
  color: #f8fafc;
}

:root.dark .app-toast-body {
  color: #94a3b8;
}

:root.dark .app-toast-icon {
  background: #0f172a;
  color: #94a3b8;
}

:root.dark .is-bell .app-toast-icon {
  background: rgb(234 179 8 / 0.15);
  color: #facc15;
}

:root.dark .app-toast-progress {
  background: #475569;
}

:root.dark .is-bell .app-toast-progress {
  background: #eab308;
}

.app-toast-enter-active {
  transition: transform 420ms cubic-bezier(0.2, 1.15, 0.35, 1), opacity 200ms ease-out;
}

.app-toast-leave-active {
  transition: transform 240ms cubic-bezier(0.4, 0, 1, 1), opacity 200ms ease-in;
}

.app-toast-enter-from,
.app-toast-leave-to {
  opacity: 0;
  transform: translateY(calc(100% + 2rem));
}

@media (prefers-reduced-motion: reduce) {
  .app-toast-enter-active,
  .app-toast-leave-active {
    transition: opacity 200ms ease;
  }

  .app-toast-enter-from,
  .app-toast-leave-to {
    transform: translateY(var(--drag, 0px));
  }

  .app-toast-progress {
    display: none;
  }
}
</style>
