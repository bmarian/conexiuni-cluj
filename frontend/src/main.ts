import { createApp } from 'vue'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import { useUserStore } from './stores/user'
import './main.css'
import ro from './locales/ro.json'
import en from './locales/en.json'

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

useUserStore(pinia).updateUserCurrentLocation()

app.mount('#app')
