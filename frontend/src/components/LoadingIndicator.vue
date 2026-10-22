<script setup lang="ts">
defineProps<{ text: string }>()
</script>

<template>
  <div class="loading-indicator">
    <div class="bus-loader-container">
      <svg class="w-12 h-12 text-sky-500 animate-bus-run" viewBox="0 0 24 24"
           fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round"
              d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 00-10.026 0 1.106 1.106 0 00-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12"/>
      </svg>
    </div>
    <p class="loading-text animate-pulse">{{ text }}</p>
  </div>
</template>

<style scoped>
.loading-indicator {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem 1rem;
  gap: 1rem;
  background: #f8fafc;
  border-radius: 1rem;
  border: 1.5px solid #e2e8f0;
}

.dark .loading-indicator {
  background: #1e293b;
  border-color: #334155;
}

.bus-loader-container {
  overflow: hidden;
  width: 100px;
  display: flex;
  justify-content: center;
}

.animate-bus-run {
  animation: bus-run 1.2s infinite linear;
}

.loading-text {
  font-size: 0.875rem;
  font-weight: 600;
  color: #64748b;
}

.dark .loading-text {
  color: #94a3b8;
}
</style>

<style>
@keyframes bus-run {
  0% { transform: translateX(-50px); opacity: 0; }
  20% { opacity: 1; }
  80% { opacity: 1; }
  100% { transform: translateX(50px); opacity: 0; }
}
</style>
