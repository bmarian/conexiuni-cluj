<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import type {RouteChange} from '@/stores/routeUpdates'
import {CloseIcon, NotificationsIcon} from '@/utils/icons.ts'
import {
  changeRows,
  changeSummary,
  changeTitle,
  daysLabel,
  formatChangeDate,
  formatServiceStart,
  groupByDays,
  groupByDirection,
} from '@/utils/routeChanges.ts'

const ENTRY_LIMIT = 12

const props = defineProps<{ change: RouteChange }>()
const emit = defineEmits<{ dismiss: [] }>()

const {t, locale} = useI18n()

const showMore = ref(false)
const openRows = ref(new Set<string>())

watch(() => props.change.id, () => {
  showMore.value = false
  openRows.value = new Set()
})

const detailsId = computed(() => `route-change-${props.change.id}`)
const title = computed(() => changeTitle(props.change, t))
const detected = computed(() => formatChangeDate(props.change.detected_at, locale.value, false))
const summary = computed(() => changeSummary(props.change, t))

const sections = computed(() => groupByDirection(props.change).map((group) => ({
  direction: group.direction,
  toward: group.toward,
  days: groupByDays(group.items).map((day) => ({
    key: `${group.direction}-${day.key}`,
    label: daysLabel(day.days, t),
    since: day.since ? formatServiceStart(day.since, locale.value) : '',
    rows: changeRows(day.items, t).map((row) => {
      const key = `${group.direction}-${day.key}-${row.kind}`
      const entries = openRows.value.has(key) ? row.entries : row.entries.slice(0, ENTRY_LIMIT)
      return {
        key,
        label: row.label,
        entries,
        hidden: row.entries.length - entries.length,
        names: row.kind === 'stops_added' || row.kind === 'stops_removed',
      }
    }),
  })),
})))

function openRow(key: string) {
  openRows.value = new Set(openRows.value).add(key)
}
</script>

<template>
  <section
    class="change-banner"
    :style="{ '--line': change.route_color || '#64748b' }"
    data-kbd-section="change-banner"
    data-kbd-axis="x"
    :aria-label="title"
  >
    <NotificationsIcon class="m-icon size-5 mt-0.5 line-tint"/>
    <div class="flex-1 min-w-0">
      <p class="text-title-small"><span class="font-bold">{{ title }}</span> <span class="text-on-surface-variant">· {{ detected }}</span></p>
      <p class="text-body-small text-on-surface-variant">{{ summary }}</p>

      <div v-if="showMore" :id="detailsId">
        <section v-for="section in sections" :key="section.direction">
          <div v-if="section.toward" class="flex items-center gap-2 pt-2.5">
            <span class="text-label-large">{{ t('routeChangeToward', {name: section.toward}) }}</span>
            <span class="rule"></span>
          </div>
          <div v-for="day in section.days" :key="day.key">
            <p v-if="day.label" class="flex gap-1.5 pt-1.5 text-label-medium">
              <span class="font-bold">{{ day.label }}</span>
              <span v-if="day.since" class="text-on-surface-variant">{{ t('routeChangeSince', {date: day.since}) }}</span>
            </p>
            <div v-for="row in day.rows" :key="row.key">
              <p class="pt-1 text-label-small text-on-surface-variant">{{ row.label }}</p>
              <p v-if="row.entries.length" class="entries" :class="{ 'is-names': row.names }">
                <span v-for="(entry, i) in row.entries" :key="i" class="entry" :class="{ 'is-struck': entry.struck }">
                  <s v-if="entry.old" class="old">{{ entry.old }}</s>{{ entry.text }}{{ row.names && i < row.entries.length - 1 ? ',' : '' }}
                </span>
                <button v-if="row.hidden > 0" type="button" class="more" :data-kbd-item="`change-row-${row.key}`" @click="openRow(row.key)">
                  {{ t('routeChangeMore', {n: row.hidden}) }}
                </button>
              </p>
            </div>
          </div>
        </section>
      </div>

      <button
        type="button"
        class="toggle"
        :aria-expanded="showMore"
        :aria-controls="detailsId"
        data-kbd-item="change-more"
        @click="showMore = !showMore"
      >{{ showMore ? t('routeChangeShowLess') : t('routeChangeShowMore') }}</button>
    </div>

    <button type="button" class="icon-btn" :aria-label="t('dismiss')" :title="t('dismiss')" data-kbd-item="change-dismiss" @click="emit('dismiss')">
      <CloseIcon class="m-icon"/>
    </button>
  </section>
</template>

<style scoped>
.change-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 4px 8px 16px;
  background: color-mix(in srgb, var(--line) 10%, transparent);
  color: var(--md-on-surface);
}

.line-tint {
  color: var(--line);
}

.rule {
  flex: 1;
  height: 1px;
  background: color-mix(in srgb, var(--line) 30%, transparent);
}

.entries {
  display: flex;
  flex-wrap: wrap;
  column-gap: 14px;
  font-size: 12px;
  line-height: 16px;
}

.entries.is-names {
  column-gap: 6px;
}

.entry {
  font-weight: 600;
}

.entry.is-struck {
  color: var(--md-outline);
  text-decoration: line-through;
}

.old {
  margin-right: 5px;
  font-weight: 400;
  color: var(--md-outline);
}

.more,
.toggle {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--md-primary);
  cursor: pointer;
}

.more {
  font-size: 12px;
}

.toggle {
  margin-top: 6px;
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  letter-spacing: 0.1px;
}
</style>
