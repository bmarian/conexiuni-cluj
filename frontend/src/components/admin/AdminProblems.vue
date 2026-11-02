<script setup lang="ts">
import {computed} from 'vue'
import type {Problem} from '@/components/admin/types.ts'
import {CheckCircleIcon, CloseIcon, WarningAmberIcon} from '@/utils/icons.ts'

type Scoped = Problem & {scope: string}

const props = defineProps<{problems: Scoped[]; place: string}>()
const emit = defineEmits<{dismiss: [problems: Scoped[], dismissed: boolean]}>()

const open = computed(() => props.problems.filter((p) => !p.dismissed))
const dismissed = computed(() => props.problems.filter((p) => p.dismissed))
</script>

<template>
  <ul v-if="open.length" class="problems">
    <li v-for="p in open" :key="`${p.scope}|${p.key}`">
      <WarningAmberIcon class="m-icon" />
      <span>{{ p.text }}</span>
      <button type="button" class="icon-btn dismiss" aria-label="Dismiss" title="Dismiss" @click="emit('dismiss', [p], true)">
        <CloseIcon class="m-icon" />
      </button>
    </li>
  </ul>
  <p v-else class="all-good">
    <CheckCircleIcon class="m-icon" />
    <span>Nothing wrong in {{ place }}</span>
  </p>
  <button v-if="dismissed.length" type="button" class="btn btn-text show" @click="emit('dismiss', dismissed, false)">
    Show {{ dismissed.length }} dismissed
  </button>
</template>

<style scoped>
.problems {
  margin: 0 0 12px;
  padding: 4px 0;
  border-radius: 16px;
  background: var(--md-error-container);
  color: var(--md-on-error-container);
  list-style: none;
}

.problems li {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 40px;
  padding: 0 0 0 16px;
  font-size: 14px;
  line-height: 20px;
  letter-spacing: 0.25px;
  overflow-wrap: anywhere;
}

.problems li > span {
  flex: 1;
  min-width: 0;
  padding-block: 8px;
}

.problems .m-icon {
  width: 20px;
  height: 20px;
}

.dismiss {
  color: inherit;
}

.all-good {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0 0 12px;
  padding: 16px;
  border-radius: 16px;
  background: var(--md-surface-container);
  font-size: 14px;
  line-height: 20px;
  color: var(--md-on-surface-variant);
}

.show {
  margin: -8px 0 4px 4px;
}
</style>
