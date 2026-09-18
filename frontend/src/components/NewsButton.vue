<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {useSettingsStore} from '@/stores/settings'
import {type RouteChange, useRouteUpdatesStore} from '@/stores/routeUpdates'
import {apiRequest} from '@/utils/api'
import {changeDirection, changeKindLabels, changeTitle, formatChangeDate} from '@/utils/routeChanges'
import {useKbdLayer, useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import IconBellFilled from '@/components/icons/IconBellFilled.vue'

interface NewsItem {
  url: string
  date: string
  title: string
}

const props = withDefaults(defineProps<{ topOffset?: string }>(), {topOffset: '3.5rem'})

const {t, locale} = useI18n()
const settings = useSettingsStore()
const routeUpdates = useRouteUpdatesStore()
const router = useRouter()
const currentRoute = useRoute()
const isDark = computed(() => settings.isDark)

const isOpen = ref(false)
const rootRef = ref<HTMLElement | null>(null)

const newsItems = ref<NewsItem[]>([])
const loading = ref(false)
const error = ref(false)

// Kept for the whole time the popover is open, so the "new" dots don't vanish on first render.
const freshIds = ref(new Set<number>())

async function fetchNews() {
  loading.value = true
  error.value = false
  try {
    newsItems.value = await apiRequest<NewsItem[]>('news')
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

watch([isOpen, () => routeUpdates.changes], ([open], [wasOpen]) => {
  if (!open) return
  if (!wasOpen) freshIds.value = new Set()
  if (!routeUpdates.hasUnseen) return
  freshIds.value = new Set([...freshIds.value, ...routeUpdates.unseenIds])
  routeUpdates.markAllSeen()
})

const changeRows = computed(() => routeUpdates.changes.map((change) => ({
  change,
  date: formatChangeDate(change.detected_at, locale.value),
  title: changeTitle(change, t),
  kinds: changeKindLabels(change, t).join(' · '),
  isNew: freshIds.value.has(change.id),
})))

function openChange(change: RouteChange) {
  if (!change.route_id) return
  isOpen.value = false
  const onSameRoute = currentRoute.name === 'route' && Number(currentRoute.params.routeId) === change.route_id
  const direction = onSameRoute ? String(currentRoute.params.direction) : changeDirection(change)
  void router.push({
    name: 'route',
    params: {routeId: String(change.route_id), direction},
    query: {change: String(change.id)},
  })
}

const popoverRef = ref<HTMLElement | null>(null)
const {closeLayers} = useKeyboardNav()

useKbdLayer(isOpen, {el: () => popoverRef.value, close: () => { isOpen.value = false }})

useKbdShortcuts({
  n: () => {
    const open = !isOpen.value
    closeLayers()
    isOpen.value = open
  },
}, {global: true})

function toggle() {
  isOpen.value = !isOpen.value
}

function onDocumentPointerDown(e: PointerEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

function onVisibilityChange() {
  if (document.visibilityState === 'visible') routeUpdates.refreshIfStale()
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocumentPointerDown)
  document.addEventListener('visibilitychange', onVisibilityChange)
  void fetchNews()
  void routeUpdates.fetchChanges()
})

onUnmounted(() => {
  document.removeEventListener('pointerdown', onDocumentPointerDown)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})

const topValue = computed(() => props.topOffset)
</script>

<template>
  <div ref="rootRef" class="news-root" :class="{ 'is-dark': isDark }"
       :style="isOpen ? { zIndex: 9999 } : {}">
    <button
      type="button"
      class="news-btn"
      :class="{'has-updates': routeUpdates.hasUnseen}"
      :title="routeUpdates.hasUnseen ? t('newsHasUpdates') : t('news')"
      :aria-label="routeUpdates.hasUnseen ? t('newsHasUpdates') : t('news')"
      :aria-expanded="isOpen"
      @click="toggle"
    >
      <span v-if="settings.legacyBlueActive" class="emoji-icon" aria-hidden="true">📰</span>
      <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
           width="16" height="16" aria-hidden="true">
        <path d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2"/>
        <path d="M18 14h-8M15 18h-5M10 6h8v4h-8z"/>
      </svg>
      <span v-if="routeUpdates.hasUnseen" class="news-dot" aria-hidden="true"></span>
    </button>

    <div v-if="isOpen" ref="popoverRef" class="news-popover" role="dialog" :aria-label="t('news')">
      <p class="news-popover-title">{{ t('news') }}</p>

      <div class="news-scroll">
        <section class="news-section">
          <div class="news-section-head">
            <span v-if="settings.legacyBlueActive" class="news-section-icon" aria-hidden="true">🔔</span>
            <IconBellFilled v-else class="news-section-icon news-section-icon-bell"/>
            <span class="news-section-title">{{ t('newsYourLines') }}</span>
            <span class="news-source-tag news-source-ours">{{ t('newsSourceOurs') }}</span>
          </div>

          <p v-if="!routeUpdates.followed.length" class="news-hint">{{ t('newsFollowHint') }}</p>
          <div v-else-if="routeUpdates.loading && !changeRows.length" class="news-state">
            <span>{{ t('newsLoading') }}</span>
          </div>
          <div v-else-if="routeUpdates.error && !changeRows.length" class="news-state news-state-error">
            <span>{{ t('newsError') }}</span>
          </div>
          <p v-else-if="!changeRows.length" class="news-hint">{{ t('newsNoChanges') }}</p>
          <div v-else class="news-list" data-kbd-section="news-ours">
            <button
              v-for="row in changeRows"
              :key="row.change.id"
              type="button"
              class="news-item news-change"
              :class="{ 'is-new': row.isNew }"
              :disabled="!row.change.route_id"
              :data-kbd-item="`change-${row.change.id}`"
              @click="openChange(row.change)"
            >
              <span class="news-change-badge" :style="{ backgroundColor: row.change.route_color || '#64748b' }">
                {{ row.change.route_short_name }}
              </span>
              <span class="news-change-body">
                <span class="news-date">
                  {{ row.date }}
                  <span v-if="row.isNew" class="news-new-dot" :aria-label="t('newsNew')"></span>
                </span>
                <span class="news-change-title">{{ row.title }}</span>
                <span class="news-change-kinds">{{ row.kinds }}</span>
              </span>
            </button>
          </div>
        </section>

        <section class="news-section">
          <div class="news-section-head">
            <span v-if="settings.legacyBlueActive" class="news-section-icon" aria-hidden="true">📰</span>
            <svg v-else class="news-section-icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"
                 fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"
                 stroke-linejoin="round" aria-hidden="true">
              <path d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2"/>
              <path d="M18 14h-8M15 18h-5M10 6h8v4h-8z"/>
            </svg>
            <span class="news-section-title">{{ t('newsCtp') }}</span>
            <span class="news-source-tag">ctpcj.ro</span>
          </div>

          <div v-if="loading && newsItems.length === 0" class="news-state">
            <span>{{ t('newsLoading') }}</span>
          </div>
          <div v-else-if="error && newsItems.length === 0" class="news-state news-state-error">
            <span>{{ t('newsError') }}</span>
          </div>
          <div v-else-if="newsItems.length === 0" class="news-state">
            <span>{{ t('newsEmpty') }}</span>
          </div>
          <div v-else class="news-list" data-kbd-section="news">
            <a
              v-for="item in newsItems"
              :key="item.url"
              :href="item.url"
              target="_blank"
              rel="noopener noreferrer"
              class="news-item"
              :data-kbd-item="`news-${item.url}`"
            >
              <span class="news-date">{{ item.date }}</span>
              <span class="news-title">
                {{ item.title }}
                <svg class="news-external" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
                     stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
                     aria-hidden="true">
                  <path d="M7 17 17 7M8 7h9v9"/>
                </svg>
              </span>
            </a>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.news-root {
  position: fixed;
  top: calc(v-bind(topValue) + env(safe-area-inset-top));
  right: calc(0.75rem + env(safe-area-inset-right));
  z-index: 3000;
  transition: right 250ms cubic-bezier(0.32, 0.72, 0, 1);
}

@media (max-width: 1023px) and (orientation: landscape) {
  .news-root {
    top: calc(0.75rem + env(safe-area-inset-top));
    right: calc(0.75rem + env(safe-area-inset-right) + (var(--controls-row-index, 0) * 2.75rem));
  }

  .news-root.landscape-open {
    right: calc(var(--landscape-drawer-width) + 0.75rem + env(safe-area-inset-right) + (var(--controls-row-index, 0) * 2.75rem));
  }
}

@media (min-width: 1024px) {
  .news-root {
    right: calc(30vw + 0.75rem + env(safe-area-inset-right));
  }
}

.news-btn {
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
  position: relative;
}

.news-btn::after {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: inherit;
  border: 2px solid transparent;
  pointer-events: none;
}

.news-btn:hover {
  background: #f1f5f9;
  color: #0f172a;
}

.news-root.is-dark .news-btn {
  background: #0f172a;
  color: #f1f5f9;
  box-shadow: 0 4px 16px -2px rgba(0, 0, 0, 0.4), 0 1px 4px rgba(0, 0, 0, 0.24);
}

.news-root.is-dark .news-btn:hover {
  background: #1e293b;
  color: #f8fafc;
}

.news-dot {
  position: absolute;
  top: -0.2rem;
  right: -0.2rem;
  width: 0.625rem;
  height: 0.625rem;
  border-radius: 9999px;
  background: #eab308;
  border: 2px solid #ffffff;
}

.news-root.is-dark .news-dot {
  border-color: #0f172a;
}

.news-popover {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  width: min(20rem, calc(100vw - 1.5rem));
  max-height: min(28rem, calc(100dvh - 7rem - env(safe-area-inset-top) - env(safe-area-inset-bottom)));
  background: #ffffff;
  border-radius: 0.875rem;
  box-shadow: 0 10px 30px -4px rgba(0, 0, 0, 0.18), 0 1px 6px rgba(0, 0, 0, 0.08);
  padding: 0.875rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.news-root.is-dark .news-popover {
  background: #1e293b;
  box-shadow: 0 10px 30px -4px rgba(0, 0, 0, 0.5), 0 1px 6px rgba(0, 0, 0, 0.24);
}

.news-popover-title {
  margin: 0 0 0.25rem 0;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: #94a3b8;
  flex-shrink: 0;
}

.news-root.is-dark .news-popover-title {
  color: #64748b;
}

.news-scroll {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
  overflow-y: auto;
  min-height: 0;
}

.news-section {
  display: flex;
  flex-direction: column;
}

.news-section-head {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  padding-bottom: 0.375rem;
  margin-bottom: 0.125rem;
  border-bottom: 1px solid #e2e8f0;
}

.news-root.is-dark .news-section-head {
  border-bottom-color: #334155;
}

.news-section-icon {
  width: 0.875rem;
  height: 0.875rem;
  flex-shrink: 0;
  color: #64748b;
  font-size: 0.75rem;
  line-height: 1;
}

.news-section-icon-bell {
  color: #eab308;
}

.news-section-title {
  flex: 1;
  font-size: 0.75rem;
  font-weight: 700;
  color: #0f172a;
}

.news-root.is-dark .news-section-title {
  color: #f1f5f9;
}

.news-source-tag {
  flex-shrink: 0;
  padding: 0.05rem 0.4rem;
  border-radius: 9999px;
  background: #f1f5f9;
  color: #64748b;
  font-size: 0.6rem;
  font-weight: 700;
  letter-spacing: 0.03em;
}

.news-source-ours {
  background: #fef9c3;
  color: #854d0e;
}

.news-root.is-dark .news-source-tag {
  background: #0f172a;
  color: #94a3b8;
}

.news-root.is-dark .news-source-ours {
  background: rgb(234 179 8 / 0.15);
  color: #facc15;
}

.news-hint {
  margin: 0;
  padding: 0.5rem 0.25rem;
  font-size: 0.72rem;
  line-height: 1.4;
  color: #64748b;
}

.news-root.is-dark .news-hint {
  color: #94a3b8;
}

.news-state {
  font-size: 0.75rem;
  color: #94a3b8;
  padding: 0.5rem 0;
  text-align: center;
}

.news-state-error {
  color: #ef4444;
}

.news-root.is-dark .news-state {
  color: #64748b;
}

.news-root.is-dark .news-state-error {
  color: #f87171;
}

.news-list {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.news-item {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  padding: 0.4rem 0.25rem;
  border-radius: 0.375rem;
  text-decoration: none;
  border-bottom: 1px solid #f1f5f9;
  transition: background 100ms ease;
}

.news-item:last-child {
  border-bottom: none;
}

.news-item:hover {
  background: #f8fafc;
}

.news-root.is-dark .news-item {
  border-bottom-color: #1e293b;
}

.news-root.is-dark .news-item:hover {
  background: #334155;
}

.news-change {
  flex-direction: row;
  align-items: flex-start;
  gap: 0.5rem;
  width: 100%;
  border-top: 0;
  border-left: 0;
  border-right: 0;
  background: transparent;
  text-align: left;
  font: inherit;
  cursor: pointer;
}

.news-change:disabled {
  cursor: default;
  opacity: 0.6;
}

.news-change.is-new {
  background: #fefce8;
}

.news-root.is-dark .news-change.is-new {
  background: rgb(234 179 8 / 0.08);
}

.news-change-badge {
  flex-shrink: 0;
  min-width: 2rem;
  height: 1.5rem;
  padding: 0 0.375rem;
  margin-top: 0.1rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  color: #ffffff;
  font-size: 0.75rem;
  font-weight: 800;
}

.news-change-body {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}

.news-change-title {
  font-size: 0.75rem;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.3;
}

.news-root.is-dark .news-change-title {
  color: #f1f5f9;
}

.news-change-kinds {
  font-size: 0.7rem;
  font-weight: 500;
  color: #854d0e;
  line-height: 1.35;
}

.news-root.is-dark .news-change-kinds {
  color: #facc15;
}

.news-new-dot {
  display: inline-block;
  width: 0.4rem;
  height: 0.4rem;
  margin-left: 0.25rem;
  border-radius: 9999px;
  background: #eab308;
  vertical-align: middle;
}

.news-date {
  font-size: 0.65rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: #94a3b8;
  white-space: nowrap;
}

.news-root.is-dark .news-date {
  color: #475569;
}

.news-title {
  font-size: 0.75rem;
  font-weight: 500;
  color: #1d4ed8;
  line-height: 1.35;
  word-break: break-word;
  overflow-wrap: anywhere;
  text-transform: capitalize;
}

.news-root.is-dark .news-title {
  color: #93c5fd;
}

.news-external {
  display: inline-block;
  width: 0.65rem;
  height: 0.65rem;
  margin-left: 0.15rem;
  vertical-align: baseline;
  opacity: 0.7;
}

@keyframes news-blink-ring {
  0%, 100% { border-color: transparent; }
  50% { border-color: rgba(234, 179, 8, 0.85); }
}

.news-btn.has-updates::after {
  animation: news-blink-ring 1s ease-in-out 30;
}

@media (prefers-reduced-motion: reduce) {
  .news-btn.has-updates::after {
    animation: none;
  }
}
</style>
