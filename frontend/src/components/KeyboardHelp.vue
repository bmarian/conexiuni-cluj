<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useKbdLayer, useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import {CloseIcon} from '@/utils/icons.ts'
import {useOnline} from '@/composables/useOnline'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'

const {t} = useI18n()
const routeUpdates = useRouteUpdatesStore()
const {isOnline} = useOnline()
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
      ...(routeUpdates.hasFeed && isOnline.value ? [{keys: ['n'], label: t('news')}] : []),
      {keys: ['c'], label: t('city')},
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
          <button type="button" class="icon-btn" :aria-label="t('dismiss')" :title="t('dismiss')" @click="isOpen = false">
            <CloseIcon class="m-icon"/>
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
  padding: 16px;
  background: var(--md-scrim);
}

.kbd-help {
  width: min(640px, 100%);
  max-height: calc(100dvh - 32px);
  overflow-y: auto;
  padding: 16px 24px 24px;
  border-radius: 28px;
  background: var(--md-surface-container-high);
  color: var(--md-on-surface);
  box-shadow: var(--md-elevation-3);
  outline: none;
}

.kbd-help-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 -12px 12px 0;
}

.kbd-help-title {
  font-size: 24px;
  line-height: 32px;
}

.kbd-help-groups {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  gap: 16px 24px;
}

.kbd-help-group-title {
  margin-bottom: 6px;
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  color: var(--md-primary);
}

.kbd-help-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 3px 0;
  font-size: 14px;
  line-height: 20px;
}

.kbd-help-keys {
  display: flex;
  gap: 4px;
  min-width: 72px;
}

.kbd-help-keys kbd {
  min-width: 24px;
  padding: 1px 6px;
  border-radius: 6px;
  background: var(--md-surface-container-highest);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  text-align: center;
  color: var(--md-on-surface);
}

.kbd-help-label {
  color: var(--md-on-surface-variant);
}
</style>
