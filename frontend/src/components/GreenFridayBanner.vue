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
  <div v-if="isFriday && !dismissed" class="green-friday" data-kbd-section="banner" data-kbd-axis="x">
    <span class="tile"><EcoIcon class="m-icon" /></span>
    <div class="flex-1 min-w-0">
      <p class="title">{{ t('greenFridayTitle') }}</p>
      <p class="desc">{{ t('greenFridayDesc') }}</p>
    </div>
    <button type="button" class="icon-btn close" :aria-label="t('dismiss')" :title="t('dismiss')" data-kbd-item="banner-dismiss" @click="dismiss">
      <CloseIcon class="m-icon" />
    </button>
  </div>
</template>

<style scoped>
.green-friday {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 4px 8px 16px;
  background: linear-gradient(135deg, #dcfce7, #d1fae5);
}

.tile {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: #10b981;
  color: #fff;
}

.title {
  font-size: 14px;
  line-height: 20px;
  font-weight: 800;
  color: #065f46;
}

.desc {
  font-size: 12px;
  line-height: 16px;
  letter-spacing: 0.4px;
  color: #047857;
}

.close {
  color: #059669;
}
</style>
