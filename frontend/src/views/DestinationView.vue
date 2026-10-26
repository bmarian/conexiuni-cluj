<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {network, type NetworkRoute} from '@/utils/network.ts'
import {type Destination, type DestinationSummary, fetchDestination, fetchDestinations, namedLike, type Ride, rideMinutes} from '@/utils/destinationPages.ts'
import {ApiError} from '@/utils/api.ts'
import {cityParams, cityPath} from '@/utils/city.ts'
import {canonicalUrl} from '@/utils/site.ts'
import {areaPath, linePath, stopPath} from '@/utils/pagePaths.ts'
import {servedName} from '@/utils/servedName.ts'
import {useMapStore} from '@/stores/map.ts'
import {useMapPicks} from '@/composables/useMapPicks.ts'
import {DirectionsBusIcon, DirectionsIcon, MapIcon, PlaceIcon} from '@/utils/icons.ts'
import type {Stop} from '@/types/tranzy.ts'
import type {ChipAction} from '@/components/ui/types.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ActionChips from '@/components/ui/ActionChips.vue'
import SectionLabel from '@/components/ui/SectionLabel.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import ListRow from '@/components/ui/ListRow.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import StationRow from '@/components/rows/StationRow.vue'
import StationLines from '@/components/rows/StationLines.vue'
import LoadingIndicator from '@/components/LoadingIndicator.vue'

const RIDES_SHOWN = 3

const props = defineProps<{slug: string}>()

const {t} = useI18n()
const router = useRouter()
const currentRoute = useRoute()
const servedAs = computed(() => servedName(currentRoute.path))
const mapStore = useMapStore()
const {openStop} = useMapPicks()

const destination = ref<Destination | null>(null)
const loadError = ref(false)
const notFound = ref(false)
const areas = ref<DestinationSummary[]>([])
const opened = ref(new Set<string>())

async function load() {
  loadError.value = false
  try {
    destination.value = await fetchDestination(props.slug)
    mapStore.setFlyToLocation(destination.value.lat, destination.value.lon)
  } catch (err) {
    destination.value = null
    if (!(err instanceof ApiError && err.status === 404)) {
      loadError.value = true
      return
    }
    notFound.value = true
    areas.value = await fetchDestinations().catch(() => [])
  }
}

watch(
  () => props.slug,
  () => {
    destination.value = null
    notFound.value = false
    opened.value = new Set()
    void load()
  },
  {immediate: true},
)

const asked = computed(() => props.slug.replace(/-/g, ' '))
const similar = computed(() => (network.value ? namedLike(props.slug, areas.value, network.value) : {areas: [], stops: []}))

const area = computed(() => destination.value?.name ?? '')

const lines = computed(() => {
  const index = network.value
  if (!index || !destination.value) return []
  return destination.value.route_ids.map((id) => index.routeById.get(id)).filter((r): r is NetworkRoute => !!r)
})

const stops = computed(() => {
  const index = network.value
  if (!index || !destination.value) return []
  return destination.value.stop_ids.map((id) => index.stopById.get(id)).filter((s): s is Stop => !!s)
})

const origins = computed(() => {
  const index = network.value
  if (!index || !destination.value) return []
  return destination.value.from.map((origin) => ({
    ...origin,
    rides: origin.rides.flatMap((ride) => {
      const route = index.routeById.get(ride.route_id)
      if (!route) return []
      const detail = [
        ride.every_min ? t('destEvery', {minutes: ride.every_min}) : '',
        `${ride.first}–${ride.last}`,
        ride.day_type === 'weekday' ? '' : t('destNoWeekdays'),
      ]
        .filter(Boolean)
        .join(' · ')
      const board = index.stopById.get(ride.board)?.stop_name ?? ''
      const alight = index.stopById.get(ride.alight)?.stop_name ?? ''
      return [{ride, route, title: `${board} → ${alight}`, detail}]
    }),
  }))
})

function openRoute(ride: Ride) {
  void router.push(rideHref(ride))
}

const rideHref = (ride: Ride) => linePath(ride.route_id, ride.direction === 1 ? '1' : '0')
const stopHref = (stop: Stop) => stopPath(stop.stop_id)

function showAll(slug: string) {
  opened.value = new Set([...opened.value, slug])
}

const chips = computed<ChipAction[]>(() => {
  const dest = destination.value
  if (!dest) return []
  return [
    {
      label: t('directions'),
      icon: DirectionsIcon,
      primary: true,
      kbd: 'directions',
      onClick: () => void router.push({name: 'plan', query: {lat: String(dest.lat), lon: String(dest.lon), name: dest.name}}),
    },
  ]
})

useHead(() => {
  if (notFound.value) return {title: t('destNotFound', {name: asked.value}), meta: [{name: 'robots', content: 'noindex'}]}
  // Keep the server-injected meta for crawlers until the destination loads.
  if (!destination.value) return {}
  const params = {...cityParams(), area: area.value}
  const title = t('headDestTitle', params)
  const description = t('headDestDesc', params)
  const url = canonicalUrl(areaPath(props.slug))
  return {
    title,
    meta: [
      {name: 'description', content: description},
      {property: 'og:title', content: title},
      {property: 'og:description', content: description},
      {property: 'og:url', content: url},
      {name: 'twitter:title', content: title},
      {name: 'twitter:description', content: description},
    ],
    link: [{rel: 'canonical', href: url}],
  }
})
</script>

