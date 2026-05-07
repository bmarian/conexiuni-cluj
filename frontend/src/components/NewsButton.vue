<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'

interface NewsItem {
  url: string
  date: string
  title: string
}

const props = withDefaults(defineProps<{ topOffset?: string }>(), {topOffset: '3.5rem'})

const {t} = useI18n()
const settings = useSettingsStore()
const isDark = computed(() => settings.isDark)

const isOpen = ref(false)
const rootRef = ref<HTMLElement | null>(null)

const newsItems = ref<NewsItem[]>([])
const loading = ref(false)
const error = ref(false)

let timer: ReturnType<typeof setTimeout> | null = null

async function fetchNews() {
  loading.value = true
  error.value = false
  try {
    const res = await fetch('/api/news')
    if (!res.ok) throw new Error()
    newsItems.value = await res.json()
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
  timer = setTimeout(fetchNews, 8 * 60 * 60 * 1000)
}

function toggle() {
  isOpen.value = !isOpen.value
  if (isOpen.value && newsItems.value.length === 0 && !loading.value) {
    fetchNews()
  }
}

function onDocumentPointerDown(e: PointerEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocumentPointerDown)
  fetchNews()
})

onUnmounted(() => {
  document.removeEventListener('pointerdown', onDocumentPointerDown)
  if (timer !== null) clearTimeout(timer)
})

const topValue = computed(() => props.topOffset)
</script>

<template>
  <div ref="rootRef" class="news-root" :class="{ 'is-dark': isDark }"
       :style="isOpen ? { zIndex: 9999 } : {}">
    <button
      type="button"
      class="news-btn"
      :title="t('news')"
      :aria-label="t('news')"
      :aria-expanded="isOpen"
      @click="toggle"
    >
      <span v-if="settings.traditionalActive" class="emoji-icon" aria-hidden="true">📰</span>
      <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
           width="16" height="16" aria-hidden="true">
        <path d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2"/>
        <path d="M18 14h-8M15 18h-5M10 6h8v4h-8z"/>
      </svg>
    </button>

    <div v-if="isOpen" class="news-popover" role="dialog" :aria-label="t('news')">
      <p class="news-popover-title">{{ t('newsPopoverTitle') }}</p>

      <div v-if="loading && newsItems.length === 0" class="news-state">
        <span>{{ t('newsLoading') }}</span>
      </div>
      <div v-else-if="error && newsItems.length === 0" class="news-state news-state-error">
        <span>{{ t('newsError') }}</span>
      </div>
      <div v-else-if="newsItems.length === 0" class="news-state">
        <span>{{ t('newsEmpty') }}</span>
      </div>
      <div v-else class="news-list">
        <a
          v-for="item in newsItems"
          :key="item.url"
          :href="item.url"
          target="_blank"
          rel="noopener noreferrer"
          class="news-item"
        >
          <span class="news-date">{{ item.date }}</span>
          <span class="news-title">{{ item.title }}</span>
        </a>
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
  .news-root.landscape-open {
    right: calc(var(--landscape-drawer-width) + 0.75rem + env(safe-area-inset-right));
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

.news-popover {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  min-width: 15rem;
  max-width: 22rem;
  max-height: 20rem;
  overflow-y: auto;
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
  margin: 0 0 0.5rem 0;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: #94a3b8;
  flex-shrink: 0;
}

.news-root.is-dark .news-popover-title {
  color: #64748b;
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
</style>
