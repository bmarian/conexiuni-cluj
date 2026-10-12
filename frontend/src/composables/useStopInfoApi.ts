import {ref} from "vue";
import {buildStopInfo, network, networkLoaded, refreshNetwork} from "@/utils/network.ts";

export function useStopInfoApi() {
  const stopInfo = ref()
  const error = ref()

  async function fetchStopData(stopId: string) {
    stopInfo.value = undefined
    error.value = undefined
    await networkLoaded
    if (!network.value) await refreshNetwork(true)
    const local = network.value && buildStopInfo(network.value, Number(stopId))
    if (local) {
      stopInfo.value = local
      return
    }
    error.value = new Error(`no network data for stop ${stopId}`)
  }

  return {stopInfo, error, fetchStopData}
}
