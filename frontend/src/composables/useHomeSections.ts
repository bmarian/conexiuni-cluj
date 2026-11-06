import {readonly, ref} from 'vue'
import {type HomeSectionId, orderSections} from '@/utils/homeSections.ts'
import {readSaved} from '@/utils/storage.ts'

const ORDER_KEY = 'home.order'
const COLLAPSED_KEY = 'home.collapsed'

function readList(key: string): string[] {
  const saved = readSaved(key)
  return Array.isArray(saved) ? saved.filter((id): id is string => typeof id === 'string') : []
}

const order = ref<HomeSectionId[]>(orderSections(readList(ORDER_KEY)))
const collapsed = ref(new Set(readList(COLLAPSED_KEY)))

window.addEventListener('storage', (e) => {
  if (e.key === ORDER_KEY) order.value = orderSections(readList(ORDER_KEY))
  else if (e.key === COLLAPSED_KEY) collapsed.value = new Set(readList(COLLAPSED_KEY))
})

export function useHomeSections() {
  function setOrder(next: HomeSectionId[]) {
    order.value = orderSections(next)
    localStorage.setItem(ORDER_KEY, JSON.stringify(order.value))
  }

  function toggle(id: HomeSectionId) {
    const next = new Set(collapsed.value)
    if (!next.delete(id)) next.add(id)
    collapsed.value = next
    localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]))
  }

  const isCollapsed = (id: HomeSectionId) => collapsed.value.has(id)

  return {order: readonly(order), setOrder, toggle, isCollapsed}
}
