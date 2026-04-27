<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useOnline } from '@/composables/useOnline'

const { t } = useI18n()
const { isOnline } = useOnline()

defineProps<{ landscapeOpen?: boolean }>()
</script>

<template>
  <div
    v-if="!isOnline"
    class="offline-root"
    :class="{ 'landscape-open': landscapeOpen }"
  >
    <div class="offline-pill" role="status" aria-live="polite" :title="t('offlineBanner')">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M2 2l20 20"/>
        <path d="M8.5 16.5a5 5 0 017 0"/>
        <path d="M2 8.82a15 15 0 014.17-2.65"/>
        <path d="M10.66 5c4.01-.36 8.14.9 11.34 3.76"/>
        <path d="M16.85 11.25a10 10 0 012.22 1.68"/>
        <path d="M5 13a10 10 0 015.17-2.69"/>
        <line x1="12" y1="20" x2="12.01" y2="20"/>
      </svg>
      <span>{{ t('offlineShort') }}</span>
    </div>
  </div>
</template>

<style scoped>
/* Positioning anchor — centers within the visible map area */
.offline-root {
  position: fixed;
  top: calc(0.75rem + env(safe-area-inset-top));
  /* Portrait / default: center of full viewport */
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  pointer-events: none;
  /* Slide with the drawer at the same cadence as SettingsButton */
  transition: left 250ms cubic-bezier(0.32, 0.72, 0, 1);
}

/* Landscape mobile: when drawer is open, center within remaining map area */
@media (max-width: 1023px) and (orientation: landscape) {
  .offline-root.landscape-open {
    /* map fills 100vw - drawer-width; its centre is half of that */
    left: calc((100vw - var(--landscape-drawer-width)) / 2);
  }
}

/* Desktop: map is always 70 vw on the left; centre = 35 vw */
@media (min-width: 1024px) {
  .offline-root {
    left: 35vw;
  }
}

.offline-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.3rem 0.65rem;
  background: rgba(180, 83, 9, 0.95);
  color: #fff;
  font-size: 0.75rem;
  font-weight: 600;
  border-radius: 9999px;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.2);
  backdrop-filter: blur(4px);
  white-space: nowrap;
}

.offline-pill svg {
  width: 0.85rem;
  height: 0.85rem;
  flex-shrink: 0;
}
</style>

