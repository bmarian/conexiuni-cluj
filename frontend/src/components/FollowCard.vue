<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useFollowsStore} from '@/stores/follows.ts'
import {useUserStore} from '@/stores/user.ts'
import {formatMinutesFromNow} from '@/utils/time.ts'
import type {Followed} from '@/utils/follow.ts'

const props = defineProps<{ followed: Followed }>()

const {t} = useI18n()
const router = useRouter()
const follows = useFollowsStore()
const {userTime} = storeToRefs(useUserStore())

const live = computed(() => follows.live[props.followed.id])
const now = computed(() => userTime.value ?? new Date())
const minutes = computed(() => (live.value ? formatMinutesFromNow(live.value.minutes, now.value, t('now')) : null))
</script>

<template>
  <div class="follow-card" role="button" tabindex="0" :data-kbd-item="`follow-${followed.id}`"
       @click="router.push(`/stop/${followed.stopId}`)" @keydown.enter.prevent="router.push(`/stop/${followed.stopId}`)">
    <div class="min-w-0 flex-1 flex flex-col gap-1">
      <p class="follow-kicker">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 17h5l-1.4-1.4A2 2 0 0 1 18 14.2V11a6 6 0 1 0-12 0v3.2a2 2 0 0 1-.6 1.4L4 17h5m6 0a3 3 0 1 1-6 0m6 0H9"/>
        </svg>
        {{ t('following') }}
      </p>
      <div class="flex items-center gap-2 min-w-0">
        <span class="follow-badge" :style="{ backgroundColor: followed.color }">{{ followed.line }}</span>
        <span class="follow-dest">→ {{ followed.destination }}</span>
      </div>
      <p class="follow-stop">{{ followed.stopName }}</p>
    </div>
    <div class="flex flex-col items-end gap-2 shrink-0">
      <span v-if="minutes" class="follow-time" :class="{ 'is-live': live?.live }">{{ minutes }}</span>
      <span v-else class="follow-waiting">{{ t('followWaiting') }}</span>
      <button type="button" class="follow-stop-btn" @click.stop="follows.remove(followed.id)">{{ t('stopFollowing') }}</button>
    </div>
  </div>
</template>

<style scoped>
.follow-card {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 0.75rem 0.75rem 1rem;
  border-radius: 1rem;
  background: var(--color-emerald-50, #ecfdf5);
  border: 1px solid var(--color-emerald-200, #a7f3d0);
  color: var(--color-emerald-900, #064e3b);
  cursor: pointer;
}

.follow-kicker {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.8125rem;
  font-weight: 700;
}

.follow-badge {
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
}

.follow-dest {
  font-size: 0.875rem;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.follow-stop {
  font-size: 0.75rem;
  opacity: 0.75;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.follow-time {
  font-size: 0.8125rem;
  font-weight: 800;
  padding: 0.2rem 0.55rem;
  border-radius: 0.5rem;
  background: var(--color-emerald-100, #d1fae5);
  font-variant-numeric: tabular-nums;
}

.follow-time.is-live {
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
}

.follow-waiting {
  font-size: 0.75rem;
  opacity: 0.75;
}

.follow-stop-btn {
  padding: 0.35rem 0.85rem;
  border-radius: 9999px;
  font-size: 0.8125rem;
  font-weight: 700;
  background: var(--color-emerald-200, #a7f3d0);
  color: var(--color-emerald-900, #064e3b);
}

.follow-stop-btn:hover {
  background: var(--color-emerald-300, #6ee7b7);
}

:root.dark .follow-card {
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.35);
  color: var(--color-emerald-100, #d1fae5);
}

:root.dark .follow-time {
  background: rgba(16, 185, 129, 0.25);
}

:root.dark .follow-time.is-live {
  background: var(--color-emerald-500, #10b981);
}

:root.dark .follow-stop-btn {
  background: rgba(16, 185, 129, 0.3);
  color: var(--color-emerald-100, #d1fae5);
}

:root.dark .follow-stop-btn:hover {
  background: rgba(16, 185, 129, 0.45);
}
</style>
