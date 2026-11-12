import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {network, type NetworkIndex} from '@/utils/network.ts'
import {flipLine, type GroupLegs, type GroupLine, type Leg, legLines, legsOf, readLines, stretchNames, writeLines} from '@/utils/lineGroups.ts'
import {groupQuery} from '@/utils/pagePaths.ts'

const EDITED_BACK = '|'

export function useStretchLabel() {
  const {t} = useI18n()
  return (from?: string, to?: string): string => {
    if (from && to) return `${from} → ${to}`
    if (from) return t('from', {name: from})
    return to ? t('groupUntil', {name: to}) : t('groupWholeLine')
  }
}

export const writeEdited = (index: NetworkIndex, legs: GroupLegs) =>
  legs.back?.length ? `${writeLines(index, legs.lines)}${EDITED_BACK}${writeLines(index, legs.back)}` : writeLines(index, legs.lines)

export function readEdited(index: NetworkIndex, raw: string): GroupLegs {
  const [lines = '', back = ''] = raw.split(EDITED_BACK)
  return legsOf(readLines(index, lines), readLines(index, back))
}

export const readLeg = (raw: unknown): Leg => (raw === 'intors' ? 'intors' : 'dus')

export function useGroupBuilder() {
  const route = useRoute()
  const router = useRouter()
  const label = useStretchLabel()

  const legs = computed<GroupLegs>(() => {
    const index = network.value
    return index ? legsOf(readLines(index, String(route.query.linii ?? '')), readLines(index, String(route.query.intors ?? ''))) : {lines: []}
  })
  const leg = computed(() => readLeg(route.query.sens))
  const lines = computed(() => legLines(legs.value, leg.value))
  const editing = computed(() => (route.query.schimba ? String(route.query.schimba) : undefined))

  const withLeg = (next: GroupLine[], on = leg.value): GroupLegs => (on === 'intors' ? legsOf(legs.value.lines, next) : legsOf(next, legs.value.back))

  const query = (next: GroupLegs, on: Leg) => ({
    ...(network.value ? groupQuery(network.value, next, on) : {linii: ''}),
    ...(editing.value ? {schimba: editing.value} : {}),
  })

  const depth = () => Number(history.state?.groupDepth) || 0

  function openAdd() {
    void router.push({name: 'group-add', query: query(legs.value, leg.value), state: {groupDepth: depth() + 1}})
  }

  function openStops(line: GroupLine) {
    if (!network.value) return
    void router.push({
      name: 'group-stops',
      query: {...query(legs.value, leg.value), linie: writeLines(network.value, [line])},
      state: {groupDepth: depth() + 1},
    })
  }

  function showGroup(next: GroupLine[], on = leg.value) {
    void router.replace({name: 'group-pick', query: query(withLeg(next, on), on)})
  }

  function showLeg(on: Leg) {
    void router.replace({name: 'group-pick', query: query(legs.value, on)})
  }

  function fillBack() {
    const index = network.value
    if (index)
      showGroup(
        legs.value.lines.map((l) => flipLine(index, l)),
        'intors',
      )
  }

  async function backToGroup(next: GroupLine[]) {
    const back = depth()
    const on = leg.value
    if (back) {
      const landed = new Promise<void>((resolve) => {
        const stop = router.afterEach(() => {
          stop()
          resolve()
        })
      })
      router.go(-back)
      await landed
    }
    showGroup(next, on)
  }

  function stretchLabel(line: GroupLine): string {
    const {from, to} = network.value ? stretchNames(network.value, line) : {}
    return label(from, to)
  }

  return {legs, leg, lines, editing, openAdd, openStops, showGroup, showLeg, fillBack, backToGroup, stretchLabel}
}
