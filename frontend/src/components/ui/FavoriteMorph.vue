<script setup lang="ts">
import {ref, watch} from 'vue'
import {FavoriteIcon} from '@/utils/icons.ts'

// subject: what it shows; a new one starts still. favorite: undefined until known, so loading never animates.
const {color, favorite, subject} = defineProps<{color: string; favorite?: boolean; subject?: string}>()

const played = ref<'on' | 'off' | 'fill' | null>(null)
const before = ref<string>()
const run = ref(0)
watch(
  () => [subject, favorite, color] as const,
  ([now, saved, shown], [then, wasSaved, was]) => {
    if (now !== then || wasSaved === undefined) {
      played.value = null
      before.value = undefined
      run.value = 0
      return
    }
    if (saved !== wasSaved && saved !== undefined) played.value = saved ? 'on' : 'off'
    else if (shown !== was) played.value = 'fill'
    else return
    before.value = was
    run.value++
  },
)
</script>

<template>
  <span
    :key="run"
    class="morph"
    :class="played && `is-${played}`"
    :style="{'--from': before ?? color, '--to': color, backgroundColor: played === 'fill' ? before : color}"
  >
    <span v-if="played === 'fill'" class="fill"></span>
    <span class="content"><slot /></span>
    <span v-if="played === 'on' || played === 'off'" class="mark">
      <template v-if="played === 'on'">
        <FavoriteIcon class="heart" />
        <span v-for="k in 8" :key="k" class="spark" :style="{'--k': k}"></span>
      </template>
      <template v-else>
        <FavoriteIcon class="heart is-left" />
        <FavoriteIcon class="heart is-right" />
      </template>
    </span>
  </span>
</template>

<style scoped>
.morph {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.content {
  display: contents;
}

.content > :deep(*) {
  position: relative;
}

.fill {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background-color: var(--to);
  animation: fill-in var(--spring-fast-ms) var(--spring-fast) both;
}

.is-fill {
  animation: pop var(--spring-pop-ms) var(--spring-pop);
}

.is-on,
.is-off {
  animation:
    pop var(--spring-pop-ms) var(--spring-pop),
    mark-on 1300ms linear both;
}

.is-off {
  --mark: var(--md-outline);
}

.is-on {
  --mark: var(--md-rose);
}

.is-on > .content > :deep(*),
.is-off > .content > :deep(*) {
  animation:
    shrink 160ms ease-in both,
    grow var(--spring-pop-ms) var(--spring-pop) 1100ms both;
}

.mark {
  position: absolute;
  height: 55%;
  aspect-ratio: 1;
}

.heart {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  fill: #fff;
}

.is-on .heart {
  animation:
    heart-in var(--spring-pop-ms) var(--spring-pop) 100ms both,
    shrink 160ms ease-in 950ms forwards;
}

.is-off .mark {
  animation: appear var(--spring-pop-ms) var(--spring-pop) 100ms both;
}

.heart.is-left,
.heart.is-right {
  transform-origin: 50% 100%;
}

.heart.is-left {
  clip-path: polygon(0 0, 50% 0, 40% 35%, 58% 55%, 46% 75%, 50% 100%, 0 100%);
  animation: break-left 650ms ease-in 500ms both;
}

.heart.is-right {
  clip-path: polygon(50% 0, 100% 0, 100% 100%, 50% 100%, 46% 75%, 58% 55%, 40% 35%);
  animation: break-right 650ms ease-in 500ms both;
}

.spark {
  position: absolute;
  top: 39%;
  left: 39%;
  width: 22%;
  height: 22%;
  border-radius: 50%;
  background: var(--md-rose);
  pointer-events: none;
  animation: spark 520ms linear 100ms both;
}

@keyframes fill-in {
  from {
    clip-path: circle(0 at 50% 50%);
  }
  to {
    clip-path: circle(75% at 50% 50%);
  }
}

@keyframes pop {
  from {
    scale: 0.8;
  }
}

@keyframes mark-on {
  0% {
    background-color: var(--from);
  }
  12%,
  75% {
    background-color: var(--mark);
  }
  100% {
    background-color: var(--to);
  }
}

@keyframes shrink {
  from {
    scale: 1;
  }
  to {
    scale: 0;
  }
}

@keyframes grow {
  from {
    scale: 0;
  }
  to {
    scale: 1;
  }
}

@keyframes heart-in {
  from {
    scale: 0.25;
    rotate: -41deg;
  }
  to {
    scale: 1;
    rotate: 0deg;
  }
}

@keyframes appear {
  from {
    scale: 0.25;
  }
  to {
    scale: 1;
  }
}

@keyframes break-left {
  25% {
    translate: -8% 0;
    rotate: -18deg;
    opacity: 1;
  }
  to {
    translate: -16% 90%;
    rotate: -34deg;
    opacity: 0;
  }
}

@keyframes break-right {
  25% {
    translate: 8% 0;
    rotate: 18deg;
    opacity: 1;
  }
  to {
    translate: 16% 90%;
    rotate: 34deg;
    opacity: 0;
  }
}

@keyframes spark {
  from {
    transform: rotate(calc(var(--k) * 45deg)) translateY(-250%) scale(1);
    opacity: 1;
  }
  to {
    transform: rotate(calc(var(--k) * 45deg)) translateY(-590%) scale(0);
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .morph,
  .morph * {
    animation: none !important;
  }

  .mark {
    display: none;
  }
}
</style>
