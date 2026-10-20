// Measures text as `probe` would render it, without laying anything out.
export function textMeasurer(probe: HTMLElement): (text: string) => number {
  const style = getComputedStyle(probe)
  const context = document.createElement('canvas').getContext('2d')
  if (context) context.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`
  const upper = style.textTransform === 'uppercase'
  const spacing = parseFloat(style.letterSpacing) || 0
  const padding = (parseFloat(style.paddingLeft) || 0) + (parseFloat(style.paddingRight) || 0)
  const widths = new Map<string, number>()
  return (text) => {
    let width = widths.get(text)
    if (width === undefined) {
      const shown = upper ? text.toUpperCase() : text
      width = (context?.measureText(shown).width ?? shown.length * parseFloat(style.fontSize)) + spacing * shown.length + padding
      widths.set(text, width)
    }
    return width
  }
}
