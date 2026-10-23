<script setup lang="ts">
defineProps<{ href?: string }>()
const emit = defineEmits<{ open: [] }>()

function follow(e: MouseEvent) {
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
  e.preventDefault()
  emit('open')
}
</script>

<template>
  <a v-if="href" :href="href" class="row-link" draggable="false" @click="follow"><slot/></a>
  <div v-else role="button" tabindex="0" @click="emit('open')" @keydown.enter.prevent="emit('open')"><slot/></div>
</template>

<style scoped>
.row-link {
  text-decoration: none;
  -webkit-touch-callout: none;
}
</style>
