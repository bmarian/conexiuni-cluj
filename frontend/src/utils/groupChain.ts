import type {UserLocation} from '@/types/tranzy.ts'
import type {NetworkIndex} from '@/utils/network.ts'
import {chainOf, type GroupLine, type Leg, legsNear} from '@/utils/lineGroups.ts'
import {displayColor} from '@/utils/lineColor.ts'

export const legsChain = (index: NetworkIndex, legs: Leg[]) => chainOf(index, legs, (name, color) => displayColor(name, color, true))

export const chainNear = (index: NetworkIndex, lines: GroupLine[], loc: UserLocation | null) => legsChain(index, legsNear(index, lines, loc))
