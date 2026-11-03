<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import AdminSub from '@/components/admin/AdminSub.vue'
import SwitchToggle from '@/components/ui/SwitchToggle.vue'

type AdminCity = {slug: string; name: string; enabled: boolean; provider: string}

const emit = defineEmits<{changed: []}>()

const list = ref<AdminCity[]>([])
const loadError = ref('')
const busy = ref('')
const switchError = ref('')

const byName = new Intl.Collator(undefined, {sensitivity: 'base'})

// Grouped by the source the data is credited to, uncredited cities last.
const groups = computed(() => {
  const sorted = [...list.value].sort((a, b) => Number(!a.provider) - Number(!b.provider) || byName.compare(a.provider, b.provider) || byName.compare(a.name, b.name))
  const out: Array<{name: string; cities: AdminCity[]}> = []
  for (const city of sorted) {
    const name = city.provider || 'Other sources'
    if (out.at(-1)?.name !== name) out.push({name, cities: []})
    out.at(-1)!.cities.push(city)
  }
  return out
})

async function load() {
  try {
    const res = await fetch('/api/admin/cities', {credentials: 'same-origin'})
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    list.value = (await res.json()) as AdminCity[]
    loadError.value = ''
  } catch (err) {
    loadError.value = (err as Error).message
  }
}

async function setEnabled(city: AdminCity, enabled: boolean) {
  if (busy.value || city.enabled === enabled) return
  busy.value = city.slug
  switchError.value = ''
  try {
    const res = await fetch(`/api/admin/cities/${city.slug}`, {
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      credentials: 'same-origin',
      body: JSON.stringify({enabled}),
    })
    const body = await res.json().catch(() => null)
    if (res.status === 409) throw new Error('Last city on')
    if (!res.ok) throw new Error(body?.error ?? `HTTP ${res.status}`)
    list.value = body as AdminCity[]
    emit('changed')
  } catch (err) {
    switchError.value = (err as Error).message
  } finally {
    busy.value = ''
  }
}

onMounted(load)
</script>

<template>
  <p v-if="loadError" class="admin-note admin-bad">Could not load ({{ loadError }})</p>
  <template v-else>
    <section v-for="group in groups" :key="group.name">
      <AdminSub :text="group.name" />
      <ul class="cities">
        <li v-for="city in group.cities" :key="city.slug">
          <span class="text-body-large truncate" :class="{'text-on-surface-variant': !city.enabled}">{{ city.name }}</span>
          <SwitchToggle :checked="city.enabled" :disabled="!!busy" :aria-label="city.name" @change="setEnabled(city, $event)" />
        </li>
      </ul>
    </section>
  </template>
  <p v-if="switchError" class="admin-note admin-bad">{{ switchError }}</p>
</template>

<style scoped>
.cities {
  margin: 0;
  padding: 0;
  list-style: none;
}

.cities li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 56px;
  padding: 0 16px;
}
</style>
