<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import HeaderNavigation from '@/components/HeaderNavigation.vue'
import SettingsExportImport from '@/components/SettingsExportImport.vue'
import {useSettingsStore} from '@/stores/settings'
import {TRANSFER_QUERY, transferKeyFromHash} from '@/utils/transferLink.ts'
import {cities, currentCity} from '@/utils/city.ts'
import {regionSettingsFor} from '@/utils/regionSettings.ts'

const {t, locale} = useI18n()
const route = useRoute()
const router = useRouter()
const regionSlug = ref(currentCity.value)
const regionOpen = ref(false)
const regionSettings = computed(() => regionSettingsFor(regionSlug.value))
const regionName = computed(() => cities.value.find((c) => c.slug === regionSlug.value)?.name ?? regionSlug.value)
const settings = useSettingsStore()

useHead(() => ({
  title: t('headSettingsTitle'),
  meta: [{name: 'robots', content: 'noindex'}],
}))

const isAdminAuthed = ref(false)
const sendCode = computed(() => {
  const code = route.query[TRANSFER_QUERY]
  return typeof code === 'string' ? code : undefined
})
const sendKey = computed(() => transferKeyFromHash(route.hash))

onMounted(() => {
  try {
    isAdminAuthed.value = localStorage.getItem('admin:authed') === '1'
  } catch {
    isAdminAuthed.value = false
  }
})

function setLocale(newLocale: 'ro' | 'en') {
  settings.setLocale(newLocale)
  locale.value = newLocale
}

// Settings has no simplified version, so switching it on goes to the menu.
function toggleSimplified() {
  const on = !settings.watchActive
  settings.setSimplified(on)
  if (on) void router.push({name: 'home'})
}
</script>

