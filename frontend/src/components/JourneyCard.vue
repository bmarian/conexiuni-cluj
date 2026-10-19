<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useJourneyStore} from '@/stores/journey.ts'
import {useUserStore} from '@/stores/user.ts'
import {stepDetail, stepTitle} from '@/utils/journeyText.ts'
import {formatMinutesFromNow} from '@/utils/time.ts'
import {cityPath} from '@/utils/city.ts'

const {t} = useI18n()
const router = useRouter()
const store = useJourneyStore()
const {userTime} = storeToRefs(useUserStore())

const now = computed(() => userTime.value ?? new Date())
const step = computed(() => store.step)
const title = computed(() => (step.value ? stepTitle(step.value) : t('followWaiting')))
const detail = computed(() => (step.value ? stepDetail(step.value, now.value.getTime()) : null))
const bus = computed(() => (step.value?.busMinutes != null ? formatMinutesFromNow(step.value.busMinutes, now.value, t('now')) : null))
</script>

<template>
  <div v-if="store.journey" class="journey-card" data-kbd-item="journey-card">
    <p class="journey-kicker">
      <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2L4.5 20.3l.7.7L12 18l6.8 3 .7-.7z"/></svg>
      {{ t('journeyTo', {name: store.journey.destination.name}) }}
    </p>
    <div class="flex items-center gap-2.5">
      <span v-if="step?.line" class="journey-badge" :style="{ backgroundColor: step.color ?? '#64748b' }">{{ step.line }}</span>
      <div class="min-w-0 flex-1">
        <p class="journey-title">{{ title }}</p>
        <p v-if="detail" class="journey-detail">{{ detail }}</p>
      </div>
      <span v-if="bus" class="journey-time">{{ bus }}</span>
    </div>
    <div class="flex flex-wrap justify-end items-center gap-1">
      <button type="button" class="journey-text-btn" @click="store.stop()">{{ t('journeyEnd') }}</button>
      <button type="button" class="journey-text-btn" @click="store.next()">{{ t('journeyNext') }}</button>
      <button type="button" class="journey-go" @click="router.push(cityPath('/nav'))">
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2L4.5 20.3l.7.7L12 18l6.8 3 .7-.7z"/></svg>
        {{ t('navResume') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.journey-card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem 0.75rem 0.5rem 1rem;
  border-radius: 1rem;
  background: var(--color-indigo-50, #eef2ff);
  border: 1px solid var(--color-indigo-200, #c7d2fe);
  color: var(--color-indigo-950, #1e1b4b);
}

.journey-kicker {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.8125rem;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.journey-badge {
  min-width: 2.25rem;
  height: 1.75rem;
  padding: 0 0.4rem;
  border-radius: 0.5rem;
  color: var(--color-white, #fff);
  font-size: 0.8125rem;
  font-weight: 900;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.journey-title {
  font-size: 0.875rem;
  font-weight: 700;
}

.journey-detail {
  font-size: 0.75rem;
  opacity: 0.8;
}

.journey-time {
  font-size: 0.8125rem;
  font-weight: 800;
  padding: 0.2rem 0.55rem;
  border-radius: 0.5rem;
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
  font-variant-numeric: tabular-nums;
}

.journey-text-btn {
  padding: 0.4rem 0.6rem;
  white-space: nowrap;
  border-radius: 9999px;
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--color-indigo-700, #4338ca);
}

.journey-text-btn:hover {
  background: var(--color-indigo-100, #e0e7ff);
}

.journey-go {
  display: inline-flex;
  white-space: nowrap;
  align-items: center;
  gap: 0.375rem;
  padding: 0.45rem 0.95rem;
  border-radius: 9999px;
  font-size: 0.8125rem;
  font-weight: 700;
  background: var(--color-indigo-600, #4f46e5);
  color: var(--color-white, #fff);
}

:root.dark .journey-card {
  background: rgba(99, 102, 241, 0.14);
  border-color: rgba(129, 140, 248, 0.35);
  color: var(--color-indigo-100, #e0e7ff);
}

:root.dark .journey-text-btn {
  color: var(--color-indigo-300, #a5b4fc);
}

:root.dark .journey-text-btn:hover {
  background: rgba(99, 102, 241, 0.2);
}
</style>
