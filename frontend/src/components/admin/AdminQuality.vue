<script setup lang="ts">
import {computed} from 'vue'
import AdminCard from '@/components/admin/AdminCard.vue'
import AdminDayBars, {type DayBar} from '@/components/admin/AdminDayBars.vue'
import AdminSub from '@/components/admin/AdminSub.vue'
import type {Overview} from '@/components/admin/types.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import {came, dayName, minSec, num, pct} from '@/utils/adminFormat.ts'

const BUSES_SHOWN = 3

const props = defineProps<{overview: Overview; name: string; realtime: boolean}>()

const horizonLabels: Record<string, string> = {
  all: 'All',
  '1-2': '1–2 min',
  '3-5': '3–5 min',
  '6-10': '6–10 min',
  '11-20': '11–20 min',
  '21-30': '21–30 min',
}
const sourceLabels: Record<string, string> = {ours: 'Ours', ctp: 'CTP'}
const planReasons: Record<string, string> = {walk: 'only a long walk', outside: 'outside the network'}
const busReasons: Record<string, string> = {trip: 'unknown trip', route: 'off route'}

const fix = computed(() => props.overview.fix)
const buses = (list: string[]) => list.slice(0, BUSES_SHOWN).join(', ') + (list.length > BUSES_SHOWN ? ` +${list.length - BUSES_SHOWN}` : '')

const liveRows = computed(() => {
  const week = props.overview.accuracy.week
  return [...week.filter((a) => a.label === 'all'), ...week.filter((a) => a.label !== 'all')]
})

const accuracyBars = computed<DayBar[]>(() =>
  props.overview.accuracy.daily.map((d) => ({
    date: d.date,
    value: d.n ? d.within_1 : 0,
    caption: d.n ? `${dayName(d.date)} · ${pct(d.within_1)} of ${num(d.n)}` : `${dayName(d.date)} · none`,
  })),
)
</script>

<template>
  <div class="admin-cols">
    <div>
      <AdminCard v-if="realtime" title="Lines with wrong times" :scope="name" hint="7 days">
        <div v-if="fix.lines.length" class="admin-scroll">
          <table class="admin-table">
            <thead>
              <tr>
                <th>Line</th>
                <th>Off by 1+ min</th>
                <th>Off on average</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="l in fix.lines" :key="l.id">
                <td><LineBadge plain :name="l.name" :color="l.color" /></td>
                <td>{{ num(l.off) }} of {{ num(l.n) }}</td>
                <td>{{ minSec(l.avg_abs_sec) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="admin-note">None</p>
      </AdminCard>

      <AdminCard v-if="realtime" title="Buses not on the map" :scope="name" hint="7 days">
        <div v-if="fix.buses.length" class="admin-scroll">
          <table class="admin-table">
            <thead>
              <tr>
                <th>Line</th>
                <th>Why</th>
                <th>Of its buses</th>
                <th>Bus numbers</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="b in fix.buses" :key="`${b.reason}|${b.id}`">
                <td><LineBadge plain :name="b.name" :color="b.color" /></td>
                <td class="strong">{{ busReasons[b.reason] ?? b.reason }}</td>
                <td>{{ pct(b.share) }}</td>
                <td>{{ buses(b.buses) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="admin-note">None</p>
      </AdminCard>

      <AdminCard title="Searches with no result" :scope="name" hint="7 days">
        <div v-if="fix.searches.length" class="admin-scroll">
          <table class="admin-table">
            <thead>
              <tr>
                <th>Typed</th>
                <th>Times</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in fix.searches" :key="s.query">
                <td class="strong wrap">{{ s.query }}</td>
                <td>{{ num(s.count) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="admin-note">None</p>
      </AdminCard>

      <AdminCard title="Trips with no bus" :scope="name" hint="7 days">
        <div v-if="fix.plans.length" class="admin-scroll">
          <table class="admin-table">
            <thead>
              <tr>
                <th>From → to</th>
                <th>Why</th>
                <th>Times</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in fix.plans" :key="`${p.reason}|${p.from}|${p.to}`">
                <td class="strong wrap">{{ p.from }} → {{ p.to }}</td>
                <td>{{ planReasons[p.reason] ?? p.reason }}</td>
                <td>{{ num(p.count) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="admin-note">None</p>
      </AdminCard>
    </div>

    <div>
      <AdminCard v-if="realtime" title="Accuracy" :scope="name" hint="7 days">
        <div class="admin-scroll">
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
        </div>
        <template v-if="accuracyBars.length">
          <AdminSub :text="`Last ${accuracyBars.length} days`" unit="within 1 min" />
          <AdminDayBars :days="accuracyBars" :max="1" />
        </template>
      </AdminCard>
    </div>
  </div>
</template>
