import type {StopInfo} from "@/types/tranzy.ts";
import {ref} from "vue";
import {apiRequest, LOW_ACCURACY_SHELF_LIFE} from "@/utils/request_cache.ts";

const pendingRequests = new Map<string, Promise<StopInfo>>()

export function useStopInfoApi() {
  const stopInfo = ref()
  const error = ref()

  async function fetchStopData(stopId: string) {
    stopInfo.value = undefined
    error.value = undefined

    if (pendingRequests.has(stopId)) {
      try {
        stopInfo.value = await pendingRequests.get(stopId)
      } catch (e) {
        error.value = e
      }
      return
    }

    const requestPromise = apiRequest(`stop_info?stop_id=${stopId}`, LOW_ACCURACY_SHELF_LIFE) as Promise<StopInfo>
    pendingRequests.set(stopId, requestPromise)

    try {
      stopInfo.value = await requestPromise
    } catch (e) {
      error.value = e
      console.error(e)
    } finally {
      pendingRequests.delete(stopId)
    }
  }

  return { stopInfo, error, fetchStopData }
}
