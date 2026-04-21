import { createApp } from 'vue'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'
import { registerSW } from 'virtual:pwa-register'

import App from './App.vue'
import router from './router'

registerSW({ immediate: true })
import { useUserStore } from './stores/user'
import { useFavoritesStore } from './stores/favorites'
import { apiRequest, LOW_ACCURACY_SHELF_LIFE } from './utils/request_cache'
import './main.css'
import ro from './locales/ro.json'
import en from './locales/en.json'

void apiRequest('routes', LOW_ACCURACY_SHELF_LIFE)
void apiRequest('stops', LOW_ACCURACY_SHELF_LIFE)

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

const userStore = useUserStore(pinia)
userStore.startTimeTracker()
userStore.startSchemeWatcher()

const favoritesStore = useFavoritesStore(pinia)
void favoritesStore.hydrate().then(() => favoritesStore.preloadFavorites())

app.mount('#app')
