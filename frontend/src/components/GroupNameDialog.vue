<script setup lang="ts">
import {nextTick, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useKbdLayer} from '@/composables/useKeyboardNav.ts'
import StationLines from '@/components/rows/StationLines.vue'
import type {NetworkRoute} from '@/utils/network.ts'

const open = defineModel<boolean>('open', {required: true})
const props = defineProps<{routes: NetworkRoute[]; name?: string}>()
const emit = defineEmits<{save: [name: string]}>()

const {t} = useI18n()
const panel = ref<HTMLElement | null>(null)
const field = ref<HTMLInputElement | null>(null)
const value = ref('')

watch(open, (isOpen) => {
  if (!isOpen) return
  value.value = props.name ?? ''
  void nextTick(() => field.value?.focus())
})

const close = () => {
  open.value = false
}

function save() {
  emit('save', value.value.trim())
  close()
}

useKbdLayer(open, {el: () => panel.value, close})
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="dialog-scrim" @click.self="close">
      <form ref="panel" class="name-dialog dialog dialog-small" role="dialog" :aria-label="t('groupNameTitle')" @submit.prevent="save">
        <h2 class="dialog-title">{{ t('groupNameTitle') }}</h2>
        <StationLines :routes="routes" />
        <label class="field">
          <span class="field-label">{{ t('groupName') }}</span>
          <input ref="field" v-model="value" class="field-input" type="text" maxlength="40" />
        </label>
        <div class="dialog-actions pt-2">
          <button type="button" class="btn btn-text" @click="close">{{ t('cancel') }}</button>
          <button type="submit" class="btn btn-text">{{ t('save') }}</button>
        </div>
      </form>
    </div>
  </Teleport>
</template>

<style scoped>
.name-dialog {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 16px;
  border-radius: 4px 4px 0 0;
  background: var(--md-surface-container-highest);
  box-shadow: inset 0 -2px 0 var(--md-primary);
}

.field-label {
  font-size: 12px;
  line-height: 16px;
  color: var(--md-primary);
}

.field-input {
  border: 0;
  outline: 0;
  padding: 0;
  background: transparent;
  color: var(--md-on-surface);
  font: inherit;
  font-size: 16px;
  line-height: 24px;
}
</style>
