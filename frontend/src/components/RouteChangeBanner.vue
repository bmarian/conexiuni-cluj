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
  groupByDays,
  groupByDirection,
} from '@/utils/routeChanges.ts'

const CHIP_LIMIT = 8

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
const title = computed(() => t('routeChangeBannerTitle', {
  route: props.change.route_short_name,
  what: changeTitle(props.change, t),
}))
const detected = computed(() => formatChangeDate(props.change.detected_at, locale.value))
const summary = computed(() => changeSummary(props.change, t))

const sections = computed(() => groupByDirection(props.change).map((group) => ({
  direction: group.direction,
  toward: group.toward,
  days: groupByDays(group.items).map((day) => ({
    key: `${group.direction}-${day.key}`,
    label: daysLabel(day.days, t),
    since: day.since,
    rows: changeRows(day.items, t).map((row) => {
      const key = `${group.direction}-${day.key}-${row.kind}`
      const visible = openRows.value.has(key) ? row.chips : row.chips.slice(0, CHIP_LIMIT)
      return {key, kind: row.kind, label: row.label, chips: visible, hidden: row.chips.length - visible.length}
    }),
  })),
})))

function openRow(key: string) {
  openRows.value = new Set(openRows.value).add(key)
}
</script>

<template>
  <section class="rc-banner" data-kbd-section="change-banner" data-kbd-axis="x" :aria-label="title">
    <div class="rc-head">
      <span class="rc-icon" aria-hidden="true">
        <span v-if="settings.legacyBlueActive" class="rc-emoji">🔔</span>
        <IconBellFilled v-else/>
      </span>
      <div class="rc-heading">
        <p class="rc-title">{{ title }}</p>
        <p class="rc-detected">{{ t('routeChangeDetected', {date: detected}) }}</p>
        <p class="rc-summary">{{ summary }}</p>
      </div>
      <button type="button" class="rc-close" :aria-label="t('dismiss')" data-kbd-item="change-dismiss"
              @click="emit('dismiss')">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
             stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M18 6L6 18M6 6l12 12"/>
        </svg>
      </button>
    </div>

    <div v-if="showMore" :id="detailsId" class="rc-details">
      <div v-for="section in sections" :key="section.direction" class="rc-section">
        <p v-if="section.toward" class="rc-toward">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
               stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M5 12h14M12 5l7 7-7 7"/>
          </svg>
          {{ t('routeChangeToward', {name: section.toward}) }}
        </p>
        <div v-for="day in section.days" :key="day.key" class="rc-day">
          <p v-if="day.label" class="rc-day-head">
            <span class="rc-day-name">{{ day.label }}</span>
            <span v-if="day.since" class="rc-since">{{ t('routeChangeSince', {date: day.since}) }}</span>
          </p>
          <ul class="rc-items">
            <li v-for="row in day.rows" :key="row.key" class="rc-item" :class="`is-${row.kind}`">
              <span class="rc-bullet" aria-hidden="true"></span>
              <div class="rc-item-body">
                <p class="rc-line">{{ row.label }}</p>
                <div v-if="row.chips.length" class="rc-chips">
                  <span v-for="(chip, i) in row.chips" :key="i" class="rc-chip">{{ chip }}</span>
                  <button v-if="row.hidden > 0" type="button" class="rc-chip rc-more"
                          :data-kbd-item="`change-row-${row.key}`" @click="openRow(row.key)">
                    +{{ row.hidden }}
                  </button>
                </div>
              </div>
            </li>
          </ul>
        </div>
      </div>
    </div>

    <button type="button" class="rc-toggle" :aria-expanded="showMore" :aria-controls="detailsId"
            data-kbd-item="change-more" @click="showMore = !showMore">
      {{ showMore ? t('routeChangeShowLess') : t('routeChangeShowMore') }}
      <svg :class="{ 'is-open': showMore }" viewBox="0 0 24 24" fill="none" stroke="currentColor"
           stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M6 9l6 6 6-6"/>
      </svg>
    </button>
  </section>
</template>

<style scoped>
.rc-banner {
  margin: -1.25rem -1.5rem 1rem;
  padding: 0.75rem 1rem 0.875rem 1.5rem;
  background: linear-gradient(135deg, #fefce8, #fef3c7);
  border-bottom: 1px solid #fde68a;
  color: #713f12;
}

.rc-head {
  display: flex;
  align-items: flex-start;
  gap: 0.625rem;
}

.rc-icon {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  margin-top: 0.1rem;
  color: #eab308;
}

.rc-emoji {
  font-size: 1rem;
  line-height: 1;
}

.rc-heading {
  flex: 1;
  min-width: 0;
}

.rc-title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 800;
  color: #713f12;
}

