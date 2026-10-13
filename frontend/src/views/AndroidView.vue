<script setup lang="ts">
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import HeaderNavigation from '@/components/HeaderNavigation.vue'
import {useSettingsStore} from '@/stores/settings'
import {BETA_GROUP_URL, TESTING_URL} from '@/utils/androidApp.ts'

const {t} = useI18n()
const settings = useSettingsStore()

useHead(() => ({
  title: t('headAndroidTitle'),
}))
</script>

<template>
  <div class="settings-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-7">
    <div class="flex items-center -mb-3">
      <HeaderNavigation/>
    </div>

    <header v-if="settings.paperActive" class="pp-masthead">
      <p class="pp-kicker">Conexiuni Cluj</p>
      <h1 class="pp-title pp-title-sm">{{ t('androidApp') }}</h1>
      <p class="pp-subtitle">{{ t('androidAppDesc') }}</p>
    </header>
    <header v-else class="aa-head">
      <h1 class="aa-title">
        {{ t('androidApp') }}
        <span class="beta-badge">{{ t('beta') }}</span>
      </h1>
      <p class="aa-desc">{{ t('androidAppDesc') }}</p>
    </header>

    <section class="aa-part">
      <h2 class="section-label">{{ t('androidAppPhone') }}</h2>
      <p>{{ t('androidBetaSteps') }}</p>
      <div class="aa-buttons" data-kbd-section="android-beta" data-kbd-axis="x" data-kbd-entry="1">
        <a :href="BETA_GROUP_URL" class="aa-button" target="_blank" rel="noopener" data-kbd-item="android-group">
          {{ t('androidBetaGroup') }}
        </a>
        <a :href="TESTING_URL" class="aa-button" target="_blank" rel="noopener" data-kbd-item="android-get">
          {{ t('androidBetaGet') }}
        </a>
      </div>
      <p class="aa-desc">{{ t('androidAppNeeds') }}</p>
    </section>

    <section class="aa-part">
      <h2 class="section-label">{{ t('androidAppWatch') }}</h2>
      <p>{{ t('androidBetaWatch') }}</p>
      <p class="aa-desc">{{ t('androidAppWatchNeeds') }}</p>
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

.aa-head {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.aa-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
  font-size: 1.375rem;
  font-weight: 800;
}

.aa-part {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-size: 0.875rem;
  line-height: 1.5;
}

.section-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
}

.aa-desc {
  font-size: 0.75rem;
  line-height: 1.45;
  color: #64748b;
}

:root.dark .aa-desc {
  color: #94a3b8;
}

.aa-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin: 0.25rem 0;
}

.aa-button {
  padding: 0.6rem 1.1rem;
  border-radius: 9999px;
  font-weight: 700;
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
}

.aa-button:hover {
  background: var(--color-emerald-600, #059669);
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
</style>
