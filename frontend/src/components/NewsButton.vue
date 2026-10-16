<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {useSettingsStore} from '@/stores/settings'
import {type RouteChange, useRouteUpdatesStore} from '@/stores/routeUpdates'
import {apiRequest} from '@/utils/api'
import {changeDirection, changeKindLabels, changeTitle} from '@/utils/routeChanges'
import {buildNewsFeed, newsSource, type NewsItem} from '@/utils/newsFeed'
import {useKbdLayer, useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'

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

const hasUpdates = computed(() => routeUpdates.hasUnseen)

watch([isOpen, () => routeUpdates.changes], ([open], [wasOpen]) => {
  if (!open) return
  if (!wasOpen) freshIds.value = new Set()
  if (!hasUpdates.value) return
  freshIds.value = new Set([...freshIds.value, ...routeUpdates.unseenIds])
  routeUpdates.markAllSeen()
})

const feed = computed(() => buildNewsFeed(routeUpdates.changes, newsItems.value, new Date(), locale.value, t).map((entry) =>
  entry.kind === 'change'
    ? {
      ...entry,
      title: changeTitle(entry.change, t),
      meta: [entry.date, ...changeKindLabels(entry.change, t)].join(' · '),
      isNew: freshIds.value.has(entry.change.id),
    }
    : {...entry, title: entry.item.title, meta: `${entry.date} · ${newsSource(entry.item.url)}`, isNew: false},
))

const feedLoading = computed(() => loading.value || routeUpdates.loading)
const feedError = computed(() => error.value || routeUpdates.error)

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
  void routeUpdates.fetchChanges()
  document.addEventListener('pointerdown', onDocumentPointerDown)
  document.addEventListener('visibilitychange', onVisibilityChange)
  void fetchNews()
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
      :class="{'has-updates': hasUpdates}"
      :title="hasUpdates ? t('newsHasUpdates') : t('news')"
      :aria-label="hasUpdates ? t('newsHasUpdates') : t('news')"
      :aria-expanded="isOpen"
      @click="toggle"
    >
      <span v-if="settings.legacyBlueActive" class="emoji-icon" aria-hidden="true">📰</span>
      <span v-else-if="settings.paperActive" class="pp-slip-label">{{ t('paperNews') }}</span>
      <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
           width="16" height="16" aria-hidden="true">
        <path d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2"/>
        <path d="M18 14h-8M15 18h-5M10 6h8v4h-8z"/>
      </svg>
      <span v-if="hasUpdates" class="news-dot" aria-hidden="true"></span>
    </button>

    <div v-if="isOpen" ref="popoverRef" class="news-popover" role="dialog" :aria-label="t('news')">
      <p class="news-popover-title">{{ t('news') }}</p>

      <div class="news-scroll">
        <div v-if="feed.length" class="news-list" data-kbd-section="news">
          <template v-for="entry in feed" :key="entry.key">
            <button
              v-if="entry.kind === 'change'"
              type="button"
              class="news-item"
              :class="{ 'is-new': entry.isNew }"
              :disabled="!entry.change.route_id"
              :data-kbd-item="entry.key"
              @click="openChange(entry.change)"
            >
              <span class="news-lead">
                <span class="news-change-badge" :style="{ backgroundColor: entry.change.route_color || '#64748b' }">
                  {{ entry.change.route_short_name }}
                </span>
              </span>
              <span class="news-body">
                <span class="news-title">
                  {{ entry.title }}
                  <span v-if="entry.isNew" class="news-unread"><span class="sr-only">{{ t('newsNew') }}</span></span>
                </span>
                <span class="news-meta">{{ entry.meta }}</span>
              </span>
            </button>
            <a
              v-else
              :href="entry.item.url"
              target="_blank"
              rel="noopener noreferrer"
              class="news-item"
              :data-kbd-item="entry.key"
            >
              <span class="news-lead" aria-hidden="true">
                <span v-if="settings.legacyBlueActive" class="emoji-icon">📰</span>
                <span v-else-if="settings.paperActive" class="news-lead-word">CTP</span>
                <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
                     stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
                     width="16" height="16">
                  <path d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2"/>
                  <path d="M18 14h-8M15 18h-5M10 6h8v4h-8z"/>
                </svg>
              </span>
              <span class="news-body">
                <span class="news-title">{{ entry.title }}</span>
                <span class="news-meta">
                  {{ entry.meta }}
                  <svg class="news-external" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
                       stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
                       aria-hidden="true">
                    <path d="M7 17 17 7M8 7h9v9"/>
                  </svg>
                </span>
              </span>
            </a>
          </template>
        </div>
        <div v-else-if="feedLoading" class="news-state">{{ t('newsLoading') }}</div>
        <div v-else-if="feedError" class="news-state news-state-error">{{ t('newsError') }}</div>
        <div v-else class="news-state">{{ t('newsEmpty') }}</div>

        <p v-if="!routeUpdates.followed.length" class="news-hint">
          {{ t('newsFollowHint') }}
        </p>
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
    --anchor-right: calc(0.75rem + env(safe-area-inset-right) + (var(--controls-row-index, 0) * 2.75rem));
    top: calc(0.75rem + env(safe-area-inset-top));
    right: var(--anchor-right);
  }

  .news-root.landscape-open {
    --anchor-right: calc(var(--landscape-drawer-width) + 0.75rem + env(safe-area-inset-right) + (var(--controls-row-index, 0) * 2.75rem));
  }

  .news-popover {
    right: min(0px, 100vw - var(--anchor-right) - 0.75rem - env(safe-area-inset-left) - var(--popover-width)) !important;
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
  --popover-width: min(20rem, calc(100vw - 1.5rem));
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  width: var(--popover-width);
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
  overflow-y: auto;
  min-height: 0;
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
}

