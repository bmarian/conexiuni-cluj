<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useKbdLayer} from '@/composables/useKeyboardNav.ts'
import {KeyboardIcon, ScheduleIcon} from '@/utils/icons.ts'

type Part = 'hour' | 'minute'

const DIAL = 256
const OUTER = 101
const INNER = 69
const INNER_REACH = 74
const TWELVE = Array.from({length: 12}, (_, i) => i)
const STEPS: Record<string, number> = {k: 1, ArrowUp: 1, j: -1, ArrowDown: -1}

const props = defineProps<{time: string}>()
const emit = defineEmits<{pick: [time: string]}>()
const open = defineModel<boolean>('open', {required: true})

const {t} = useI18n()
const panel = ref<HTMLElement | null>(null)
const dial = ref<HTMLElement | null>(null)
const hour = ref(0)
const minute = ref(0)
const part = ref<Part>('hour')
const typing = ref(false)
const draft = ref<{part: Part; text: string} | null>(null)
const pressed = ref(false)
const dragging = ref(false)
const turn = ref(0)

useKbdLayer(open, {el: () => panel.value, close: cancel, capture: true})

const pad = (n: number) => String(n).padStart(2, '0')
const shown = (p: Part) => (draft.value?.part === p ? draft.value.text : pad(p === 'hour' ? hour.value : minute.value))
const angle = computed(() => (part.value === 'hour' ? (hour.value % 12) * 30 : minute.value * 6))
const reach = computed(() => (part.value === 'hour' && hour.value >= 12 ? INNER : OUTER))

const marks = computed(() => {
  const rings =
    part.value === 'hour'
      ? [TWELVE.map((h) => ({value: h, at: h, r: OUTER})), TWELVE.map((h) => ({value: h + 12, at: h, r: INNER}))]
      : [TWELVE.map((i) => ({value: i * 5, at: i, r: OUTER}))]
  return rings.flat().map(({value, at, r}) => {
    const rad = (at * Math.PI) / 6
    return {value, x: DIAL / 2 + r * Math.sin(rad), y: DIAL / 2 - r * Math.cos(rad)}
  })
})

watch(angle, (to) => {
  turn.value += ((((to - turn.value) % 360) + 540) % 360) - 180
})

watch(open, async (isOpen) => {
  if (!isOpen) return
  const [h, m] = props.time.split(':').map(Number)
  hour.value = h || 0
  minute.value = m || 0
  part.value = 'hour'
  typing.value = false
  draft.value = null
  turn.value = (hour.value % 12) * 30
  await nextTick()
  focusPart('hour')
})

function inputOf(p: Part) {
  return panel.value?.querySelector<HTMLInputElement>(`[data-part="${p}"]`)
}

function focusPart(p: Part) {
  part.value = p
  draft.value = null
  const input = inputOf(p)
  input?.focus({preventScroll: true})
  input?.select()
}

function typed(p: Part) {
  return draft.value?.part === p ? draft.value.text : ''
}

function onType(e: InputEvent) {
  if (e.inputType.startsWith('delete')) {
    draft.value = {part: part.value, text: typed(part.value).slice(0, -1)}
    return
  }
  for (const digit of (e.data ?? '').replace(/\D/g, '')) typeDigit(digit)
}

function typeDigit(digit: string) {
  const p = part.value
  const text = typed(p) + digit
  const max = p === 'hour' ? 23 : 59
  if (p === 'hour') hour.value = Math.min(Number(text), max)
  else minute.value = Math.min(Number(text), max)
  if (text.length < 2 && Number(text) * 10 <= max) {
    draft.value = {part: p, text}
    return
  }
  if (p === 'hour') return focusPart('minute')
  draft.value = null
  inputOf(p)?.select()
}

function nudge(by: number) {
  draft.value = null
  if (part.value === 'hour') hour.value = (hour.value + by + 24) % 24
  else minute.value = (minute.value + by + 60) % 60
}

function aim(e: PointerEvent, snap: boolean) {
  const box = dial.value!.getBoundingClientRect()
  const x = e.clientX - box.left - box.width / 2
  const y = e.clientY - box.top - box.height / 2
  const deg = ((Math.atan2(x, -y) * 180) / Math.PI + 360) % 360
  if (part.value === 'hour') {
    const h = Math.round(deg / 30) % 12
    hour.value = Math.hypot(x, y) * (DIAL / box.width) < INNER_REACH ? h + 12 : h
  } else {
    minute.value = snap ? (Math.round(deg / 30) * 5) % 60 : Math.round(deg / 6) % 60
  }
}

