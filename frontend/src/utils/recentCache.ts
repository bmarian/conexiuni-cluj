export function recentCache<T>(size: number) {
  const values = new Map<string, T>()
  return {
    get(key: string): T | undefined {
      const hit = values.get(key)
      if (hit !== undefined) {
        values.delete(key)
        values.set(key, hit)
      }
      return hit
    },
    set(key: string, value: T) {
      values.delete(key)
      values.set(key, value)
      while (values.size > size) values.delete(values.keys().next().value!)
    },
    clear() {
      values.clear()
    },
  }
}
