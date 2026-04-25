<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const isFriday = computed(() => new Date().getDay() === 5)

const todayKey = new Date().toISOString().slice(0, 10)
const dismissed = ref(localStorage.getItem('greenFridayDismissed') === todayKey)

function dismiss() {
  localStorage.setItem('greenFridayDismissed', todayKey)
  dismissed.value = true
}
</script>

<template>
  <div v-if="isFriday && !dismissed" class="green-friday-banner">
    <svg class="green-friday-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z"/>
      <path d="M7 12.5c1.5-2 3.5-3 5-3 2 0 3.5 1.5 3 3.5-.5 2-2.5 3-4 2.5"/>
      <path d="M12 9.5V7"/>
    </svg>
    <div class="green-friday-text">
      <p class="green-friday-title">{{ t('greenFridayTitle') }}</p>
      <p class="green-friday-desc">{{ t('greenFridayDesc') }}</p>
    </div>
    <button type="button" class="green-friday-close" :aria-label="t('dismiss')" @click="dismiss">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>
</template>

<style scoped>
.green-friday-banner {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 1rem;
  background: linear-gradient(135deg, #dcfce7, #d1fae5);
  border-bottom: 1px solid #a7f3d0;
}

.green-friday-icon {
  width: 1.75rem;
  height: 1.75rem;
  color: #059669;
  flex-shrink: 0;
}

.green-friday-text {
  flex: 1;
  min-width: 0;
}

.green-friday-title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 800;
  color: #065f46;
}

.green-friday-desc {
  margin: 0.1rem 0 0;
  font-size: 0.75rem;
  font-weight: 500;
  color: #047857;
}

.green-friday-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
  border: 0;
  border-radius: 9999px;
  background: transparent;
  color: #059669;
  cursor: pointer;
  transition: background 120ms ease;
}
.green-friday-close:hover {
  background: rgb(16 185 129 / 0.15);
}
.green-friday-close svg {
  width: 0.75rem;
  height: 0.75rem;
}
</style>
