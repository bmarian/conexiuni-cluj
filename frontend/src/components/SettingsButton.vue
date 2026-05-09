<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'
import SettingsExportImport from '@/components/SettingsExportImport.vue'

type Theme = 'light' | 'dark' | 'system'

const {t, locale} = useI18n()
const settings = useSettingsStore()
const isDark = computed(() => settings.isDark)

const isOpen = ref(false)
const rootRef = ref<HTMLElement | null>(null)

let arcadeClickCount = 0

function toggle() {
  isOpen.value = !isOpen.value

  if (!settings.arcadeUnlocked) {
    arcadeClickCount++
    if (arcadeClickCount === 5) {
      settings.showToast(t('arcadeInsertCoinToast'))
    } else if (arcadeClickCount === 10) {
      settings.showToast(t('arcadeGameStartToast'))
      settings.unlockArcade()
      settings.activateArcade()
      isOpen.value = false
    }
  }
}

function onDocumentPointerDown(e: PointerEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => document.addEventListener('pointerdown', onDocumentPointerDown))
onUnmounted(() => document.removeEventListener('pointerdown', onDocumentPointerDown))

function setTheme(theme: Theme) {
  settings.setTheme(theme)
}

const activeSpecialTheme = computed(() => {
  if (settings.arcadeActive) return 'arcade'
  if (settings.legacyBlueActive) return 'legacy-blue'
  return 'default'
})

function onSpecialThemeChange(e: Event) {
  const val = (e.target as HTMLSelectElement).value
  if (val === 'arcade') settings.activateArcade()
  else if (val === 'legacy-blue') settings.activateLegacyBlue()
  else {
    settings.deactivateArcade();
    settings.deactivateLegacyBlue()
  }
}

function setLocale(newLocale: 'ro' | 'en') {
  settings.setLocale(newLocale)
  locale.value = newLocale
}
</script>

