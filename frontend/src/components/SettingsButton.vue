<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'

type Theme = 'light' | 'dark' | 'system'

const { t, locale } = useI18n()
const settings = useSettingsStore()
const isDark = computed(() => settings.isDark)

const isOpen = ref(false)
const rootRef = ref<HTMLElement | null>(null)

let eggClickCount = 0

function toggle() {
  isOpen.value = !isOpen.value

  if (!settings.easterEggUnlocked) {
    eggClickCount++
    if (eggClickCount === 5) {
      settings.showToast('are you hungry?')
    } else if (eggClickCount === 10) {
      settings.showToast('me too')
      settings.unlockEasterEgg()
      settings.activateEasterEgg()
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

function onChomperChange(e: Event) {
  const val = (e.target as HTMLSelectElement).value
  if (val === 'on') settings.activateEasterEgg()
  else settings.deactivateEasterEgg()
}

function setLocale(newLocale: 'ro' | 'en') {
  settings.setLocale(newLocale)
  locale.value = newLocale
}
</script>

<template>
  <div ref="rootRef" class="settings-root" :class="{ 'is-dark': isDark }">
    <button
      type="button"
      class="settings-btn"
      :title="t('settings')"
      :aria-label="t('settings')"
      :aria-expanded="isOpen"
      @click="toggle"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="16" height="16" aria-hidden="true">
        <path d="M12.22 2h-.44a2 2 0 00-2 2v.18a2 2 0 01-1 1.73l-.43.25a2 2 0 01-2 0l-.15-.08a2 2 0 00-2.73.73l-.22.38a2 2 0 00.73 2.73l.15.1a2 2 0 011 1.72v.51a2 2 0 01-1 1.74l-.15.09a2 2 0 00-.73 2.73l.22.38a2 2 0 002.73.73l.15-.08a2 2 0 012 0l.43.25a2 2 0 011 1.73V20a2 2 0 002 2h.44a2 2 0 002-2v-.18a2 2 0 011-1.73l.43-.25a2 2 0 012 0l.15.08a2 2 0 002.73-.73l.22-.39a2 2 0 00-.73-2.73l-.15-.08a2 2 0 01-1-1.74v-.5a2 2 0 011-1.74l.15-.09a2 2 0 00.73-2.73l-.22-.38a2 2 0 00-2.73-.73l-.15.08a2 2 0 01-2 0l-.43-.25a2 2 0 01-1-1.73V4a2 2 0 00-2-2z"/>
        <circle cx="12" cy="12" r="3"/>
      </svg>
    </button>

    <div v-if="isOpen" class="settings-popover" role="dialog" :aria-label="t('settings')">
      <p class="section-label">{{ t('theme') }}</p>
      <div class="option-group" role="group" :aria-label="t('theme')">
        <button type="button" class="option-btn" :class="{ active: settings.theme === 'light' }" @click="setTheme('light')">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="13" height="13" aria-hidden="true">
            <circle cx="12" cy="12" r="4"/>
            <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/>
          </svg>
          {{ t('themeLight') }}
        </button>
        <button type="button" class="option-btn" :class="{ active: settings.theme === 'dark' }" @click="setTheme('dark')">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="13" height="13" aria-hidden="true">
            <path d="M12 3a6 6 0 009 9 9 9 0 11-9-9z"/>
          </svg>
          {{ t('themeDark') }}
        </button>
        <button type="button" class="option-btn" :class="{ active: settings.theme === 'system' }" @click="setTheme('system')">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="13" height="13" aria-hidden="true">
            <rect x="2" y="3" width="20" height="14" rx="2"/>
            <path d="M8 21h8M12 17v4"/>
          </svg>
          {{ t('themeSystem') }}
        </button>
      </div>

        <div v-if="settings.easterEggUnlocked" class="select-wrap">
          <select
            class="theme-select"
            :value="settings.easterEggActive ? 'on' : 'off'"
            :class="{ 'is-chomper': settings.easterEggActive }"
            @change="onChomperChange"
            :aria-label="t('chomperTheme')"
          >
            <option value="off">{{ t('themeDefault') }}</option>
            <option value="on">{{ t('chomperTheme') }}</option>
          </select>
          <svg class="select-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
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

    </div>
  </div>
</template>

<style scoped>
.settings-root {
  position: fixed;
  top: calc(0.75rem + env(safe-area-inset-top));
  right: calc(0.75rem + env(safe-area-inset-right));
  z-index: 4600;
  transition: right 250ms cubic-bezier(0.32, 0.72, 0, 1);
}

@media (max-width: 1023px) and (orientation: landscape) {
  .settings-root.landscape-open {
    right: calc(var(--landscape-drawer-width) + 0.75rem + env(safe-area-inset-right));
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
.theme-select:hover  { border-color: #94a3b8; background: #f8fafc; }
.theme-select:focus  { border-color: #94a3b8; }
.theme-select.is-chomper {
  border-color: #fcd34d;
  background: #fef9c3;
  color: #92400e;
}

.settings-root.is-dark .theme-select {
  border-color: #334155;
  color: #cbd5e1;
  background: transparent;
}
.settings-root.is-dark .theme-select:hover  { border-color: #475569; background: #1e293b; }
.settings-root.is-dark .theme-select option { background: #1e293b; color: #cbd5e1; }
.settings-root.is-dark .theme-select.is-chomper {
  border-color: #d97706;
  background: #422006;
  color: #fde68a;
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
.settings-root.is-dark .select-chevron { color: #475569; }
</style>
