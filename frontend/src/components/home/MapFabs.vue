<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useKbdShortcuts} from '@/composables/useKeyboardNav.ts'
import {useMapStore} from '@/stores/map.ts'
import {useSettingsStore} from '@/stores/settings'
import {cityInfo} from '@/utils/city.ts'
import {CropFreeIcon, MyLocationIcon, TrafficIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const mapStore = useMapStore()
const settings = useSettingsStore()
const spins = ref(0)
const hasLines = computed(() => mapStore.shapesToDisplay.length > 0)
const canTraffic = computed(() => hasLines.value && !!cityInfo()?.traffic)

function center() {
  spins.value++
  mapStore.centerRequest++
}

function toggleTraffic() {
  settings.setTraffic(!settings.traffic)
}

useKbdShortcuts(
  {
    T: () => {
      if (canTraffic.value) toggleTraffic()
    },
  },
  {global: true},
)
</script>

<template>
  <div class="map-fabs">
    <Transition name="fab">
      <button
        v-if="canTraffic"
        type="button"
        class="fab fab-small squish"
        :class="{'is-on': settings.traffic}"
        :aria-label="t('traffic')"
        :aria-pressed="settings.traffic"
        :title="t('traffic')"
        @click="toggleTraffic"
      >
        <TrafficIcon class="m-icon" />
      </button>
    </Transition>
    <Transition name="fab">
      <button v-if="hasLines" type="button" class="fab fab-small squish" :aria-label="t('fitRoutes')" :title="t('fitRoutes')" @click="mapStore.fitRequest++">
        <CropFreeIcon class="m-icon" />
      </button>
    </Transition>
    <button type="button" class="fab squish text-primary" :aria-label="t('Center')" :title="t('Center')" @click="center">
      <MyLocationIcon class="m-icon spin" :style="{rotate: `${spins * 90}deg`}" />
    </button>
  </div>
</template>

<style scoped>
.map-fabs {
  --squish: 0.85;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 12px;
  pointer-events: none;
}

.map-fabs > * {
  pointer-events: auto;
}

.fab.text-primary {
  color: var(--md-primary);
}

.fab.is-on {
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
}

.spin {
  transition: rotate var(--spring-pop-ms) var(--spring-pop);
}

.fab-enter-active {
  transition:
    scale var(--spring-pop-ms) var(--spring-pop),
    opacity 130ms linear;
}

.fab-leave-active {
  transition:
    scale 130ms ease-in,
    opacity 130ms linear;
}

.fab-enter-from,
.fab-leave-to {
  scale: 0.4;
  opacity: 0;
}
</style>
