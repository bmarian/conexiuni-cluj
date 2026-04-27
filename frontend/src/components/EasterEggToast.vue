<script setup lang="ts">
import {useSettingsStore} from '@/stores/settings'

const settings = useSettingsStore()
</script>

<template>
  <Transition name="egg-toast">
    <div v-if="settings.toastMessage" class="egg-toast" role="status" aria-live="polite">
      {{ settings.toastMessage }}
    </div>
  </Transition>
</template>

<style scoped>
.egg-toast {
  position: fixed;
  bottom: 42%;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  padding: 0.625rem 1.375rem;
  background: #fef9c3;
  border: 2px solid #f59e0b;
  color: #78350f;
  font-size: 0.9375rem;
  font-weight: 700;
  border-radius: 9999px;
  box-shadow: 0 4px 24px rgba(245, 158, 11, 0.45);
  pointer-events: none;
  white-space: nowrap;
}

.egg-toast-enter-active,
.egg-toast-leave-active {
  transition: opacity 0.3s ease, transform 0.35s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.egg-toast-enter-from {
  opacity: 0;
  transform: translateX(-50%) scale(0.75);
}

.egg-toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) scale(0.9) translateY(-10px);
}
</style>
