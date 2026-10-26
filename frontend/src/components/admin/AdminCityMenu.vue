<script setup lang="ts">
import {computed, ref} from 'vue'
import type {CitySummary} from '@/components/admin/types.ts'
import {usePopup} from '@/composables/usePopup.ts'
import {CheckIcon, ExpandMoreIcon} from '@/utils/icons.ts'

const props = defineProps<{cities: CitySummary[]; city: string}>()
const emit = defineEmits<{pick: [slug: string]}>()

const root = ref<HTMLElement | null>(null)
const {open, show, close} = usePopup(() => [root.value])
const name = computed(() => props.cities.find((c) => c.slug === props.city)?.name ?? props.city)

function pick(slug: string) {
  close()
  emit('pick', slug)
}
</script>

<template>
  <div ref="root" class="city-menu">
    <button type="button" class="trigger state" aria-haspopup="menu" :aria-expanded="open" @click="open ? close() : show()">
      <h2 class="text-title-large truncate">{{ name }}</h2>
      <ExpandMoreIcon class="m-icon chevron" :class="{'is-open': open}" />
    </button>
    <Transition name="menu">
      <div v-if="open" class="menu popup-menu" role="menu">
        <button
          v-for="c in cities"
          :key="c.slug"
          type="button"
          role="menuitemradio"
          class="popup-item state"
          :aria-checked="c.slug === city"
          @click="pick(c.slug)"
        >
          <span class="flex-1 truncate">{{ c.name }}</span>
          <CheckIcon v-if="c.slug === city" class="m-icon text-primary" />
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.city-menu {
  position: relative;
  min-width: 0;
}

.trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  margin-left: -8px;
  padding: 2px 4px 2px 8px;
  border: 0;
  border-radius: 12px;
  background: transparent;
  color: var(--md-on-surface);
  cursor: pointer;
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
  left: -8px;
  z-index: 10;
  min-width: 220px;
  max-height: min(420px, 60vh);
  overflow-y: auto;
  transform-origin: top left;
}
</style>
