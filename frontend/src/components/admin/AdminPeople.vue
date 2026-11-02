<script setup lang="ts">
import {computed} from 'vue'
import AdminCard from '@/components/admin/AdminCard.vue'
import AdminDayBars, {type DayBar} from '@/components/admin/AdminDayBars.vue'
import AdminStats from '@/components/admin/AdminStats.vue'
import AdminSub from '@/components/admin/AdminSub.vue'
import type {Named, Overview} from '@/components/admin/types.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import {dayName, num, platformLabels} from '@/utils/adminFormat.ts'

const props = defineProps<{overview: Overview; name: string}>()

const people = computed(() => props.overview.people)

const stats = computed(() => [
  {value: num(props.overview.now.people), label: 'watching'},
  {value: num(people.value.today), label: 'people today'},
  {value: num(people.value.new), label: 'new today'},
])

const bars = computed<DayBar[]>(() =>
  people.value.daily.map((d) => {
    const split = (['web', 'android', 'wear'] as const).filter((p) => d[p] > 0).map((p) => `${platformLabels[p]} ${num(d[p])}`)
    return {date: d.date, value: d.total, caption: `${dayName(d.date)} · ${num(d.total)}${split.length ? ` (${split.join(', ')})` : ''}`}
  }),
)

const versionsToday = computed(() => people.value.versions.filter((v) => v.today > 0))

const sections = computed(() => [
  {title: 'On screens', hint: 'now', unit: 'people', lines: props.overview.now.lines, stops: props.overview.now.stops},
  {title: 'Most opened', hint: 'today', unit: 'opens', lines: people.value.lines, stops: people.value.stops},
])
const hasAny = (lines: Named[], stops: Named[]) => lines.length > 0 || stops.length > 0
</script>

<template>
  <div class="admin-cols">
    <div>
      <AdminStats :stats="stats" />
      <AdminCard title="People" :scope="name">
        <div class="admin-scroll">
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
                <td>{{ num(people.today) }}</td>
                <td>{{ num(people.last_week) }}</td>
              </tr>
              <tr v-for="p in people.platforms" :key="p.platform">
                <td>{{ platformLabels[p.platform] ?? p.platform }}</td>
                <td>{{ num(p.today) }}</td>
                <td>{{ num(p.last_week) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <template v-if="bars.length">
          <AdminSub :text="`Last ${bars.length} days`" unit="people" />
          <AdminDayBars :days="bars" />
        </template>
        <AdminSub text="What they did today" />
        <dl class="admin-rows">
          <div>
            <dt>Trips planned</dt>
            <dd>{{ num(people.plans) }}</dd>
          </div>
          <div>
            <dt>Place searches</dt>
            <dd>{{ num(people.places) }}</dd>
          </div>
          <div>
            <dt>Web installs</dt>
            <dd>{{ num(people.installs) }}</dd>
          </div>
        </dl>
        <template v-if="versionsToday.length">
          <AdminSub text="App versions today" unit="people" />
          <dl class="admin-rows">
            <div v-for="v in versionsToday" :key="v.platform + v.version">
              <dt>{{ platformLabels[v.platform] ?? v.platform }} {{ v.version }}</dt>
              <dd>{{ num(v.today) }}</dd>
            </div>
          </dl>
        </template>
      </AdminCard>
    </div>

    <div>
      <AdminCard v-for="s in sections" :key="s.title" :title="s.title" :scope="name" :hint="s.hint">
        <p v-if="!hasAny(s.lines, s.stops)" class="admin-note">None</p>
        <template v-if="s.lines.length">
          <AdminSub text="Lines" :unit="s.unit" />
          <p class="admin-lines">
            <span v-for="l in s.lines" :key="l.id"><LineBadge plain :name="l.name" :color="l.color" />{{ num(l.count) }}</span>
          </p>
        </template>
        <template v-if="s.stops.length">
          <AdminSub text="Stops" :unit="s.unit" />
          <dl class="admin-rows">
            <div v-for="stop in s.stops" :key="stop.id">
              <dt class="name">{{ stop.name }}</dt>
              <dd>{{ num(stop.count) }}</dd>
            </div>
          </dl>
        </template>
      </AdminCard>
    </div>
  </div>
</template>