<template>
  <div ref="rootRef" class="settings-root" :class="{ 'is-dark': isDark }"
       :style="isOpen ? { zIndex: 9999 } : {}">
    <button
      type="button"
      class="settings-btn"
      :title="t('settings')"
      :aria-label="t('settings')"
      :aria-expanded="isOpen"
      @click="toggle"
    >
      <span v-if="settings.legacyBlueActive" class="emoji-icon" aria-hidden="true">⚙️</span>
      <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
           width="16" height="16" aria-hidden="true">
        <path
          d="M12.22 2h-.44a2 2 0 00-2 2v.18a2 2 0 01-1 1.73l-.43.25a2 2 0 01-2 0l-.15-.08a2 2 0 00-2.73.73l-.22.38a2 2 0 00.73 2.73l.15.1a2 2 0 011 1.72v.51a2 2 0 01-1 1.74l-.15.09a2 2 0 00-.73 2.73l.22.38a2 2 0 002.73.73l.15-.08a2 2 0 012 0l.43.25a2 2 0 011 1.73V20a2 2 0 002 2h.44a2 2 0 002-2v-.18a2 2 0 011-1.73l.43-.25a2 2 0 012 0l.15.08a2 2 0 002.73-.73l.22-.39a2 2 0 00-.73-2.73l-.15-.08a2 2 0 01-1-1.74v-.5a2 2 0 011-1.74l.15-.09a2 2 0 00.73-2.73l-.22-.38a2 2 0 00-2.73-.73l-.15.08a2 2 0 01-2 0l-.43-.25a2 2 0 01-1-1.73V4a2 2 0 00-2-2z"/>
        <circle cx="12" cy="12" r="3"/>
      </svg>
    </button>

    <div v-if="isOpen" class="settings-popover" role="dialog" :aria-label="t('settings')">
      <p class="section-label">{{ t('theme') }}</p>
      <div class="option-group" role="group" :aria-label="t('theme')">
        <button type="button" class="option-btn" :class="{ active: settings.theme === 'light' }"
                @click="setTheme('light')">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">☀️</span>
          <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               width="13" height="13" aria-hidden="true">
            <circle cx="12" cy="12" r="4"/>
            <path
              d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/>
          </svg>
          {{ t('themeLight') }}
        </button>
        <button type="button" class="option-btn" :class="{ active: settings.theme === 'dark' }"
                @click="setTheme('dark')">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">🌙</span>
          <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               width="13" height="13" aria-hidden="true">
            <path d="M12 3a6 6 0 009 9 9 9 0 11-9-9z"/>
          </svg>
          {{ t('themeDark') }}
        </button>
        <button type="button" class="option-btn" :class="{ active: settings.theme === 'system' }"
                @click="setTheme('system')">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">🖥️</span>
          <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               width="13" height="13" aria-hidden="true">
            <rect x="2" y="3" width="20" height="14" rx="2"/>
            <path d="M8 21h8M12 17v4"/>
          </svg>
          {{ t('themeSystem') }}
        </button>
      </div>

      <div v-if="settings.arcadeUnlocked || settings.legacyBlueUnlocked" class="select-wrap">
        <select
          class="theme-select"
          :value="activeSpecialTheme"
          :class="{
              'is-arcade': settings.arcadeActive,
              'is-legacy-blue': settings.legacyBlueActive,
            }"
          @change="onSpecialThemeChange"
          :aria-label="t('theme')"
        >
          <option value="default">{{ t('themeDefault') }}</option>
          <option v-if="settings.arcadeUnlocked" value="arcade">{{ t('arcadeTheme') }}</option>
          <option v-if="settings.legacyBlueUnlocked" value="legacy-blue">{{
              t('legacyBlueTheme')
            }}
          </option>
        </select>
        <svg class="select-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M6 9l6 6 6-6"/>
        </svg>
      </div>


      <p class="section-label">{{ t('language') }}</p>
      <div class="option-group" role="group" :aria-label="t('language')">
        <button
          type="button"
          class="option-btn"
          :class="{ active: settings.locale === 'ro' }"
          @click="setLocale('ro')"
        >
          Română
        </button>
        <button
          type="button"
          class="option-btn"
          :class="{ active: settings.locale === 'en' }"
          @click="setLocale('en')"
        >
          English
        </button>
      </div>

      <p class="section-label">{{ t('display') }}</p>
      <div class="option-group display-grid" role="group" :aria-label="t('display')">
        <button type="button" class="option-btn" :class="{ active: settings.showWeather }"
                :title="t('weather')" @click="settings.setShowWeather(!settings.showWeather)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">☁️</span>
          <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               width="13" height="13" aria-hidden="true" style="flex-shrink:0">
            <path d="M17.5 19H9a7 7 0 1 1 6.71-9h1.79a4.5 4.5 0 1 1 0 9Z"/>
          </svg>
          <span class="btn-text">{{ t('weather') }}</span>
        </button>
        <button type="button" class="option-btn" :class="{ active: settings.showNews }"
                :title="t('news')" @click="settings.setShowNews(!settings.showNews)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">📰</span>
          <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               width="13" height="13" aria-hidden="true" style="flex-shrink:0">
            <path d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2"/>
            <path d="M18 14h-8M15 18h-5M10 6h8v4h-8z"/>
          </svg>
          <span class="btn-text">{{ t('news') }}</span>
        </button>
        <button type="button" class="option-btn" :class="{ active: settings.autoCenterOnMe }"
                :title="t('autoCenterOnMe')" @click="settings.setAutoCenterOnMe(!settings.autoCenterOnMe)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">📍</span>
          <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               width="13" height="13" aria-hidden="true" style="flex-shrink:0">
            <circle cx="12" cy="12" r="3"/>
            <path d="M12 2v3M12 19v3M2 12h3M19 12h3"/>
            <circle cx="12" cy="12" r="8"/>
          </svg>
          <span class="btn-text">{{ t('autoCenterOnMe') }}</span>
        </button>
        <button type="button" class="option-btn" :class="{ active: settings.autoFitMap }"
                :title="t('autoFitMap')" @click="settings.setAutoFitMap(!settings.autoFitMap)">
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">🗺️</span>
          <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               width="13" height="13" aria-hidden="true" style="flex-shrink:0">
            <path d="M8 3H5a2 2 0 0 0-2 2v3M21 8V5a2 2 0 0 0-2-2h-3M3 16v3a2 2 0 0 0 2 2h3M16 21h3a2 2 0 0 0 2-2v-3"/>
          </svg>
          <span class="btn-text">{{ t('autoFitMap') }}</span>
        </button>
      </div>

      <SettingsExportImport />

    </div>
  </div>
</template>

<style scoped>
.settings-root {
  position: fixed;
  top: calc(0.75rem + env(safe-area-inset-top));
  right: calc(0.75rem + env(safe-area-inset-right));
  z-index: 3000;
  transition: right 250ms cubic-bezier(0.32, 0.72, 0, 1);
}

