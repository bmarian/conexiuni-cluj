import {beforeEach, describe, expect, it, vi} from 'vitest'

const store = new Map<string, string>()
vi.stubGlobal('localStorage', {
  getItem: (key: string) => store.get(key) ?? null,
  setItem: (key: string, value: string) => store.set(key, value),
})
vi.stubGlobal('window', {addEventListener: () => {}})

async function load() {
  vi.resetModules()
  return import('@/composables/useFirstSteps.ts')
}

describe('useFirstSteps', () => {
  beforeEach(() => store.clear())

  it('shows every step to someone starting with nothing saved', async () => {
    const m = await load()
    m.startFirstSteps(true)
    expect(m.useFirstSteps().left.value).toEqual(['plan', 'stop', 'line'])
  })

  it('never shows the card to someone who already saved something', async () => {
    const m = await load()
    m.startFirstSteps(false)
    m.startFirstSteps(true)
    expect(m.useFirstSteps().left.value).toEqual([])
  })

  it('drops done steps and keeps them done after a reload', async () => {
    let m = await load()
    m.startFirstSteps(true)
    m.markFirstStep('stop')
    expect(m.useFirstSteps().done.value).toBe(1)
    m = await load()
    expect(m.useFirstSteps().left.value).toEqual(['plan', 'line'])
  })

  it('stays hidden once closed', async () => {
    let m = await load()
    m.startFirstSteps(true)
    m.hideFirstSteps()
    m = await load()
    m.startFirstSteps(true)
    expect(m.useFirstSteps().left.value).toEqual([])
  })

  it('ignores steps before the card was decided', async () => {
    const m = await load()
    m.markFirstStep('plan')
    m.startFirstSteps(true)
    expect(m.useFirstSteps().left.value).toEqual(['plan', 'stop', 'line'])
  })
})