.rc-detected {
  margin: 0.1rem 0 0;
  font-size: 0.7rem;
  font-weight: 500;
  color: #a16207;
}

.rc-summary {
  margin: 0.45rem 0 0;
  font-size: 0.78rem;
  font-weight: 500;
  line-height: 1.4;
  color: #713f12;
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
  color: #a16207;
  cursor: pointer;
  transition: background 120ms ease;
}

.rc-close:hover {
  background: rgb(234 179 8 / 0.18);
}

.rc-close svg {
  width: 0.75rem;
  height: 0.75rem;
}

.rc-details {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin: 0.625rem 0 0 1.875rem;
  padding: 0.625rem 0.75rem;
  border: 1px solid #fde68a;
  border-radius: 0.75rem;
  background: rgb(255 255 255 / 0.6);
}

.rc-section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.rc-section + .rc-section {
  padding-top: 0.75rem;
  border-top: 1px dashed #fde68a;
}

.rc-toward {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  margin: 0;
  font-size: 0.72rem;
  font-weight: 800;
  color: #854d0e;
}

.rc-toward svg {
  width: 0.8rem;
  height: 0.8rem;
  flex-shrink: 0;
}

.rc-day-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  column-gap: 0.4rem;
  margin: 0 0 0.3rem;
  font-size: 0.72rem;
}

.rc-day-name {
  font-weight: 700;
  color: #713f12;
}

.rc-since {
  font-size: 0.68rem;
  color: #a16207;
}

.rc-items {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.rc-item {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
}

.rc-bullet {
  flex-shrink: 0;
  width: 0.45rem;
  height: 0.45rem;
  margin-top: 0.32rem;
  border-radius: 9999px;
  background: #a16207;
}

.is-retimed .rc-bullet {
  background: #0ea5e9;
}

.is-trips_added .rc-bullet,
.is-stops_added .rc-bullet {
  background: #22c55e;
}

.is-trips_removed .rc-bullet,
.is-stops_removed .rc-bullet {
  background: #ef4444;
}

.rc-item-body {
  flex: 1;
  min-width: 0;
}

.rc-line {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 500;
  line-height: 1.35;
  color: #713f12;
}

.rc-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-top: 0.3rem;
}

.rc-chip {
  padding: 0.1rem 0.4rem;
  border: 1px solid #fde68a;
  border-radius: 0.375rem;
  background: rgb(255 255 255 / 0.8);
  font-size: 0.7rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: #713f12;
}

.rc-more {
  color: #a16207;
  cursor: pointer;
}

.rc-more:hover {
  background: #ffffff;
}

.rc-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  margin: 0.625rem 0 0 1.875rem;
  padding: 0.3rem 0.7rem;
  border: 1px solid #fcd34d;
  border-radius: 9999px;
  background: rgb(255 255 255 / 0.7);
  font-size: 0.72rem;
  font-weight: 700;
  color: #854d0e;
  cursor: pointer;
  transition: background 120ms ease;
}

.rc-toggle:hover {
  background: #ffffff;
}

.rc-toggle svg {
  width: 0.75rem;
  height: 0.75rem;
  transition: transform 200ms ease;
}

.rc-toggle svg.is-open {
  transform: rotate(180deg);
}

:root.dark .rc-banner {
  background: linear-gradient(135deg, #2a2208, #1f1a09);
  border-bottom-color: #3f3410;
  color: #fde68a;
}

:root.dark .rc-icon {
  color: #facc15;
}

:root.dark .rc-title,
:root.dark .rc-day-name,
:root.dark .rc-chip {
  color: #fde68a;
}

:root.dark .rc-summary,
:root.dark .rc-line {
  color: #fef3c7;
}

:root.dark .rc-detected,
:root.dark .rc-since,
:root.dark .rc-close,
:root.dark .rc-more {
  color: #ca8a04;
}

:root.dark .rc-toward {
  color: #facc15;
}

:root.dark .rc-details {
  border-color: #3f3410;
  background: rgb(0 0 0 / 0.22);
}

:root.dark .rc-section + .rc-section {
  border-top-color: #3f3410;
}

:root.dark .rc-chip {
  border-color: #3f3410;
  background: rgb(0 0 0 / 0.25);
}

:root.dark .rc-more:hover {
  background: rgb(0 0 0 / 0.4);
}

:root.dark .rc-toggle {
  border-color: #854d0e;
  background: rgb(0 0 0 / 0.25);
  color: #facc15;
}

:root.dark .rc-toggle:hover {
  background: rgb(0 0 0 / 0.4);
}
</style>
