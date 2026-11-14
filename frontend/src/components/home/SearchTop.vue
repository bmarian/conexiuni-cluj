<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import type {Stop} from '@/types/tranzy.ts'
import {INCOMING_SUFFIX} from '@/types/tranzy.ts'
import {formatMeters} from '@/utils/geo.ts'
import {network} from '@/utils/network.ts'
import {placeDetail} from '@/utils/places.ts'
import type {SearchAnswer, SearchBus, SearchOtherLine, SearchPlace} from '@/utils/search.ts'
import {directedDestination} from '@/utils/time.ts'
import {DirectionsBusIcon, DirectionsIcon, InfoIcon, LocationCityIcon, OpenInNewIcon, PlaceIcon, ScheduleIcon, SwapHorizIcon} from '@/utils/icons.ts'
import ActionChips from '@/components/ui/ActionChips.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import ListRow from '@/components/ui/ListRow.vue'
import RowIcon from '@/components/ui/RowIcon.vue'
import RouteRow from '@/components/rows/RouteRow.vue'
import StationRow from '@/components/rows/StationRow.vue'

const props = defineProps<{answer: SearchAnswer; distanceTo: (lat: number, lon: number) => string | null}>()
const emit = defineEmits<{
  line: [routeId: number, timetable: boolean]
  stop: [stop: Stop]
  place: [place: SearchPlace]
  bus: [bus: SearchBus]
  other: [line: SearchOtherLine]
  city: [slug: string]
  trip: []
  link: [url: string]
}>()

const {t} = useI18n()

const best = computed(() => props.answer.best)
const line = computed(() => (best.value === 'line' ? network.value?.routeById.get(props.answer.lines[0]!) : undefined))
const stop = computed(() => (best.value === 'stop' ? network.value?.stopById.get(props.answer.stops[0]!.id) : undefined))
const place = computed(() => props.answer.places[0])
const bus = computed(() => (best.value === 'bus' ? props.answer.buses[0] : undefined))
const busLine = computed(() => (bus.value ? network.value?.routeById.get(bus.value.route_id) : undefined))
const other = computed(() => (best.value === 'other' ? props.answer.other[0] : undefined))
const city = computed(() => (best.value === 'city' ? props.answer.cities[0] : undefined))
const trip = computed(() => (best.value === 'trip' ? props.answer.trip : undefined))
const far = computed(() => (best.value === 'far' ? props.answer.far : undefined))
const farLine = computed(() => (far.value?.route_id ? network.value?.routeById.get(far.value.route_id) : undefined))
const farStop = computed(() => (far.value ? network.value?.stopById.get(far.value.stop_id) : undefined))

const lineActions = computed(() => [
  {
    label: t('timetable'),
    icon: ScheduleIcon,
    selected: props.answer.show === 'timetable',
    kbd: 'best-timetable',
    onClick: () => line.value && emit('line', line.value.route_id, true),
  },
])

const otherActions = computed(() =>
  other.value
    ? [
        {
          label: t('searchSwitchCity', {city: other.value.city_name}),
          icon: SwapHorizIcon,
          selected: true,
          kbd: 'best-city',
          onClick: () => emit('other', other.value!),
        },
      ]
    : [],
)

const busToward = computed(() =>
  bus.value && busLine.value ? directedDestination(busLine.value.route_long_name, bus.value.trip_id.endsWith(INCOMING_SUFFIX) ? '1' : '0') : '',
)

const tripTitle = computed(() => (trip.value ? `${trip.value.from?.name ?? t('planOriginCurrentLocation')} → ${trip.value.to.name}` : ''))

const host = (url: string) => {
  try {
    return new URL(url).host.replace(/^www\./, '')
  } catch {
    return url
  }
}
</script>

