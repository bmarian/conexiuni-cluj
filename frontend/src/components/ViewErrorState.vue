<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import IconBus404 from '@/components/icons/IconBus404.vue'
import IconBack from '@/components/icons/IconBack.vue'

const {t} = useI18n()

withDefaults(defineProps<{
  title?: string
  description?: string
  backLabel?: string
}>(), {
  title: undefined,
  description: undefined,
  backLabel: undefined,
})

const emit = defineEmits<{ back: [] }>()
</script>

<template>
  <div class="ves-root">
    <div class="ves-card">
      <div class="ves-illus">
        <IconBus404/>
      </div>
      <h1 class="ves-title">{{ title ?? t('notFound') }}</h1>
      <p class="ves-desc">{{ description ?? t('notFoundDesc') }}</p>
      <button type="button" class="ves-back-btn" @click="emit('back')">
        <IconBack class="ves-back-icon" aria-hidden="true"/>
        {{ backLabel ?? t('back') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.ves-root {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1 1 auto;
  min-height: 0;
  padding: 2rem 1.5rem 3rem;
}

.ves-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  max-width: 20rem;
  width: 100%;
  gap: 0;
}

.ves-illus {
  width: 100%;
  max-width: 180px;
  margin-bottom: 1.375rem;
  color: #94a3b8;
}

.ves-title {
  font-size: 1.15rem;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 0.5rem;
  line-height: 1.3;
}

.ves-desc {
  font-size: 0.84rem;
  color: #64748b;
  margin: 0 0 1.5rem;
  line-height: 1.65;
}

.ves-back-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.55rem 1.25rem;
  background: #1e293b;
  color: #f8fafc;
  border: none;
  border-radius: 0.625rem;
  font-size: 0.875rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 150ms ease, transform 100ms ease;
}

.ves-back-btn:hover {
  background: #0f172a;
}

.ves-back-btn:active {
  transform: scale(0.97);
}

.ves-back-icon {
  width: 1rem;
  height: 1rem;
  flex-shrink: 0;
}
</style>
