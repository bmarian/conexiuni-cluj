<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import IconBack from '@/components/icons/IconBack.vue'
import IconHome from '@/components/icons/IconHome.vue'

const router = useRouter()
const { t } = useI18n()

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/')
  }
}

const goHome = () => {
  router.push('/')
}
</script>

<template>
  <div class="flex items-center justify-between w-full">
    <button
      @click="goBack"
      class="flex items-center gap-1.5 px-2 py-1.5 rounded-xl text-sm font-semibold text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors duration-150"
      :title="t('back')"
    >
      <IconBack class="w-4 h-4"/>
      <span>{{ t('back') }}</span>
    </button>

    <button
      @click="goHome"
      class="flex items-center gap-1.5 px-2 py-1.5 rounded-xl text-sm font-semibold text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors duration-150"
      :title="t('home')"
    >
      <IconHome class="w-4 h-4"/>
      <span>{{ t('home') }}</span>
    </button>
  </div>
</template>
