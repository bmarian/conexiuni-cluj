import {computed, ref, watch, type Ref} from 'vue'
import {cities} from '@/utils/city.ts'
import {fold} from '@/utils/text.ts'

const ROWS_SHOWN = 5

export function useCityPicker(open: Ref<boolean>) {
  const query = ref('')
  const matches = computed(() => {
    const folded = fold(query.value.trim())
    return folded ? cities.value.filter((city) => fold(city.name).includes(folded) || fold(city.operator).includes(folded)) : cities.value
  })
  const list = ref<HTMLElement>()
  const listHeight = ref<string>()

  watch(open, (isOpen) => {
    if (!isOpen) query.value = ''
  })

  watch(list, (el, _, onCleanup) => {
    const row = el?.firstElementChild as HTMLElement | null | undefined
    if (!row) return
    const observer = new ResizeObserver(() => {
      // 0 once a search takes the row out.
      if (row.offsetHeight) listHeight.value = `${row.offsetHeight * ROWS_SHOWN}px`
    })
    observer.observe(row)
    onCleanup(() => observer.disconnect())
  })

  return {query, matches, list, listHeight}
}
