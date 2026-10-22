<script setup lang="ts">
defineProps<{ text?: string }>()
</script>

<template>
  <div class="loading-row" role="status">
    <span class="indicator" aria-hidden="true"><span class="shape"></span></span>
    <p v-if="text" class="text-body-medium text-on-surface-variant">{{ text }}</p>
  </div>
</template>

<style scoped>
.loading-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 24px;
}

.indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
}

.shape {
  width: 38px;
  height: 38px;
  background: var(--md-primary);
  animation: morph 2.6s cubic-bezier(0.42, 0, 0.2, 1) infinite, turn 1.3s linear infinite;
}

@keyframes morph {
  0%, 100% {
    border-radius: 50%;
    scale: 0.9;
  }
  20% {
    border-radius: 30%;
    scale: 1;
  }
  40% {
    border-radius: 62% 38% 55% 45% / 45% 55% 38% 62%;
    scale: 0.95;
  }
  60% {
    border-radius: 18% 50%;
    scale: 1;
  }
  80% {
    border-radius: 40% 60% 40% 60% / 60% 40% 60% 40%;
    scale: 0.92;
  }
}

@keyframes turn {
  to {
    rotate: 360deg;
  }
}

@media (prefers-reduced-motion: reduce) {
  .shape {
    animation-duration: 6s, 6s;
  }
}
</style>
