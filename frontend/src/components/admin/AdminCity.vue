<script setup lang="ts">
import {computed, ref} from 'vue'
import AdminCalendar from '@/components/admin/AdminCalendar.vue'
import AdminCard from '@/components/admin/AdminCard.vue'
import AdminCityMenu from '@/components/admin/AdminCityMenu.vue'
import AdminDayBars, {type DayBar} from '@/components/admin/AdminDayBars.vue'
import AdminErrors from '@/components/admin/AdminErrors.vue'
import AdminFix from '@/components/admin/AdminFix.vue'
import AdminHiddenStops from '@/components/admin/AdminHiddenStops.vue'
import AdminScope from '@/components/admin/AdminScope.vue'
import AdminStats from '@/components/admin/AdminStats.vue'
import type {CitySummary, LearningWindow, Overview} from '@/components/admin/types.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import LiveBadge from '@/components/ui/LiveBadge.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import TimetableBadge from '@/components/ui/TimetableBadge.vue'
import {ago, bytes, came, clockTime, dayName, dur, minSec, num, pct, platformLabels, plural, windowLabels} from '@/utils/adminFormat.ts'
import {ExpandMoreIcon, LocationCityIcon, SettingsIcon} from '@/utils/icons.ts'

const props = defineProps<{overview: Overview | null; cities: CitySummary[]; city: string; clock: number}>()
const emit = defineEmits<{pick: [slug: string]; saved: []}>()

const horizonLabels: Record<string, string> = {
  all: 'All',
  '1-2': '1–2 min',
  '3-5': '3–5 min',
  '6-10': '6–10 min',
  '11-20': '11–20 min',
  '21-30': '21–30 min',
}

const sourceLabels: Record<string, string> = {
  ours: 'Ours',
  ctp: 'CTP',
}

const changeLabels: Record<string, string> = {
  later: 'later',
  earlier: 'earlier',
  trips_added: 'trips added',
  trips_removed: 'trips removed',
  stops_added: 'stops added',
  stops_removed: 'stops removed',
}

const learning = ref<LearningWindow[]>([])

const summary = computed(() => props.cities.find((c) => c.slug === props.city))
const realtime = computed(() => summary.value?.realtime ?? true)

const liveToday = computed(() => props.overview?.accuracy.today.find((a) => a.label === 'all'))

const stats = computed(() => {
  const o = props.overview
  if (!o) return []
  return [
    {value: num(o.now.people), label: 'watching'},
    {value: num(o.people.today), label: 'people today'},
    realtime.value
      ? {value: liveToday.value?.n ? pct(liveToday.value.within_1) : '–', label: 'within 1 min'}
      : {value: num(o.people.plans), label: 'trips planned'},
  ]
})

const liveRows = computed(() => {
  const week = props.overview?.accuracy.week ?? []
  return [...week.filter((a) => a.label === 'all'), ...week.filter((a) => a.label !== 'all')]
})

const pollText = computed(() => {
  const n = props.overview?.now
  if (!n || !n.poll_sec) return 'off'
  return n.polled_by === 'viewers' ? `every ${dur(n.poll_sec)}` : `every ${dur(n.poll_sec)}, learner`
})

const peopleBars = computed<DayBar[]>(() =>
  (props.overview?.people.daily ?? []).map((d) => {
    const split = (['web', 'android', 'wear'] as const).filter((p) => d[p] > 0).map((p) => `${platformLabels[p]} ${num(d[p])}`)
    return {
      date: d.date,
      value: d.total,
      caption: `${dayName(d.date)} · ${num(d.total)}${split.length ? ` (${split.join(', ')})` : ''}`,
    }
  }),
)

const accuracyBars = computed<DayBar[]>(() =>
  (props.overview?.accuracy.daily ?? []).map((d) => ({
    date: d.date,
    value: d.n ? d.within_1 : 0,
    caption: d.n ? `${dayName(d.date)} · ${pct(d.within_1)} of ${num(d.n)}` : `${dayName(d.date)} · none`,
  })),
)

const versionsToday = computed(() => (props.overview?.people.versions ?? []).filter((v) => v.today > 0))

