<script setup lang="ts">
import {computed, onMounted} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useRouteUpdatesStore} from '@/stores/routeUpdates.ts'
import {useOnline} from '@/composables/useOnline.ts'
import {useSearch} from '@/composables/useSearch.ts'
import {useFavoriteDrag} from '@/composables/useFavoriteDrag.ts'
import {cityPath} from '@/utils/city.ts'
import {DeleteIcon, NewspaperIcon, SearchIcon, SettingsIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const router = useRouter()
const routeUpdates = useRouteUpdatesStore()
const {isOnline} = useOnline()
const search = useSearch()
const {holding, overZone, zone} = useFavoriteDrag()

const showNews = computed(() => isOnline.value && routeUpdates.hasFeed)

onMounted(() => void routeUpdates.fetchUpdates())
</script>

<template>
  <div class="home-chrome">
    <div v-if="holding" ref="zone" class="search-pill remove-zone" :class="{'is-over': overZone}" aria-hidden="true">
      <DeleteIcon class="m-icon" />
      <span class="text-title-small">{{ t('removeFromFavorites') }}</span>
    </div>
    <div v-else class="search-pill" role="button" tabindex="-1" @click="search.open.value = true">
      <SearchIcon class="m-icon text-on-surface-variant" />
      <span class="search-text text-body-large" :class="{'is-kept': search.query.value}">
        {{ search.query.value || t('searchPlaceholder') }}
      </span>
      <button
        v-if="showNews"
        type="button"
        class="icon-btn"
        :aria-label="t('news')"
        :title="t('news')"
        @click.stop="router.push(cityPath('/anunturi'))"
      >
        <span class="relative inline-flex">
          <NewspaperIcon class="m-icon" />
          <span v-if="routeUpdates.hasUnseen" class="news-dot"></span>
        </span>
      </button>
      <button type="button" class="icon-btn" :aria-label="t('settings')" :title="t('settings')" @click.stop="router.push(cityPath('/setari'))">
        <SettingsIcon class="m-icon" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.home-chrome {
  padding-block: calc(8px + var(--safe-top)) 8px;
  pointer-events: none;
}

.search-pill {
  margin: 0 16px;
  box-shadow: var(--md-elevation-2);
  pointer-events: auto;
}

.remove-zone {
  justify-content: center;
  gap: 8px;
  padding: 0;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface);
  cursor: default;
  transition:
    scale var(--spring-press-ms) var(--spring-press),
    background-color 120ms linear,
    color 120ms linear;
}

.remove-zone.is-over {
  scale: 1.04;
  background: var(--md-error-container);
  color: var(--md-on-error-container);
}

.search-text {
  flex: 1;
  min-width: 0;
  padding-left: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--md-on-surface-variant);
}

.search-text.is-kept {
  color: var(--md-on-surface);
}

.news-dot {
  position: absolute;
  top: -1px;
  right: -1px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #eab308;
  box-shadow: 0 0 0 1.5px var(--md-surface-container-high);
}
</style>
