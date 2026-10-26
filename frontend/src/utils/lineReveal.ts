import {SWEEP_MS} from '@/utils/lineSweep.ts'
import type {LatLng} from '@/utils/mapMarkers.ts'
import {spring} from '@/utils/motion.ts'
import {locate, type Path} from '@/utils/vehicleMotion.ts'

export type Growth = {path: Path; start?: number}

// growLine's easeInOut solved for time.
const timeAt = (done: number) => (done < 0.5 ? Math.sqrt(done / 2) : 1 - Math.sqrt(2 * (1 - done)) / 2)

// Infinity until the line starts growing.
function reachedAt({path, start}: Growth, point: LatLng): number | undefined {
  const at = path.length ? locate(path, point) : null
  if (!at) return undefined
  return start === undefined ? Infinity : start + timeAt(at.s / path.length) * SWEEP_MS
}

// While lines grow, their stops pop in as the front reaches them and buses once every line is drawn.
// Before [now] counts as already shown, so a marker rebuilt mid-growth doesn't pop twice.
export class LineReveal {
  private growths = new Map<string, Growth>()
  private pops = new WeakMap<HTMLElement, Animation>()

  // Its stops and every bus stay hidden until it starts.
  grow(line: string, path: Path): Growth {
    const growth: Growth = {path}
    this.growths.set(line, growth)
    return growth
  }

  // Undefined once the line has changed since grow.
  start(line: string, growth: Growth): number | undefined {
    if (this.growths.get(line) !== growth) return undefined
    growth.start = performance.now()
    return growth.start
  }

  end(line: string) {
    this.growths.delete(line)
  }

  stop(body: HTMLElement, point: LatLng, now = performance.now()): boolean {
    const times = this.live(now).flatMap((growth) => reachedAt(growth, point) ?? [])
    return this.popAt(body, times.length ? Math.min(...times) : undefined, now)
  }

  vehicle(body: HTMLElement, now = performance.now()): boolean {
    const ends = this.live(now).map(({start}) => (start === undefined ? Infinity : start + SWEEP_MS))
    return this.popAt(body, ends.length ? Math.max(...ends) : undefined, now)
  }

  show(body: HTMLElement) {
    this.pops.get(body)?.cancel()
  }

  private live(now: number): Growth[] {
    for (const [line, {start}] of this.growths) if (start !== undefined && now - start >= SWEEP_MS) this.growths.delete(line)
    return [...this.growths.values()]
  }

  private popAt(body: HTMLElement, at: number | undefined, now: number): boolean {
    this.show(body)
    if (at === undefined || at < now) return false
    const pop = body.animate([{scale: 0}, {scale: 1}], {...spring('pop'), fill: 'backwards'})
    if (at === Infinity) pop.pause()
    // On growLine's clock: a delay would only start counting from the next frame.
    else pop.startTime = at
    this.pops.set(body, pop)
    return true
  }
}
