<script setup lang="ts">
import {onMounted, ref} from 'vue'

type AdminCity = {slug: string; name: string; enabled: boolean}

const emit = defineEmits<{changed: []}>()

const list = ref<AdminCity[]>([])
const loadError = ref('')
const busy = ref('')
const switchError = ref('')

async function load() {
  try {
    const res = await fetch('/api/admin/cities', {credentials: 'same-origin'})
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    list.value = await res.json() as AdminCity[]
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
  <p v-if="loadError" class="bad">Could not load ({{ loadError }})</p>
  <dl v-else class="kv">
    <div v-for="city in list" :key="city.slug">
      <dt class="name" :class="{ off: !city.enabled }">{{ city.name }}</dt>
      <dd>
        <div class="segmented" role="radiogroup" :aria-label="city.name">
          <button
            v-for="on in [true, false]"
            :key="String(on)"
            type="button"
            role="radio"
            :class="{ 'is-selected': city.enabled === on }"
            :aria-checked="city.enabled === on"
            :disabled="!!busy"
            @click="setEnabled(city, on)"
          >
            {{ on ? 'On' : 'Off' }}
          </button>
        </div>
      </dd>
    </div>
  </dl>
  <p v-if="switchError" class="bad hint">{{ switchError }}</p>
</template>

<style scoped>
.bad {
  color: var(--a-bad);
}

.hint {
  margin: 0.375rem 0 0;
  font-size: 0.8125rem;
}

.kv {
  margin: 0;
}

.kv > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  min-height: 2.25rem;
  padding: 0.25rem 0;
  border-bottom: 1px solid var(--a-hair);
}

.kv > div:last-child {
  border-bottom: 0;
}

.kv dt.name {
  flex-shrink: 1;
  min-width: 0;
  color: var(--a-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kv dt.off {
  color: var(--a-faint);
}

.kv dd {
  margin: 0;
}

.segmented {
  display: flex;
  gap: 2px;
  width: 7rem;
  padding: 2px;
  border-radius: 0.75rem;
  background: var(--a-hair);
}

.segmented button {
  flex: 1;
  min-width: 0;
  min-height: 2rem;
  padding: 0.25rem 0.375rem;
  border: 0;
  border-radius: 0.625rem;
  background: none;
  color: var(--a-muted);
  font: inherit;
  font-size: 0.8125rem;
  font-weight: 600;
  cursor: pointer;
}

.segmented button.is-selected {
  background: var(--a-bg);
  color: var(--a-text);
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.12);
}

.segmented button:disabled {
  cursor: default;
}
</style>
