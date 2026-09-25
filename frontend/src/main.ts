import {createApp} from 'vue'
import {createHead} from '@unhead/vue/client'
import {createI18n} from 'vue-i18n'
import {createPinia} from 'pinia'
import {registerSW} from 'virtual:pwa-register'

import App from './App.vue'
import router from './router'
import {useUserStore} from './stores/user'
import {useSettingsStore} from './stores/settings'
import {useFavoritesStore} from './stores/favorites'
import {usePushStore} from './stores/push'
import {useRoutesApi} from './composables/useRoutesApi'
import {useStopsApi} from './composables/useStopsApi'
import './main.css'
import './styles/arcade.css'
import './styles/legacy-blue.css'
import ro from './locales/ro.json'
import en from './locales/en.json'

registerSW({immediate: true})

const sendPWAInstallEvent = () => {
  const body = JSON.stringify({metric: 'pwa_install', key: 'appinstalled'})
  const blob = new Blob([body], {type: 'application/json'})
  if (navigator.sendBeacon?.('/api/stats/event', blob)) return
  void fetch('/api/stats/event', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body,
    keepalive: true,
  }).catch(() => {})
}

window.addEventListener('appinstalled', sendPWAInstallEvent)

const isAdminPath = window.location.pathname.startsWith('/admin')

if (!isAdminPath) {
  void useRoutesApi().fetchRoutes()
  void useStopsApi().fetchStops()
}

const i18n = createI18n({
  legacy: false,
  locale: 'ro',
  fallbackLocale: 'en',
  messages: {
    ro,
    en
  }
})
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

if (!isAdminPath) void usePushStore(pinia).sync()

// Sent by push-sw.js when a notification is tapped while the app is already open.
navigator.serviceWorker?.addEventListener('message', (event) => {
  const url = event.data?.type === 'open-url' ? event.data.url : null
  if (typeof url === 'string' && url.startsWith('/')) void router.push(url)
})

void router.isReady().then(() => app.mount('#app'))
