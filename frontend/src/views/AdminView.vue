<script setup lang="ts">
import {computed, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useRoute, useRouter} from 'vue-router'
import AdminCityMenu from '@/components/admin/AdminCityMenu.vue'
import AdminPeople from '@/components/admin/AdminPeople.vue'
import AdminQuality from '@/components/admin/AdminQuality.vue'
import AdminSettings from '@/components/admin/AdminSettings.vue'
import AdminStatus from '@/components/admin/AdminStatus.vue'
import '@/components/admin/admin.css'
import ChoiceRow from '@/components/ui/ChoiceRow.vue'
import LiveBadge from '@/components/ui/LiveBadge.vue'
import TimetableBadge from '@/components/ui/TimetableBadge.vue'
import type {Choice} from '@/components/ui/types.ts'
import {useAdmin} from '@/composables/useAdmin.ts'
import {cityPath, reloadCities} from '@/utils/city.ts'
import {clockTime} from '@/utils/adminFormat.ts'
import {CloseIcon, RefreshIcon, WarningAmberIcon} from '@/utils/icons.ts'

type Tab = 'status' | 'people' | 'quality' | 'settings'
const TABS: Tab[] = ['status', 'people', 'quality', 'settings']

useHead(() => ({
  title: 'Admin',
  meta: [{name: 'robots', content: 'noindex,nofollow'}],
}))

const route = useRoute()
const router = useRouter()
const {state, site, overview, city, loading, loadError, clock, load, pick, dismiss, login, logout} = useAdmin()

const token = ref('')
const loginError = ref('')
const scrolled = ref(false)
const scroller = ref<HTMLElement | null>(null)

const tab = computed<Tab>(() => {
  const hash = route.hash.slice(1) as Tab
  return TABS.includes(hash) ? hash : 'status'
})
const summary = computed(() => site.value?.cities.find((c) => c.slug === city.value))
const name = computed(() => summary.value?.name ?? city.value)
const realtime = computed(() => summary.value?.realtime ?? true)
const hasProblems = computed(() => [...(site.value?.problems ?? []), ...(overview.value?.problems ?? [])].some((p) => !p.dismissed))
const tabs = computed<Choice<Tab>[]>(() => [
  {value: 'status', label: 'Status'},
  {value: 'people', label: 'People'},
  {value: 'quality', label: 'Quality'},
  {value: 'settings', label: 'Settings'},
])

const updatedAt = computed(() => (site.value ? clockTime(site.value.generated_at) : ''))
const refreshLabel = computed(() => (loadError.value ? `Refresh failed (${loadError.value})` : `Refresh · updated ${updatedAt.value}`))

function openTab(next: Tab) {
  void router.replace({hash: next === 'status' ? '' : `#${next}`})
  scroller.value?.scrollTo({top: 0})
}

async function submit() {
  const t = token.value.trim()
  if (!t) return
  loginError.value = await login(t)
  if (!loginError.value) token.value = ''
}

async function onCitiesChanged() {
  await reloadCities()
  void load()
}
</script>

<template>
  <Teleport to="body">
    <div ref="scroller" class="admin" @scroll.passive="scrolled = ($event.target as HTMLElement).scrollTop > 0">
      <form v-if="state === 'login'" class="login" @submit.prevent="submit">
        <h1 class="text-headline-small font-bold">Admin</h1>
        <input type="text" name="username" value="admin" autocomplete="username" readonly tabindex="-1" aria-hidden="true" class="hidden-username" />
        <input
          v-model="token"
          class="admin-field"
          type="password"
          name="password"
          autocomplete="current-password"
          spellcheck="false"
          placeholder="Token"
          aria-label="Token"
          autofocus
        />
        <button type="submit" class="btn btn-filled">Unlock</button>
        <p v-if="loginError" class="admin-bad text-body-medium">{{ loginError }}</p>
      </form>

      <template v-else>
        <div class="top" :class="{'is-scrolled': scrolled}">
          <header class="bar">
            <div class="place">
              <AdminCityMenu v-if="site" :cities="site.cities" :city="city" @pick="pick" />
              <h1 v-else class="text-title-large">Admin</h1>
              <template v-if="summary">
                <LiveBadge v-if="realtime" />
                <TimetableBadge v-else />
              </template>
            </div>
            <button type="button" class="btn btn-text refresh" :disabled="loading" :aria-label="refreshLabel" :title="refreshLabel" @click="load">
              <WarningAmberIcon v-if="loadError && !loading" class="m-icon text-error" />
              <RefreshIcon v-else class="m-icon" :class="{'animate-spin motion-reduce:animate-none': loading}" /><span class="time">{{ updatedAt }}</span>
            </button>
            <button type="button" class="icon-btn" aria-label="Close" title="Close" @click="router.push(cityPath('/'))">
              <CloseIcon class="m-icon" />
            </button>
          </header>
          <nav class="tabs">
            <ChoiceRow :options="tabs" :model-value="tab" aria-label="Section" @update:model-value="openTab">
              <template #label="{option, checked}">
                <WarningAmberIcon v-if="option.value === 'status' && hasProblems" class="tab-warn" :class="{'is-checked': checked}" />
                <span class="truncate">{{ option.label }}</span>
              </template>
            </ChoiceRow>
          </nav>
        </div>

        <main v-if="site && overview" class="page">
          <AdminStatus
            v-if="tab === 'status'"
            :site="site"
            :overview="overview"
            :city="city"
            :name="name"
            :realtime="realtime"
            :clock="clock"
            @dismiss="dismiss"
          />
          <AdminPeople v-else-if="tab === 'people'" :overview="overview" :name="name" />
          <AdminQuality v-else-if="tab === 'quality'" :overview="overview" :name="name" :realtime="realtime" />
          <AdminSettings v-else :city="city" :name="name" @saved="load" @changed="onCitiesChanged" @logout="logout" />
        </main>
        <p v-else-if="!loadError" class="admin-note page">Loading…</p>
      </template>
    </div>
  </Teleport>
</template>

<style scoped>
.admin {
  position: fixed;
  inset: 0;
  z-index: 50;
  overflow-y: auto;
  overflow-x: hidden;
  background: var(--md-surface);
  color: var(--md-on-surface);
  -webkit-text-size-adjust: 100%;
}

.login {
  width: min(22rem, calc(100% - 2rem));
  margin: 18vh auto 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.hidden-username {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}

.top {
  position: sticky;
  top: 0;
  z-index: 20;
  padding-top: env(safe-area-inset-top);
  background: var(--md-surface);
  transition: background-color 175ms linear;
}

.top.is-scrolled {
  background: var(--md-surface-container);
}

.bar,
.tabs,
.page {
  max-width: 40rem;
  margin: 0 auto;
}

.bar {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 64px;
  padding: 0 4px 0 16px;
}

.place {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.refresh {
  padding: 0 12px;
  font-variant-numeric: tabular-nums;
}

.tabs {
  padding: 0 16px 8px;
}

.tabs :deep(.choice) {
  gap: 4px;
  padding: 0 4px;
}

.tab-warn {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  color: var(--md-error);
}

.tab-warn.is-checked {
  color: inherit;
}

.page {
  padding: 8px 16px max(32px, env(safe-area-inset-bottom));
}

@media (max-width: 26rem) {
  .refresh {
    padding: 0 8px;
  }

  .time {
    display: none;
  }
}

@media (min-width: 64rem) {
  .bar,
  .tabs,
  .page {
    max-width: 90rem;
  }

  .tabs :deep(.choice-row) {
    max-width: 40rem;
  }
}
</style>
