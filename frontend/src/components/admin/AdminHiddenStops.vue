<script setup lang="ts">
import {defineAsyncComponent, onMounted, ref} from 'vue'
import type {HiddenStation} from '@/components/admin/types.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import {getJSON, putJSON} from '@/composables/useAdmin.ts'
import {refreshNetwork} from '@/utils/network.ts'

const AdminStopsMap = defineAsyncComponent(() => import('@/components/admin/AdminStopsMap.vue'))

const props = defineProps<{city: string}>()

const stations = ref<HiddenStation[]>([])
const focused = ref<HiddenStation | null>(null)
const loadError = ref('')
const saveError = ref('')
const busy = ref(false)

const url = () => `/api/admin/hidden-stops?city=${props.city}`

async function load() {
  try {
    stations.value = await getJSON<HiddenStation[]>(url())
  } catch (err) {
    loadError.value = (err as Error).message
  }
}

async function bringBack(station: HiddenStation) {
  busy.value = true
  saveError.value = ''
  try {
    stations.value = await putJSON<HiddenStation[]>(url(), {stop_ids: station.stop_ids, hidden: false})
    focused.value = null
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
  <p v-else-if="!stations.length" class="admin-note">None</p>
  <template v-else>
    <AdminStopsMap :stations="stations" :focused="focused" @pick="focused = $event" />
    <ul class="admin-picks">
      <li v-for="s in stations" :key="s.stop_ids[0]">
        <button type="button" class="admin-pick state" :class="{'is-focused': s === focused}" @click="focused = s">
          <span class="name">{{ s.name }}</span>
          <span class="lines">
            <LineBadge v-for="l in s.lines" :key="l.id" plain :name="l.name" :color="l.color" />
          </span>
        </button>
        <button type="button" class="btn btn-text" :disabled="busy" @click="bringBack(s)">Add back</button>
      </li>
    </ul>
    <p v-if="saveError" class="admin-note admin-bad">{{ saveError }}</p>
  </template>
</template>

<style scoped>
.name {
  display: block;
}

.lines {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 4px;
}
</style>
