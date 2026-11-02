<script setup lang="ts">
import {computed, ref} from 'vue'
import type {LoggedProblem} from '@/components/admin/types.ts'
import {ago, num} from '@/utils/adminFormat.ts'

const SHOWN = 5

const props = defineProps<{failures: LoggedProblem[]; clock: number}>()

const all = ref(false)
const shown = computed(() => (all.value ? props.failures : props.failures.slice(0, SHOWN)))
</script>

<template>
  <ul v-if="failures.length" class="admin-log">
    <li v-for="p in shown" :key="p.text">
      <span class="admin-log-head">{{ ago(p.last, clock) }}<template v-if="p.count > 1"> · {{ num(p.count) }}×</template></span>
      <code>{{ p.text }}</code>
    </li>
  </ul>
  <p v-else class="admin-note">None</p>
  <button v-if="failures.length > SHOWN" type="button" class="btn btn-text admin-more" @click="all = !all">
    {{ all ? 'Fewer' : `All ${failures.length}` }}
  </button>
</template>
