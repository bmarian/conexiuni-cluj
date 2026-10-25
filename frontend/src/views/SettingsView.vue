<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import SettingsSync from '@/components/SettingsSync.vue'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ChoiceRow from '@/components/ui/ChoiceRow.vue'
import ToggleRow from '@/components/ui/ToggleRow.vue'
import ListItem from '@/components/ui/ListItem.vue'
import LiveBadge from '@/components/ui/LiveBadge.vue'
import TimetableBadge from '@/components/ui/TimetableBadge.vue'
import type {Choice} from '@/components/ui/types.ts'
import {useSettingsStore} from '@/stores/settings'
import {useCityPicker} from '@/composables/useCityPicker.ts'
import {chooseCity, cityChoice, cityInfo, currentCity, locationGranted} from '@/utils/city.ts'
import {regionSettingsFor} from '@/utils/regionSettings.ts'
import {isIOS} from '@/utils/ios.ts'
import {adminAuthed} from '@/composables/useAdmin.ts'
import {
  BrightnessMediumIcon,
  CloseIcon,
  CodeIcon,
  CropFreeIcon,
  DarkModeIcon,
  DownloadIcon,
  EcoIcon,
  EmailIcon,
  ExpandMoreIcon,
  LightModeIcon,
  MyLocationIcon,
  PaletteIcon,
  PlaceIcon,
  SearchIcon,
  TuneIcon,
  WarningAmberIcon,
} from '@/utils/icons.ts'

type Theme = 'light' | 'dark' | 'system'
type Locale = 'ro' | 'en'

const CONTACT_EMAIL = 'ceautobuziau@gmail.com'
const SOURCE_URL = 'https://github.com/bmarian/conexiuni-cluj'
const release = __APP_RELEASE__

const {t, locale} = useI18n()
const route = useRoute()
const router = useRouter()
const settings = useSettingsStore()

useHead(() => ({
  title: t('headSettingsTitle'),
  meta: [{name: 'robots', content: 'noindex'}],
}))

const isAdminAuthed = ref(false)

