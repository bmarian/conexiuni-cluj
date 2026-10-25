<script setup lang="ts">
defineProps<{
  top?: string | 'walk' | null
  bottom?: string | 'walk' | null
  dot: 'station' | 'stop' | 'none'
  color: string
  compact?: boolean
  dimmed?: boolean
}>()
</script>

<template>
  <div class="timeline-row" :class="{'is-compact': compact, 'is-dimmed': dimmed}" :style="{'--line': color}">
    <span class="rail">
      <span v-if="top" class="half is-top" :class="{'is-walk': top === 'walk'}" :style="top !== 'walk' ? {background: top} : undefined"></span>
      <span v-if="bottom" class="half is-bottom" :class="{'is-walk': bottom === 'walk'}" :style="bottom !== 'walk' ? {background: bottom} : undefined"></span>
      <span v-if="dot !== 'none'" class="dot" :class="`is-${dot}`"></span>
    </span>
    <div class="body">
      <slot />
    </div>
    <slot name="trailing" />
  </div>
</template>

<style scoped>
.timeline-row {
  display: flex;
  align-items: stretch;
  padding: 0 16px;
}

.timeline-row.is-dimmed {
  opacity: 0.45;
}

.rail {
  position: relative;
  flex-shrink: 0;
  width: 20px;
}

.half {
  position: absolute;
  left: 8px;
  width: 4px;
}

.half.is-top {
  top: 0;
  bottom: 50%;
}

.half.is-bottom {
  top: 50%;
  bottom: 0;
}

.half.is-walk {
  left: 8.2px;
  width: 3.6px;
  background: radial-gradient(circle, var(--md-outline) 1.8px, transparent 2px) center top / 3.6px 7px repeat-y;
}

.dot {
  position: absolute;
  top: 50%;
  left: 50%;
  translate: -50% -50%;
  border-radius: 50%;
  background: var(--md-surface-container);
}

.dot.is-station {
  width: 14px;
  height: 14px;
  box-shadow:
    inset 0 0 0 2.5px var(--line),
    inset 0 0 0 4px var(--md-surface-container),
    inset 0 0 0 7px var(--line);
}

.dot.is-stop {
  width: 8px;
  height: 8px;
  box-shadow: inset 0 0 0 1.5px var(--md-outline-variant);
}

.body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 10px 0 10px 12px;
}

.is-compact .body {
  padding-top: 4px;
  padding-bottom: 4px;
}
</style>
