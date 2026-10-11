<script setup lang="ts">
import {ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {androidAppHidden, canInstallAndroidApp, hideAndroidApp, openAndroidApp} from '@/utils/androidApp.ts'

const {t} = useI18n()
const shown = ref(false)
if (!androidAppHidden()) void canInstallAndroidApp().then((ok) => { shown.value = ok })

function dismiss() {
  hideAndroidApp()
  shown.value = false
}
</script>

<template>
  <div v-if="shown" class="green-friday-banner android-banner" data-kbd-section="android-banner" data-kbd-axis="x">
    <button type="button" class="android-banner-open" data-kbd-item="android-app" @click="openAndroidApp">
      <span class="android-banner-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M17.6 9.48l1.84-3.18c.16-.31.04-.69-.26-.85a.64.64 0 0 0-.83.22l-1.88 3.24a11.4 11.4 0 0 0-8.94 0L5.65 5.67a.64.64 0 0 0-.87-.2c-.28.18-.37.54-.22.83L6.4 9.48A10.8 10.8 0 0 0 1 18h22a10.8 10.8 0 0 0-5.4-8.52zM7 15.25a1.25 1.25 0 1 1 0-2.5 1.25 1.25 0 0 1 0 2.5zm10 0a1.25 1.25 0 1 1 0-2.5 1.25 1.25 0 0 1 0 2.5z"/>
        </svg>
      </span>
      <span class="green-friday-text">
        <span class="beta-badge">{{ t('experimental') }}</span>
        <span class="green-friday-title">{{ t('androidAppAsk') }}</span>
      </span>
    </button>
    <button type="button" class="green-friday-close" :aria-label="t('dismiss')" data-kbd-item="android-banner-dismiss" @click="dismiss">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M18 6 6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>
</template>

<style scoped>
.green-friday-banner {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.5rem 1rem;
  background: linear-gradient(135deg, #dcfce7, #d1fae5);
  border-bottom: 1px solid #a7f3d0;
}

.android-banner-open {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  text-align: left;
}

.android-banner-icon {
  width: 2.25rem;
  height: 2.25rem;
  flex-shrink: 0;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(16 185 129 / 0.15);
  color: #059669;
}

.android-banner-icon svg {
  width: 1.25rem;
  height: 1.25rem;
}

.green-friday-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.2rem;
}

.green-friday-title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 800;
  color: #065f46;
}

.beta-badge {
  display: inline-flex;
  padding: 0.05rem 0.375rem;
  border: 1px solid #fde68a;
  border-radius: 9999px;
  background: #fffbeb;
  color: #b45309;
  font-size: 0.5625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
}

.green-friday-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
  border-radius: 9999px;
  color: #059669;
  transition: background 120ms ease;
}

.green-friday-close:hover {
  background: rgb(16 185 129 / 0.15);
}

.green-friday-close svg {
  width: 0.75rem;
  height: 0.75rem;
}

:root.dark .android-banner-icon {
  color: #34d399;
}

:root.dark .beta-badge {
  border-color: rgb(245 158 11 / 0.35);
  background: rgb(245 158 11 / 0.12);
  color: #fbbf24;
}
</style>
