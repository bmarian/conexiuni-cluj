<script setup lang="ts">
import MapComponent from "@/components/MapComponent.vue";
</script>

<template>
  <main class="bg-slate-100 text-slate-900 dark:bg-slate-900 dark:text-slate-100 lg:flex">
    <MapComponent class="h-[40vh] lg:min-h-screen w-screen lg:w-[70vw]" />
    <div class="bg-slate-100 dark:bg-slate-900 relative h-[60vh] overflow-auto lg:min-h-screen w-screen lg:w-[30vw]">
      <RouterView/>
    </div>
  </main>
</template>