// A scanned sync QR: the code, and its key after #, which never reaches the server. Both leave the address bar at once.
const pairCode = ref<string>()
const pairKey = ref<string>()
watch(
  () => route.query.pair,
  (code) => {
    if (typeof code !== 'string') return
    pairKey.value = new URLSearchParams(route.hash.replace(/^#/, '')).get('k') ?? undefined
    pairCode.value = code
    void router.replace({query: {}, hash: ''})
  },
  {immediate: true},
)

onMounted(() => {
  isAdminAuthed.value = adminAuthed()
})

const languages: Choice<Locale>[] = [
  {value: 'ro', label: 'Română'},
  {value: 'en', label: 'English'},
]

function setLocale(newLocale: Locale) {
  settings.setLocale(newLocale)
  locale.value = newLocale
}

const themes = computed<Choice<Theme>[]>(() => [
  {value: 'light', label: t('themeLight'), icon: LightModeIcon},
  {value: 'dark', label: t('themeDark'), icon: DarkModeIcon},
  {value: 'system', label: t('themeSystem'), icon: BrightnessMediumIcon},
])

const automatic = computed(() => locationGranted.value && !cityChoice.value)
const cityOpen = ref(false)
const {query: cityQuery, matches: shownCities, list: cityList, listHeight: cityListHeight} = useCityPicker(cityOpen)
const info = computed(() => cityInfo())
const regionSettings = computed(() => regionSettingsFor(currentCity.value))

async function choose(slug: string | null) {
  cityOpen.value = false
  const target = await chooseCity(slug)
  if (target !== currentCity.value) void router.push(`/${target}/`)
}
</script>

<template>
  <div class="settings-view flex flex-col">
    <SheetTopBar :title="t('settings')" />
    <h1 class="text-headline-small font-bold px-4 py-1">{{ t('settings') }}</h1>

    <h2 class="section">{{ t('language') }}</h2>
    <p v-if="settings.locale !== 'ro'" class="note">
      <WarningAmberIcon class="m-icon size-[18px] text-amber" />
      <span>{{ t('languageDesc') }}</span>
    </p>
    <ChoiceRow
      :options="languages"
      :model-value="settings.locale as Locale"
      kbd="lang"
      class="mx-4 my-2"
      data-kbd-section="language"
      data-kbd-entry="1"
      @update:model-value="setLocale"
    />

    <h2 class="section">{{ t('display') }}</h2>
    <ChoiceRow
      :options="themes"
      :model-value="settings.theme as Theme"
      kbd="theme"
      class="mx-4 my-2"
      data-kbd-section="theme"
      @update:model-value="settings.setTheme"
    />
    <div data-kbd-section="display">
      <ToggleRow
        :icon="PaletteIcon"
        :title="t('colorfulLines')"
        :description="t('colorfulLinesDesc')"
        :checked="settings.colorfulLines"
        data-kbd-item="colorful-lines"
        @change="settings.setColorfulLines"
      />
    </div>

    <h2 class="section">{{ t('settingsMap') }}</h2>
    <div data-kbd-section="map">
      <ToggleRow
        :icon="MyLocationIcon"
        :title="t('autoCenterOnMe')"
        :description="t('autoCenterOnMeDesc')"
        :checked="settings.autoCenterOnMe"
        data-kbd-item="auto-center"
        @change="settings.setAutoCenterOnMe"
      />
      <ToggleRow
        :icon="CropFreeIcon"
        :title="t('autoFitMap')"
        :description="t('autoFitMapDesc')"
        :checked="settings.autoFitMap"
        data-kbd-item="auto-fit"
        @change="settings.setAutoFitMap"
      />
    </div>

    <h2 class="section">{{ t('city') }}</h2>
    <div data-kbd-section="city">
      <ListItem
        :icon="PlaceIcon"
        :title="automatic ? t('cityAuto') : (info?.name ?? currentCity)"
        :description="automatic ? info?.name : info?.operator"
        :aria-expanded="cityOpen"
        data-kbd-item="city"
        @click="cityOpen = !cityOpen"
      >
        <template #trailing>
          <ExpandMoreIcon class="m-icon text-on-surface-variant chevron" :class="{'is-open': cityOpen}" />
        </template>
      </ListItem>
      <Transition name="expand">
        <div v-if="cityOpen" class="expand-body">
          <label class="search-pill city-search">
            <SearchIcon class="m-icon text-on-surface-variant" />
            <input
              v-model="cityQuery"
              type="search"
              class="search-input text-body-large"
              data-kbd-item="city-search"
              :placeholder="t('citySearch')"
              :aria-label="t('citySearch')"
              autocomplete="off"
              enterkeyhint="search"
              @keydown.enter="($event.target as HTMLInputElement).blur()"
            />
            <button v-if="cityQuery" type="button" class="icon-btn" :aria-label="t('clearSearch')" @click="cityQuery = ''">
              <CloseIcon class="m-icon" />
            </button>
          </label>
          <ListItem v-if="locationGranted && !cityQuery.trim()" :title="t('cityAuto')" data-kbd-item="city-auto" @click="choose(null)">
            <template #leading><span class="radio" :class="{'is-on': automatic}"></span></template>
          </ListItem>
          <div ref="cityList" class="city-list" :style="{maxHeight: cityListHeight}">
            <ListItem
              v-for="city in shownCities"
              :key="city.slug"
              :title="city.name"
              :description="city.operator || undefined"
              :data-kbd-item="`city-${city.slug}`"
              @click="choose(city.slug)"
            >
              <template #leading><span class="radio" :class="{'is-on': !automatic && city.slug === currentCity}"></span></template>
              <template #trailing>
                <LiveBadge v-if="city.realtime" />
                <TimetableBadge v-else />
              </template>
            </ListItem>
            <p v-if="!shownCities.length" class="city-none text-body-medium text-on-surface-variant">{{ t('noResults') }}</p>
          </div>
        </div>
      </Transition>
      <ToggleRow
        v-for="setting in regionSettings"
        :key="setting.key"
        :icon="EcoIcon"
        :title="t(setting.titleKey)"
        :description="t(setting.descKey)"
        :checked="settings.regionSetting(currentCity, setting.key)"
        :data-kbd-item="setting.key"
        @change="settings.setRegionSetting(currentCity, setting.key, $event)"
      />
    </div>

    <h2 class="section">{{ t('syncTitle') }}</h2>
    <SettingsSync :pair-code="pairCode" :pair-key="pairKey" />

    <template v-if="isAdminAuthed">
      <h2 class="section">Admin</h2>
      <div data-kbd-section="admin">
        <ListItem :icon="TuneIcon" :title="t('adminDashboard')" :description="t('settingsAdminDesc')" data-kbd-item="admin" @click="router.push('/admin')" />
      </div>
    </template>

    <template v-if="!isIOS">
      <h2 class="section">{{ t('androidApp') }}</h2>
      <div data-kbd-section="android-app">
        <ListItem
          :icon="DownloadIcon"
          :title="t('androidBetaAsk')"
          :description="t('androidAppDesc')"
          data-kbd-item="android-app"
          @click="router.push({name: 'android'})"
        >
          <template #badge
            ><span class="beta">{{ t('beta') }}</span></template
          >
        </ListItem>
      </div>
    </template>

    <h2 class="section">{{ t('contact') }}</h2>
    <div data-kbd-section="contact">
      <ListItem as="a" :href="`mailto:${CONTACT_EMAIL}`" :icon="EmailIcon" :title="CONTACT_EMAIL" :description="t('contactDesc')" data-kbd-item="contact" />
      <ListItem
        as="a"
        :href="SOURCE_URL"
        target="_blank"
        rel="noopener"
        :icon="CodeIcon"
        title="GitHub"
        :description="t('sourceCodeDesc')"
        data-kbd-item="source"
      />
    </div>
    <p class="version text-body-small text-on-surface-variant">
      {{ t('openSource') }}<template v-if="release"> · v{{ release }}</template>
    </p>
    <div class="h-8"></div>
  </div>
</template>

<style scoped>
.section {
  padding: 20px 16px 4px;
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  letter-spacing: 0.1px;
  color: var(--md-primary);
}

.version {
  padding: 24px 16px 0;
  text-align: center;
}

.note {
  display: flex;
  gap: 10px;
  margin: 4px 16px;
  padding: 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--md-amber) 12%, transparent);
  font-size: 12px;
  line-height: 16px;
  letter-spacing: 0.4px;
  color: var(--md-on-surface);
}

.chevron {
  transition: rotate 200ms ease;
}

.chevron.is-open {
  rotate: 180deg;
}

.expand-body {
  display: grid;
  margin: 0 16px 8px;
  padding: 8px 0;
  border-radius: 16px;
  background: var(--md-surface-container);
}

.city-search {
  margin: 0 8px 4px;
}

.city-list {
  display: grid;
  grid-auto-rows: max-content;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.city-none {
  padding: 12px 16px;
}

.expand-enter-active,
.expand-leave-active {
  transition:
    opacity 200ms linear,
    translate var(--spring-fast-ms) var(--spring-fast);
}

.expand-enter-from,
.expand-leave-to {
  opacity: 0;
  translate: 0 -8px;
}

.radio {
  position: relative;
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  margin: 2px;
  border-radius: 50%;
  box-shadow: inset 0 0 0 2px var(--md-on-surface-variant);
}

.radio.is-on {
  box-shadow: inset 0 0 0 2px var(--md-primary);
}

.radio.is-on::after {
  content: '';
  position: absolute;
  inset: 5px;
  border-radius: 50%;
  background: var(--md-primary);
}

.beta {
  padding: 1px 6px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--md-amber) 18%, transparent);
  color: #b45309;
  font-size: 11px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}
</style>
