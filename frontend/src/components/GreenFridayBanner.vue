<script setup lang="ts">
import {computed, nextTick, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import {cityDate, cityTime} from '@/utils/cityClock.ts'
import {CloseIcon, EcoIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const kbd = useKeyboardNav()
const isFriday = computed(() => cityTime(new Date()).weekday === 5)
const todayKey = cityDate(new Date())
const dismissed = ref(localStorage.getItem('greenFridayDismissed') === todayKey)

function dismiss() {
  // A one-item section has no neighbour, so focus would drop onto the body.
  const wasKeyboard = kbd.keyboardMode.value
  const neighbor = wasKeyboard ? kbd.neighborKey() : null
  localStorage.setItem('greenFridayDismissed', todayKey)
  dismissed.value = true
  if (!wasKeyboard) return
  void nextTick(() => {
    if (!neighbor || !kbd.focusKey(neighbor)) kbd.focusEntry()
  })
}
</script>

<template>
  <div v-if="isFriday && !dismissed" class="banner-gutter">
    <div class="green-friday" data-kbd-section="banner" data-kbd-axis="x">
      <span class="icon-well"><EcoIcon class="m-icon size-5" /></span>
      <div class="flex-1 min-w-0">
        <p class="text-title-small">{{ t('greenFridayTitle') }}</p>
        <p class="text-body-medium text-on-surface-variant">{{ t('greenFridayDesc') }}</p>
      </div>
      <button type="button" class="icon-btn text-outline" :aria-label="t('dismiss')" :title="t('dismiss')" data-kbd-item="banner-dismiss" @click="dismiss">
        <CloseIcon class="m-icon size-[18px]" />
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Same gutter as .drawer-view. */
.banner-gutter {
  flex-shrink: 0;
  overflow-y: hidden;
  scrollbar-gutter: stable;
}

.green-friday {
  --eco: #2e9e57;
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 8px 12px 0;
  padding: 10px 4px 10px 14px;
  border-radius: 20px;
  background: color-mix(in oklch, var(--eco) 9%, var(--md-surface-container));
  color: var(--md-on-surface);
}

.icon-well {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: color-mix(in oklch, var(--eco) 24%, var(--md-surface-container-high));
  color: color-mix(in oklch, var(--eco) 75%, var(--md-on-surface));
}
</style>
