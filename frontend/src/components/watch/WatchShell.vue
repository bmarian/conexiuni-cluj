<script setup lang="ts">
import {computed, nextTick, onUnmounted} from 'vue'
import {useI18n} from 'vue-i18n'
import {type RouteLocationNormalized, useRoute, useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import WatchMap from '@/components/watch/WatchMap.vue'
import WatchHome from '@/views/watch/WatchHome.vue'
import WatchRoutes from '@/views/watch/WatchRoutes.vue'
import WatchStops from '@/views/watch/WatchStops.vue'
import WatchStop from '@/views/watch/WatchStop.vue'
import WatchRoute from '@/views/watch/WatchRoute.vue'
import '@/styles/watch.css'

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const settings = useSettingsStore()
const {userTime} = storeToRefs(useUserStore())
const {stops} = useStopsApi()

const browse = computed(() => route.name === 'home' ? route.query.browse : undefined)
const queryString = (key: string) => typeof route.query[key] === 'string' ? route.query[key] as string : undefined

const stopMapOpen = computed(() => route.name === 'stop' && route.query.map !== undefined)
const stopFocus = computed(() => {
  const stop = stops.value.find((s) => String(s.stop_id) === route.params.stopId)
  return stop ? {lat: stop.stop_lat, lng: stop.stop_lon} : null
})

const clock = computed(() => {
  const now = userTime.value || new Date()
  return `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}`
})

// The bezel only scrolls the page itself, so every screen scrolls the document and
// going back has to put it where it was. Map screens park it in their scroll sink.
const savedScroll = new Map<number, number>()

const isMapScreen = (to: RouteLocationNormalized) =>
  (to.name === 'stop' && to.query.map !== undefined)
  || (to.name === 'route' && to.query.timetable === undefined)

const historyPosition = () => (window.history.state?.position as number | undefined) ?? 0
let currentPosition = historyPosition()
let popped = false

function restoreScroll(top: number) {
  let frames = 0
  const step = () => {
    const room = document.documentElement.scrollHeight - window.innerHeight
    if (room >= top || frames++ > 90) {
      window.scrollTo(0, top)
      return
    }
    requestAnimationFrame(step)
  }
  step()
}

const removeBefore = router.beforeEach(() => {
  savedScroll.set(currentPosition, window.scrollY)
})
const stopListening = router.options.history.listen((_to, _from, info) => {
  popped = info.type === 'pop'
})
const removeAfter = router.afterEach((to, _from, failure) => {
  if (failure) return
  currentPosition = historyPosition()
  const top = popped ? savedScroll.get(currentPosition) : undefined
  popped = false
  if (isMapScreen(to)) return
  void nextTick(() => top === undefined ? window.scrollTo(0, 0) : restoreScroll(top))
})

onUnmounted(() => {
  removeBefore()
  removeAfter()
  stopListening()
})
</script>

<template>
  <div class="wt-shell">
    <div class="wt-clock" aria-hidden="true">{{ clock }}</div>
    <div class="wt-fade wt-fade-top" aria-hidden="true"></div>
    <div class="wt-fade wt-fade-bottom" aria-hidden="true"></div>

    <div v-show="!stopMapOpen">
      <WatchRoutes v-if="browse === 'routes'"/>
      <WatchStops
        v-else-if="browse === 'stops'"
        :letter="queryString('letter')"
        :prefix="queryString('prefix')"
      />
      <WatchHome v-else-if="route.name === 'home'"/>
      <WatchStop
        v-else-if="route.name === 'stop'"
        :key="`stop-${route.params.stopId}`"
        :stop-id="String(route.params.stopId)"
      />
      <WatchRoute
        v-else-if="route.name === 'route'"
        :key="`route-${route.params.routeId}`"
        :route-id="String(route.params.routeId)"
        :direction="String(route.params.direction)"
      />
      <div v-else class="wt-list">
        <p class="wt-note">{{ t('simplifiedUnavailable') }}</p>
        <button type="button" class="wt-row wt-row-action" @click="router.push({name: 'home'})">
          {{ t('home') }}
        </button>
        <button type="button" class="wt-row wt-row-action" @click="settings.setSimplified(false)">
          {{ t('simplifiedTurnOff') }}
        </button>
      </div>
    </div>

    <WatchMap v-if="stopMapOpen" :focus="stopFocus"/>
  </div>
</template>
