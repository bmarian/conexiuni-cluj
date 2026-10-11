<script setup lang="ts">
import {onMounted, onUnmounted} from 'vue'
import {useI18n} from 'vue-i18n'
import {ANDROID_APP_URL, androidAppDialog, WEAR_APP_URL, WEAR_INSTALLER_URL} from '@/utils/androidApp.ts'

const {t} = useI18n()

const close = () => { androidAppDialog.value = false }
const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') close() }
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <div class="rs-scrim" @click.self="close">
      <section class="rs-sheet" role="dialog" aria-modal="true" :aria-label="t('androidApp')">
        <div class="rs-scroll">
          <header class="aa-head">
            <h2 class="aa-title">{{ t('androidApp') }}</h2>
            <span class="beta-badge">{{ t('experimental') }}</span>
            <button type="button" class="rs-close" :aria-label="t('close')" @click="close">
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </header>

          <div class="aa-part">
            <p>{{ t('androidAppSteps') }}</p>
            <p class="aa-note">{{ t('androidAppNeeds') }}</p>
            <a :href="ANDROID_APP_URL" class="rs-save aa-button">{{ t('androidAppDownload') }}</a>
          </div>

          <div class="aa-part">
            <h3 class="aa-subtitle">{{ t('androidAppWatch') }}</h3>
            <p>{{ t('androidAppWatchSteps') }}</p>
            <div class="aa-links">
              <a :href="WEAR_APP_URL" class="rs-text-btn">{{ t('androidAppWatchDownload') }}</a>
              <a :href="WEAR_INSTALLER_URL" class="rs-text-btn" target="_blank" rel="noopener">Wear Installer 2</a>
            </div>
          </div>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.rs-scrim {
  position: fixed;
  inset: 0;
  z-index: 10000;
  background: rgba(15, 23, 42, 0.45);
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.rs-sheet {
  width: 100%;
  max-width: 28rem;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  background: var(--color-white, #fff);
  color: var(--color-slate-900, #0f172a);
  border-radius: 1.25rem 1.25rem 0 0;
  box-shadow: 0 -8px 32px rgba(0, 0, 0, 0.2);
}

@media (min-width: 768px) {
  .rs-scrim {
    align-items: center;
  }

  .rs-sheet {
    border-radius: 1.25rem;
  }
}

.rs-scroll {
  overflow-y: auto;
  padding: 1.25rem 1.25rem calc(1.25rem + env(safe-area-inset-bottom));
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.aa-head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.aa-title {
  font-size: 1.125rem;
  font-weight: 800;
}

.aa-head .rs-close {
  margin-left: auto;
}

.aa-part {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-size: 0.875rem;
  line-height: 1.5;
}

.aa-subtitle {
  font-size: 0.9375rem;
  font-weight: 700;
}

.aa-note {
  font-size: 0.75rem;
  color: var(--color-slate-500, #64748b);
}

.aa-button {
  align-self: flex-start;
  margin-top: 0.25rem;
}

.aa-links {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-left: -0.75rem;
}

:root[data-paper] .aa-links,
:root[data-legacy-blue] .aa-links {
  gap: 0.5rem;
  margin-left: 0;
}

.rs-close {
  width: 2rem;
  height: 2rem;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-slate-400, #94a3b8);
}

.rs-close:hover {
  background: var(--color-slate-100, #f1f5f9);
}

.rs-save {
  padding: 0.6rem 1.1rem;
  border-radius: 9999px;
  font-weight: 700;
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
}

.rs-text-btn {
  padding: 0.5rem 0.75rem;
  border-radius: 9999px;
  font-weight: 600;
  color: var(--color-emerald-600, #059669);
}

.rs-text-btn:hover {
  background: var(--color-emerald-50, #ecfdf5);
}

.beta-badge {
  display: inline-flex;
  padding: 0.1rem 0.375rem;
  border: 1px solid #fde68a;
  border-radius: 9999px;
  background: #fffbeb;
  color: #b45309;
  font-size: 0.625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
}

:root.dark .rs-sheet {
  background: var(--color-slate-900, #0f172a);
  color: var(--color-slate-100, #f1f5f9);
}

:root.dark .aa-note {
  color: var(--color-slate-400, #94a3b8);
}

:root.dark .rs-close:hover {
  background: var(--color-slate-800, #1e293b);
}

:root.dark .rs-text-btn {
  color: var(--color-emerald-400, #34d399);
}

:root.dark .rs-text-btn:hover {
  background: rgb(16 185 129 / 0.12);
}

:root.dark .beta-badge {
  border-color: rgb(245 158 11 / 0.35);
  background: rgb(245 158 11 / 0.12);
  color: #fbbf24;
}
</style>
