<script setup lang="ts">
import {computed} from 'vue'
import AdminCard from '@/components/admin/AdminCard.vue'
import AdminErrors from '@/components/admin/AdminErrors.vue'
import AdminFailures from '@/components/admin/AdminFailures.vue'
import AdminProblems from '@/components/admin/AdminProblems.vue'
import AdminSub from '@/components/admin/AdminSub.vue'
import type {Overview, Problem, Site} from '@/components/admin/types.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import {ago, bytes, clockTime, dayName, dur, num, pct, plural, windowLabels} from '@/utils/adminFormat.ts'
import {ExpandMoreIcon} from '@/utils/icons.ts'

const props = defineProps<{site: Site; overview: Overview; city: string; name: string; realtime: boolean; clock: number}>()
const emit = defineEmits<{dismiss: [problems: (Problem & {scope: string})[], dismissed: boolean]}>()

const changeLabels: Record<string, string> = {
  later: 'later',
  earlier: 'earlier',
  trips_added: 'trips added',
  trips_removed: 'trips removed',
  stops_added: 'stops added',
  stops_removed: 'stops removed',
}

const problems = computed(() => [
  ...props.site.problems.map((p) => ({...p, scope: 'app'})),
  ...props.overview.problems.map((p) => ({...p, scope: props.city})),
])

const server = computed(() => props.site.server)
const uptime = computed(() => dur((props.clock - new Date(server.value.started_at).getTime()) / 1000))
const diskUsed = computed(() => (server.value.disk_total ? 1 - server.value.disk_free / server.value.disk_total : null))

const pollText = computed(() => {
  const n = props.overview.now
  if (!n.poll_sec) return 'off'
  return n.polled_by === 'viewers' ? `every ${dur(n.poll_sec)}` : `every ${dur(n.poll_sec)}, learner`
})

const quotaGroups = computed(() => {
  const {quotas, learning} = props.overview.quota
  const learnerSharesMain = learning.enabled && !learning.dedicated
  return (['main', 'learner'] as const)
    .map((key) => ({
      key,
      label: quotas.find((q) => q.key === key)?.key_label ?? '',
      meters: quotas
        .filter((q) => q.key === key)
        .map((q) => ({...q, learner: learnerSharesMain && key === 'main' && q.kind === 'positions' ? learning.used : null})),
    }))
    .filter((g) => g.meters.length)
})
const share = (used: number, limit: number) => (limit > 0 ? Math.min(1, used / limit) : 0)
</script>

<template>
  <AdminProblems :problems="problems" :place="name" @dismiss="(list, dismissed) => emit('dismiss', list, dismissed)" />

  <div class="admin-cols">
    <div>
      <AdminCard title="Errors" :scope="name" hint="today">
        <AdminErrors :errors="overview.traffic.error_details" :clock="clock" />
      </AdminCard>

      <AdminCard title="Failures" :scope="name" hint="24 h">
        <AdminFailures :failures="overview.failures" :clock="clock" />
      </AdminCard>

      <AdminCard title="Server" scope="Whole app" app>
        <dl class="admin-rows">
          <div>
            <dt>Up</dt>
            <dd>
              {{ uptime }}<template v-if="server.revision"> · {{ server.revision }}{{ server.modified ? '+' : '' }}</template>
            </dd>
          </div>
          <div>
            <dt>Requests today</dt>
            <dd>
              {{ num(server.requests) }} · <span :class="{'admin-bad': server.errors > 0}">{{ plural(server.errors, 'error') }}</span>
            </dd>
          </div>
          <div>
            <dt>Memory</dt>
            <dd>{{ bytes(server.heap_bytes) }}</dd>
          </div>
          <div v-if="diskUsed != null">
            <dt>Disk</dt>
            <dd>{{ pct(diskUsed) }} used · {{ bytes(server.disk_free) }} free</dd>
          </div>
          <div>
            <dt>Databases</dt>
            <dd>{{ bytes(server.db_bytes) }}</dd>
          </div>
          <div v-if="server.sync">
            <dt>Sync</dt>
            <dd>
              {{ num(server.sync.chains) }} groups · {{ num(server.sync.devices.web ?? 0) }} web, {{ num(server.sync.devices.android ?? 0) }} phone,
              {{ num(server.sync.devices.wear ?? 0) }} watch · {{ bytes(server.sync.db_bytes) }}
            </dd>
          </div>
        </dl>
      </AdminCard>

      <AdminCard title="Server failures" scope="Whole app" app hint="24 h">
        <AdminFailures :failures="server.problems" :clock="clock" />
      </AdminCard>
    </div>

    <div>
      <AdminCard v-if="realtime" title="Buses" :scope="name" hint="now">
        <dl class="admin-rows">
          <div>
            <dt>Tracked</dt>
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
      </AdminCard>

      <AdminCard v-if="quotaGroups.length" title="Tranzy" :scope="name" hint="today">
        <template v-for="g in quotaGroups" :key="g.key">
          <AdminSub :text="g.label" />
          <div v-for="m in g.meters" :key="m.label" class="meter">
            <p>
              <span>{{ m.label }}</span
              ><b
                >{{ num(m.used) }} / {{ num(m.limit) }}<template v-if="m.learner != null"> · learner {{ num(m.learner) }}</template></b
              >
            </p>
            <div class="track"><span :class="{'is-full': m.limit > 0 && m.used >= m.limit}" :style="{width: pct(share(m.used, m.limit))}"></span></div>
          </div>
        </template>
      </AdminCard>

      <AdminCard title="Learning" :scope="name">
        <dl class="admin-rows">
          <div>
            <dt>Timetables</dt>
            <dd>{{ overview.data.warmup_running ? 'refreshing' : ago(overview.data.warmup_at, clock) }} · next {{ clockTime(overview.data.warmup_next) }}</dd>
          </div>
          <div v-if="overview.data.timetable_fetches">
            <dt>Timetable updates today</dt>
            <dd>
              {{ num(overview.data.timetable_fetches.done) }}
              <span v-if="overview.data.timetable_fetches.failed" class="admin-bad"> · {{ num(overview.data.timetable_fetches.failed) }} failed</span>
            </dd>
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
        </dl>
        <AdminSub text="Timetable changes" unit="this week" />
        <dl v-if="overview.data.route_changes.length" class="admin-rows">
          <div v-for="c in overview.data.route_changes" :key="`${c.line}-${c.detected_at}`" class="line-row">
            <dt class="name">
              <LineBadge plain :name="c.line" :color="c.color" />
              <span class="truncate">{{ c.kinds.map((k) => changeLabels[k] ?? k).join(', ') }}</span>
            </dt>
            <dd>{{ ago(c.detected_at, clock) }}</dd>
          </div>
        </dl>
        <p v-else class="admin-note">None</p>
        <template v-if="overview.data.learning.length">
          <AdminSub text="Learns from" />
          <dl class="admin-rows">
            <div v-for="w in overview.data.learning" :key="w.day_type">
              <dt>{{ windowLabels[w.day_type] ?? w.day_type }}</dt>
              <dd>
                {{ w.days }} days<template v-if="w.from"> since {{ dayName(w.from) }}</template
                ><template v-if="w.last_year_days"> + {{ w.last_year_days }} last year</template>
              </dd>
            </div>
          </dl>
        </template>
      </AdminCard>

      <AdminCard title="Requests" :scope="name" hint="today">
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
          <div class="admin-scroll">
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
          </div>
        </details>
      </AdminCard>
    </div>
  </div>
</template>

<style scoped>
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
