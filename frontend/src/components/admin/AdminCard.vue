<script setup lang="ts">
defineProps<{title: string; scope?: string; app?: boolean; hint?: string}>()
</script>

<template>
  <section class="admin-card">
    <header>
      <h3>{{ title }}</h3>
      <span v-if="scope || hint">
        <span v-if="scope" class="scope" :class="{'is-app': app}">{{ scope }}</span>
        <template v-if="scope && hint"> · </template>{{ hint }}
      </span>
    </header>
    <slot />
  </section>
</template>
