<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import AdminLinesMap from '@/components/admin/AdminLinesMap.vue'
import AdminSub from '@/components/admin/AdminSub.vue'
import type {AdminLine, HiddenLines} from '@/components/admin/types.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import {getJSON, putJSON} from '@/composables/useAdmin.ts'
import {ago} from '@/utils/adminFormat.ts'
import {refreshNetwork} from '@/utils/network.ts'

const props = defineProps<{city: string}>()

const lines = ref<HiddenLines | null>(null)
const focused = ref<string | null>(null)
const loadError = ref('')
const saveError = ref('')
const busy = ref(false)

const url = () => `/api/admin/hidden-lines?city=${props.city}`
const lastBus = (line: AdminLine) => (line.last_run ? `Last bus ${ago(line.last_run * 1000, Date.now())}` : 'No bus seen')

const groups = computed(() =>
  lines.value
    ? [
        {label: '', list: lines.value.hidden, hide: false, action: 'Add back'},
        {label: 'No bus for a week', list: lines.value.idle, hide: true, action: 'Hide'},
      ].filter((g) => g.list.length)
    : [],
)
const drawn = computed(() => (lines.value ? [...lines.value.hidden, ...lines.value.idle].filter((l) => l.shapes.length) : []))

async function load() {
  try {
    lines.value = await getJSON<HiddenLines>(url())
  } catch (err) {
    loadError.value = (err as Error).message
  }
}

async function save(line: AdminLine, hidden: boolean) {
  busy.value = true
  saveError.value = ''
  try {
    lines.value = await putJSON<HiddenLines>(url(), {names: [line.name], hidden})
    void refreshNetwork(true)
  } catch (err) {
    saveError.value = (err as Error).message
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <p v-if="loadError" class="admin-note admin-bad">Could not load ({{ loadError }})</p>
  <p v-else-if="!lines" class="admin-note">Loading…</p>
  <template v-else>
    <AdminLinesMap v-if="drawn.length" :lines="drawn" :focused="focused" @pick="focused = $event" />
    <p v-if="!lines.hidden.length" class="admin-note">None hidden</p>
    <template v-for="g in groups" :key="g.action">
      <AdminSub v-if="g.label" :text="g.label" />
      <ul class="admin-picks">
        <li v-for="l in g.list" :key="l.name">
          <button type="button" class="admin-pick line state" :class="{'is-focused': l.name === focused}" :title="l.long_name" @click="focused = l.name">
            <LineBadge plain :name="l.name" :color="l.color" />
            <span class="truncate">{{ lastBus(l) }}</span>
          </button>
          <button type="button" class="btn btn-text" :disabled="busy" @click="save(l, g.hide)">{{ g.action }}</button>
        </li>
      </ul>
    </template>
    <p v-if="saveError" class="admin-note admin-bad">{{ saveError }}</p>
  </template>
</template>

<style scoped>
.line {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>
