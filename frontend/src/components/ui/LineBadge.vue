<script setup lang="ts">
const {height = 28, minWidth = 40, corner = 8} = defineProps<{
  name: string
  color?: string
  height?: number
  minWidth?: number
  corner?: number
}>()
</script>

<template>
  <span
    class="line-badge"
    :style="{
      backgroundColor: color || 'var(--md-outline)',
      minHeight: `${height}px`,
      minWidth: `${minWidth}px`,
      borderRadius: `${corner}px`,
      fontSize: `${height * 0.45}px`,
    }"
  >{{ name }}</span>
</template>

<style scoped>
.line-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  padding: 0 6px;
  color: #fff;
  font-weight: 900;
  line-height: 1;
  white-space: nowrap;
}
</style>
