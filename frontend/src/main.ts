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
import {apiRequest} from './utils/request_cache'
import './main.css'
import './styles/arcade.css'
import './styles/legacy-blue.css'
import ro from './locales/ro.json'
import en from './locales/en.json'

registerSW({immediate: true})

void apiRequest('routes')
void apiRequest('stops')

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
void favoritesStore.hydrate().then(() => favoritesStore.preloadFavorites())

app.mount('#app')
