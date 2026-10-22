<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {DirectionsBusIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const router = useRouter()

withDefaults(defineProps<{
  title?: string
  description?: string
  backLabel?: string
}>(), {
  title: undefined,
  description: undefined,
  backLabel: undefined,
})

const goHome = () => {
  router.replace({name: 'home'})
}
</script>

<template>
  <div class="error-state" data-kbd-section="error" data-kbd-entry="1">
    <DirectionsBusIcon class="m-icon size-16 text-outline-variant"/>
    <h1 class="text-title-medium font-bold">{{ title ?? t('notFound') }}</h1>
    <p class="text-body-medium text-on-surface-variant text-center">{{ description ?? t('notFoundDesc') }}</p>
    <button type="button" class="btn btn-filled" data-kbd-item="error-home" @click="goHome">{{ backLabel ?? t('home') }}</button>
  </div>
</template>

<style scoped>
.error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 32px;
}
</style>
