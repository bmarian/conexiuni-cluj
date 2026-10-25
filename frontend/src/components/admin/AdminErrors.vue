<script setup lang="ts">
import {computed, ref} from 'vue'
import type {ErrorDetail} from '@/components/admin/types.ts'
import {ago, num} from '@/utils/adminFormat.ts'

const SHOWN = 5

const props = defineProps<{errors: (ErrorDetail & {city?: string})[]; clock: number}>()

const all = ref(false)
const shown = computed(() => (all.value ? props.errors : props.errors.slice(0, SHOWN)))
const method = (e: ErrorDetail) => e.endpoint.split(' ')[0]
</script>

<template>
  <ul v-if="errors.length" class="admin-log">
    <li v-for="e in shown" :key="`${e.city}|${e.endpoint}|${e.status}|${e.text}`">
      <span class="admin-log-head">
        <template v-if="e.city">{{ e.city }} · </template>{{ ago(e.last_at, clock) }} · {{ e.status
        }}<template v-if="e.count > 1"> · {{ num(e.count) }}×</template>
      </span>
      <code>{{ method(e) }} {{ e.path }}</code>
      <code class="admin-bad">{{ e.text || '–' }}</code>
    </li>
  </ul>
  <p v-else class="admin-note">None</p>
  <button v-if="errors.length > SHOWN" type="button" class="btn btn-text admin-more" @click="all = !all">
    {{ all ? 'Fewer' : `All ${errors.length}` }}
  </button>
</template>
