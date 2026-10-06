<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useRouteStore} from '@/stores/route.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {OUTGOING_SUFFIX, type Route} from '@/types/tranzy.ts'

const {t} = useI18n()
const router = useRouter()
const routeStore = useRouteStore()
const {routes, error, fetchRoutes} = useRoutesApi()
const {fetchShapeInfo} = useRouteShapeInfoApi()

const sortedRoutes = computed(() => [...routes.value].sort((a, b) =>
  a.route_short_name.localeCompare(b.route_short_name, undefined, {numeric: true})))

const openingRouteId = ref<number | null>(null)

async function openRoute(route: Route) {
  if (openingRouteId.value !== null) return
  openingRouteId.value = route.route_id
  try {
    routeStore.setSelectedRoute(await fetchShapeInfo(route), `${route.route_id}${OUTGOING_SUFFIX}`, '', '')
    await router.push({name: 'route', params: {routeId: String(route.route_id), direction: '0'}})
  } catch (e) {
    console.error('Failed to load route:', e)
  } finally {
    openingRouteId.value = null
  }
}

onMounted(fetchRoutes)
</script>

<template>
  <div class="wt-list">
    <header class="wt-head">
      <h1 class="wt-heading">{{ t('allRoutes') }}</h1>
    </header>

    <p v-if="!sortedRoutes.length && error" class="wt-note">{{ t('watchLoadFailed') }}</p>
    <div class="wt-grid">
      <button
        v-for="route in sortedRoutes"
        :key="route.route_id"
        type="button"
        class="wt-tile wt-fish"
        :class="{ 'is-busy': openingRouteId === route.route_id }"
        :style="{ backgroundColor: route.route_color }"
        @click="openRoute(route)"
      >{{ route.route_short_name }}</button>
    </div>
  </div>
</template>
