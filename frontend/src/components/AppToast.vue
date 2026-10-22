<script setup lang="ts">
import {ref, watch} from 'vue'
import {useSettingsStore} from '@/stores/settings'

const settings = useSettingsStore()

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
  <div class="snackbar-host" role="status" aria-live="polite">
    <Transition name="snackbar" mode="out-in">
      <div
        v-if="settings.toast"
        :key="settings.toast.id"
        class="snackbar"
        :class="{ 'is-dragging': dragging }"
        :style="{ '--drag': `${dragY}px` }"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerCancel"
      >
        <span class="text-body-medium">{{ settings.toast.title }}</span>
        <span v-if="settings.toast.body" class="snackbar-body">{{ settings.toast.body }}</span>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.snackbar-host {
  position: fixed;
  left: var(--panel-shown, 0px);
  right: 0;
  bottom: calc(16px + env(safe-area-inset-bottom));
  z-index: 9999;
  display: flex;
  justify-content: center;
  padding: 0 16px;
  pointer-events: none;
}

.snackbar {
  display: flex;
  flex-direction: column;
  width: min(560px, 100%);
  min-height: 48px;
  justify-content: center;
  padding: 14px 16px;
  border-radius: 4px;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface);
  box-shadow: var(--md-elevation-3);
  pointer-events: auto;
  cursor: pointer;
  touch-action: none;
  user-select: none;
  translate: 0 var(--drag, 0px);
  transition: translate 220ms cubic-bezier(0.2, 0, 0, 1);
}

.snackbar.is-dragging {
  transition: none;
}

.snackbar-body {
  font-size: 12px;
  line-height: 16px;
  letter-spacing: 0.4px;
  color: color-mix(in srgb, var(--md-on-surface) 80%, transparent);
}

.snackbar-enter-active {
  transition: translate var(--spring-fast-ms) var(--spring-fast), opacity 150ms linear;
}

.snackbar-leave-active {
  transition: translate 200ms cubic-bezier(0.3, 0, 1, 1), opacity 150ms linear;
}

.snackbar-enter-from,
.snackbar-leave-to {
  opacity: 0;
  translate: 0 calc(100% + 16px);
}

@media (prefers-reduced-motion: reduce) {
  .snackbar-enter-from,
  .snackbar-leave-to {
    translate: 0 var(--drag, 0px);
  }
}
</style>
