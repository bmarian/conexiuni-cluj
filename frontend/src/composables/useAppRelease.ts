import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {latestRelease, type AppRelease, type AppRepo} from '@/utils/androidApp.ts'

export function useAppRelease(repo: AppRepo) {
  const {t, locale} = useI18n()
  const release = ref<AppRelease | null>(null)
  void latestRelease(repo).then((r) => { release.value = r })
  return computed(() => {
    const r = release.value
    if (!r) return null
    const date = new Intl.DateTimeFormat(locale.value, {day: 'numeric', month: 'short', year: 'numeric'}).format(new Date(r.date))
    return t('appVersion', {version: r.version, date})
  })
}
