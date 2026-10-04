<script setup lang="ts">
import {computed, nextTick, onUnmounted, provide, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useWatchLocation} from '@/composables/useWatchLocation.ts'
import WatchMap from '@/components/watch/WatchMap.vue'
import WatchHome from '@/views/watch/WatchHome.vue'
import WatchStop from '@/views/watch/WatchStop.vue'
import WatchRoute from '@/views/watch/WatchRoute.vue'
import '@/styles/watch.css'

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const settings = useSettingsStore()
const {userTime} = storeToRefs(useUserStore())

const {status: locationStatus} = useWatchLocation()
provide('watchLocation', locationStatus)

const mapOpen = computed(() => route.query.map !== undefined)

const clock = computed(() => {
  const now = userTime.value || new Date()
  return `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}`
})

// The bezel only scrolls the page itself, so every screen scrolls the document and
// going back has to put it where it was.
const savedScroll = new Map<number, number>()
const restoring = ref(false)
provide('watchRestoring', restoring)

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
  restoring.value = top !== undefined
  if (to.query.map !== undefined) return
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

    <div v-show="!mapOpen">
      <WatchHome v-if="route.name === 'home'"/>
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

    <WatchMap v-if="mapOpen"/>
  </div>
</template>
