<script setup lang="ts">
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import IconNotFoundFace from '@/components/icons/IconNotFoundFace.vue'

const router = useRouter()
const {t} = useI18n()
</script>

<template>
  <div class="not-found-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col items-center justify-center gap-5">
    <div class="w-16 h-16 rounded-2xl bg-slate-100 dark:bg-slate-800 flex items-center justify-center">
      <IconNotFoundFace class="w-8 h-8 text-slate-400 dark:text-slate-500"/>
    </div>
    <div class="text-center">
      <h1 class="text-xl font-black text-slate-800 dark:text-white mb-1">{{ t('notFound') }}</h1>
      <p class="text-sm text-slate-500 dark:text-slate-400 max-w-xs leading-relaxed">{{ t('notFoundDesc') }}</p>
    </div>
    <button
      @click="router.push({ name: 'home' })"
      class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 text-sm font-bold hover:opacity-90 transition-opacity"
    >
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/>
      </svg>
      {{ t('home') }}
    </button>
  </div>
</template>

<style scoped>
.not-found-container {
  padding: 2rem 1.5rem;
  height: 100%;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}
</style>
