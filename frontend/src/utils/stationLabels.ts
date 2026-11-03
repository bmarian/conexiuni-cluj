import type {Marker} from 'maplibre-gl'
import {labelBox, labelSidesClear} from '@/utils/spider.ts'
import type {ClusterBox, Rect} from '@/utils/stopClusters.ts'

export type LabelledStation = {id: string; name: string; marker: Marker}

export function createStationLabels() {
  const shown = new Map<string, {el: HTMLElement; label: HTMLElement}>()

  const keep = (stations: LabelledStation[]) => {
    const wanted = new Map(stations.map(({id, marker}) => [id, marker.getElement()]))
    for (const [id, {el, label}] of shown) {
      if (wanted.get(id) === el && label.parentElement === el) continue
      label.remove()
      el.style.zIndex = ''
      shown.delete(id)
    }
  }

  const place = (stations: LabelledStation[], labelWidth: (name: string) => number, obstacles: Rect[], area: Rect) => {
    const items = stations.map(({marker, name}) => {
      const r = marker.getElement().getBoundingClientRect()
      const {x, y} = marker.getOffset()
      const icon: ClusterBox = {size: [r.width, r.height], anchor: [-x, -y]}
      return {station: {x: r.left - x, y: r.top - y}, icon, labelWidth: labelWidth(name)}
    })
    labelSidesClear(items, obstacles, area).forEach((side, i) => {
      const {id, name, marker} = stations[i]!
      const {icon, labelWidth: width} = items[i]!
      let label = shown.get(id)?.label
      if (!label) {
        const el = marker.getElement()
        label = document.createElement('div')
        label.className = 'stop-cluster-label stop-label'
        label.textContent = name
        el.append(label)
        el.style.zIndex = '900'
        shown.set(id, {el, label})
      }
      const box = labelBox({x: icon.anchor[0], y: icon.anchor[1]}, icon, width, side)
      const {style} = label
      style.top = `${box.top}px`
      style.left = side === 'right' ? `${box.left}px` : side === 'below' ? '50%' : ''
      style.right = side === 'left' ? `${icon.size[0] - box.right}px` : ''
      style.translate = side === 'below' ? '-50% 0' : ''
    })
  }

  return {keep, place}
}
