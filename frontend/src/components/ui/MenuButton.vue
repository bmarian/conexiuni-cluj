<script setup lang="ts">
import {nextTick, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {usePopup} from '@/composables/usePopup.ts'
import {MoreVertIcon} from '@/utils/icons.ts'
import type {MenuAction} from '@/components/ui/types.ts'

defineProps<{actions: MenuAction[]}>()

const {t} = useI18n()
const button = ref<HTMLElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const at = ref({top: 0, right: 0})
const {open, show, close} = usePopup(() => [menu.value, button.value])

async function toggle() {
  if (open.value) return close()
  const box = button.value!.getBoundingClientRect()
  at.value = {top: box.bottom + 4, right: window.innerWidth - box.right}
  show()
  await nextTick()
  menu.value?.querySelector('button')?.focus({preventScroll: true})
}

function pick(action: MenuAction) {
  close()
  action.onClick()
}
</script>

<template>
  <button
    ref="button"
    type="button"
    class="icon-btn"
    data-kbd-item="menu"
    :aria-label="t('moreOptions')"
    :title="t('moreOptions')"
    :aria-expanded="open"
    @click="toggle"
  >
    <MoreVertIcon class="m-icon" />
  </button>
  <Teleport to="body">
    <Transition name="menu">
      <div v-if="open" ref="menu" class="menu popup-menu" role="menu" :style="{top: `${at.top}px`, right: `${at.right}px`}">
        <button v-for="action in actions" :key="action.label" type="button" class="popup-item state" role="menuitem" @click="pick(action)">
          <component :is="action.icon" class="m-icon text-on-surface-variant" />
          <span>{{ action.label }}</span>
        </button>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.menu {
  min-width: 200px;
  max-width: 280px;
  transform-origin: top right;
}

.popup-item {
  padding-left: 12px;
}
</style>