<template>
  <div
    class="settings-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-7">
    <div class="flex items-center -mb-3">
      <HeaderNavigation/>
    </div>

    <header v-if="settings.paperActive" class="pp-masthead">
      <p class="pp-kicker">Ce autobuz iau?</p>
      <h1 class="pp-title pp-title-sm">{{ t('settings') }}</h1>
      <p class="pp-subtitle">{{ t('paperSettingsSubtitle') }}</p>
    </header>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('language') }}</h2>
      <p v-if="settings.locale !== 'ro'" class="setting-note">
        <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">⚠️</span>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"
             stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M10.3 3.8 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.8a2 2 0 0 0-3.4 0Z"/>
          <path d="M12 9v4M12 17h.01"/>
        </svg>
        {{ t('languageDesc') }}
      </p>
      <div class="option-group" role="group" :aria-label="t('language')"
           data-kbd-section="language" data-kbd-axis="x" data-kbd-entry="1">
        <button type="button" class="option-btn" :class="{ active: settings.locale === 'ro' }"
                data-kbd-item="lang-ro" :data-kbd-active="settings.locale === 'ro'"
                @click="setLocale('ro')">
          Română
        </button>
        <button type="button" class="option-btn" :class="{ active: settings.locale === 'en' }"
                data-kbd-item="lang-en" :data-kbd-active="settings.locale === 'en'"
                @click="setLocale('en')">
          English
        </button>
      </div>
    </section>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('display') }}</h2>
      <div class="setting-rows" data-kbd-section="display">
        <button type="button" class="option-btn setting-row"
                :class="{ active: settings.showVehicleExtras }"
                :aria-pressed="settings.showVehicleExtras" data-kbd-item="vehicle-extras"
                @click="settings.setShowVehicleExtras(!settings.showVehicleExtras)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">♿</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="3.5" r="2.5" fill="currentColor" stroke="none"/>
            <path d="M10 8v6h5l2 5"/>
            <circle cx="9" cy="19.5" r="3.5"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t('vehicleExtras') }}</span>
            <span class="setting-desc">{{ t('vehicleExtrasDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.showVehicleExtras" viewBox="0 0 24 24" fill="none"
                 stroke="currentColor" stroke-width="3.5" stroke-linecap="round"
                 stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>

        <button v-if="settings.watchScreen" type="button" class="option-btn setting-row"
                :class="{ active: settings.watchActive }"
                :aria-pressed="settings.watchActive" data-kbd-item="simplified-layout"
                @click="toggleSimplified">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">🔘</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="9"/>
            <path d="M8 10h8M9.5 14h5"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t('simplifiedLayout') }}</span>
            <span class="setting-desc">{{ t('simplifiedLayoutDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.watchActive" viewBox="0 0 24 24" fill="none"
                 stroke="currentColor" stroke-width="3.5" stroke-linecap="round"
                 stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>
      </div>
    </section>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('settingsMap') }}</h2>
      <div class="setting-rows" data-kbd-section="map">
        <button type="button" class="option-btn setting-row"
                :class="{ active: settings.autoCenterOnMe }"
                :aria-pressed="settings.autoCenterOnMe" data-kbd-item="auto-center"
                @click="settings.setAutoCenterOnMe(!settings.autoCenterOnMe)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">📍</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="3"/>
            <path d="M12 2v3M12 19v3M2 12h3M19 12h3"/>
            <circle cx="12" cy="12" r="8"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t('autoCenterOnMe') }}</span>
            <span class="setting-desc">{{ t('autoCenterOnMeDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.autoCenterOnMe" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>

        <button type="button" class="option-btn setting-row" :class="{ active: settings.autoFitMap }"
                :aria-pressed="settings.autoFitMap" data-kbd-item="auto-fit"
                @click="settings.setAutoFitMap(!settings.autoFitMap)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">🗺️</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path
              d="M8 3H5a2 2 0 0 0-2 2v3M21 8V5a2 2 0 0 0-2-2h-3M3 16v3a2 2 0 0 0 2 2h3M16 21h3a2 2 0 0 0 2-2v-3"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t('autoFitMap') }}</span>
            <span class="setting-desc">{{ t('autoFitMapDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.autoFitMap" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>
      </div>
    </section>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('settingsNotifications') }}</h2>
      <div class="setting-rows" data-kbd-section="notifications">
        <button type="button" class="option-btn setting-row"
                :class="{ active: settings.busAlerts }"
                :aria-pressed="settings.busAlerts" data-kbd-item="bus-alerts"
                @click="settings.setBusAlerts(!settings.busAlerts)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">⏰</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="13" r="8"/>
            <path d="M12 9v4l2 2M5 3 2 6m17-3 3 3"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">
              {{ t('busAlerts') }}
              <span class="beta-badge">{{ t('experimental') }}</span>
            </span>
            <span class="setting-desc">{{ t('busAlertsDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.busAlerts" viewBox="0 0 24 24" fill="none"
                 stroke="currentColor" stroke-width="3.5" stroke-linecap="round"
                 stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>
        <p class="setting-note">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">⚠️</span>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"
               stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M10.3 3.8 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.8a2 2 0 0 0-3.4 0Z"/>
            <path d="M12 9v4M12 17h.01"/>
          </svg>
          {{ t('busAlertsNote') }}
        </p>
      </div>
    </section>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('region') }}</h2>
      <div class="setting-rows" data-kbd-section="region">
        <button type="button" class="option-btn setting-row" :aria-expanded="regionOpen" data-kbd-item="region"
                @click="regionOpen = !regionOpen">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">📍</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z"/>
            <circle cx="12" cy="10" r="3"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ regionName }}</span>
            <span class="setting-desc">{{ t('regionDesc') }}</span>
          </span>
          <span class="region-chevron" :class="{ 'is-open': regionOpen }" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
                 stroke-linecap="round" stroke-linejoin="round">
              <path d="m6 9 6 6 6-6"/>
            </svg>
          </span>
        </button>
      </div>
      <div v-if="regionOpen" class="option-group wrap" role="group" :aria-label="t('region')"
           data-kbd-section="region-choice" data-kbd-axis="x">
        <button v-for="city in cities" :key="city.slug" type="button" class="option-btn"
                :class="{ active: city.slug === regionSlug }" :data-kbd-item="'region-' + city.slug"
                @click="regionSlug = city.slug; regionOpen = false">
          {{ city.name }}
        </button>
      </div>
      <div v-if="regionSettings.length" class="setting-rows" data-kbd-section="region-settings">
        <button v-for="setting in regionSettings" :key="setting.key" type="button" class="option-btn setting-row"
                :class="{ active: settings.regionSetting(regionSlug, setting.key) }"
                :aria-pressed="settings.regionSetting(regionSlug, setting.key)" :data-kbd-item="setting.key"
                @click="settings.setRegionSetting(regionSlug, setting.key, !settings.regionSetting(regionSlug, setting.key))">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">🌿</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path
              d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19 2c1 2 2 4.18 2 8 0 5.5-4.78 10-10 10Z"/>
            <path d="M2 21c0-3 1.85-5.36 5.08-6C9.5 14.52 12 13 13 12"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t(setting.titleKey) }}</span>
            <span class="setting-desc">{{ t(setting.descKey) }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.regionSetting(regionSlug, setting.key)" viewBox="0 0 24 24" fill="none"
                 stroke="currentColor" stroke-width="3.5" stroke-linecap="round"
                 stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>
      </div>
      <p v-else class="setting-desc">{{ t('regionNone', {city: regionName}) }}</p>
    </section>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('transfer') }}</h2>
      <p class="setting-desc">{{ t('transferDesc') }}</p>
      <SettingsExportImport :send-code="sendCode" :send-key="sendKey"/>
    </section>

    <section v-if="isAdminAuthed" class="flex flex-col gap-2">
      <h2 class="section-label">Admin</h2>
      <div class="setting-rows" data-kbd-section="admin">
        <button type="button" class="option-btn setting-row" data-kbd-item="admin"
                @click="router.push('/admin')">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">🛡️</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M12 2 4 6v6c0 5 3.5 9 8 10 4.5-1 8-5 8-10V6l-8-4z"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t('adminDashboard') }}</span>
            <span class="setting-desc">{{ t('settingsAdminDesc') }}</span>
          </span>
        </button>
      </div>
    </section>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('androidApp') }}</h2>
      <div class="setting-rows" data-kbd-section="android-app">
        <RouterLink :to="{name: 'android'}" class="option-btn setting-row" data-kbd-item="android-app">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">🤖</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M17.6 9.48l1.84-3.18c.16-.31.04-.69-.26-.85a.64.64 0 0 0-.83.22l-1.88 3.24a11.4 11.4 0 0 0-8.94 0L5.65 5.67a.64.64 0 0 0-.87-.2c-.28.18-.37.54-.22.83L6.4 9.48A10.8 10.8 0 0 0 1 18h22a10.8 10.8 0 0 0-5.4-8.52zM7 15.25a1.25 1.25 0 1 1 0-2.5 1.25 1.25 0 0 1 0 2.5zm10 0a1.25 1.25 0 1 1 0-2.5 1.25 1.25 0 0 1 0 2.5z"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">
              {{ t('androidBetaAsk') }}
              <span class="beta-badge">{{ t('beta') }}</span>
            </span>
            <span class="setting-desc">{{ t('androidAppDesc') }}</span>
          </span>
        </RouterLink>
      </div>
    </section>
  </div>
