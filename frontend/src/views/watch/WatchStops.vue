<script setup lang="ts">
import {computed, onMounted} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import type {Stop} from '@/types/tranzy.ts'

// 800 stops is far too many to scroll with a bezel, so they are picked by initial
// first, and by the first two letters when one initial holds more than this.
const SPLIT_OVER = 40
// Metropolitan stops are named "M-Name", "M - Name" or "M Name" and filed under the name.
const METRO_PREFIX = /^M(\s*-\s*|\s+)(?=\S)/
// CTP writes squares both as "P-ța" and "Piața"; both belong under "Pi".
const SQUARE_PREFIX = /^P-ța\b/

const props = defineProps<{ letter?: string; prefix?: string }>()

const {t} = useI18n()
const router = useRouter()
const {stops, error, fetchStops} = useStopsApi()

const sortKey = (name: string) =>
  name.replace(METRO_PREFIX, '').replace(SQUARE_PREFIX, 'Piața').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toUpperCase()

const initialOf = (key: string) => /^[A-Z]/.test(key) ? key[0]! : '#'

const keyed = computed(() => stops.value
  .map((stop) => ({stop, key: sortKey(stop.stop_name)}))
  .sort((a, b) => a.key.localeCompare(b.key, undefined, {numeric: true})))

type Screen =
  | { tiles: Array<{ label: string; query: Record<string, string> }> }
  | { stops: Stop[] }

const screen = computed((): Screen => {
  if (props.prefix) {
    return {stops: keyed.value.filter((k) => k.key.startsWith(props.prefix!)).map((k) => k.stop)}
  }
  if (props.letter) {
    const inLetter = keyed.value.filter((k) => initialOf(k.key) === props.letter)
    if (inLetter.length <= SPLIT_OVER) return {stops: inLetter.map((k) => k.stop)}
    const prefixes = [...new Set(inLetter.map((k) => k.key.slice(0, 2)))]
    return {
      tiles: prefixes.map((p) => ({
        label: p[0] + p.slice(1).toLowerCase(),
        query: {browse: 'stops', letter: props.letter!, prefix: p},
      })),
    }
  }
  const letters = [...new Set(keyed.value.map((k) => initialOf(k.key)))]
    .sort((a, b) => a === '#' ? 1 : b === '#' ? -1 : a.localeCompare(b))
  return {tiles: letters.map((l) => ({label: l, query: {browse: 'stops', letter: l}}))}
})

const heading = computed(() => {
  const p = props.prefix
  if (p) return p[0] + p.slice(1).toLowerCase()
  return props.letter || t('allStops')
})

onMounted(fetchStops)
</script>

<template>
  <div class="wt-list">
    <header class="wt-head">
      <h1 class="wt-heading">{{ heading }}</h1>
    </header>

    <p v-if="!stops.length && error" class="wt-note">{{ t('watchLoadFailed') }}</p>
    <div v-if="'tiles' in screen" class="wt-grid wt-grid-letters">
      <button
        v-for="tile in screen.tiles"
        :key="tile.label"
        type="button"
        class="wt-tile wt-tile-letter wt-fish"
        @click="router.push({name: 'home', query: tile.query})"
      >{{ tile.label }}</button>
    </div>
    <template v-else>
      <button
        v-for="stop in screen.stops"
        :key="stop.stop_id"
        type="button"
        class="wt-row wt-fish"
        @click="router.push({name: 'stop', params: {stopId: String(stop.stop_id)}})"
      >
        <span class="wt-stop-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
          </svg>
        </span>
        <span class="wt-row-text">
          <span class="wt-row-title">{{ stop.stop_name }}</span>
        </span>
      </button>
    </template>
  </div>
</template>
