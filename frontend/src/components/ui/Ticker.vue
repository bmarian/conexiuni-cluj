<script setup lang="ts">
defineProps<{ text: string }>()
</script>

<template>
  <span class="ticker">
    <Transition name="tick">
      <span :key="text">{{ text }}</span>
    </Transition>
  </span>
</template>

<style scoped>
.ticker {
  display: inline-grid;
  white-space: nowrap;
}

.ticker > * {
  grid-area: 1 / 1;
}

.tick-enter-active {
  transition: translate var(--spring-slide-ms) var(--spring-slide), opacity 200ms linear;
}

.tick-leave-active {
  transition: translate 300ms ease-in, opacity 150ms linear;
}

.tick-enter-from {
  translate: 0 100%;
  opacity: 0;
}

.tick-leave-to {
  translate: 0 -100%;
  opacity: 0;
}
</style>
