import {onMounted, type Ref} from 'vue'

const drawerOf = (view: HTMLElement | undefined) => view?.closest<HTMLElement>('.drawer-view') ?? null

// The shared drawer scroller keeps the last page's offset.
export function useScrollStart(view: Ref<HTMLElement | undefined>) {
  onMounted(() => {
    const scroller = drawerOf(view.value)
    if (scroller) scroller.scrollTop = 0
  })
}

export function scrollBelow(view: HTMLElement, pinned: HTMLElement, row: HTMLElement) {
  const scroller = drawerOf(view)
  const list = row.offsetParent as HTMLElement | null
  if (!scroller || !list) return
  // Offsets, not rectangles: the page may still be sliding in.
  scroller.scrollTop = list.offsetTop - view.offsetTop + row.offsetTop - parseFloat(getComputedStyle(pinned).top) - pinned.offsetHeight
}
