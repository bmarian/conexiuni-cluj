<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import AppInstallDialog from '@/components/home/AppInstallDialog.vue'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useSettingsStore} from '@/stores/settings'
import {useOnline} from '@/composables/useOnline.ts'
import {offersAndroidApp} from '@/utils/androidApp.ts'
import {planQuery} from '@/utils/planQuery.ts'
import {placeIcon} from '@/utils/placeLabel.ts'
import {AndroidIcon, PlaceIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const router = useRouter()
const favorites = useFavoritesStore()
const settings = useSettingsStore()
const {isOnline} = useOnline()
const installing = ref(false)

const showApp = computed(() => offersAndroidApp && isOnline.value && !settings.appPromptDismissed)
const places = computed(() => (isOnline.value ? favorites.favoritePlans.map((place) => ({place, ...placeIcon(place.label ?? place.name)})) : []))
</script>

<template>
  <div v-if="showApp || places.length" class="place-chips" data-kbd-section="places" data-kbd-axis="x">
    <!-- TODO: Remove when android app is released -->
    <button v-if="showApp && false" type="button" class="place-chip app-chip squish" @click="installing = true">
      <span class="app-bot"><AndroidIcon class="m-icon size-[18px]" /></span>
      <span>{{ t('getApp') }}</span>
    </button>
    <button
      v-for="{place, icon, name} in places"
      :key="`${place.lat}-${place.lon}-${place.originLat ?? ''}`"
      type="button"
      class="place-chip squish"
      :data-kbd-item="`place-${place.lat}-${place.lon}-${place.originLat ?? ''}`"
      @click="router.push({name: 'plan', query: planQuery(place)})"
    >
      <span v-if="icon" class="place-emoji">{{ icon }}</span>
      <PlaceIcon v-else class="m-icon size-[18px] text-tertiary" />
      <span v-if="place.originName" class="from">{{ place.originName }}</span>
      <span v-if="place.originName" class="text-on-surface-variant">→</span>
      <span class="truncate">{{ name }}</span>
    </button>
    <AppInstallDialog v-model:open="installing" />
  </div>
</template>

<style scoped>
.place-chips {
  display: flex;
  gap: 8px;
  padding: 0 16px 4px;
  overflow-x: auto;
  scrollbar-width: none;
  touch-action: pan-x pan-y;
}

.place-chips::-webkit-scrollbar {
  display: none;
}

.place-chip {
  --squish: 0.93;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  max-width: 260px;
  height: 36px;
  margin-block: 6px;
  padding: 0 14px 0 12px;
  border: 0;
  border-radius: 18px;
  background: var(--md-surface-container-high);
  color: var(--md-on-surface);
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  letter-spacing: 0.1px;
  cursor: pointer;
}

.place-emoji {
  font-size: 16px;
  line-height: 18px;
}

.from {
  max-width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 400;
  color: var(--md-on-surface-variant);
}

.app-chip {
  padding-left: 6px;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
}

.app-bot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--android-ink);
  color: var(--android-green);
}
</style>
