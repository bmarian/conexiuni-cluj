<script setup lang="ts">
import {computed} from 'vue'
import type {Fix} from '@/components/admin/types.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import {minSec, num, pct} from '@/utils/adminFormat.ts'

const BUSES_SHOWN = 3

const props = defineProps<{fix: Fix}>()

const planReasons: Record<string, string> = {walk: 'long walk', outside: 'outside'}
const busReasons: Record<string, string> = {trip: 'unknown trip', route: 'off route'}

const empty = computed(() => !props.fix.lines.length && !props.fix.buses.length && !props.fix.searches.length && !props.fix.plans.length)

const buses = (list: string[]) => list.slice(0, BUSES_SHOWN).join(', ') + (list.length > BUSES_SHOWN ? ` +${list.length - BUSES_SHOWN}` : '')
</script>

<template>
  <p v-if="empty" class="admin-note">None</p>
  <template v-if="fix.lines.length">
    <SubLabel text="Wrong times" />
    <dl class="admin-rows">
      <div v-for="l in fix.lines" :key="l.id" class="line-row">
        <dt class="name"><LineBadge plain :name="l.name" :color="l.color" /></dt>
        <dd>{{ num(l.off) }} of {{ num(l.n) }} · {{ minSec(l.avg_abs_sec) }}</dd>
      </div>
    </dl>
  </template>
  <template v-if="fix.buses.length">
    <SubLabel text="Buses not placed" />
    <dl class="admin-rows">
      <div v-for="b in fix.buses" :key="`${b.reason}|${b.id}`" class="line-row">
        <dt class="name">
          <LineBadge plain :name="b.name" :color="b.color" />
          <span class="truncate">{{ busReasons[b.reason] ?? b.reason }}</span>
        </dt>
        <dd>{{ pct(b.share) }} · {{ buses(b.buses) }}</dd>
      </div>
    </dl>
  </template>
  <template v-if="fix.searches.length">
    <SubLabel text="Searches with no result" />
    <dl class="admin-rows">
      <div v-for="s in fix.searches" :key="s.query">
        <dt class="name">{{ s.query }}</dt>
        <dd>{{ num(s.count) }}</dd>
      </div>
    </dl>
  </template>
  <template v-if="fix.plans.length">
    <SubLabel text="Plans with no bus" />
    <dl class="admin-rows">
      <div v-for="p in fix.plans" :key="`${p.reason}|${p.from}|${p.to}`">
        <dt class="name">{{ p.from }} → {{ p.to }}</dt>
        <dd>{{ planReasons[p.reason] ?? p.reason }} · {{ num(p.count) }}</dd>
      </div>
    </dl>
  </template>
</template>