<template>
  <section v-if="best" class="search-top" data-kbd-section="search-top">
    <template v-if="line">
      <RouteRow
        :name="line.route_short_name"
        :color="line.route_color"
        :title="line.route_long_name"
        data-kbd-item="best-line"
        @open="emit('line', line.route_id, answer.show === 'timetable')"
      />
      <ActionChips :actions="lineActions" class="chips" />
    </template>
    <StationRow v-else-if="stop" :stop="stop" :distance="distanceTo(stop.stop_lat, stop.stop_lon)" data-kbd-item="best-stop" @open="emit('stop', stop)" />
    <ListRow
      v-else-if="best === 'place' && place"
      :title="place.name"
      :subtitle="placeDetail(place) || undefined"
      :trailing="distanceTo(parseFloat(place.lat), parseFloat(place.lon)) ?? undefined"
      chevron
      data-kbd-item="best-place"
      @open="emit('place', place)"
    >
      <template #icon><RowIcon :icon="PlaceIcon" tone="tertiary" /></template>
    </ListRow>
    <ListRow v-else-if="bus" :title="t('searchBus', {label: bus.label})" chevron data-kbd-item="best-bus" @open="emit('bus', bus)">
      <template #icon><RowIcon :icon="DirectionsBusIcon" tone="primary" /></template>
      <template v-if="busLine" #lines>
        <span class="flex items-center gap-1.5 text-body-small text-on-surface-variant">
          <LineBadge :name="busLine.route_short_name" :color="busLine.route_color" :height="18" :min-width="28" :corner="4" />
          <span class="truncate">→ {{ busToward }}</span>
        </span>
      </template>
    </ListRow>
    <template v-else-if="other">
      <ListRow :title="other.long_name" :subtitle="other.city_name" chevron data-kbd-item="best-other" @open="emit('other', other)">
        <template #icon><LineBadge :name="other.name" :color="other.color" plain /></template>
      </ListRow>
      <ActionChips :actions="otherActions" class="chips" />
    </template>
    <ListRow v-else-if="city" :title="city.name" :subtitle="t('searchChangeCity')" chevron data-kbd-item="best-city" @open="emit('city', city.slug)">
      <template #icon><RowIcon :icon="LocationCityIcon" /></template>
    </ListRow>
    <ListRow v-else-if="trip" :title="tripTitle" :subtitle="t('planTitle')" chevron data-kbd-item="best-trip" @open="emit('trip')">
      <template #icon><RowIcon :icon="DirectionsIcon" tone="tertiary" /></template>
    </ListRow>
    <template v-else-if="far">
      <ListRow
        :title="t(far.town ? 'searchFarTown' : 'searchFarPlace', {name: far.name})"
        :subtitle="farLine ? t('searchFarClosest', {line: farLine.route_short_name, distance: formatMeters(far.meters)}) : undefined"
      >
        <template #icon><RowIcon :icon="InfoIcon" /></template>
      </ListRow>
      <ListRow
        v-if="farLine && farStop && place"
        :title="`→ ${farStop.stop_name}`"
        :subtitle="t('searchFarThen', {distance: formatMeters(far.meters), name: far.name})"
        chevron
        data-kbd-item="best-far"
        @open="emit('place', place)"
      >
        <template #icon><LineBadge :name="farLine.route_short_name" :color="farLine.route_color" /></template>
      </ListRow>
    </template>
  </section>
  <section v-else-if="answer.notice" data-kbd-section="search-top">
    <ListRow v-if="answer.notice.kind === 'city_missing'" :title="t('searchCityMissing', {name: answer.notice.name})">
      <template #icon><RowIcon :icon="InfoIcon" /></template>
    </ListRow>
    <ListRow v-else-if="answer.notice.kind === 'bus_off'" :title="t('searchBusOff', {label: answer.notice.name})">
      <template #icon><RowIcon :icon="InfoIcon" /></template>
    </ListRow>
    <template v-else-if="answer.notice.kind === 'tickets'">
      <ListRow :title="t('searchTickets')">
        <template #icon><RowIcon :icon="InfoIcon" /></template>
      </ListRow>
      <ListRow
        v-if="answer.notice.url"
        :title="t('searchOperatorSite', {operator: answer.notice.operator})"
        :subtitle="host(answer.notice.url)"
        chevron
        data-kbd-item="operator-site"
        @open="emit('link', answer.notice.url)"
      >
        <template #icon><RowIcon :icon="OpenInNewIcon" tone="primary" /></template>
      </ListRow>
    </template>
  </section>
</template>

<style scoped>
.search-top {
  margin: 8px 12px 4px;
  padding: 2px 0;
  border-radius: 20px;
  background: var(--md-surface-container);
}

.chips {
  padding: 0 16px 8px 68px;
}
</style>
