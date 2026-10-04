<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings.ts'

const {t} = useI18n()
const settings = useSettingsStore()
</script>

<template>
  <div class="slp" role="dialog" aria-modal="true" :aria-label="t('simplifiedLayout')">
    <p class="slp-question">{{ t('simplifiedPrompt') }}</p>
    <div class="slp-actions">
      <button type="button" class="slp-btn slp-btn-primary" @click="settings.setSimplified(true)">{{ t('yes') }}</button>
      <button type="button" class="slp-btn" @click="settings.setSimplified(false)">{{ t('no') }}</button>
    </div>
    <p class="slp-note">{{ t('simplifiedPromptNote') }}</p>
  </div>
</template>

<style scoped>
.slp {
  position: fixed;
  inset: 0;
  z-index: 9000;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  padding: 0 15%;
  background: #fff;
  color: #0f172a;
  text-align: center;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

:root.dark .slp {
  background: #0f172a;
  color: #f1f5f9;
}

.slp-question {
  font-size: 1.125rem;
  font-weight: 700;
  line-height: 1.3;
}

.slp-actions {
  display: flex;
  gap: 0.5rem;
  width: 100%;
  max-width: 15rem;
}

.slp-btn {
  flex: 1;
  padding: 0.75rem 0.5rem;
  border: 1px solid transparent;
  border-radius: 0.75rem;
  background: transparent;
  color: #64748b;
  font-size: 1rem;
  font-weight: 600;
  cursor: pointer;
}

.slp-btn-primary {
  background: #eff6ff;
  color: #1d4ed8;
  border-color: #bfdbfe;
}

:root.dark .slp-btn {
  color: #94a3b8;
}

:root.dark .slp-btn-primary {
  background: #1e3a5f;
  color: #93c5fd;
  border-color: #1d4ed8;
}

.slp-note {
  font-size: 0.75rem;
  color: #94a3b8;
}
</style>
