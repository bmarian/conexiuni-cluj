<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import HeaderNavigation from '@/components/HeaderNavigation.vue'
import SettingsExportImport from '@/components/SettingsExportImport.vue'
import {useSettingsStore} from '@/stores/settings'
import {usePushStore} from '@/stores/push'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'

const {t, locale} = useI18n()
const router = useRouter()
const settings = useSettingsStore()
const push = usePushStore()
const routeUpdates = useRouteUpdatesStore()

useHead(() => ({
  title: t('headSettingsTitle'),
  meta: [{name: 'robots', content: 'noindex'}],
}))

const isAdminAuthed = ref(false)

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

async function togglePush() {
  if (push.busy) return
  if (push.enabled) {
    await push.disable()
    settings.showToast(t('pushOffToast'), {icon: 'bell-off'})
    return
  }
  const result = await push.enable()
  if (result === 'enabled') {
    settings.showToast(t('pushOnToast'), {
      body: t(routeUpdates.followed.length ? 'pushOnToastBody' : 'pushOnToastNoLines'),
      icon: 'bell',
    })
    return
  }
  const key = {
    unsupported: 'pushUnsupportedToast',
    denied: 'pushDeniedToast',
    brave: 'pushBraveToast',
    failed: 'pushFailedToast',
  }[result]
  const isIOS = /iPhone|iPad/.test(navigator.userAgent)
  const body = result === 'unsupported' && !isIOS ? undefined : t(`${key}Body`)
  settings.showToast(t(key), {body, icon: 'bell-off'})
}
</script>

<template>
  <div
    class="settings-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-7">
    <div class="flex items-center -mb-3">
      <HeaderNavigation/>
    </div>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('language') }}</h2>
      <p class="setting-note">
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
                :class="{ active: settings.showWeather }" :aria-pressed="settings.showWeather"
                data-kbd-item="show-weather" @click="settings.setShowWeather(!settings.showWeather)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">☁️</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M17.5 19H9a7 7 0 1 1 6.71-9h1.79a4.5 4.5 0 1 1 0 9Z"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t('weather') }}</span>
            <span class="setting-desc">{{ t('weatherDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.showWeather" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>

        <button type="button" class="option-btn setting-row" :class="{ active: settings.showNews }"
                :aria-pressed="settings.showNews" data-kbd-item="show-news"
                @click="settings.setShowNews(!settings.showNews)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">📰</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path
              d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2"/>
            <path d="M18 14h-8M15 18h-5M10 6h8v4h-8z"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t('news') }}</span>
            <span class="setting-desc">{{ t('newsDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.showNews" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>

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

        <button type="button" class="option-btn setting-row"
                :class="{ active: settings.showGreenFriday }"
                :aria-pressed="settings.showGreenFriday" data-kbd-item="green-friday"
                @click="settings.setShowGreenFriday(!settings.showGreenFriday)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">🌿</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path
              d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19 2c1 2 2 4.18 2 8 0 5.5-4.78 10-10 10Z"/>
            <path d="M2 21c0-3 1.85-5.36 5.08-6C9.5 14.52 12 13 13 12"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">{{ t('greenFridayTitle') }}</span>
            <span class="setting-desc">{{ t('greenFridaySettingDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.showGreenFriday" viewBox="0 0 24 24" fill="none"
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
                :class="{ active: settings.showTimetableChanges }"
                :aria-pressed="settings.showTimetableChanges" data-kbd-item="timetable-changes"
                @click="settings.setShowTimetableChanges(!settings.showTimetableChanges)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">🔔</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/>
            <path d="M13.73 21a2 2 0 0 1-3.46 0"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">
              {{ t('timetableChanges') }}
              <span class="beta-badge">{{ t('beta') }}</span>
            </span>
            <span class="setting-desc">{{ t('timetableChangesDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="settings.showTimetableChanges" viewBox="0 0 24 24" fill="none"
                 stroke="currentColor" stroke-width="3.5" stroke-linecap="round"
                 stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>

        <button v-if="settings.showTimetableChanges" type="button" class="option-btn setting-row"
                :class="{ active: push.enabled }" :aria-pressed="push.enabled"
                :aria-busy="push.busy" data-kbd-item="push-notifications" @click="togglePush">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-md" aria-hidden="true">📲</span>
          <svg v-else class="setting-row-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/>
            <path d="M13.73 21a2 2 0 0 1-3.46 0"/>
            <path d="M2 8c0-2.2.7-4.3 2-6M22 8a10 10 0 0 0-2-6"/>
          </svg>
          <span class="setting-text">
            <span class="setting-title">
              {{ t('pushNotifications') }}
              <span class="beta-badge">{{ t('beta') }}</span>
            </span>
            <span class="setting-desc">{{ t('pushNotificationsDesc') }}</span>
          </span>
          <span class="setting-check" aria-hidden="true">
            <svg v-if="push.enabled" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
          </span>
        </button>
      </div>
    </section>

    <section class="flex flex-col gap-2">
      <h2 class="section-label">{{ t('exportImport') }}</h2>
      <p class="setting-desc">{{ t('exportImportDesc') }}</p>
      <SettingsExportImport/>
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

/* Same badge as .live-badge, in the amber this page already uses for caveats. */
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

/* Same shape as .suspended-banner, in amber: a caveat, not a failure. */
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

/* A toggle is still an option-btn, just laid out as a full-width row with its explanation. */
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
</style>
