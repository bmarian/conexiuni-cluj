<script setup lang="ts">
import {inject, onMounted, onUnmounted} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import QrCode from '@/components/QrCode.vue'
import {describeExport, useSettingsTransfer} from '@/composables/useSettingsTransfer.ts'
import {useTransferReceiver} from '@/composables/useTransferReceiver.ts'

const DONE_PAUSE_MS = 1500

const {t} = useI18n()
const router = useRouter()
const scrollTopOnReturn = inject<() => void>('watchScrollTopOnReturn', () => {})
const {applyFavorites} = useSettingsTransfer()
let doneTimer: ReturnType<typeof setTimeout> | undefined

const {state, url, spacedCode, summary, start, accept} = useTransferReceiver(describeExport, (data) => {
  applyFavorites(data)
  navigator.vibrate?.(60)
  doneTimer = setTimeout(goHome, DONE_PAUSE_MS)
})

function goHome() {
  scrollTopOnReturn()
  if (window.history.state?.back === '/') router.back()
  else void router.replace({name: 'home'})
}

onMounted(start)
onUnmounted(() => clearTimeout(doneTimer))
</script>

<template>
  <div class="wt-list wt-pair">
    <template v-if="state === 'waiting'">
      <div class="wt-qr">
        <QrCode :text="url" :label="spacedCode"/>
      </div>
      <p class="wt-pair-code">{{ spacedCode }}</p>
      <p class="wt-note">{{ t('transferHint') }}</p>
    </template>

    <template v-else-if="state === 'review' && summary">
      <p class="wt-heading wt-pair-review">{{ t('watchPairReview') }}</p>
      <p class="wt-pair-counts">
        <span>{{ t('favoriteStops') }}: {{ summary.stops }}</span>
        <span>{{ t('favoriteRoutes') }}: {{ summary.lines }}</span>
      </p>
      <button type="button" class="wt-float" @click="accept">{{ t('watchPairCopy') }}</button>
      <button type="button" class="wt-row wt-row-action" @click="goHome">{{ t('cancel') }}</button>
    </template>

    <template v-else-if="state === 'done'">
      <span class="wt-pair-done" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4.5 12.75l6 6 9-13.5"/>
        </svg>
      </span>
      <p class="wt-heading">{{ t('watchPairDone') }}</p>
    </template>

    <template v-else-if="state === 'expired' || state === 'failed' || state === 'broken'">
      <p class="wt-note">{{ t({expired: 'transferExpired', failed: 'transferFailed', broken: 'transferBroken'}[state]) }}</p>
      <button type="button" class="wt-row wt-row-action" @click="start">
        {{ t('transferNewCode') }}
      </button>
    </template>

    <p v-else class="wt-note">{{ t('transferLoading') }}</p>
  </div>
</template>
