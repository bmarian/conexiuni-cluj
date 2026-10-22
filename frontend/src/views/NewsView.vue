<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useHead} from '@unhead/vue'
import {type RouteChange, useRouteUpdatesStore} from '@/stores/routeUpdates'
import {changeDirection, changeKindLabels, changeTitle} from '@/utils/routeChanges'
import {buildNewsFeed, newsSource} from '@/utils/newsFeed'
import {cityParams} from '@/utils/city.ts'
import {NewspaperIcon, OpenInNewIcon} from '@/utils/icons.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import LineBadge from '@/components/ui/LineBadge.vue'

const {t, locale} = useI18n()
const router = useRouter()
const routeUpdates = useRouteUpdatesStore()

useHead(() => ({title: `${t('news')} · ${t('headHomeTitle', cityParams())}`}))

// Stays bold for the visit, so the "new" marks don't vanish on first render.
const freshIds = ref(new Set<number>())

watch(() => routeUpdates.changes, () => {
  if (!routeUpdates.hasUnseen) return
  freshIds.value = new Set([...freshIds.value, ...routeUpdates.unseenIds])
  routeUpdates.markAllSeen()
}, {immediate: true})

const feed = computed(() => buildNewsFeed(routeUpdates.updates, new Date(), locale.value, t).map((entry) =>
  entry.kind === 'change'
    ? {
      ...entry,
      title: changeTitle(entry.change, t),
      meta: [entry.date, ...changeKindLabels(entry.change, t)].join(' · '),
      isNew: freshIds.value.has(entry.change.id),
    }
    : {...entry, title: entry.news.title, meta: `${entry.date} · ${newsSource(entry.news.url)}`, isNew: false},
))

function openChange(change: RouteChange) {
  if (!change.route_id) return
  void router.push({
    name: 'route',
    params: {routeId: String(change.route_id), direction: changeDirection(change)},
    query: {change: String(change.id)},
  })
}

function onVisibilityChange() {
  if (document.visibilityState === 'visible') routeUpdates.refreshIfStale()
}

onMounted(() => {
  void routeUpdates.fetchUpdates()
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onUnmounted(() => document.removeEventListener('visibilitychange', onVisibilityChange))
</script>

<template>
  <div class="news-view">
    <SheetTopBar :title="t('news')"/>
    <div v-if="feed.length" data-kbd-section="news" data-kbd-entry="1">
      <template v-for="(entry, i) in feed" :key="entry.key">
        <button
          v-if="entry.kind === 'change'"
          type="button"
          class="feed-row state squish"
          :class="{ cascade: i < 12 }"
          :style="{ '--i': i }"
          :disabled="!entry.change.route_id"
          :data-kbd-item="entry.key"
          @click="openChange(entry.change)"
        >
          <span class="lead">
            <LineBadge :name="entry.change.route_short_name" :color="entry.change.route_color || '#64748b'" :height="28" :min-width="36" :corner="6"/>
          </span>
          <span class="body">
            <span class="title-line">
              <span class="text-body-large" :class="{ 'font-bold': entry.isNew }">{{ entry.title }}</span>
              <span v-if="entry.isNew" class="new-dot"><span class="sr-only">{{ t('newsNew') }}</span></span>
            </span>
            <span class="text-body-small text-on-surface-variant">{{ entry.meta }}</span>
          </span>
        </button>
        <a
          v-else
          :href="entry.news.url"
          target="_blank"
          rel="noopener noreferrer"
          class="feed-row state squish"
          :class="{ cascade: i < 12 }"
          :style="{ '--i': i }"
          :data-kbd-item="entry.key"
        >
          <span class="lead"><NewspaperIcon class="m-icon size-[22px] text-on-surface-variant"/></span>
          <span class="body">
            <span class="text-body-large">{{ entry.title }}</span>
            <span class="flex items-center gap-1 text-body-small text-on-surface-variant">
              {{ entry.meta }}
              <OpenInNewIcon class="m-icon size-3"/>
            </span>
          </span>
        </a>
      </template>
    </div>
    <p v-else-if="routeUpdates.loading" class="hint">{{ t('newsLoading') }}</p>
    <p v-else-if="routeUpdates.error" class="hint text-error">{{ t('newsError') }}</p>
    <p v-else class="hint">{{ t('newsEmpty') }}</p>
    <div class="h-6"></div>
  </div>
</template>

<style scoped>
.feed-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
  padding: 10px 16px;
  border: 0;
  background: transparent;
  color: var(--md-on-surface);
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}

.feed-row:disabled {
  cursor: default;
}

.lead {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  min-width: 40px;
  min-height: 28px;
}

.body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.title-line {
  display: flex;
  align-items: center;
  gap: 6px;
}

.new-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #eab308;
}

.hint {
  padding: 8px 16px;
  font-size: 12px;
  line-height: 16px;
  color: var(--md-on-surface-variant);
}

.hint.text-error {
  color: var(--md-error);
}
</style>
