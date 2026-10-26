<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {spring, reducedMotion} from '@/utils/motion.ts'

const props = defineProps<{text: string}>()

// Same words: only the digits that change roll, down when the number shrinks.
const wording = computed(() => props.text.replace(/\d+/g, '#'))
const parts = computed(() => props.text.split(/(\d+)/))
const numbers = (text: string) => (text.match(/\d+/g) ?? []).map(Number)
// Screen readers and search engines read rolling digits one by one.
const counted = computed(() => /\d/.test(props.text))

const down = ref(false)
watch(
  () => props.text,
  (now, was) => {
    const before = numbers(was)
    const after = numbers(now)
    const i = after.findIndex((n, k) => n !== before[k])
    down.value = i !== -1 && after[i]! < before[i]!
  },
  {flush: 'pre'},
)

function resize(el: Element, done: () => void, grow: boolean) {
  if (reducedMotion()) return done()
  const box = el as HTMLElement
  const width = `${box.offsetWidth}px`
  const {duration, easing} = spring('fast')
  box.animate([{width: grow ? '0px' : width}, {width: grow ? width : '0px'}], {duration, easing, fill: grow ? 'none' : 'forwards'}).finished.then(done, done)
}
</script>

<template>
  <span class="ticker">
    <span v-if="counted" class="sr-only">{{ text }}</span>
    <Transition name="tick">
      <span :key="wording" :aria-hidden="counted || undefined">
        <template v-for="(part, i) in parts" :key="i">
          <span v-if="i % 2" class="number">
            <TransitionGroup :css="false" @enter="(el, done) => resize(el, done, true)" @leave="(el, done) => resize(el, done, false)">
              <span v-for="(digit, k) in part" :key="part.length - k" class="digit">
                <Transition :name="down ? 'roll-down' : 'tick'">
                  <span :key="digit">{{ digit }}</span>
                </Transition>
              </span>
            </TransitionGroup>
          </span>
          <template v-else>{{ part }}</template>
        </template>
      </span>
    </Transition>
  </span>
</template>

<style scoped>
.ticker,
.digit {
  display: inline-grid;
  white-space: nowrap;
}

.ticker > *,
.digit > * {
  grid-area: 1 / 1;
}

.ticker {
  max-width: 100%;
}

.ticker > * {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.number {
  display: inline-flex;
  font-variant-numeric: tabular-nums;
}

.digit {
  overflow: hidden;
}

.tick-enter-active,
.roll-down-enter-active {
  transition:
    translate var(--spring-slide-ms) var(--spring-slide),
    opacity 175ms linear;
}

.tick-leave-active,
.roll-down-leave-active {
  transition:
    translate 260ms ease-in,
    opacity 130ms linear;
}

.tick-enter-from,
.roll-down-leave-to {
  translate: 0 100%;
  opacity: 0;
}

.tick-leave-to,
.roll-down-enter-from {
  translate: 0 -100%;
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .ticker * {
    transition: none !important;
  }
}
</style>
