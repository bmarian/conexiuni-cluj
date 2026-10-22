<script setup lang="ts">
import {onBeforeUnmount, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import HeaderNavigation from '@/components/HeaderNavigation.vue'
import {useKbdLayer} from '@/composables/useKeyboardNav.ts'
import {BETA_GROUP_URL, TESTING_URL} from '@/utils/androidApp.ts'

const {t} = useI18n()
const router = useRouter()

useHead(() => ({
  title: t('headAndroidTitle'),
}))

const shotFiles = import.meta.glob<string>('/src/assets/android-guide/*.jpg', {eager: true, import: 'default'})

function shots(step: string): string[] {
  const prefix = `/src/assets/android-guide/${step}-`
  return Object.keys(shotFiles).filter((path) => path.startsWith(prefix)).sort().map((path) => shotFiles[path]!)
}

const openShot = ref<{ src: string, step: string } | null>(null)
const viewerRef = ref<HTMLElement | null>(null)

function openViewer(src: string, step: string) {
  history.pushState(history.state, '')
  router.listening = false
  openShot.value = {src, step}
}

function closeViewer() {
  if (openShot.value) history.back()
}

function onPopState() {
  if (!openShot.value) return
  openShot.value = null
  router.listening = true
}

window.addEventListener('popstate', onPopState)
onBeforeUnmount(() => {
  window.removeEventListener('popstate', onPopState)
  router.listening = true
})

useKbdLayer(() => openShot.value !== null, {
  el: () => viewerRef.value,
  close: closeViewer,
})

const steps = [
  {id: '1', title: 'androidStep1Title', text: 'androidStep1Text', link: BETA_GROUP_URL, button: 'androidBetaGroup'},
  {id: '2', title: 'androidStep2Title', text: 'androidStep2Text', link: TESTING_URL, button: 'androidBetaTesting'},
  {id: '3', title: 'androidStep3Title', text: 'androidStep3Text'},
]
</script>

<template>
  <div class="settings-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-7">
    <div class="flex items-center -mb-3">
      <HeaderNavigation/>
    </div>

    <header class="aa-head">
      <h1 class="aa-title">
        {{ t('androidPageTitle') }}
        <span class="beta-badge">{{ t('beta') }}</span>
      </h1>
    </header>

    <section class="aa-part">
      <h2 class="section-label">{{ t('androidAppPhone') }}</h2>
      <p>{{ t('androidBetaSteps') }}</p>
      <ol class="aa-steps" data-kbd-section="android-steps" data-kbd-entry="1">
        <li v-for="step in steps" :key="step.id" class="aa-step">
          <span class="aa-step-num" aria-hidden="true">{{ step.id }}</span>
          <div class="aa-step-body">
            <h3 class="aa-step-title">{{ t(step.title) }}</h3>
            <p>{{ t(step.text) }}</p>
            <a v-if="step.link" :href="step.link" class="aa-button" target="_blank" rel="noopener"
               :data-kbd-item="`android-step-${step.id}`">
              {{ t(step.button) }}
            </a>
            <div v-if="shots(step.id).length" class="aa-shots">
              <button v-for="src in shots(step.id)" :key="src" type="button" class="aa-shot"
                      @click="openViewer(src, step.id)">
                <img :src="src" :alt="t('androidShotAlt', {n: step.id})" loading="lazy">
              </button>
            </div>
          </div>
        </li>
      </ol>
    </section>

    <section class="aa-part">
      <h2 class="section-label">{{ t('androidAppWatch') }}</h2>
      <p>{{ t('androidBetaWatch') }}</p>
    </section>

    <Teleport to="body">
      <Transition name="aa-viewer">
        <div v-if="openShot" ref="viewerRef" class="aa-viewer" role="dialog" aria-modal="true"
             :aria-label="t('androidShotAlt', {n: openShot.step})" @click="closeViewer">
          <img :src="openShot.src" alt="" class="aa-viewer-img">
          <button type="button" class="aa-viewer-close" :aria-label="t('close')">×</button>
        </div>
      </Transition>
    </Teleport>
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

.aa-steps {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  margin: 0.5rem 0;
  padding: 0;
  list-style: none;
}

.aa-step {
  display: grid;
  grid-template-columns: 1.75rem minmax(0, 1fr);
  gap: 0.75rem;
}

.aa-step-num {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 9999px;
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
  font-weight: 800;
  font-size: 0.875rem;
}

.aa-step-body {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.5rem;
  padding-top: 0.2rem;
}

.aa-step-title {
  font-size: 0.9375rem;
  font-weight: 700;
}

.aa-shots {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  width: 100%;
  margin-top: 0.25rem;
}

.aa-shot {
  display: block;
  width: 100%;
  max-width: 18rem;
  padding: 0;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 1rem;
  background: none;
  cursor: zoom-in;
}

.aa-shot img {
  display: block;
  width: 100%;
}

:root.dark .aa-shot {
  border-color: #334155;
}

.aa-viewer {
  position: fixed;
  inset: 0;
  z-index: 10000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: env(safe-area-inset-top, 0px) env(safe-area-inset-right, 0px) env(safe-area-inset-bottom, 0px) env(safe-area-inset-left, 0px);
  background: rgb(0 0 0 / 0.92);
  cursor: zoom-out;
}

.aa-viewer-img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.aa-viewer-close {
  position: absolute;
  top: calc(env(safe-area-inset-top, 0px) + 0.75rem);
  right: calc(env(safe-area-inset-right, 0px) + 0.75rem);
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 9999px;
  background: rgb(255 255 255 / 0.15);
  color: #fff;
  font-size: 1.5rem;
  line-height: 1;
}

.aa-viewer-enter-active,
.aa-viewer-leave-active {
  transition: opacity 0.15s ease;
}

.aa-viewer-enter-from,
.aa-viewer-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .aa-viewer-enter-active,
  .aa-viewer-leave-active {
    transition: none;
  }
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
