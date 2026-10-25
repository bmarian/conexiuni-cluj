export function safeAreaPx(): {top: number; bottom: number} {
  const root = getComputedStyle(document.documentElement)
  return {
    top: parseFloat(root.getPropertyValue('--safe-top')) || 0,
    bottom: parseFloat(root.getPropertyValue('--safe-bottom')) || 0,
  }
}
