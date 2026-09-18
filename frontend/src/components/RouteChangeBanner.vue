<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import type {RouteChange} from '@/stores/routeUpdates'
import {useSettingsStore} from '@/stores/settings'
import IconBellFilled from '@/components/icons/IconBellFilled.vue'
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
const settings = useSettingsStore()

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
    class="rc-banner"
    :style="{ '--line': change.route_color || '#64748b' }"
    data-kbd-section="change-banner"
    data-kbd-axis="x"
    :aria-label="title"
  >
    <span class="rc-icon" aria-hidden="true">
      <span v-if="settings.legacyBlueActive" class="rc-emoji">🔔</span>
      <IconBellFilled v-else/>
    </span>

    <div class="rc-body">
      <p class="rc-title">{{ title }} <span class="rc-date">· {{ detected }}</span></p>
      <p class="rc-summary">{{ summary }}</p>

      <div v-if="showMore" :id="detailsId" class="rc-details">
        <section v-for="section in sections" :key="section.direction">
          <div v-if="section.toward" class="rc-direction">
            <span class="section-label-text">{{ t('routeChangeToward', {name: section.toward}) }}</span>
            <span class="rc-rule"></span>
          </div>

          <div v-for="day in section.days" :key="day.key" class="rc-day">
            <p v-if="day.label" class="rc-day-name">
              {{ day.label }}
              <span v-if="day.since" class="rc-since">{{ t('routeChangeSince', {date: day.since}) }}</span>
            </p>
            <div v-for="row in day.rows" :key="row.key" class="rc-row">
              <p class="rc-row-label">{{ row.label }}</p>
              <p v-if="row.entries.length" class="rc-entries" :class="{ 'is-names': row.names }">
                <span v-for="(entry, i) in row.entries" :key="i" class="rc-entry">
                  <template v-if="entry.old"><s class="rc-old">{{ entry.old }}</s>{{ entry.text }}</template>
                  <s v-else-if="entry.struck" class="rc-cancelled">{{ entry.text }}</s>
                  <template v-else>{{ entry.text }}</template>
                </span>
                <button v-if="row.hidden > 0" type="button" class="rc-more"
                        :data-kbd-item="`change-row-${row.key}`" @click="openRow(row.key)">
                  {{ t('routeChangeMore', {n: row.hidden}) }}
                </button>
              </p>
            </div>
          </div>
        </section>
      </div>

      <button
        type="button"
        class="rc-toggle flex items-center gap-1 mt-1.5 px-2 py-1 -ml-2 rounded-xl text-xs font-semibold text-slate-500 dark:text-slate-400 hover:bg-black/5 dark:hover:bg-white/5 hover:text-slate-700 dark:hover:text-slate-200 transition-colors duration-150"
        :aria-expanded="showMore"
        :aria-controls="detailsId"
        data-kbd-item="change-more"
        @click="showMore = !showMore"
      >
        {{ showMore ? t('routeChangeShowLess') : t('routeChangeShowMore') }}
        <svg class="rc-chevron w-3 h-3" :class="{ 'is-open': showMore }" viewBox="0 0 24 24" fill="none"
             stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
             aria-hidden="true">
          <path d="M6 9l6 6 6-6"/>
        </svg>
      </button>
    </div>

    <button type="button" class="rc-close" :aria-label="t('dismiss')" data-kbd-item="change-dismiss"
            @click="emit('dismiss')">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
           stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </section>
</template>

<style scoped>
/* Tinted with the line's own colour, the same one its badge uses further down. */
.rc-banner {
  display: flex;
  align-items: flex-start;
  gap: 0.625rem;
  margin: -1.25rem -1.5rem 1rem;
  padding: 0.875rem 1rem 0.75rem 1.5rem;
  background: color-mix(in srgb, var(--line) 7%, #ffffff);
  border-bottom: 1px solid color-mix(in srgb, var(--line) 18%, #ffffff);
}

.rc-icon {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.125rem;
  height: 1.125rem;
  margin-top: 0.1rem;
  color: var(--line);
}

.rc-emoji {
  font-size: 0.95rem;
  line-height: 1;
}

.rc-body {
  flex: 1;
  min-width: 0;
}

.rc-title {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 700;
  line-height: 1.3;
  color: #0f172a;
}

.rc-date {
  font-weight: 500;
  color: #94a3b8;
}

.rc-summary {
  margin: 0.2rem 0 0;
  font-size: 0.8125rem;
  line-height: 1.45;
  color: #475569;
}

.rc-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
  border: 0;
  border-radius: 9999px;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  transition: background 120ms ease, color 120ms ease;
}

.rc-close:hover {
  background: rgb(15 23 42 / 0.06);
  color: #475569;
}

.rc-close svg {
  width: 0.75rem;
  height: 0.75rem;
}

.rc-details {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  margin-top: 0.875rem;
}

.rc-direction {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.section-label-text {
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rc-rule {
  flex: 1;
  height: 1px;
  background: color-mix(in srgb, var(--line) 20%, transparent);
}

.rc-day + .rc-day {
  margin-top: 0.75rem;
}

.rc-day-name {
  margin: 0 0 0.3rem;
  font-size: 0.78rem;
  font-weight: 700;
  color: #334155;
}

.rc-since {
  margin-left: 0.2rem;
  font-weight: 500;
  color: #94a3b8;
}

.rc-row + .rc-row {
  margin-top: 0.45rem;
}

.rc-row-label {
  margin: 0;
  font-size: 0.72rem;
  color: #64748b;
}

.rc-entries {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  column-gap: 0.9rem;
  row-gap: 0.1rem;
  margin: 0.1rem 0 0;
  font-size: 0.8125rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: #1e293b;
}

.rc-entries.is-names {
  column-gap: 0.35rem;
  font-variant-numeric: normal;
}

.rc-entries.is-names .rc-entry:not(:last-of-type)::after {
  content: ',';
}

.rc-old {
  margin-right: 0.3rem;
}

.rc-old,
.rc-cancelled {
  font-weight: 500;
  color: #94a3b8;
}

.rc-more {
  padding: 0;
  border: 0;
  background: none;
  font: inherit;
  font-weight: 600;
  color: #64748b;
  cursor: pointer;
}

.rc-more:hover {
  color: #334155;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.rc-chevron {
  transition: transform 200ms ease;
}

.rc-chevron.is-open {
  transform: rotate(180deg);
}

:root.dark .rc-banner {
  background: color-mix(in srgb, var(--line) 14%, #0f172a);
  border-bottom-color: color-mix(in srgb, var(--line) 28%, #0f172a);
}

:root.dark .rc-icon {
  color: color-mix(in srgb, var(--line) 65%, #ffffff);
}

:root.dark .rc-title {
  color: #f1f5f9;
}

:root.dark .rc-summary {
  color: #cbd5e1;
}

:root.dark .rc-date,
:root.dark .rc-since,
:root.dark .rc-old,
:root.dark .rc-cancelled {
  color: #64748b;
}

:root.dark .rc-day-name,
:root.dark .rc-entries {
  color: #e2e8f0;
}

:root.dark .rc-row-label,
:root.dark .rc-more {
  color: #94a3b8;
}

:root.dark .rc-more:hover {
  color: #e2e8f0;
}

:root.dark .rc-rule {
  background: color-mix(in srgb, var(--line) 35%, transparent);
}

:root.dark .rc-close {
  color: #64748b;
}

:root.dark .rc-close:hover {
  background: rgb(255 255 255 / 0.06);
  color: #cbd5e1;
}
</style>
