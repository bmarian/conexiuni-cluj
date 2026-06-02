<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'

const props = defineProps<{ search: string }>()
const {t, locale} = useI18n()
const router = useRouter()

function onMetroClick() {
  router.push({name: 'bsod'})
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

const showMetro = computed(() => {
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
  <div v-if="showMetro" class="metro-legacy-wrap">
    <div
      class="metro-legacy-row"
      role="button"
      tabindex="0"
      :aria-label="t('metroLegacyLine')"
      @click="onMetroClick"
      @keydown.enter.space.prevent="onMetroClick"
    >
      <div class="metro-legacy-badge">M1</div>
      <div class="metro-legacy-text">
        <span class="metro-legacy-name">{{ t('metroLegacyLine') }}</span>
        <span class="metro-legacy-quote">{{ quote }}</span>
      </div>
      <span class="metro-legacy-icon">🚧</span>
    </div>
  </div>
</template>

<style scoped>

.metro-legacy-wrap {
  margin-top: 0.25rem;
}

.metro-legacy-row {
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

.metro-legacy-row:hover {
  opacity: 1;
}

.metro-legacy-row:focus-visible {
  outline: 2px solid #94a3b8;
  outline-offset: 2px;
}

.metro-legacy-badge {
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

.metro-legacy-text {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  min-width: 0;
}

.metro-legacy-name {
  font-size: 0.875rem;
  font-weight: 600;
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:global(.dark) .metro-legacy-name {
  color: #475569;
}

.metro-legacy-quote {
  font-size: 0.7rem;
  font-style: italic;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:global(.dark) .metro-legacy-quote {
  color: #334155;
}

.metro-legacy-icon {
  font-size: 1rem;
  line-height: 1;
  flex-shrink: 0;
}
</style>