@media (max-width: 1023px) and (orientation: landscape) {
  .settings-root {
    top: calc(0.75rem + env(safe-area-inset-top));
    right: calc(0.75rem + env(safe-area-inset-right) + (var(--controls-row-index, 0) * 2.75rem));
  }

  .settings-root.landscape-open {
    right: calc(var(--landscape-drawer-width) + 0.75rem + env(safe-area-inset-right) + (var(--controls-row-index, 0) * 2.75rem));
  }
}

@media (min-width: 1024px) {
  .settings-root {
    right: calc(30vw + 0.75rem + env(safe-area-inset-right));
  }
}

.settings-btn {
  width: 2.25rem;
  height: 2.25rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 0.875rem;
  background: #ffffff;
  color: #334155;
  box-shadow: 0 2px 10px -1px rgba(0, 0, 0, 0.14), 0 1px 3px rgba(0, 0, 0, 0.08);
  cursor: pointer;
  transition: background 150ms ease, color 150ms ease;
}

.settings-btn:hover {
  background: #f1f5f9;
  color: #0f172a;
}

.settings-root.is-dark .settings-btn {
  background: #0f172a;
  color: #f1f5f9;
  box-shadow: 0 4px 16px -2px rgba(0, 0, 0, 0.4), 0 1px 4px rgba(0, 0, 0, 0.24);
}

.settings-root.is-dark .settings-btn:hover {
  background: #1e293b;
  color: #f8fafc;
}

.settings-popover {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  min-width: 13rem;
  max-height: calc(100dvh - 4rem - env(safe-area-inset-top) - env(safe-area-inset-bottom));
  overflow-y: auto;
  background: #ffffff;
  border-radius: 0.875rem;
  box-shadow: 0 10px 30px -4px rgba(0, 0, 0, 0.18), 0 1px 6px rgba(0, 0, 0, 0.08);
  padding: 0.875rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.settings-root.is-dark .settings-popover {
  background: #1e293b;
  box-shadow: 0 10px 30px -4px rgba(0, 0, 0, 0.5), 0 1px 6px rgba(0, 0, 0, 0.24);
}

.section-label {
  margin: 0 0 0.4rem 0;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: #94a3b8;
}

.section-label + .option-group + .section-label {
  margin-top: 0.75rem;
}

.settings-root.is-dark .section-label {
  color: #64748b;
}

.option-group {
  display: flex;
  gap: 0.25rem;
}

.display-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.display-grid .option-btn {
  overflow: hidden;
  min-width: 0;
}

.display-grid .option-btn .btn-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
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

.settings-root.is-dark .option-btn {
  color: #64748b;
}

.settings-root.is-dark .option-btn:hover {
  background: #334155;
  color: #e2e8f0;
}

.settings-root.is-dark .option-btn.active {
  background: #1e3a5f;
  color: #93c5fd;
  border-color: #1d4ed8;
}

.select-wrap {
  position: relative;
  width: 100%;
}

.theme-select {
  width: 100%;
  padding: 0.4rem 2rem 0.4rem 0.625rem;
  margin: 0.4rem 0;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  background: transparent;
  color: #334155;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  appearance: none;
  -webkit-appearance: none;
  outline: none;
  transition: border-color 120ms ease, background 120ms ease;
}

.theme-select:hover {
  border-color: #94a3b8;
  background: #f8fafc;
}

.theme-select:focus {
  border-color: #94a3b8;
}

.theme-select.is-arcade {
  border-color: #fcd34d;
  background: #fef9c3;
  color: #92400e;
}

.theme-select.is-legacy-blue {
  border-color: #245EDC;
  background: #EEF3FF;
  color: #1A3A8C;
}

.settings-root.is-dark .theme-select {
  border-color: #334155;
  color: #cbd5e1;
  background: transparent;
}

.settings-root.is-dark .theme-select:hover {
  border-color: #475569;
  background: #1e293b;
}

.settings-root.is-dark .theme-select option {
  background: #1e293b;
  color: #cbd5e1;
}

.settings-root.is-dark .theme-select.is-arcade {
  border-color: #d97706;
  background: #422006;
  color: #fde68a;
}

.settings-root.is-dark .theme-select.is-arcade option {
  background: #422006;
  color: #fde68a;
}

.settings-root.is-dark .theme-select.is-legacy-blue {
  border-color: #2A508C;
  background: #10193A;
  color: #90B4E0;
}

.select-chevron {
  position: absolute;
  right: 0.5rem;
  top: 50%;
  transform: translateY(-50%);
  width: 0.875rem;
  height: 0.875rem;
  color: #94a3b8;
  pointer-events: none;
}

.settings-root.is-dark .select-chevron {
  color: #475569;
}
</style>
