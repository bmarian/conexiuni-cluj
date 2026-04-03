<script setup lang="ts">
import HeaderComponent from '@/components/HeaderComponent.vue'
</script>

<template>
  <HeaderComponent />
  <main class="min-h-screen bg-slate-100 text-slate-900 dark:bg-slate-900 dark:text-slate-100">
    <RouterView class="container px-4 mx-auto"/>
  </main>
</template>
