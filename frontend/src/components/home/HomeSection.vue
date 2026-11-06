<script setup lang="ts">
import {ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useLongPress} from '@/composables/useLongPress.ts'
import {DragIndicatorIcon, ExpandMoreIcon} from '@/utils/icons.ts'

const props = defineProps<{title: string; collapsed: boolean; arranging: boolean; headKey: string}>()
const emit = defineEmits<{toggle: []; hold: []}>()
const {t} = useI18n()

const press = useLongPress<null>(() => emit('hold'))
const hold = press.bind(null)

function tap() {
  press.click(() => {
    if (!props.arranging) emit('toggle')
  })
}

const folded = () => props.collapsed || props.arranging

// Clipping all the time would cut a lifted row.
const clipping = ref(folded())
watch(folded, () => (clipping.value = true))

function settled(e: TransitionEvent) {
  if (e.target === e.currentTarget && e.propertyName === 'grid-template-rows' && !folded()) clipping.value = false
}
</script>

<template>
  <section class="home-section" :class="{'is-arranging': arranging}">
    <div :data-kbd-section="headKey">
      <button
        type="button"
        class="section-head state"
        :aria-expanded="!folded()"
        :title="arranging ? undefined : t(collapsed ? 'sectionExpand' : 'sectionCollapse')"
        :data-kbd-item="headKey"
        v-bind="hold"
        @click="tap"
      >
        <span class="text-title-medium truncate">{{ title }}</span>
        <DragIndicatorIcon v-if="arranging" class="m-icon head-icon" />
        <ExpandMoreIcon v-else class="m-icon head-icon chevron" :class="{'is-collapsed': collapsed}" />
      </button>
    </div>
    <div class="section-body" :class="{'is-collapsed': folded(), 'is-clipping': clipping}" @transitionend="settled">
      <div class="section-inner">
        <div class="section-content"><slot /></div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.home-section {
  margin: 8px 12px 0;
  border-radius: 20px;
  background: var(--md-surface-container);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  min-height: 52px;
  padding: 0 14px 0 16px;
  border-radius: 20px;
  color: var(--md-on-surface);
  text-align: left;
  cursor: pointer;
  user-select: none;
  -webkit-touch-callout: none;
}

.is-arranging .section-head {
  cursor: grab;
}

.head-icon {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  color: var(--md-on-surface-variant);
}

.chevron {
  transition: rotate var(--spring-fast-ms) var(--spring-fast);
}

.chevron.is-collapsed {
  rotate: -90deg;
}

.section-body {
  display: grid;
  grid-template-rows: 1fr;
  transition: grid-template-rows var(--spring-sheet-ms) var(--spring-sheet);
}

.section-body.is-collapsed {
  grid-template-rows: 0fr;
  visibility: hidden;
  transition:
    grid-template-rows var(--spring-sheet-ms) var(--spring-sheet),
    visibility 0s linear var(--spring-sheet-ms);
}

.section-inner {
  min-height: 0;
}

.section-content {
  padding-bottom: 6px;
}

.is-clipping > .section-inner {
  overflow: hidden;
}
</style>
