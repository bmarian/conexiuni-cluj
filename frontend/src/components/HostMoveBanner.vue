<script setup lang="ts">
import {computed, ref, shallowRef} from 'vue'
import {useI18n} from 'vue-i18n'
import {apiPost} from '@/utils/api.ts'
import {askNotifications} from '@/utils/notify.ts'
import {pushSupported} from '@/utils/pushSubscription.ts'
import {clearMoveNotice, moveNotice} from '@/utils/hostMove.ts'
import {usePushStore} from '@/stores/push.ts'
import {useRemindersStore} from '@/stores/reminders.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {DEFAULT_CITY} from '@/utils/city.ts'

const {t} = useI18n()
const push = usePushStore()
const reminders = useRemindersStore()
const settings = useSettingsStore()
const notice = shallowRef(moveNotice())
const busy = ref(false)

const needsNotifications = computed(() =>
  !!notice.value && pushSupported() && (notice.value.routeAlerts || notice.value.reminders))
const desc = computed(() =>
  notice.value?.standalone ? `${t('hostMoveSaved')} ${t('hostMoveHomeScreen')}` : t('hostMoveSaved'))

function dismiss() {
  clearMoveNotice()
  notice.value = null
}

async function turnOn() {
  const moved = notice.value
  if (!moved || busy.value) return
  busy.value = true
  try {
    if (moved.routeAlerts) {
      await push.ask()
      if (!push.enabled) return
    } else if (await askNotifications() !== 'granted') {
      settings.showToast(t('pushDeniedToast'), {body: t('pushDeniedToastBody'), icon: 'bell-off'})
      return
    }
    // The old host only had Cluj.
    await reminders.sync(DEFAULT_CITY)
    // The old host's subscription would keep sending the same reminders.
    if (moved.pushEndpoint) await apiPost('push/unsubscribe', {endpoint: moved.pushEndpoint}, DEFAULT_CITY).catch(() => {})
    dismiss()
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="notice" class="green-friday-banner" data-kbd-section="moved-banner" data-kbd-axis="x">
    <div class="green-friday-text">
      <p class="green-friday-title">{{ t('hostMoveTitle') }}</p>
      <p class="green-friday-desc">{{ desc }}</p>
      <button
        v-if="needsNotifications"
        type="button"
        class="green-friday-desc moved-banner-action"
        data-kbd-item="moved-notifications"
        :disabled="busy"
        @click="turnOn"
      >
        {{ t('hostMoveNotifications') }}
      </button>
    </div>
    <button type="button" class="green-friday-close" :aria-label="t('dismiss')" data-kbd-item="moved-dismiss" @click="dismiss">
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

.moved-banner-action {
  display: block;
  padding: 0;
  font-weight: 700;
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.moved-banner-action:disabled {
  opacity: 0.6;
  cursor: default;
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
</style>
