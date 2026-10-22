<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import {useOnline} from '@/composables/useOnline'
import {WifiOffIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const {isOnline} = useOnline()
</script>

<template>
  <Transition name="offline">
    <div v-if="!isOnline" class="offline-pill" role="status" aria-live="polite" :title="t('offlineBanner')">
      <WifiOffIcon class="m-icon size-3.5"/>
      <span>{{ t('offlineShort') }}</span>
    </div>
  </Transition>
</template>

<style scoped>
.offline-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 9999px;
  background: rgb(180 83 9 / 0.95);
  color: #fff;
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  white-space: nowrap;
  pointer-events: none;
}

.offline-enter-active,
.offline-leave-active {
  transition: opacity 200ms linear;
}

.offline-enter-from,
.offline-leave-to {
  opacity: 0;
}
</style>
