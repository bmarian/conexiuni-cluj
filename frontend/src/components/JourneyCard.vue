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
import {NavigationIcon} from '@/utils/icons.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import TimePill from '@/components/ui/TimePill.vue'

const {t} = useI18n()
const router = useRouter()
const store = useJourneyStore()
const {userTime} = storeToRefs(useUserStore())

const now = computed(() => userTime.value ?? new Date())
const step = computed(() => store.step)
const title = computed(() => (step.value ? stepTitle(step.value) : t('followWaiting')))
const detail = computed(() => (step.value ? stepDetail(step.value, now.value.getTime()) : null))
const bus = computed(() => (step.value?.busMinutes != null ? formatMinutesFromNow(step.value.busMinutes, now.value, t('now')) : null))
const open = () => router.push(cityPath('/nav'))
</script>

<template>
  <div v-if="store.journey" class="journey-card squish" role="button" tabindex="0" data-kbd-item="journey-card" @click="open" @keydown.enter.prevent="open">
    <p class="flex items-center gap-1.5 text-label-large">
      <NavigationIcon class="m-icon size-4"/>
      <span class="line-clamp-2">{{ t('journeyTo', {name: store.journey.destination.name}) }}</span>
    </p>
    <div class="flex items-center gap-2.5">
      <LineBadge v-if="step?.line" :name="step.line" :color="step.color ?? '#64748b'" :height="28" :min-width="36" :corner="8"/>
      <div class="flex-1 min-w-0">
        <p class="text-title-small">{{ title }}</p>
        <p v-if="detail" class="text-body-small opacity-80">{{ detail }}</p>
      </div>
      <TimePill v-if="bus" :text="bus" live/>
    </div>
    <div class="flex flex-wrap justify-end items-center gap-1">
      <button type="button" class="btn btn-text" @click.stop="store.stop()">{{ t('journeyEnd') }}</button>
      <button type="button" class="btn btn-text" @click.stop="store.next()">{{ t('journeyNext') }}</button>
      <button type="button" class="btn btn-filled" @click.stop="open">
        <NavigationIcon class="m-icon"/>
        {{ t('navResume') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.journey-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 12px 8px 16px;
  border-radius: 12px;
  background: var(--md-tertiary-container);
  color: var(--md-on-tertiary-container);
  cursor: pointer;
}
</style>
