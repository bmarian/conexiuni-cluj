<script setup lang="ts">
import {computed, ref} from 'vue'
import AdminCalendar from '@/components/admin/AdminCalendar.vue'
import AdminCard from '@/components/admin/AdminCard.vue'
import AdminCities from '@/components/admin/AdminCities.vue'
import AdminHiddenLines from '@/components/admin/AdminHiddenLines.vue'
import AdminHiddenStops from '@/components/admin/AdminHiddenStops.vue'
import ListItem from '@/components/ui/ListItem.vue'
import ToggleRow from '@/components/ui/ToggleRow.vue'
import {lineStatsOn, setLineStats, setStopControls, stopControlsOn} from '@/composables/useAdmin.ts'
import {LogoutIcon, QueryStatsIcon, VisibilityOffIcon} from '@/utils/icons.ts'

defineProps<{city: string; name: string}>()
const emit = defineEmits<{saved: []; changed: []; logout: []}>()

function deviceSwitch(read: () => boolean, write: (on: boolean) => void) {
  const on = ref(read())
  return computed({
    get: () => on.value,
    set: (next: boolean) => {
      write(next)
      on.value = next
    },
  })
}

const lineStats = deviceSwitch(lineStatsOn, setLineStats)
const stopControls = deviceSwitch(stopControlsOn, setStopControls)
</script>

<template>
  <div class="admin-cols">
    <div>
      <div class="tt-head">
        <span class="text-title-small text-on-surface-variant">{{ name }}</span>
        <span class="divider"></span>
      </div>
      <AdminCard title="Timetable calendar">
        <AdminCalendar :key="city" :city="city" @saved="emit('saved')" />
      </AdminCard>
      <AdminCard title="Hidden lines">
        <AdminHiddenLines :key="city" :city="city" />
      </AdminCard>
      <AdminCard title="Hidden stops">
        <AdminHiddenStops :key="city" :city="city" />
      </AdminCard>
    </div>

    <div>
      <div class="tt-head">
        <span class="text-title-small text-on-surface-variant">Whole app</span>
        <span class="divider"></span>
      </div>
      <AdminCard title="Cities in the app">
        <AdminCities @changed="emit('changed')" />
      </AdminCard>
      <div class="tt-head">
        <span class="text-title-small text-on-surface-variant">This device</span>
        <span class="divider"></span>
      </div>
      <div class="admin-card">
        <ToggleRow :icon="QueryStatsIcon" title="Line stats" :checked="lineStats" @change="lineStats = $event" />
        <ToggleRow :icon="VisibilityOffIcon" title="Hide stops and lines" :checked="stopControls" @change="stopControls = $event" />
        <ListItem :icon="LogoutIcon" title="Log out" @click="emit('logout')" />
      </div>
    </div>
  </div>
</template>
