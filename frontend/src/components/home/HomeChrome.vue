<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import AppInstallDialog from "@/components/home/AppInstallDialog.vue";
import { useFavoritesStore } from "@/stores/favorites.ts";
import { useRouteUpdatesStore } from "@/stores/routeUpdates.ts";
import { useSettingsStore } from "@/stores/settings";
import { useOnline } from "@/composables/useOnline.ts";
import { useSearch } from "@/composables/useSearch.ts";
import { offersAndroidApp } from "@/utils/androidApp.ts";
import { cityPath } from "@/utils/city.ts";
import { planQuery } from "@/utils/planQuery.ts";
import { AndroidIcon, NewspaperIcon, PlaceIcon, SearchIcon, SettingsIcon } from "@/utils/icons.ts";

const { t } = useI18n();
const router = useRouter();
const favorites = useFavoritesStore();
const routeUpdates = useRouteUpdatesStore();
const settings = useSettingsStore();
const { isOnline } = useOnline();
const search = useSearch();
const installing = ref(false);

const showApp = computed(() => offersAndroidApp && isOnline.value && !settings.appPromptDismissed);
const showNews = computed(() => isOnline.value && routeUpdates.hasFeed);
const places = computed(() => (isOnline.value ? favorites.favoritePlans : []));

onMounted(() => void routeUpdates.fetchUpdates());
</script>

<template>
  <div class="home-chrome">
    <div class="search-pill" role="button" tabindex="-1" @click="search.open.value = true">
      <SearchIcon class="m-icon text-on-surface-variant" />
      <span class="search-text text-body-large" :class="{ 'is-kept': search.query.value }">
        {{ search.query.value || t("searchPlaceholder") }}
      </span>
      <button
        type="button"
        class="icon-btn"
        :aria-label="t('settings')"
        :title="t('settings')"
        @click.stop="router.push(cityPath('/setari'))"
      >
        <SettingsIcon class="m-icon" />
      </button>
    </div>
    <div v-if="showApp || showNews || places.length" class="chips">
      <!-- TODO: Remove when android app is released -->
      <button
        v-if="showApp && false"
        type="button"
        class="map-chip app-chip squish"
        @click="installing = true"
      >
        <span class="app-bot"><AndroidIcon class="m-icon size-[18px]" /></span>
        <span>{{ t("getApp") }}</span>
      </button>
      <button
        v-if="showNews"
        type="button"
        class="map-chip squish"
        @click="router.push(cityPath('/anunturi'))"
      >
        <span class="relative inline-flex">
          <NewspaperIcon class="m-icon size-[18px] text-on-surface-variant" />
          <span v-if="routeUpdates.hasUnseen" class="news-dot"></span>
        </span>
        <span>{{ t("news") }}</span>
      </button>
      <button
        v-for="place in places"
        :key="`${place.lat}-${place.lon}-${place.originLat ?? ''}`"
        type="button"
        class="map-chip squish"
        @click="router.push({ name: 'plan', query: planQuery(place) })"
      >
        <PlaceIcon class="m-icon size-[18px] text-tertiary" />
        <span class="truncate">{{ place.label ?? place.name }}</span>
      </button>
    </div>
    <AppInstallDialog v-model:open="installing" />
  </div>
</template>

<style scoped>
.home-chrome {
  padding-top: calc(8px + var(--safe-top));
  pointer-events: none;
}

.search-pill,
.chips {
  pointer-events: auto;
}

.search-pill {
  margin: 0 16px;
  box-shadow: var(--md-elevation-2);
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

.chips {
  display: flex;
  gap: 8px;
  width: fit-content;
  max-width: 100%;
  padding: 10px 16px;
  overflow-x: auto;
  scrollbar-width: none;
}

.chips::-webkit-scrollbar {
  display: none;
}

.map-chip {
  --squish: 0.93;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  max-width: 220px;
  height: 36px;
  margin-block: 6px;
  padding: 0 14px 0 12px;
  border: 0;
  border-radius: 18px;
  background: var(--md-surface-container-high);
  color: var(--md-on-surface);
  box-shadow: var(--md-elevation-1);
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  letter-spacing: 0.1px;
  cursor: pointer;
}

.app-chip {
  padding-left: 6px;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
}

.app-bot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--android-ink);
  color: var(--android-green);
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
