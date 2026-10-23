import {describe, expect, it} from 'vitest'
import {createI18n} from 'vue-i18n'
import en from '@/locales/en.json'
import ro from '@/locales/ro.json'

describe('page titles', () => {
  for (const [locale, messages] of Object.entries({ro, en})) {
    it(`keep the site name in ${locale}`, () => {
      const {t} = createI18n({legacy: false, locale, messages: {[locale]: messages}}).global
      const titles = Object.keys(messages).filter((key) => key.startsWith('head') && key.endsWith('Title'))
      for (const key of titles.filter((key) => messages[key as keyof typeof messages].includes('Ce autobuz iau?'))) {
        expect(t(key, {}), key).toContain('Ce autobuz iau?')
      }
    })
  }
})
