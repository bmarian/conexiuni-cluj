<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useKbdLayer, useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'

const {t} = useI18n()
const {closeLayers} = useKeyboardNav()
const isOpen = ref(false)
const panelRef = ref<HTMLElement | null>(null)

useKbdLayer(isOpen, {el: () => panelRef.value, close: () => { isOpen.value = false }})

useKbdShortcuts({
  '?': () => {
    const open = !isOpen.value
    closeLayers()
    isOpen.value = open
  },
}, {global: true})

const groups = computed(() => [
  {
    title: t('kbdHelpNavigate'),
    rows: [
      {keys: ['j', 'k'], label: t('kbdMoveVertical')},
      {keys: ['h', 'l'], label: t('kbdMoveSide')},
      {keys: ['Tab'], label: t('kbdSection')},
      {keys: ['Enter'], label: t('kbdOpen')},
      {keys: ['Esc', '⌫'], label: t('kbdBack')},
    ],
  },
  {
    title: t('kbdHelpGlobal'),
    rows: [
      {keys: ['g'], label: t('home')},
      {keys: ['s', '/'], label: t('kbdSearch')},
      {keys: ['o'], label: t('settings')},
      {keys: ['n'], label: t('news')},
      {keys: ['w'], label: t('weather')},
      {keys: ['m'], label: t('kbdDrawer')},
      {keys: ['?'], label: t('kbdHelp')},
    ],
  },
  {
    title: t('kbdHelpFavorites'),
    rows: [
      {keys: ['x'], label: t('kbdRemove')},
      {keys: ['J', 'K'], label: t('kbdReorder')},
    ],
  },
  {
    title: t('kbdHelpDetails'),
    rows: [
      {keys: ['f'], label: t('kbdFavorite')},
      {keys: ['c'], label: t('shareUrl')},
    ],
  },
  {
    title: t('route'),
    rows: [
      {keys: ['d'], label: t('kbdDirection')},
      {keys: ['b'], label: t('kbdFollow')},
      {keys: ['t'], label: t('kbdTimetable')},
    ],
  },
  {
    title: t('planTitle'),
    rows: [
      {keys: ['r'], label: t('kbdRefresh')},
      {keys: ['v'], label: t('kbdSwap')},
    ],
  },
])
</script>

<template>
  <Teleport to="body">
    <div v-if="isOpen" class="kbd-help-backdrop" @click.self="isOpen = false">
      <div ref="panelRef" class="kbd-help" role="dialog" :aria-label="t('kbdHelpTitle')">
        <div class="kbd-help-head">
          <h2 class="kbd-help-title">{{ t('kbdHelpTitle') }}</h2>
          <button type="button" class="kbd-help-close" :aria-label="t('dismiss')" @click="isOpen = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
                 stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>
        <div class="kbd-help-groups">
          <section v-for="group in groups" :key="group.title" class="kbd-help-group">
            <h3 class="kbd-help-group-title">{{ group.title }}</h3>
            <div v-for="row in group.rows" :key="row.label" class="kbd-help-row">
              <span class="kbd-help-keys">
                <kbd v-for="key in row.keys" :key="key">{{ key }}</kbd>
              </span>
              <span class="kbd-help-label">{{ row.label }}</span>
            </div>
          </section>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.kbd-help-backdrop {
  position: fixed;
  inset: 0;
  z-index: 10000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background: rgba(15, 23, 42, 0.45);
}

.kbd-help {
  width: min(40rem, 100%);
  max-height: calc(100dvh - 2rem);
  overflow-y: auto;
  padding: 1rem 1.125rem 1.25rem;
  border-radius: 0.875rem;
  background: #ffffff;
  color: #0f172a;
  box-shadow: 0 10px 30px -4px rgba(0, 0, 0, 0.18), 0 1px 6px rgba(0, 0, 0, 0.08);
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
  outline: none;
}

:root.dark .kbd-help {
  background: #1e293b;
  color: #f1f5f9;
  box-shadow: 0 10px 30px -4px rgba(0, 0, 0, 0.5), 0 1px 6px rgba(0, 0, 0, 0.24);
}

.kbd-help-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}

.kbd-help-title {
  font-size: 1rem;
  font-weight: 800;
}

.kbd-help-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 9999px;
  color: #94a3b8;
  cursor: pointer;
}

.kbd-help-close svg {
  width: 1rem;
  height: 1rem;
}

.kbd-help-close:hover {
  background: #f1f5f9;
  color: #475569;
}

:root.dark .kbd-help-close:hover {
  background: #334155;
  color: #e2e8f0;
}

.kbd-help-groups {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  gap: 1rem 1.5rem;
}

.kbd-help-group-title {
  margin-bottom: 0.375rem;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: #94a3b8;
}

:root.dark .kbd-help-group-title {
  color: #64748b;
}

.kbd-help-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.2rem 0;
  font-size: 0.8125rem;
}

.kbd-help-keys {
  display: flex;
  gap: 0.25rem;
  min-width: 4.5rem;
}

.kbd-help-keys kbd {
  min-width: 1.5rem;
  padding: 0.05rem 0.375rem;
  border: 1px solid #e2e8f0;
  border-bottom-width: 2px;
  border-radius: 0.375rem;
  background: #f8fafc;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.75rem;
  font-weight: 600;
  text-align: center;
  color: #334155;
}

:root.dark .kbd-help-keys kbd {
  border-color: #475569;
  background: #0f172a;
  color: #e2e8f0;
}

.kbd-help-label {
  color: #475569;
}

:root.dark .kbd-help-label {
  color: #cbd5e1;
}
</style>