</template>

<style scoped>
.settings-view-container {
  padding: 1.25rem 1.5rem 2rem;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

.section-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
}

.setting-rows {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.setting-text {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}

.setting-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.375rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #334155;
}

.beta-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.375rem;
  border: 1px solid #fde68a;
  border-radius: 9999px;
  background: #fffbeb;
  color: #b45309;
  font-size: 0.625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
}

:root.dark .beta-badge {
  border-color: rgb(245 158 11 / 0.35);
  background: rgb(245 158 11 / 0.15);
  color: #fcd34d;
}

.setting-desc {
  font-size: 0.75rem;
  font-weight: 400;
  line-height: 1.45;
  color: #64748b;
}

:root.dark .setting-title {
  color: #e2e8f0;
}

:root.dark .setting-desc {
  color: #94a3b8;
}

.setting-note {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  padding: 0.55rem 0.75rem;
  border-radius: 0.625rem;
  border: 1px solid #fde68a;
  background: #fffbeb;
  color: #92400e;
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1.45;
}

.setting-note svg {
  width: 0.9rem;
  height: 0.9rem;
  flex-shrink: 0;
  margin-top: 0.08rem;
}

:root.dark .setting-note {
  border-color: rgb(245 158 11 / 0.3);
  background: rgb(245 158 11 / 0.12);
  color: #fcd34d;
}

