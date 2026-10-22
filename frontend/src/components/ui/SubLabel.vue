<script setup lang="ts">
defineProps<{ text: string }>()
</script>

<template>
  <h3 class="text-label-medium text-on-surface-variant px-4 py-1">{{ text }}</h3>
</template>