<template>
  <div class="destination-view">
    <SheetTopBar :title="area || (notFound ? asked : '')" />

    <div v-if="loadError" class="flex flex-col items-start gap-3 p-4">
      <p class="text-body-medium text-on-surface-variant">{{ t('loadFailed') }}</p>
      <button type="button" class="btn btn-tonal" @click="load">{{ t('retry') }}</button>
    </div>

    <template v-else-if="notFound">
      <header class="header">
        <h1 class="text-title-large font-bold">{{ t('destNotFound', {name: asked}) }}</h1>
      </header>
      <section v-if="similar.areas.length" class="pt-2" data-kbd-section="dest-like-areas" data-kbd-entry="1">
        <SectionLabel :text="t('destinations')" :icon="PlaceIcon" />
        <ListRow
          v-for="like in similar.areas"
          :key="like.slug"
          :title="like.name"
          :subtitle="t('destCounts', {stops: like.stops, lines: like.lines})"
          :data-kbd-item="`dest-${like.slug}`"
          :href="areaPath(like.slug)"
          chevron
          @open="router.push({name: 'destination', params: {slug: like.slug}})"
        />
      </section>
      <section v-if="similar.stops.length" class="pt-2" data-kbd-section="dest-like-stops" data-kbd-entry="2">
        <SectionLabel :text="t('stops')" :icon="DirectionsBusIcon" />
        <StationRow
          v-for="stop in similar.stops"
          :key="stop.stop_id"
          :stop="stop"
          :data-kbd-item="`stop-${stop.stop_id}`"
          :href="stopHref(stop)"
          @open="openStop(String(stop.stop_id))"
        />
      </section>
    </template>

    <LoadingIndicator v-else-if="!destination || !network" :text="servedAs ? t('destQuestion', {area: servedAs}) : undefined" />

    <template v-else>
      <header class="header">
        <p class="text-label-medium text-primary">{{ t(destination.district ? 'destKind_district' : 'destKind_town') }}</p>
        <h1 class="text-title-large font-bold">{{ t('destQuestion', {area}) }}</h1>
      </header>
      <ActionChips :actions="chips" class="pb-2" />

      <section v-if="lines.length" class="pb-2">
        <SectionLabel :text="t('destLinesIn', {area})" :icon="MapIcon" />
        <StationLines :routes="lines" class="px-4" />
      </section>

      <section class="pt-2" data-kbd-section="dest-from" data-kbd-entry="1">
        <SectionLabel :text="t('destFrom')" :icon="DirectionsBusIcon" />
        <p v-if="!origins.length" class="text-body-medium text-on-surface-variant px-4 py-2">{{ t('destNoRides') }}</p>
        <template v-for="origin in origins" :key="origin.slug">
          <SubLabel :text="t('destFromArea', {area: origin.name})" class="pt-2" />
          <ListRow
            v-for="(row, i) in origin.rides"
            v-show="i < RIDES_SHOWN || opened.has(origin.slug)"
            :key="row.ride.route_id"
            :title="row.title"
            :subtitle="row.detail"
            :trailing="rideMinutes(row.ride.minutes)"
            :data-kbd-item="`ride-${origin.slug}-${row.ride.route_id}`"
            :href="rideHref(row.ride)"
            chevron
            @open="openRoute(row.ride)"
          >
            <template #icon>
              <LineBadge :name="row.route.route_short_name" :color="row.route.route_color" :height="28" :min-width="36" :corner="6" />
            </template>
          </ListRow>
          <button v-if="origin.rides.length > RIDES_SHOWN && !opened.has(origin.slug)" type="button" class="btn btn-text ml-2" @click="showAll(origin.slug)">
            {{ t('destMoreRides', {count: origin.rides.length - RIDES_SHOWN}) }}
          </button>
        </template>
      </section>

      <section v-if="stops.length" class="pt-4" data-kbd-section="dest-stops" data-kbd-entry="2">
        <SectionLabel :text="t('destStopsIn', {area})" :icon="PlaceIcon" />
        <StationRow
          v-for="stop in stops"
          :key="stop.stop_id"
          :stop="stop"
          :data-kbd-item="`stop-${stop.stop_id}`"
          :href="stopHref(stop)"
          @open="openStop(String(stop.stop_id))"
        />
      </section>
    </template>

    <template v-if="notFound || (destination && network)">
      <ListRow
        class="mt-2"
        :title="t('destAll', cityParams())"
        :href="cityPath('/destinatii')"
        chevron
        data-kbd-item="all-destinations"
        @open="router.push({name: 'destinations'})"
      />
      <div class="h-6"></div>
    </template>
  </div>
</template>

<style scoped>
.header {
  padding: 4px 16px 8px;
}
</style>
