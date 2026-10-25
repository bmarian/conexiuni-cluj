<script setup lang="ts">
import {onBeforeUnmount, onMounted, ref} from 'vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {cityPath} from '@/utils/city.ts'
import {ArrowBackIcon, CloseIcon} from '@/utils/icons.ts'
import MenuButton from '@/components/ui/MenuButton.vue'
import type {MenuAction} from '@/components/ui/types.ts'

defineProps<{title?: string | null; menu?: MenuAction[]}>()

const PAST_HEADER_PX = 120

const router = useRouter()
const {t} = useI18n()
const bar = ref<HTMLElement | null>(null)
const pastHeader = ref(false)
const hasBack = !!history.state?.back
let scroller: HTMLElement | null = null

const onScroll = () => {
  pastHeader.value = (scroller?.scrollTop ?? 0) > PAST_HEADER_PX
}

onMounted(() => {
  scroller = bar.value?.closest('.drawer-view') ?? null
  scroller?.addEventListener('scroll', onScroll, {passive: true})
  onScroll()
})

onBeforeUnmount(() => scroller?.removeEventListener('scroll', onScroll))
</script>

<template>
  <div ref="bar" class="sheet-top-bar" :class="{'is-past': pastHeader}" data-kbd-section="nav" data-kbd-axis="x">
    <button v-if="hasBack" type="button" class="icon-btn" data-kbd-item="back" :aria-label="t('back')" :title="t('back')" @click="router.back()">
      <ArrowBackIcon class="m-icon" />
    </button>
    <span class="bar-title text-title-medium" :class="{'is-shown': pastHeader && title}">{{ title }}</span>
    <slot />
    <MenuButton v-if="menu?.length" :actions="menu" />
    <button type="button" class="icon-btn" data-kbd-item="home" :aria-label="t('home')" :title="t('home')" @click="router.push(cityPath('/'))">
      <CloseIcon class="m-icon" />
    </button>
  </div>
</template>

<style scoped>
.sheet-top-bar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  height: 48px;
  padding: 0 4px;
  background: var(--md-surface-container-low);
}

.bar-title {
  flex: 1;
  min-width: 0;
  padding: 0 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  opacity: 0;
  transition: opacity 200ms linear;
}

.bar-title.is-shown {
  opacity: 1;
}
</style>
