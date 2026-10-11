import {translate as t, i18n} from '@/i18n.ts'
import {clockAt, isClockTime} from '@/utils/follow.ts'
import {endsAtStop, lineLabel, type Journey, type JourneyStep} from '@/utils/journey.ts'

const minutes = (n: number) => t('etaMinutes', {n})

function stopsLeft(n: number): string {
  const locale = String(i18n.global.locale.value)
  const rule = new Intl.PluralRules(locale === 'en' ? 'en' : 'ro').select(n)
  if (rule === 'one') return t('journeyStopsLeftOne', {n})
  return t(rule === 'few' ? 'journeyStopsLeftFew' : 'journeyStopsLeftOther', {n})
}

export function stepTitle(step: JourneyStep): string {
  switch (step.kind) {
    case 'walk': return t('journeyWalkTo', {name: step.target.name})
    case 'wait': return t('journeyWaitFor', {line: step.line ?? ''})
    case 'ride': return t('journeyGetOffAt', {name: step.target.name})
    case 'walkEnd': return t('journeyWalkTo', {name: step.target.name})
    case 'arrived': return t('journeyArrived')
  }
}

export function stepDetail(step: JourneyStep, nowMs: number): string | null {
  const bus = step.busMinutes
  switch (step.kind) {
    case 'walk':
      if (bus !== null) {
        const line = step.line ?? ''
        if (bus <= 0) return t('journeyLineHere', {line})
        if (isClockTime(bus)) return t('journeyLineAt', {line, time: clockAt(nowMs, bus)})
        return t('journeyLineIn', {line, time: minutes(bus)})
      }
      return step.walkMinutes !== null ? t('journeyWalkMinutes', {n: step.walkMinutes}) : null
    case 'wait':
      if (bus === null) return null
      if (bus <= 0) return t('followNow')
      if (isClockTime(bus)) return t('followArrivingAt', {time: clockAt(nowMs, bus)})
      return t('followArriving', {time: minutes(bus)})
    case 'ride':
      return step.stopsLeft === 1 ? t('journeyNextStop') : stopsLeft(step.stopsLeft ?? 0)
    case 'walkEnd':
      return step.walkMinutes !== null ? t('journeyWalkMinutes', {n: step.walkMinutes}) : null
    case 'arrived':
      return null
  }
}

export function thenText(journey: Journey, step: JourneyStep): string | null {
  const leg = journey.legs[step.legIndex]
  if (!leg) return null
  const next = journey.legs[step.legIndex + 1]
  const alighting = leg.stops[leg.stops.length - 1]!
  switch (step.kind) {
    case 'walk': return t('navThenTake', {line: lineLabel(leg), name: alighting.name})
    case 'wait': return t('navThenGetOff', {name: alighting.name})
    case 'ride':
      if (next) return t('navThenChange', {line: lineLabel(next), name: next.stops[0]!.name})
      return endsAtStop(journey) ? null : t('navThenWalk', {name: journey.destination.name})
    default: return null
  }
}

export const arriveText = (etaAt: number) => t('navArriveAt', {time: clockAt(etaAt, 0)})
