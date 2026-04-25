<script setup lang="ts">
withDefaults(defineProps<{
  color: string
  pupilColor: string
  lookRight?: boolean
}>(), {
  lookRight: false,
})
</script>

<template>
  <svg viewBox="0 0 12 16" xmlns="http://www.w3.org/2000/svg">
    <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" :fill="color"/>
    <circle cx="3.5" cy="7" r="1.4" fill="white"/>
    <circle cx="8.5" cy="7" r="1.4" fill="white"/>
    <circle :cx="lookRight ? 4.1 : 3.0" cy="7.5" r="0.65" :fill="pupilColor"/>
    <circle :cx="lookRight ? 9.1 : 8.0" cy="7.5" r="0.65" :fill="pupilColor"/>
  </svg>
</template>
