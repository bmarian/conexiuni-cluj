<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'
import {cities, cityChoice, cityInfo, currentCity, locationGranted, switchCity} from '@/utils/city.ts'
import {useKbdLayer, useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'

const props = withDefaults(defineProps<{ topOffset?: string }>(), {topOffset: '3.5rem'})

const {t} = useI18n()
const settings = useSettingsStore()
const isDark = computed(() => settings.isDark)
const topValue = computed(() => props.topOffset)

const isOpen = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const popoverRef = ref<HTMLElement | null>(null)

// The bare pin stands for "nearest city", which needs location access.
const automatic = computed(() => locationGranted.value && !cityChoice.value)
const chosen = computed(() => (automatic.value ? null : cityInfo()?.name ?? null))
const label = computed(() => chosen.value ? `${t('city')}: ${chosen.value}` : t('cityAuto'))

const {closeLayers} = useKeyboardNav()
useKbdLayer(isOpen, {el: () => popoverRef.value, close: () => { isOpen.value = false }})
useKbdShortcuts({
  c: () => {
    const open = !isOpen.value
    closeLayers()
    isOpen.value = open
  },
}, {global: true})

function toggle() {
  isOpen.value = !isOpen.value
}

function choose(slug: string | null) {
  isOpen.value = false
  if (slug === cityChoice.value) return
  void switchCity(slug)
}

function onDocumentPointerDown(e: PointerEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) isOpen.value = false
}

onMounted(() => document.addEventListener('pointerdown', onDocumentPointerDown))
onUnmounted(() => document.removeEventListener('pointerdown', onDocumentPointerDown))
</script>

<template>
  <div ref="rootRef" class="city-root" :class="{ 'is-dark': isDark }" :style="isOpen ? { zIndex: 9999 } : {}">
    <button
      type="button"
      class="city-btn"
      :class="{ 'is-named': chosen }"
      :title="label"
      :aria-label="label"
      :aria-expanded="isOpen"
      @click="toggle"
    >
      <span v-if="settings.legacyBlueActive" class="emoji-icon" aria-hidden="true">📍</span>
      <span v-else-if="settings.paperActive" class="pp-slip-label">{{ t('city') }}</span>
      <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
           width="16" height="16" aria-hidden="true">
        <path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z"/>
        <circle cx="12" cy="10" r="3"/>
      </svg>
      <span v-if="chosen" class="city-name">{{ chosen }}</span>
    </button>

    <div v-if="isOpen" ref="popoverRef" class="city-popover" role="dialog" :aria-label="t('city')">
      <p class="city-popover-title">{{ t('city') }}</p>
      <div class="city-list" data-kbd-section="city">
        <button v-if="locationGranted" type="button" class="city-item" :class="{ 'is-current': automatic }" data-kbd-item="auto" @click="choose(null)">
          <span class="city-item-title">{{ t('cityAuto') }}</span>
        </button>
        <button v-for="city in cities" :key="city.slug" type="button" class="city-item"
                :class="{ 'is-current': !automatic && city.slug === currentCity }" :data-kbd-item="city.slug" @click="choose(city.slug)">
          <span class="city-item-title">{{ city.name }}</span>
          <span class="city-item-meta">
            {{ city.operator }}
            <span class="city-tag" :class="city.realtime ? 'is-live' : 'is-timetable'">{{ t(city.realtime ? 'live' : 'timetable') }}</span>
          </span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.city-root {
  position: fixed;
  top: calc(v-bind(topValue) + env(safe-area-inset-top));
  right: calc(0.75rem + env(safe-area-inset-right));
  z-index: 3000;
  transition: right 250ms cubic-bezier(0.32, 0.72, 0, 1);
}

@media (max-width: 1023px) and (orientation: landscape) {
  .city-root {
    --anchor-right: calc(0.75rem + env(safe-area-inset-right) + (var(--controls-row-index, 0) * 2.75rem));
    top: calc(0.75rem + env(safe-area-inset-top));
    right: var(--anchor-right);
  }

  .city-root.landscape-open {
    --anchor-right: calc(var(--landscape-drawer-width) + 0.75rem + env(safe-area-inset-right) + (var(--controls-row-index, 0) * 2.75rem));
  }
}

@media (min-width: 1024px) {
  .city-root {
    right: calc(30vw + 0.75rem + env(safe-area-inset-right));
  }
}

.city-btn {
  height: 2.25rem;
  min-width: 2.25rem;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  border: 0;
  border-radius: 0.875rem;
  background: #ffffff;
  color: #334155;
  box-shadow: 0 2px 10px -1px rgba(0, 0, 0, 0.14), 0 1px 3px rgba(0, 0, 0, 0.08);
  cursor: pointer;
  transition: background 150ms ease, color 150ms ease;
}

.city-btn.is-named {
  padding: 0 0.75rem 0 0.625rem;
}

.city-name {
  font-size: 0.8125rem;
  font-weight: 600;
  white-space: nowrap;
}

.city-btn:hover {
  background: #f1f5f9;
}

.is-dark .city-btn {
  background: #1e293b;
  color: #e2e8f0;
  box-shadow: 0 2px 10px -1px rgba(0, 0, 0, 0.5), 0 1px 3px rgba(0, 0, 0, 0.3);
}

.is-dark .city-btn:hover {
  background: #334155;
}

.city-popover {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  width: 15rem;
  padding: 0.75rem;
  border-radius: 1rem;
  background: #ffffff;
  color: #0f172a;
  box-shadow: 0 10px 30px -8px rgba(0, 0, 0, 0.25), 0 2px 8px rgba(0, 0, 0, 0.08);
}

.is-dark .city-popover {
  background: #1e293b;
  color: #f1f5f9;
  box-shadow: 0 10px 30px -8px rgba(0, 0, 0, 0.7), 0 2px 8px rgba(0, 0, 0, 0.4);
}

.city-popover-title {
  margin: 0 0 0.5rem;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #64748b;
}

.is-dark .city-popover-title {
  color: #94a3b8;
}

.city-list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.city-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.125rem;
  width: 100%;
  padding: 0.5rem 0.625rem;
  border: 0;
  border-radius: 0.625rem;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.city-item:hover {
  background: #f1f5f9;
}

.is-dark .city-item:hover {
  background: #334155;
}

.city-item.is-current {
  background: #e2e8f0;
}

.is-dark .city-item.is-current {
  background: #475569;
}

.city-item-title {
  font-size: 0.875rem;
  font-weight: 600;
}

.city-item-meta {
  display: flex;
  align-self: stretch;
  align-items: center;
  justify-content: space-between;
  gap: 0.375rem;
  font-size: 0.75rem;
  color: #64748b;
}

.is-dark .city-item-meta {
  color: #94a3b8;
}

.city-tag {
  padding: 0.125rem 0.375rem;
  border-radius: 9999px;
  border: 1px solid;
  font-size: 0.625rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  line-height: 1.2;
}

.city-tag.is-live {
  color: #059669;
  background: #ecfdf5;
  border-color: #a7f3d0;
}

.city-tag.is-timetable {
  color: #475569;
  background: #f1f5f9;
  border-color: #cbd5e1;
}

.is-dark .city-tag.is-live {
  color: #34d399;
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(52, 211, 153, 0.4);
}

.is-dark .city-tag.is-timetable {
  color: #cbd5e1;
  background: rgba(148, 163, 184, 0.15);
  border-color: rgba(148, 163, 184, 0.4);
}
</style>
