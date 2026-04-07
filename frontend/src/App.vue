<script setup lang="ts">
import MapComponent from "@/components/MapComponent.vue";
</script>

<template>
  <main class="overflow-hidden min-h-screen min-w-screen max-h-screen max-w-screen bg-slate-100 text-slate-900 dark:bg-slate-900 dark:text-slate-100">
    <RouterView class="container px-4 mx-auto"/>
    <MapComponent class="min-h-screen min-w-screen" />
  </main>
</template>
