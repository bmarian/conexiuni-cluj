import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {network} from '@/utils/network.ts'
import {type GroupLine, readLines, stretchNames, writeLines} from '@/utils/lineGroups.ts'

export function useStretchLabel() {
  const {t} = useI18n()
  return (from?: string, to?: string): string => {
    if (from && to) return `${from} → ${to}`
    if (from) return t('from', {name: from})
    return to ? t('groupUntil', {name: to}) : t('groupWholeLine')
  }
}

export function useGroupBuilder() {
  const route = useRoute()
  const router = useRouter()
  const label = useStretchLabel()

  const lines = computed(() => (network.value ? readLines(network.value, String(route.query.linii ?? '')) : []))
  const editing = computed(() => (route.query.schimba ? String(route.query.schimba) : undefined))

  const query = (next: GroupLine[]) => ({
    linii: network.value ? writeLines(network.value, next) : '',
    ...(editing.value ? {schimba: editing.value} : {}),
  })

  const depth = () => Number(history.state?.groupDepth) || 0

  function openAdd() {
    void router.push({name: 'group-add', query: query(lines.value), state: {groupDepth: depth() + 1}})
  }

  function openStops(line: GroupLine) {
    if (!network.value) return
    void router.push({name: 'group-stops', query: {...query(lines.value), linie: writeLines(network.value, [line])}, state: {groupDepth: depth() + 1}})
  }

  function showGroup(next: GroupLine[]) {
    void router.replace({name: 'group-pick', query: query(next)})
  }

  async function backToGroup(next: GroupLine[]) {
    const back = depth()
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
    showGroup(next)
  }

  function stretchLabel(line: GroupLine): string {
    const {from, to} = network.value ? stretchNames(network.value, line) : {}
    return label(from, to)
  }

  return {lines, editing, openAdd, openStops, showGroup, backToGroup, stretchLabel}
}
