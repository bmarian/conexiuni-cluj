<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const props = defineProps<{ search: string }>()
const { t, locale } = useI18n()
const router = useRouter()

function onEggClick() {
  router.push({ name: 'not-found' })
}

const QUOTES_RO = [
  'ETA: 2035 (optimist)',
  'Încă săpăm, încearcă un autobuz 🚌',
  'Proiect în stadiu de vis 😴',
]
const QUOTES_EN = [
  'ETA: 2035 (optimistic)',
  'Still digging, try a bus 🚌',
  'Project status: wishful thinking',
]

const showEgg = computed(() => {
  const q = props.search.trim().toLowerCase()
  if (q.length < 2) return false
  return 'metrou'.startsWith(q) || q === 'm1'
})

const quote = computed(() => {
  const pool = locale.value.startsWith('ro') ? QUOTES_RO : QUOTES_EN
  return pool[new Date().getMinutes() % pool.length]!
})
</script>

<template>
  <div v-if="showEgg" class="egg-wrap">
    <div
      class="egg-row"
      role="button"
      tabindex="0"
      :aria-label="t('metroEggLine')"
      @click="onEggClick"
      @keydown.enter.space.prevent="onEggClick"
    >
      <div class="egg-badge">M1</div>
      <div class="egg-text">
        <span class="egg-name">{{ t('metroEggLine') }}</span>
        <span class="egg-quote">{{ quote }}</span>
      </div>
      <span class="egg-icon">🚧</span>
    </div>
  </div>

  <p v-else class="no-results">{{ t('noResults') }}</p>
</template>

<style scoped>
.no-results {
  font-size: 0.875rem;
  color: #94a3b8;
  padding: 1rem 0;
  text-align: center;
}

.egg-wrap {
  margin-top: 0.25rem;
}

.egg-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 0.25rem;
  border-radius: 0.5rem;
  opacity: 0.6;
  user-select: none;
  cursor: pointer;
  transition: opacity 120ms ease, background 120ms ease;
}
.egg-row:hover { opacity: 1; }
.egg-row:focus-visible {
  outline: 2px solid #94a3b8;
  outline-offset: 2px;
}

.egg-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 2.5rem;
  height: 1.75rem;
  border-radius: 0.375rem;
  font-size: 0.75rem;
  font-weight: 900;
  color: #fff;
  background: repeating-linear-gradient(45deg, #64748b, #64748b 4px, #475569 4px, #475569 9px);
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.25);
}

.egg-text {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  min-width: 0;
}

.egg-name {
  font-size: 0.875rem;
  font-weight: 600;
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
:global(.dark) .egg-name { color: #475569; }

.egg-quote {
  font-size: 0.7rem;
  font-style: italic;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
:global(.dark) .egg-quote { color: #334155; }

.egg-icon {
  font-size: 1rem;
  line-height: 1;
  flex-shrink: 0;
}
</style>
