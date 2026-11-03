<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import LineBadge from '@/components/ui/LineBadge.vue'
import type {ChainLeg} from '@/utils/lineGroups.ts'
import {ArrowForwardIcon} from '@/utils/icons.ts'

const {legs, small = false} = defineProps<{legs: ChainLeg[]; small?: boolean}>()

const {t} = useI18n()
</script>

<template>
  <span class="line-chain" :class="{'is-small': small}">
    <template v-for="(leg, i) in legs" :key="i">
      <ArrowForwardIcon v-if="i > 0 && leg.change" class="joint" role="img" :aria-label="t('groupThen')" />
      <span v-else-if="i > 0" class="sep">+</span>
      <template v-for="(line, j) in leg.lines" :key="line.name">
        <span v-if="j > 0" class="sep" :aria-label="t('groupOr')">/</span>
        <span v-if="small" class="station-line" :style="{backgroundColor: line.color}">{{ line.name }}</span>
        <LineBadge v-else :name="line.name" :color="line.color" plain :height="36" :min-width="44" :corner="12" />
      </template>
    </template>
  </span>
</template>

<style scoped>
.line-chain {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  color: var(--md-on-surface-variant);
}

.line-chain.is-small {
  gap: 4px;
}

.joint {
  width: 20px;
  height: 20px;
  fill: currentColor;
}

.is-small .joint {
  width: 14px;
  height: 14px;
}

.sep {
  font-size: 18px;
  line-height: 24px;
  font-weight: 500;
}

.is-small .sep {
  font-size: 12px;
  line-height: 16px;
}
</style>
