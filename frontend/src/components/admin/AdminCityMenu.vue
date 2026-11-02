<script setup lang="ts">
import {computed, ref} from 'vue'
import type {CitySummary} from '@/components/admin/types.ts'
import {usePopup} from '@/composables/usePopup.ts'
import {num} from '@/utils/adminFormat.ts'
import {ExpandMoreIcon, WarningAmberIcon} from '@/utils/icons.ts'

const props = defineProps<{cities: CitySummary[]; city: string}>()
const emit = defineEmits<{pick: [slug: string]}>()

const root = ref<HTMLElement | null>(null)
const {open, show, close} = usePopup(() => [root.value])
const name = computed(() => props.cities.find((c) => c.slug === props.city)?.name ?? props.city)
const openProblems = (c: CitySummary) => c.problems.filter((p) => !p.dismissed).length
const elsewhere = computed(() => props.cities.filter((c) => c.slug !== props.city && openProblems(c) > 0))
const elsewhereText = computed(() => (elsewhere.value.length ? `Problems in ${elsewhere.value.map((c) => c.name).join(', ')}` : undefined))
const total = computed(() => props.cities.reduce((t, c) => ({watching: t.watching + c.watching, today: t.today + c.people_today}), {watching: 0, today: 0}))

function pick(slug: string) {
  close()
  emit('pick', slug)
}
</script>

<template>
  <div ref="root" class="city-menu">
    <button type="button" class="trigger state" aria-haspopup="menu" :aria-expanded="open" :title="elsewhereText" @click="open ? close() : show()">
      <h1 class="text-title-large truncate">{{ name }}</h1>
      <WarningAmberIcon v-if="elsewhere.length" class="m-icon elsewhere" :aria-label="elsewhereText" />
      <ExpandMoreIcon class="m-icon chevron" :class="{'is-open': open}" />
    </button>
    <Transition name="menu">
      <div v-if="open" class="menu popup-menu" role="menu">
        <div class="row head" aria-hidden="true">
          <span class="name"></span>
          <span class="n">Watching</span>
          <span class="n">Today</span>
        </div>
        <button
          v-for="c in cities"
          :key="c.slug"
          type="button"
          role="menuitemradio"
          class="row popup-item state"
          :class="{'is-current': c.slug === city}"
          :aria-checked="c.slug === city"
          @click="pick(c.slug)"
        >
          <span class="name">
            <span class="truncate">{{ c.name }}</span>
            <span v-if="openProblems(c)" class="warn"><WarningAmberIcon class="m-icon" />{{ openProblems(c) }}</span>
          </span>
          <span class="n">{{ num(c.watching) }}</span>
          <span class="n">{{ num(c.people_today) }}</span>
        </button>
        <div class="row popup-item total">
          <span class="name">All cities</span>
          <span class="n">{{ num(total.watching) }}</span>
          <span class="n">{{ num(total.today) }}</span>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.city-menu {
  position: relative;
  min-width: 0;
  margin-left: -8px;
}

.trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  padding: 2px 4px 2px 8px;
  border: 0;
  border-radius: 12px;
  background: transparent;
  color: var(--md-on-surface);
  cursor: pointer;
}

.trigger h1 {
  min-width: 0;
}

.elsewhere {
  margin-left: 4px;
  color: var(--md-error);
}

.chevron {
  color: var(--md-on-surface-variant);
  transition: rotate 175ms ease;
}

.chevron.is-open {
  rotate: 180deg;
}

.menu {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  z-index: 10;
  width: min(320px, calc(100vw - 16px));
  max-height: min(480px, 70vh);
  overflow-y: auto;
  transform-origin: top left;
}

.row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.head {
  padding: 4px 16px;
  font-size: 11px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  color: var(--md-on-surface-variant);
}

.name {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.n {
  min-width: 3.5rem;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.warn {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  color: var(--md-error);
  font-variant-numeric: tabular-nums;
}

.warn .m-icon {
  width: 16px;
  height: 16px;
}

.is-current {
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
}

.total {
  margin-top: 4px;
  border-top: 1px solid var(--md-outline-variant);
  font-weight: 600;
  cursor: default;
}
</style>
