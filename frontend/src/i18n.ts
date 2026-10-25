import {createI18n} from 'vue-i18n'
import ro from './locales/ro.json'
import en from './locales/en.json'

export const i18n = createI18n({
  legacy: false,
  locale: 'ro',
  fallbackLocale: 'en',
  messages: {
    ro,
    en,
  },
})

export const translate = (key: string, values: Record<string, unknown> = {}) => i18n.global.t(key, values)
