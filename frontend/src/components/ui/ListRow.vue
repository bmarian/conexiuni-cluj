<script setup lang="ts">
import RowLink from '@/components/ui/RowLink.vue'
import {CloseIcon, KeyboardArrowRightIcon} from '@/utils/icons.ts'

defineProps<{
  title: string
  titlePrefix?: string
  kicker?: string
  subtitle?: string
  trailing?: string
  removeLabel?: string
  chevron?: boolean
  busy?: boolean
  href?: string
}>()
const emit = defineEmits<{ open: [], remove: [] }>()
</script>

<template>
  <RowLink class="list-row state squish" :class="{ 'is-busy': busy }" :href="href" @open="emit('open')">
    <slot name="icon"/>
    <div class="flex-1 min-w-0">
      <p v-if="kicker" class="kicker">{{ kicker }}</p>
      <p class="text-body-large truncate">
        <span v-if="titlePrefix" class="text-on-surface-variant">{{ titlePrefix }}</span>{{ title }}
      </p>
      <p v-if="subtitle" class="text-body-small text-on-surface-variant truncate">{{ subtitle }}</p>
      <div v-if="$slots.lines" class="pt-1">
        <slot name="lines"/>
      </div>
    </div>
    <span v-if="trailing" class="text-label-medium text-on-surface-variant shrink-0">{{ trailing }}</span>
    <button
      v-if="removeLabel"
      type="button"
      class="icon-btn text-outline"
      :aria-label="removeLabel"
      :title="removeLabel"
      @click.stop="emit('remove')"
    >
      <CloseIcon class="m-icon size-[18px]"/>
    </button>
    <KeyboardArrowRightIcon v-else-if="chevron" class="m-icon size-[18px] text-outline mr-3"/>
  </RowLink>
</template>

<style scoped>
.list-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 56px;
  padding: 6px 4px 6px 16px;
  color: var(--md-on-surface);
  cursor: pointer;
}

.list-row.is-busy {
  opacity: 0.6;
  pointer-events: none;
}

.kicker {
  font-size: 11px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  text-transform: uppercase;
  color: var(--md-on-surface-variant);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
