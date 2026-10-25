<script setup lang="ts">
defineProps<{title: string; hint?: string}>()
</script>

<template>
  <section class="admin-card">
    <header>
      <h3>{{ title }}</h3>
      <span v-if="hint">{{ hint }}</span>
    </header>
    <slot />
  </section>
</template>
