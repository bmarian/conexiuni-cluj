export type HomeSectionId = 'reminders' | 'lines' | 'groups' | 'stops' | 'places' | 'recents'

export const HOME_SECTIONS: HomeSectionId[] = ['reminders', 'lines', 'groups', 'stops', 'places', 'recents']

// A section missing from the saved order goes after its default predecessor.
export function orderSections(saved: readonly string[]): HomeSectionId[] {
  const order = [...new Set(saved)].filter((id): id is HomeSectionId => HOME_SECTIONS.includes(id as HomeSectionId))
  HOME_SECTIONS.forEach((id, i) => {
    if (order.includes(id)) return
    const before = HOME_SECTIONS.slice(0, i)
      .reverse()
      .find((prev) => order.includes(prev))
    order.splice(before ? order.indexOf(before) + 1 : 0, 0, id)
  })
  return order
}

// Hidden sections keep their slots.
export function withShownOrder(order: readonly HomeSectionId[], shown: readonly HomeSectionId[]): HomeSectionId[] {
  const next = [...shown]
  return order.map((id) => (shown.includes(id) ? next.shift()! : id))
}