.news-item {
  display: flex;
  align-items: flex-start;
  gap: 0.625rem;
  width: 100%;
  padding: 0.45rem 0.25rem;
  border: 0;
  border-bottom: 1px solid #f1f5f9;
  border-radius: 0.375rem;
  background: transparent;
  text-align: left;
  text-decoration: none;
  font: inherit;
  cursor: pointer;
  transition: background 100ms ease;
}

.news-item:last-child {
  border-bottom: none;
}

.news-item:hover {
  background: #f8fafc;
}

.news-item:disabled {
  cursor: default;
  opacity: 0.6;
}

.news-root.is-dark .news-item {
  border-bottom-color: #1e293b;
}

.news-root.is-dark .news-item:hover {
  background: #334155;
}

.news-lead {
  flex-shrink: 0;
  min-width: 2.25rem;
  height: 1.5rem;
  margin-top: 0.05rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #64748b;
}

.news-root.is-dark .news-lead {
  color: #94a3b8;
}

.news-change-badge {
  min-width: 2rem;
  height: 1.5rem;
  padding: 0 0.375rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  color: #ffffff;
  font-size: 0.75rem;
  font-weight: 800;
}

.news-lead-word {
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.06em;
}

.news-body {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}

.news-title {
  font-size: 0.8rem;
  font-weight: 400;
  line-height: 1.35;
  color: #0f172a;
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  line-clamp: 3;
  overflow: hidden;
}

.news-root.is-dark .news-title {
  color: #f1f5f9;
}

.news-item.is-new .news-title {
  font-weight: 700;
}

.news-unread {
  display: inline-block;
  width: 0.375rem;
  height: 0.375rem;
  margin-left: 0.2rem;
  border-radius: 9999px;
  background: #eab308;
  vertical-align: middle;
}

.news-meta {
  font-size: 0.7rem;
  line-height: 1.35;
  color: #64748b;
}

.news-root.is-dark .news-meta {
  color: #94a3b8;
}

.news-external {
  display: inline-block;
  width: 0.65rem;
  height: 0.65rem;
  margin-left: 0.1rem;
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
