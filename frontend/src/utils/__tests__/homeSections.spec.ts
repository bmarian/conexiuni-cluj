import {describe, expect, it} from 'vitest'
import {HOME_SECTIONS, orderSections, withShownOrder} from '@/utils/homeSections.ts'

describe('orderSections', () => {
  it('falls back to the default order', () => {
    expect(orderSections([])).toEqual(HOME_SECTIONS)
  })

  it('keeps a saved order and drops unknown or repeated ids', () => {
    expect(orderSections(['recents', 'stops', 'nope', 'stops', 'lines', 'groups', 'places', 'reminders'])).toEqual([
      'recents',
      'stops',
      'lines',
      'groups',
      'places',
      'reminders',
    ])
  })

  it('puts a missing section after its default neighbour', () => {
    expect(orderSections(['recents', 'lines', 'stops', 'places', 'reminders'])).toEqual(['recents', 'lines', 'groups', 'stops', 'places', 'reminders'])
    expect(orderSections(['stops', 'lines'])).toEqual(['reminders', 'stops', 'places', 'recents', 'lines', 'groups'])
  })
})

describe('withShownOrder', () => {
  it('moves only the shown sections', () => {
    const order = orderSections([])
    expect(withShownOrder(order, ['recents', 'lines', 'stops'])).toEqual(['reminders', 'recents', 'groups', 'lines', 'places', 'stops'])
  })
})
