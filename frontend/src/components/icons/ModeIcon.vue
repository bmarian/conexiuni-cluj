<script setup lang="ts">
import type {Mode} from '@/utils/modes.ts'
import {MODE_ICON} from '@/utils/modeIcons.ts'

defineProps<{ mode: Mode }>()
</script>

<template>
  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" v-html="MODE_ICON[mode]"></svg>
</template>
