import {Map as MapLibreMap} from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import {storeToRefs} from 'pinia'
import {onBeforeUnmount, onMounted, watch, type Ref} from 'vue'
import {useUserStore} from '@/stores/user.ts'
import {FLAT_MAP_OPTIONS} from '@/utils/mapMarkers.ts'
import {mapStyle} from '@/utils/mapStyles.ts'
import '@/utils/mapWorker.ts'

export const ADMIN_MAP_PADDING_PX = 48

export function useAdminMap(container: Ref<HTMLElement | null>, ready: (map: MapLibreMap) => void) {
  const {isDarkMode} = storeToRefs(useUserStore())
  let map: MapLibreMap | null = null

  onMounted(() => {
    map = new MapLibreMap({
      container: container.value!,
      style: mapStyle(isDarkMode.value),
      attributionControl: false,
      ...FLAT_MAP_OPTIONS,
    })
    map.touchZoomRotate.disableRotation()
    ready(map)
  })

  onBeforeUnmount(() => map?.remove())

  watch(isDarkMode, (dark) => map?.setStyle(mapStyle(dark)))

  return () => map
}
