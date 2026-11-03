import {computed, type Ref} from 'vue'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {network} from '@/utils/network.ts'
import {type GroupLine, type Leg, legsNear} from '@/utils/lineGroups.ts'

// A fix that changes nothing keeps the same legs, or the map redraws every stop on each one.
export function useLegsNear(lines: Ref<GroupLine[]>) {
  const {userLocation} = storeToRefs(useUserStore())
  return computed<Leg[]>((prev) => {
    const next = network.value ? legsNear(network.value, lines.value, userLocation.value) : []
    return prev && JSON.stringify(prev) === JSON.stringify(next) ? prev : next
  })
}
