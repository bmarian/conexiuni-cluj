import {describe, expect, it} from 'vitest'
import {placeTooltipHtml} from '@/utils/mapIcons.ts'

describe('place tooltip', () => {
  it('splits the place from its area', () => {
    expect(placeTooltipHtml('Piața Unirii, Cluj-Napoca')).toBe('<div class="pin-tip-main">Piața Unirii</div><div class="pin-tip-sub">Cluj-Napoca</div>')
  })

  it('shows a name from a shared link as text, not markup', () => {
    const html = placeTooltipHtml('<img src=x onerror=alert(1)>, <b>Centru</b>')
    expect(html).not.toContain('<img')
    expect(html).not.toContain('<b>')
    expect(html).toContain('&lt;img src=x onerror=alert(1)&gt;')
  })
})
