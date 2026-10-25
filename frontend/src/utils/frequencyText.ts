import {translate as t, i18n} from '@/i18n.ts'
import type {Frequency} from '@/types/ctp.ts'

export function frequencyLabel({min_minutes: min, max_minutes: max}: Frequency): string {
  const isRange = !!(min && max && min !== max)
  const last = isRange ? max : min || max
  const locale = String(i18n.global.locale.value)
  const withDe = new Intl.PluralRules(locale === 'en' ? 'en' : 'ro').select(last) === 'other'
  if (isRange) return t(withDe ? 'frequencyEveryRangeOther' : 'frequencyEveryRange', {min, max})
  return t(withDe ? 'frequencyEveryOther' : 'frequencyEvery', {min: last})
}
