<script setup lang="ts">
import {ref, watch, type Component} from 'vue'

const props = defineProps<{icon: Component; on: boolean}>()

const played = ref<'on' | 'off' | null>(null)
const run = ref(0)
watch(
  () => props.on,
  (on) => {
    played.value = on ? 'on' : 'off'
    run.value++
  },
)
</script>

<template>
  <span class="pop-icon" :key="run" :class="played && `is-${played}`">
    <component :is="icon" class="m-icon size-full" />
    <template v-if="played === 'on'">
      <span v-for="k in 8" :key="k" class="spark" :style="{'--k': k}"></span>
    </template>
  </span>
</template>

<style scoped>
.pop-icon {
  position: relative;
  display: inline-flex;
  flex-shrink: 0;
}

.is-on > .m-icon {
  animation: pop-on var(--spring-pop-ms) var(--spring-pop);
}

.is-off > .m-icon {
  animation: pop-off var(--spring-pop-ms) var(--spring-pop);
}

.spark {
  position: absolute;
  top: 39%;
  left: 39%;
  width: 22%;
  height: 22%;
  border-radius: 50%;
  background: currentColor;
  pointer-events: none;
  animation: spark calc(450ms * var(--pace-long)) linear forwards;
}

@keyframes pop-on {
  from {
    scale: 0.25;
    rotate: -41deg;
  }
}

@keyframes pop-off {
  from {
    scale: 0.8;
    rotate: -11deg;
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
  .pop-icon > * {
    animation: none !important;
  }

  .spark {
    display: none;
  }
}
</style>
