<script setup lang="ts">
import RowLink from '@/components/ui/RowLink.vue'
import Ticker from '@/components/ui/Ticker.vue'
import {FavoriteIcon, MyLocationIcon, PlaceIcon} from '@/utils/icons.ts'

export type StopMark = 'from' | 'nearest' | 'favorite' | 'terminal' | 'plain'

defineProps<{
  name: string
  mark: StopMark
  favorite?: boolean
  first?: boolean
  last?: boolean
  times?: {label: string; isLive: boolean}[]
  trailing?: string
  trailingHighlight?: boolean
  href?: string
  paint?: boolean
}>()
const emit = defineEmits<{open: []}>()
</script>

<template>
  <RowLink class="stop-line state squish" :class="`is-${mark}`" :href="href" @open="emit('open')">
    <span class="rail" :class="{'is-first': first, 'is-last': last}">
      <template v-if="paint">
        <span class="paint is-top"></span>
        <span class="paint is-bottom"></span>
      </template>
      <span class="dot"></span>
    </span>
    <span class="flex-1 min-w-0 flex items-center gap-1 pl-2">
      <span class="name text-body-medium"><Ticker :text="name" /></span>
      <PlaceIcon v-if="mark === 'from'" class="m-icon size-3.5 mark-tint" />
      <MyLocationIcon v-else-if="mark === 'nearest'" class="m-icon size-3.5 mark-tint" />
      <FavoriteIcon v-if="favorite" class="m-icon size-3 text-rose" />
    </span>
    <span v-if="trailing" class="trailing" :class="{'is-highlight': trailingHighlight}">{{ trailing }}</span>
    <span v-else class="times">
      <span v-for="(time, i) in times" :key="i" class="time" :class="{'is-live': time.isLive}">{{ time.label }}</span>
    </span>
  </RowLink>
</template>

<style scoped>
.stop-line {
  --squish: 0.97;
  --mark: transparent;
  display: flex;
  align-items: center;
  padding: 0 16px;
  color: var(--md-on-surface-variant);
  cursor: pointer;
}

.is-from {
  --mark: #10b981;
  background: rgb(16 185 129 / 0.08);
}

.is-nearest {
  --mark: #a855f7;
  background: rgb(168 85 247 / 0.08);
}

.is-favorite {
  --mark: var(--md-rose);
  background: color-mix(in srgb, var(--md-rose) 6%, transparent);
}

.is-terminal {
  --mark: #94a3b8;
}

.rail {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 20px;
  height: 40px;
}

.rail::before {
  content: '';
  position: absolute;
  left: 9px;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--md-outline-variant);
}

.rail.is-first::before {
  top: 50%;
}

.rail.is-last::before {
  bottom: 50%;
}

.paint {
  position: absolute;
  left: 8px;
  width: 4px;
  background: var(--paint);
  clip-path: inset(0 0 100% 0);
}

.paint.is-top {
  top: 0;
  bottom: 50%;
}

.paint.is-bottom {
  top: 50%;
  bottom: 0;
}

.dot {
  position: relative;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--md-surface-container-low);
  box-shadow: inset 0 0 0 2px var(--md-outline-variant);
}

.stop-line:not(.is-plain) .dot {
  width: 12px;
  height: 12px;
  background: var(--mark);
  box-shadow: 0 0 0 2px var(--md-surface-container-low);
}

.name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.is-from .name {
  color: #10b981;
  font-weight: 700;
}

.is-nearest .name {
  color: #a855f7;
  font-weight: 600;
}

.is-terminal .name {
  color: var(--md-on-surface);
  font-weight: 600;
}

.mark-tint {
  color: var(--mark);
}

.times {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.time {
  width: 44px;
  font-size: 13px;
  line-height: 20px;
  font-weight: 600;
  text-align: right;
  white-space: nowrap;
}

.time.is-live {
  color: var(--md-primary);
  font-weight: 800;
}

.trailing {
  flex-shrink: 0;
  font-size: 14px;
  line-height: 20px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.trailing.is-highlight {
  color: var(--md-primary);
}
</style>
