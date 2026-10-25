import {ref} from 'vue'
import {buildStopInfo, loadedNetwork} from '@/utils/network.ts'

export function useStopInfoApi() {
  const stopInfo = ref()
  const error = ref()

  async function fetchStopData(stopId: string) {
    stopInfo.value = undefined
    error.value = undefined
    const index = await loadedNetwork()
    const local = index && buildStopInfo(index, Number(stopId))
    if (local) {
      stopInfo.value = local
      return
    }
    error.value = new Error(`no network data for stop ${stopId}`)
  }

  return {stopInfo, error, fetchStopData}
}
