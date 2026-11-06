import {ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {type FavoriteGroup, type FavoritePlan, type FavoriteRoute, useFavoritesStore} from '@/stores/favorites.ts'
import {useSettingsStore} from '@/stores/settings'

export type FavoriteList = 'routes' | 'stops' | 'groups' | 'plans'
type Saved = {routes: FavoriteRoute[]; stops: number[]; groups: FavoriteGroup[]; plans: FavoritePlan[]}

const holding = ref(false)
const overZone = ref(false)
const zone = ref<HTMLElement | null>(null)
let heldFrom: Saved | null = null

function pointAt(e: Event): {x: number; y: number} | null {
  if (typeof TouchEvent !== 'undefined' && e instanceof TouchEvent) {
    const touch = e.touches[0] ?? e.changedTouches[0]
    return touch ? {x: touch.clientX, y: touch.clientY} : null
  }
  const {clientX, clientY} = e as MouseEvent
  // Firefox reports 0, 0 on some drag events.
  return clientX || clientY ? {x: clientX, y: clientY} : null
}

function track(e: Event) {
  const p = pointAt(e)
  const rect = zone.value?.getBoundingClientRect()
  if (!p || !rect) return
  overZone.value = p.y <= rect.bottom && p.x >= rect.left && p.x <= rect.right
}

// Sortable cancels the next click after a drag that ends outside every list, which would be the tap on Undo.
function swallowSortableClick() {
  const target = document.createElement('span')
  document.body.append(target)
  target.dispatchEvent(new MouseEvent('click', {bubbles: true}))
  target.remove()
}

const EVENTS = ['pointermove', 'touchmove', 'dragover', 'mousemove'] as const

export function useFavoriteDrag() {
  const {t} = useI18n()
  const favorites = useFavoritesStore()
  const settings = useSettingsStore()

  const savedNow = (): Saved => ({
    routes: [...favorites.favoriteRoutes],
    stops: [...favorites.favoriteStopIds],
    groups: [...favorites.favoriteGroups],
    plans: [...favorites.favoritePlans],
  })

  function offerUndo(before: Saved) {
    const routes = before.routes.length !== favorites.favoriteRoutes.length
    const stops = before.stops.length !== favorites.favoriteStopIds.length
    const groups = before.groups.length !== favorites.favoriteGroups.length
    const plans = before.plans.length !== favorites.favoritePlans.length
    if (!routes && !stops && !groups && !plans) return
    settings.showToast(t('removedFromFavorites'), {
      action: {
        label: t('undo'),
        run: () => {
          if (routes) favorites.reorderRoutes(before.routes)
          if (stops) favorites.reorderStopIds(before.stops)
          if (groups) favorites.reorderGroups(before.groups)
          if (plans) favorites.reorderPlans(before.plans)
        },
      },
    })
  }

  function removeAt(list: FavoriteList, before: Saved, i: number) {
    if (list === 'routes') {
      const routeId = [...new Set(before.routes.map((r) => r.routeId))][i]
      favorites.reorderRoutes(before.routes.filter((r) => r.routeId !== routeId))
    } else if (list === 'stops') favorites.reorderStopIds(before.stops.filter((_, j) => j !== i))
    else if (list === 'groups') favorites.reorderGroups(before.groups.filter((_, j) => j !== i))
    else favorites.reorderPlans(before.plans.filter((_, j) => j !== i))
  }

  function holdStart() {
    heldFrom = savedNow()
    holding.value = true
    overZone.value = false
    for (const name of EVENTS) document.addEventListener(name, track, {capture: true, passive: true})
  }

  // `before` is taken at the start: the drag reorders the list on its way to the bar.
  function holdEnd(list: FavoriteList, e: {oldIndex?: number}) {
    for (const name of EVENTS) document.removeEventListener(name, track, {capture: true})
    const before = heldFrom
    const dropped = overZone.value
    heldFrom = null
    holding.value = false
    overZone.value = false
    if (!dropped || !before || e.oldIndex === undefined) return
    swallowSortableClick()
    removeAt(list, before, e.oldIndex)
    offerUndo(before)
  }

  function removeWithUndo(remove: () => void) {
    const before = savedNow()
    remove()
    offerUndo(before)
  }

  return {holding, overZone, zone, holdStart, holdEnd, removeWithUndo}
}
