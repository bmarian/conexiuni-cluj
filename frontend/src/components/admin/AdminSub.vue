<script setup lang="ts">
defineProps<{text: string; unit?: string}>()
</script>

<template>
  <h4 class="admin-sub">
    {{ text }}<span v-if="unit" class="unit">{{ unit }}</span>
  </h4>
</template>
