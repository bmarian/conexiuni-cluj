import {createApp} from 'vue'
import {createHead} from '@unhead/vue/client'
import {createPinia} from 'pinia'
import {registerSW} from 'virtual:pwa-register'

import App from './App.vue'
import router from './router'
import {useUserStore} from './stores/user'
import {useSettingsStore} from './stores/settings'
import {useFavoritesStore} from './stores/favorites'
import {useSyncStore} from './stores/sync'
import {useRoutesApi} from './composables/useRoutesApi'
import {useStopsApi} from './composables/useStopsApi'
import {startNetwork} from './utils/network'
import './main.css'
import {i18n} from './i18n'
import {apiPath, citiesReady} from './utils/city'
import {lockIOSPageZoom} from './utils/ios'
import {applyPaces} from './utils/motion'

lockIOSPageZoom()

applyPaces(document.documentElement)

const sendPWAInstallEvent = () => {
  const body = JSON.stringify({metric: 'pwa_install', key: 'appinstalled'})
  const blob = new Blob([body], {type: 'application/json'})
  if (navigator.sendBeacon?.(apiPath('stats/event'), blob)) return
  void fetch(apiPath('stats/event'), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body,
    keepalive: true,
  }).catch(() => {})
}

registerSW({
  immediate: true,
  onRegisteredSW(_url, registration) {
    if (!registration) return
    const checkForUpdate = () => {
      if (!document.hidden) registration.update().catch(() => {})
    }
    checkForUpdate()
    document.addEventListener('visibilitychange', checkForUpdate)
  },
})

window.addEventListener('appinstalled', sendPWAInstallEvent)

const isAdminPath = window.location.pathname === '/admin'

if (!isAdminPath) {
  void useRoutesApi().fetchRoutes()
  void useStopsApi().fetchStops()
  void startNetwork()
}

const app = createApp(App)
const pinia = createPinia()

app.use(i18n)
app.use(pinia)
app.use(router)
app.use(createHead())

const settingsStore = useSettingsStore(pinia)
i18n.global.locale.value = settingsStore.locale

const userStore = useUserStore(pinia)
userStore.startTimeTracker()

const favoritesStore = useFavoritesStore(pinia)
void favoritesStore.hydrate().then(() => {
  if (!isAdminPath) favoritesStore.preloadFavorites()
})

void citiesReady.then(() => router.isReady()).then(() => app.mount('#app'))

if (!isAdminPath) void citiesReady.then(() => useSyncStore(pinia).start())