.option-group {
  display: flex;
  gap: 0.25rem;
}

.option-group.wrap {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 6.75rem), 1fr));
}

.option-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.3rem;
  padding: 0.4rem 0.25rem;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: transparent;
  color: #64748b;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 120ms ease, color 120ms ease, border-color 120ms ease;
  white-space: nowrap;
}

.option-btn:hover {
  background: #f1f5f9;
  color: #0f172a;
}

.option-btn.active {
  background: #eff6ff;
  color: #1d4ed8;
  border-color: #bfdbfe;
}

:root.dark .option-btn:hover {
  background: #334155;
  color: #e2e8f0;
}

:root.dark .option-btn.active {
  background: #1e3a5f;
  color: #93c5fd;
  border-color: #1d4ed8;
}

.setting-row {
  flex: none;
  width: 100%;
  justify-content: flex-start;
  gap: 0.625rem;
  padding: 0.75rem;
  border-radius: 0.75rem;
  border-color: #f1f5f9;
  background: #f8fafc;
  text-align: left;
  white-space: normal;
}

.setting-row:hover {
  background: #f1f5f9;
}

.setting-row.active {
  background: #eff6ff;
  border-color: #bfdbfe;
}

:root.dark .setting-row {
  border-color: rgba(30, 41, 59, 0.5);
  background: rgba(30, 41, 59, 0.3);
}

:root.dark .setting-row:hover {
  background: #1e293b;
}

:root.dark .setting-row.active {
  background: #1e3a5f;
  border-color: #1d4ed8;
}

.setting-row-icon {
  width: 1.0625rem;
  height: 1.0625rem;
  flex-shrink: 0;
  align-self: flex-start;
  margin-top: 0.0625rem;
  color: #94a3b8;
}

.setting-row.active .setting-row-icon {
  color: currentColor;
}

.setting-row .setting-text {
  flex: 1;
}

/* Themes repaint an active option-btn wholesale, so its text has to follow along. */
.setting-row.active .setting-title,
.setting-row.active .setting-desc {
  color: currentColor;
}

.setting-row.active .setting-desc {
  opacity: 0.8;
}

.setting-check {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.125rem;
  height: 1.125rem;
  flex-shrink: 0;
  align-self: flex-start;
  margin-top: 0.0625rem;
  border: 1.5px solid currentColor;
  border-radius: 0.375rem;
  opacity: 0.35;
}

.setting-check svg {
  width: 0.75rem;
  height: 0.75rem;
}

.setting-row.active .setting-check {
  opacity: 1;
}

.region-chevron {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.125rem;
  height: 1.125rem;
  flex-shrink: 0;
  align-self: center;
  opacity: 0.6;
}

.region-chevron svg {
  width: 1rem;
  height: 1rem;
  transition: transform 150ms ease;
}

.region-chevron.is-open svg {
  transform: rotate(180deg);
}
</style>
