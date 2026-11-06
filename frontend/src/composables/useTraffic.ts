import {computed} from 'vue'
import {storeToRefs} from 'pinia'
import {useSettingsStore} from '@/stores/settings'
import {network} from '@/utils/network.ts'
import {liveTraffic, type TrafficByTrip, worstJam} from '@/utils/traffic.ts'

const NONE: TrafficByTrip = {}

export function useTraffic() {
  const {traffic} = storeToRefs(useSettingsStore())
  return computed(() => (traffic.value ? liveTraffic.value : NONE))
}

export function useJams() {
  const traffic = useTraffic()
  return (tripId: string, fromStopId?: number) => worstJam(traffic.value[tripId], network.value?.tripStopIds(tripId) ?? [], fromStopId)
}