const meters = computed(() => {
  const q = props.overview?.quota
  if (!q) return []
  const out = q.quotas.map(({name, used, limit}) => ({label: name, used, limit}))
  if (q.learning.enabled) {
    out.push({label: q.learning.dedicated ? 'Learner, own key' : 'Learner', used: q.learning.used, limit: q.learning.budget})
  }
  return out
})

const share = (used: number, limit: number) => (limit > 0 ? Math.min(1, used / limit) : 0)
</script>

<template>
  <section class="admin-part">
    <AdminScope :icon="LocationCityIcon" tone="tertiary">
      <template #title>
        <AdminCityMenu :cities="cities" :city="city" @pick="emit('pick', $event)" />
      </template>
      <LiveBadge v-if="realtime" />
      <TimetableBadge v-else />
    </AdminScope>

    <template v-if="overview">
      <AdminStats :stats="stats" :problems="overview.problems" />

      <div class="admin-cards">
        <AdminCard title="Errors" hint="today">
          <AdminErrors :errors="overview.traffic.error_details" :clock="clock" />
        </AdminCard>

        <AdminCard title="To fix" hint="7 days">
          <AdminFix :fix="overview.fix" />
        </AdminCard>

        <AdminCard v-if="realtime || overview.now.lines.length || overview.now.stops.length" title="Now">
          <dl v-if="realtime" class="admin-rows">
            <div>
              <dt>Buses</dt>
              <dd>{{ num(overview.now.buses_tracked) }} of {{ num(overview.now.buses_in_service) }} · {{ num(overview.now.lines_live) }} lines</dd>
            </div>
            <div>
              <dt>Positions</dt>
              <dd>{{ overview.now.feed_at ? ago(overview.now.feed_at, clock) : 'none yet' }}</dd>
            </div>
            <div>
              <dt>Polling</dt>
              <dd>{{ pollText }}</dd>
            </div>
            <div v-if="overview.now.feed_error">
              <dt>Error</dt>
              <dd class="admin-bad">{{ overview.now.feed_error }}</dd>
            </div>
          </dl>
          <template v-if="overview.now.lines.length">
            <SubLabel text="Lines on screens" />
            <p class="admin-lines">
              <span v-for="l in overview.now.lines" :key="l.id"> <LineBadge plain :name="l.name" :color="l.color" />{{ num(l.count) }} </span>
            </p>
          </template>
          <template v-if="overview.now.stops.length">
            <SubLabel text="Stops on screens" />
            <dl class="admin-rows">
              <div v-for="s in overview.now.stops" :key="s.id">
                <dt class="name">{{ s.name }}</dt>
                <dd>{{ num(s.count) }}</dd>
              </div>
            </dl>
          </template>
        </AdminCard>

        <AdminCard title="People">
          <table class="admin-table">
            <thead>
              <tr>
                <th></th>
                <th>Today</th>
                <th>Week ago</th>
              </tr>
            </thead>
            <tbody>
              <tr class="total">
                <td>All</td>
                <td>{{ num(overview.people.today) }}</td>
                <td>{{ num(overview.people.last_week) }}</td>
              </tr>
              <tr v-for="p in overview.people.platforms" :key="p.platform">
                <td>{{ platformLabels[p.platform] ?? p.platform }}</td>
                <td>{{ num(p.today) }}</td>
                <td>{{ num(p.last_week) }}</td>
              </tr>
            </tbody>
          </table>
          <AdminDayBars :days="peopleBars" />
          <dl class="admin-rows">
            <div>
              <dt>New today</dt>
              <dd>{{ num(overview.people.new) }}</dd>
            </div>
            <div>
              <dt>Trips planned</dt>
              <dd>{{ num(overview.people.plans) }}</dd>
            </div>
            <div>
              <dt>Place searches</dt>
              <dd>{{ num(overview.people.places) }}</dd>
            </div>
            <div>
              <dt>Web installs</dt>
              <dd>{{ num(overview.people.installs) }}</dd>
            </div>
            <div v-for="v in versionsToday" :key="v.platform + v.version">
              <dt>{{ platformLabels[v.platform] ?? v.platform }} {{ v.version }}</dt>
              <dd>{{ num(v.today) }}</dd>
            </div>
          </dl>
          <template v-if="overview.people.lines.length">
            <SubLabel text="Lines today" />
            <p class="admin-lines">
              <span v-for="l in overview.people.lines" :key="l.id"> <LineBadge plain :name="l.name" :color="l.color" />{{ num(l.count) }} </span>
            </p>
          </template>
          <template v-if="overview.people.stops.length">
            <SubLabel text="Stops today" />
            <dl class="admin-rows">
              <div v-for="s in overview.people.stops" :key="s.id">
                <dt class="name">{{ s.name }}</dt>
                <dd>{{ num(s.count) }}</dd>
              </div>
            </dl>
          </template>
        </AdminCard>

        <AdminCard v-if="realtime" title="Accuracy" hint="7 days">
          <table class="admin-table">
            <thead>
              <tr>
                <th></th>
                <th>Within 1 min</th>
                <th>1 in 10 off by</th>
                <th>Buses came</th>
              </tr>
            </thead>
            <tbody>
              <tr class="group">
                <th colspan="4">Live, when it said</th>
              </tr>
              <tr v-for="r in liveRows" :key="r.label" :class="{total: r.label === 'all'}">
                <td>{{ horizonLabels[r.label] ?? r.label }}</td>
                <td>{{ r.n ? pct(r.within_1) : '–' }}</td>
                <td>{{ r.n ? minSec(r.p90_abs_sec) : '–' }}</td>
                <td>{{ r.n ? came(r.bias_sec) : '–' }}</td>
              </tr>
              <tr class="group">
                <th colspan="4">Timetable</th>
              </tr>
              <tr v-for="r in overview.accuracy.timetable" :key="r.label">
                <td>{{ sourceLabels[r.label] ?? r.label }}</td>
                <td>{{ r.n ? pct(r.within_1) : '–' }}</td>
                <td>{{ r.n ? minSec(r.p90_abs_sec) : '–' }}</td>
                <td>{{ r.n ? came(r.bias_sec) : '–' }}</td>
              </tr>
            </tbody>
          </table>
          <AdminDayBars :days="accuracyBars" :max="1" />
        </AdminCard>

        <AdminCard v-if="meters.length" title="Tranzy" hint="today">
          <div v-for="m in meters" :key="m.label" class="meter">
            <p>
              <span>{{ m.label }}</span
              ><b>{{ num(m.used) }} / {{ num(m.limit) }}</b>
            </p>
            <div class="track"><span :class="{'is-full': m.limit > 0 && m.used >= m.limit}" :style="{width: pct(share(m.used, m.limit))}"></span></div>
          </div>
        </AdminCard>

        <AdminCard title="Learning">
          <dl class="admin-rows">
            <div>
              <dt>Timetables</dt>
              <dd>{{ overview.data.warmup_running ? 'refreshing' : ago(overview.data.warmup_at, clock) }} · next {{ clockTime(overview.data.warmup_next) }}</dd>
            </div>
            <template v-if="realtime">
              <div>
                <dt>Learned today</dt>
                <dd>{{ num(overview.data.samples_today) }} · {{ num(overview.data.samples_hour) }} last hour</dd>
              </div>
              <div>
                <dt>Delays</dt>
                <dd>{{ ago(overview.data.delays_at, clock) }}</dd>
              </div>
            </template>
            <div>
              <dt>Archive</dt>
              <dd v-if="!overview.data.archive.enabled" class="admin-bad">off</dd>
              <dd v-else-if="overview.data.archive.ran_at">
                {{ bytes(overview.data.archive.bytes)
                }}<template v-if="overview.data.archive.oldest_date"> since {{ dayName(overview.data.archive.oldest_date) }}</template> ·
                {{ ago(overview.data.archive.ran_at, clock) }}
              </dd>
              <dd v-else>not run yet</dd>
            </div>
            <div v-if="!overview.data.route_changes.length">
              <dt>Timetable changes</dt>
              <dd>none this week</dd>
            </div>
          </dl>
          <template v-if="overview.data.route_changes.length">
            <SubLabel text="Timetable changes" />
            <dl class="admin-rows">
              <div v-for="c in overview.data.route_changes" :key="`${c.line}-${c.detected_at}`" class="line-row">
                <dt class="name">
                  <LineBadge plain :name="c.line" :color="c.color" />
                  <span class="truncate">{{ c.kinds.map((k) => changeLabels[k] ?? k).join(', ') }}</span>
                </dt>
                <dd>{{ ago(c.detected_at, clock) }}</dd>
              </div>
            </dl>
          </template>
          <template v-if="learning.length">
            <SubLabel text="Learns from" />
            <dl class="admin-rows">
              <div v-for="w in learning" :key="w.day_type">
                <dt>{{ windowLabels[w.day_type] ?? w.day_type }}</dt>
                <dd>
                  {{ w.days }} days<template v-if="w.from"> since {{ dayName(w.from) }}</template
                  ><template v-if="w.last_year_days"> + {{ w.last_year_days }} last year</template>
                </dd>
              </div>
            </dl>
          </template>
        </AdminCard>

        <AdminCard title="Requests" hint="today">
          <dl class="admin-rows">
            <div>
              <dt>Requests</dt>
              <dd>
                {{ num(overview.traffic.requests) }} ·
                <span :class="{'admin-bad': overview.traffic.errors > 0}">{{ plural(overview.traffic.errors, 'error') }}</span>
              </dd>
            </div>
            <div>
              <dt>Database</dt>
              <dd>{{ bytes(overview.traffic.db_bytes) }}</dd>
            </div>
          </dl>
          <details v-if="overview.traffic.endpoints.length" class="admin-fold">
            <summary class="state"><span>Endpoints</span><ExpandMoreIcon class="m-icon" /></summary>
            <table class="admin-table endpoints">
              <thead>
                <tr>
                  <th></th>
                  <th>Calls</th>
                  <th>Avg</th>
                  <th>Errors</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="e in overview.traffic.endpoints" :key="e.key">
                  <td>{{ e.key }}</td>
                  <td>{{ num(e.calls) }}</td>
                  <td>{{ e.key.endsWith(' stream') ? '–' : `${num(e.avg_ms)} ms` }}</td>
                  <td :class="{'admin-bad': e.errors > 0}">{{ e.errors ? num(e.errors) : '–' }}</td>
                </tr>
              </tbody>
            </table>
          </details>
        </AdminCard>
      </div>
    </template>
    <p v-else class="admin-note">Loading…</p>

    <h3 class="admin-settings"><SettingsIcon class="m-icon" />{{ summary?.name ?? city }} settings</h3>
    <AdminCard title="Timetable calendar">
      <AdminCalendar :key="city" :city="city" @saved="emit('saved')" @learning="learning = $event" />
    </AdminCard>
    <AdminCard title="Hidden stops">
      <AdminHiddenStops :key="city" :city="city" />
    </AdminCard>
  </section>
</template>

<style scoped>
.admin-part {
  container-type: inline-size;
}

@container (min-width: 46rem) {
  .admin-cards {
    columns: 2;
    column-gap: 12px;
  }
}

.meter {
  padding: 8px 16px;
}

.meter p {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  font-size: 14px;
  line-height: 20px;
  letter-spacing: 0.25px;
  color: var(--md-on-surface-variant);
}

.meter b {
  font-weight: 500;
  color: var(--md-on-surface);
  font-variant-numeric: tabular-nums;
}

.track {
  height: 4px;
  margin-top: 8px;
  border-radius: 2px;
  background: var(--md-secondary-container);
  overflow: hidden;
}

.track span {
  display: block;
  height: 100%;
  border-radius: 2px;
  background: var(--md-primary);
}

.track span.is-full {
  background: var(--md-error);
}

.endpoints {
  font-size: 12px;
  line-height: 16px;
}

.endpoints td:first-child {
  white-space: normal;
  word-break: break-all;
}
</style>
