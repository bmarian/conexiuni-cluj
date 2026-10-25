<script setup lang="ts">
import { computed, ref } from "vue";
import AdminCard from "@/components/admin/AdminCard.vue";
import AdminCities from "@/components/admin/AdminCities.vue";
import AdminScope from "@/components/admin/AdminScope.vue";
import AdminStats from "@/components/admin/AdminStats.vue";
import type { Site } from "@/components/admin/types.ts";
import { ago, bytes, dur, num, pct } from "@/utils/adminFormat.ts";
import { ExpandMoreIcon, PublicIcon, SettingsIcon, WarningAmberIcon } from "@/utils/icons.ts";

const FAILURES_SHOWN = 5;

const props = defineProps<{ site: Site; city: string; clock: number }>();
const emit = defineEmits<{ pick: [slug: string]; changed: [] }>();

const allFailures = ref(false);
const failures = computed(() =>
  allFailures.value
    ? props.site.server.problems
    : props.site.server.problems.slice(0, FAILURES_SHOWN),
);

const server = computed(() => props.site.server);
const uptime = computed(() =>
  dur((props.clock - new Date(server.value.started_at).getTime()) / 1000),
);

const stats = computed(() => [
  { value: num(props.site.cities.reduce((n, c) => n + c.watching, 0)), label: "watching" },
  { value: num(props.site.cities.reduce((n, c) => n + c.people_today, 0)), label: "people today" },
  { value: num(server.value.requests), label: "requests" },
]);

const diskUsed = computed(() =>
  server.value.disk_total ? 1 - server.value.disk_free / server.value.disk_total : null,
);
</script>

<template>
  <section class="admin-part">
    <AdminScope :icon="PublicIcon" tone="primary" title="App">
      {{ site.cities.length }} cities · up {{ uptime }}
    </AdminScope>

    <AdminStats :stats="stats" :problems="site.problems" />

    <AdminCard title="Cities">
      <div class="cities" role="list">
        <div class="head" aria-hidden="true">
          <span></span><span>Watching</span><span>Today</span>
        </div>
        <button
          v-for="c in site.cities"
          :key="c.slug"
          type="button"
          role="listitem"
          class="city state"
          :class="{ 'is-current': c.slug === city }"
          :aria-current="c.slug === city"
          @click="emit('pick', c.slug)"
        >
          <span class="name">
            <span class="truncate">{{ c.name }}</span>
            <span v-if="c.problems.length" class="problems" :title="c.problems.join('\n')">
              <WarningAmberIcon class="m-icon" />{{ c.problems.length }}
            </span>
          </span>
          <span class="count">{{ num(c.watching) }}</span>
          <span class="count">{{ num(c.people_today) }}</span>
        </button>
      </div>
    </AdminCard>

    <AdminCard title="Server">
      <dl class="admin-rows">
        <div>
          <dt>Up</dt>
          <dd>
            {{ uptime
            }}<template v-if="server.revision">
              · {{ server.revision }}{{ server.modified ? "+" : "" }}</template
            >
          </dd>
        </div>
        <div>
          <dt>Requests today</dt>
          <dd>
            {{ num(server.requests) }} ·
            <span :class="{ 'admin-bad': server.errors > 0 }">{{ num(server.errors) }} errors</span>
          </dd>
        </div>
        <div>
          <dt>Memory</dt>
          <dd>{{ bytes(server.heap_bytes) }}</dd>
        </div>
        <div>
          <dt>Databases</dt>
          <dd>{{ bytes(server.db_bytes) }}</dd>
        </div>
        <div v-if="server.sync">
          <dt>Sync</dt>
          <dd>
            {{ num(server.sync.chains) }} groups · {{ num(server.sync.devices.web ?? 0) }} web,
            {{ num(server.sync.devices.android ?? 0) }} phone,
            {{ num(server.sync.devices.wear ?? 0) }} watch ·
            {{ bytes(server.sync.db_bytes) }}
          </dd>
        </div>
        <div v-if="diskUsed != null">
          <dt>Disk</dt>
          <dd>{{ pct(diskUsed) }} used · {{ bytes(server.disk_free) }} free</dd>
        </div>
      </dl>
    </AdminCard>

    <AdminCard title="Failures" hint="24 h">
      <ul v-if="server.problems.length" class="log">
        <li v-for="p in failures" :key="p.text">
          <span class="when"
            >{{ ago(p.last, clock)
            }}<template v-if="p.count > 1"> · {{ num(p.count) }}×</template></span
          >
          <code>{{ p.text }}</code>
        </li>
      </ul>
      <p v-else class="admin-note">None</p>
      <button
        v-if="server.problems.length > FAILURES_SHOWN"
        type="button"
        class="btn btn-text more"
        @click="allFailures = !allFailures"
      >
        {{ allFailures ? "Fewer" : `All ${server.problems.length}` }}
      </button>
    </AdminCard>

    <h3 class="admin-settings"><SettingsIcon class="m-icon" />App settings</h3>
    <details class="admin-card admin-fold">
      <summary class="state">
        <span>Cities in the app</span><ExpandMoreIcon class="m-icon" />
      </summary>
      <AdminCities @changed="emit('changed')" />
    </details>
  </section>
</template>

<style scoped>
.cities {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  padding: 0 4px;
}

.head {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: subgrid;
  column-gap: 16px;
  padding: 4px 12px;
  font-size: 11px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  color: var(--md-on-surface-variant);
  text-align: right;
}

.city {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: subgrid;
  align-items: center;
  column-gap: 16px;
  min-height: 48px;
  padding: 0 12px;
  border: 0;
  border-radius: 12px;
  background: transparent;
  color: var(--md-on-surface);
  font: inherit;
  font-size: 14px;
  line-height: 20px;
  letter-spacing: 0.25px;
  text-align: left;
  cursor: pointer;
  transition: background-color 150ms linear;
}

.city.is-current {
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
}

.name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.problems {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  color: var(--md-error);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.problems .m-icon {
  width: 16px;
  height: 16px;
}

.count {
  min-width: 2.5rem;
  text-align: right;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.log {
  margin: 0;
  padding: 0 0 4px;
  list-style: none;
}

.log li {
  padding: 8px 16px;
}

.when {
  display: block;
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  color: var(--md-on-surface-variant);
}

.more {
  margin-left: 4px;
}

.log code {
  display: block;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 16px;
  overflow-wrap: anywhere;
}
</style>
