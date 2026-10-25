import {describe, expect, it} from 'vitest'
import {stopMatchRank} from '@/utils/stopSearch.ts'

describe('stopMatchRank', () => {
  it('puts a name starting with the query first', () => {
    expect(stopMatchRank('1 decembrie (tunari)', '1 decembrie')).toBe(0)
  })

  it("then the stop's own name, then only the part telling namesakes apart", () => {
    expect(stopMatchRank('bd. 1 decembrie 1918 (strada liviu rebreanu)', '1 decembrie')).toBe(1)
    expect(stopMatchRank('liviu rebreanu (bulevardul 1 decembrie 1918, bucuresti)', '1 decembrie')).toBe(2)
  })

  it('finds a stop by the town or street it was told apart by', () => {
    expect(stopMatchRank('scoala (cornetu)', 'cornetu')).toBe(2)
    expect(stopMatchRank('scoala (cornetu)', 'tunari')).toBe(-1)
  })
})