function onDialDown(e: PointerEvent) {
  dial.value!.setPointerCapture(e.pointerId)
  pressed.value = true
  draft.value = null
  aim(e, true)
}

function onDialMove(e: PointerEvent) {
  if (!pressed.value) return
  dragging.value = true
  aim(e, false)
}

function onDialUp() {
  if (!pressed.value) return
  pressed.value = false
  dragging.value = false
  if (part.value === 'hour') focusPart('minute')
  else inputOf('minute')?.focus({preventScroll: true})
}

function toggleTyping() {
  typing.value = !typing.value
  void nextTick(() => focusPart(part.value))
}

function confirm() {
  emit('pick', `${pad(hour.value)}:${pad(minute.value)}`)
  open.value = false
}

function cancel() {
  open.value = false
}

function onKey(e: KeyboardEvent) {
  e.stopPropagation()
  if (e.key === 'Escape') {
    e.preventDefault()
    cancel()
  } else if (e.key === 'Enter' && !(e.target instanceof HTMLButtonElement)) {
    e.preventDefault()
    confirm()
  } else if (STEPS[e.key] && !e.shiftKey) {
    e.preventDefault()
    nudge(STEPS[e.key]!)
  } else if (e.key === 'h' || e.key === 'l') {
    e.preventDefault()
    focusPart(e.key === 'h' ? 'hour' : 'minute')
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="scrim">
      <div v-if="open" class="scrim" @pointerdown.self="cancel">
        <div
          ref="panel"
          class="time-dialog"
          :class="{'is-typing': typing}"
          role="dialog"
          aria-modal="true"
          aria-labelledby="time-dialog-title"
          @keydown="onKey"
        >
          <p id="time-dialog-title" class="headline">{{ t(typing ? 'timePickerEnter' : 'timePickerTitle') }}</p>
          <div class="display">
            <input
              v-for="p in ['hour', 'minute'] as const"
              :key="p"
              :data-part="p"
              class="part"
              :class="{'is-on': part === p}"
              :value="shown(p)"
              type="text"
              :inputmode="typing ? 'numeric' : 'none'"
              autocomplete="off"
              :aria-label="t(p === 'hour' ? 'timePickerHour' : 'timePickerMinute')"
              @focus="part = p"
              @beforeinput.prevent="onType($event as InputEvent)"
              @blur="draft = null"
            />
          </div>
          <div v-if="typing" class="part-labels">
            <span>{{ t('timePickerHour') }}</span>
            <span>{{ t('timePickerMinute') }}</span>
          </div>
          <div
            v-else
            ref="dial"
            class="dial"
            @mousedown.prevent
            @pointerdown="onDialDown"
            @pointermove="onDialMove"
            @pointerup="onDialUp"
            @pointercancel="onDialUp"
          >
            <Transition name="face" mode="out-in">
              <div :key="part" class="face">
                <span v-for="mark in marks" :key="mark.value" class="mark" :style="{translate: `${mark.x - 24}px ${mark.y - 24}px`}">{{
                  pad(mark.value)
                }}</span>
              </div>
            </Transition>
            <span class="hand-center"></span>
            <span class="hand" :class="{'is-dragging': dragging}" :style="{rotate: `${turn}deg`, '--reach': `${reach}px`}">
              <span class="hand-line"></span>
              <span class="hand-knob">
                <span class="knob-face" :style="{rotate: `${-turn}deg`}">
                  <span v-for="mark in marks" :key="mark.value" class="mark" :style="{translate: `${mark.x - 24}px ${mark.y - 24}px`}">{{
                    pad(mark.value)
                  }}</span>
                </span>
              </span>
            </span>
          </div>
          <div class="actions">
            <button
              type="button"
              class="icon-btn"
              :aria-label="t(typing ? 'timePickerDial' : 'timePickerKeyboard')"
              :title="t(typing ? 'timePickerDial' : 'timePickerKeyboard')"
              @click="toggleTyping"
            >
              <component :is="typing ? ScheduleIcon : KeyboardIcon" class="m-icon" />
            </button>
            <span class="flex-1"></span>
            <button type="button" class="btn btn-text" @click="cancel">{{ t('cancel') }}</button>
            <button type="button" class="btn btn-text" @click="confirm">{{ t('ok') }}</button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.scrim {
  position: fixed;
  inset: 0;
  z-index: 10000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: var(--md-scrim);
}

.scrim-enter-active,
.scrim-leave-active {
  transition: opacity 105ms linear;
}

.scrim-enter-from,
.scrim-leave-to {
  opacity: 0;
}

.time-dialog {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px 24px 20px;
  border-radius: 28px;
  background: var(--md-surface-container-high);
  color: var(--md-on-surface);
  box-shadow: var(--md-elevation-3);
  animation: dialog-in var(--spring-fast-ms) var(--spring-fast);
}

@keyframes dialog-in {
  from {
    scale: 0.9;
    opacity: 0;
  }
}

.headline {
  align-self: stretch;
  margin-bottom: 20px;
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  color: var(--md-on-surface-variant);
}

.display {
  display: grid;
  grid-template-columns: 96px 96px;
  column-gap: 24px;
  position: relative;
}

.display::after {
  content: ':';
  position: absolute;
  left: 50%;
  top: 50%;
  translate: -50% -50%;
  font-size: 57px;
  line-height: 64px;
}

.part {
  width: 96px;
  height: 80px;
  padding: 0;
  border: 0;
  border-radius: 8px;
  outline: none;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface);
  font: inherit;
  font-size: 57px;
  line-height: 64px;
  letter-spacing: -0.25px;
  font-variant-numeric: tabular-nums;
  text-align: center;
  caret-color: transparent;
  cursor: pointer;
  transition:
    background-color 130ms linear,
    color 130ms linear;
}

.part::selection {
  background: transparent;
}

.part.is-on {
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
}

.is-typing .part {
  cursor: text;
  caret-color: var(--md-primary);
}

.is-typing .part.is-on {
  box-shadow: inset 0 0 0 2px var(--md-primary);
}

.part-labels {
  display: grid;
  grid-template-columns: 96px 96px;
  column-gap: 24px;
  margin-top: 7px;
  font-size: 12px;
  line-height: 16px;
  letter-spacing: 0.4px;
  color: var(--md-on-surface-variant);
}

.dial {
  position: relative;
  flex-shrink: 0;
  width: 256px;
  height: 256px;
  margin-top: 36px;
  border-radius: 50%;
  background: var(--md-surface-container-highest);
  touch-action: none;
  user-select: none;
  cursor: pointer;
}

.hand-center {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 8px;
  height: 8px;
  margin: -4px;
  border-radius: 50%;
  background: var(--md-primary);
}

.hand {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 0;
  height: 0;
  transition: rotate var(--spring-fast-ms) var(--spring-fast);
}

.hand-line,
.hand-knob,
.knob-face {
  position: absolute;
  transition:
    height var(--spring-fast-ms) var(--spring-fast),
    bottom var(--spring-fast-ms) var(--spring-fast),
    top var(--spring-fast-ms) var(--spring-fast),
    rotate var(--spring-fast-ms) var(--spring-fast);
}

.hand-line {
  background: var(--md-primary);
  left: -1px;
  bottom: 0;
  width: 2px;
  height: var(--reach);
}

.hand-knob {
  left: -24px;
  bottom: calc(var(--reach) - 24px);
  width: 48px;
  height: 48px;
  overflow: hidden;
  border-radius: 50%;
  background: var(--md-primary);
}

.knob-face {
  left: -104px;
  top: calc(var(--reach) - 104px);
  width: 256px;
  height: 256px;
}

.knob-face .mark {
  color: var(--md-on-primary);
}

.hand.is-dragging,
.hand.is-dragging * {
  transition: none;
}

.face {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.face-enter-active,
.face-leave-active {
  transition: opacity 105ms linear;
}

.face-enter-from,
.face-leave-to {
  opacity: 0;
}

.mark {
  position: absolute;
  left: 0;
  top: 0;
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  font-size: 16px;
  line-height: 24px;
  letter-spacing: 0.5px;
  color: var(--md-on-surface);
}

.actions {
  align-self: stretch;
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 24px;
}

.actions .icon-btn {
  margin-left: -12px;
}
</style>
