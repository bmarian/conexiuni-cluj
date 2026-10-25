<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {network} from '@/utils/network.ts'
import {hasTimetableEntries} from '@/utils/time.ts'
import {groupByMode} from '@/utils/modes.ts'
import {cityParams, cityPath} from '@/utils/city.ts'
import {useOpenRoute} from '@/composables/useOpenRoute.ts'
import SubLabel from '@/components/ui/SubLabel.vue'
import ListRow from '@/components/ui/ListRow.vue'
import RouteRow from '@/components/rows/RouteRow.vue'

const {t} = useI18n()
const router = useRouter()
const {openingRouteId, openRoute} = useOpenRoute()

const groups = computed(() => groupByMode((network.value?.bundle.routes ?? []).filter((route) => route.timetable && hasTimetableEntries(route.timetable))))
</script>

<template>
  <section v-if="groups.length" class="pt-2" data-kbd-section="all-lines" data-kbd-entry="1">
    <h1 class="text-title-large font-bold px-4 pt-1 pb-1">{{ t('linesIn', cityParams()) }}</h1>
    <template v-for="group in groups" :key="group.mode">
      <SubLabel :text="t(`modeLines_${group.mode}`)" class="pt-2" />
      <RouteRow
        v-for="route in group.lines"
        :key="route.route_id"
        :name="route.route_short_name"
        :color="route.route_color"
        :title="route.route_long_name"
        :busy="openingRouteId === route.route_id"
        :href="cityPath(`/route/${route.route_id}/0`)"
        :data-kbd-item="`line-${route.route_id}`"
        @open="openRoute(route)"
      />
    </template>
    <ListRow
      class="mt-2"
      :title="t('destAll', cityParams())"
      :href="cityPath('/destinations')"
      chevron
      data-kbd-item="all-destinations"
      @open="router.push({name: 'destinations'})"
    />
  </section>
</template>
