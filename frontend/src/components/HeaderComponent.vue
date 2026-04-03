<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>

<template>
  <header class="sticky top-0 z-50 w-full border-b-2 bg-white/95 backdrop-blur supports-[backdrop-filter]:bg-white/60 dark:bg-slate-900/95 dark:supports-[backdrop-filter]:bg-slate-900/60 border-slate-200 dark:border-slate-800 shadow-sm">
    <div class="container mx-auto flex h-16 items-center justify-between px-4">
      <div class="flex items-center gap-2">
        <RouterLink to="/" class="text-xl font-bold tracking-tight text-slate-900 dark:text-slate-100">
          Conexiuni Cluj
        </RouterLink>
      </div>
      <nav class="flex items-center gap-6 text-sm font-medium">
        <RouterLink
          to="/"
          class="transition-colors hover:text-slate-900/80 text-slate-900 dark:text-slate-100 dark:hover:text-slate-100/80 [&.router-link-active]:font-semibold"
        >
          {{ $t('header.home') }}
        </RouterLink>
        <RouterLink
          to="/lines"
          class="transition-colors hover:text-slate-900/80 text-slate-600 dark:text-slate-400 dark:hover:text-slate-100/80 [&.router-link-active]:text-slate-900 dark:[&.router-link-active]:text-slate-100 [&.router-link-active]:font-semibold"
        >
          {{ $t('header.lines') }}
        </RouterLink>
        <RouterLink
          to="/stations"
          class="transition-colors hover:text-slate-900/80 text-slate-600 dark:text-slate-400 dark:hover:text-slate-100/80 [&.router-link-active]:text-slate-900 dark:[&.router-link-active]:text-slate-100 [&.router-link-active]:font-semibold"
        >
          {{ $t('header.stations') }}
        </RouterLink>
      </nav>
    </div>
  </header>
</template>
