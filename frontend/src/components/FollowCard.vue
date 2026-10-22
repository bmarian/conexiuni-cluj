<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useFollowsStore} from '@/stores/follows.ts'
import {useUserStore} from '@/stores/user.ts'
import {formatMinutesFromNow} from '@/utils/time.ts'
import type {Followed} from '@/utils/follow.ts'
import {cityPath} from '@/utils/city.ts'
import {NotificationsActiveIcon} from '@/utils/icons.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import TimePill from '@/components/ui/TimePill.vue'

const props = defineProps<{ followed: Followed }>()

const {t} = useI18n()
const router = useRouter()
const follows = useFollowsStore()
const {userTime} = storeToRefs(useUserStore())

const live = computed(() => follows.live[props.followed.id])
const now = computed(() => userTime.value ?? new Date())
const minutes = computed(() => (live.value ? formatMinutesFromNow(live.value.minutes, now.value, t('now')) : null))
const open = () => router.push(cityPath(`/stop/${props.followed.stopId}`))
</script>

<template>
  <div class="follow-card squish" role="button" tabindex="0" :data-kbd-item="`follow-${followed.id}`" @click="open" @keydown.enter.prevent="open">
    <div class="flex-1 min-w-0 flex flex-col gap-1">
      <p class="flex items-center gap-1.5 text-label-large">
        <NotificationsActiveIcon class="m-icon size-4"/>
        {{ t('following') }}
      </p>
      <div class="flex items-center gap-2 min-w-0">
        <LineBadge :name="followed.line" :color="followed.color" :height="28" :min-width="36" :corner="8"/>
        <span class="destination text-title-small">→ {{ followed.destination }}</span>
      </div>
      <p class="text-body-small opacity-75 line-clamp-2">{{ followed.stopName }}</p>
    </div>
    <div class="flex flex-col items-end gap-2 shrink-0">
      <TimePill v-if="minutes" :text="minutes" :live="live?.live"/>
      <span v-else class="text-label-medium opacity-75">{{ t('followWaiting') }}</span>
      <button type="button" class="btn btn-tonal" @click.stop="follows.remove(followed.id)">{{ t('stopFollowing') }}</button>
    </div>
  </div>
</template>

<style scoped>
.follow-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 12px 12px 16px;
  border-radius: 12px;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
  cursor: pointer;
}

.destination {
  min-width: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
