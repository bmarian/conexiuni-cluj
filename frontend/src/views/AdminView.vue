<script setup lang="ts">
import {computed, nextTick, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useRouter} from 'vue-router'
import AdminCity from '@/components/admin/AdminCity.vue'
import AdminSite from '@/components/admin/AdminSite.vue'
import '@/components/admin/admin.css'
import {useAdmin} from '@/composables/useAdmin.ts'
import {cityPath, reloadCities} from '@/utils/city.ts'
import {clockTime} from '@/utils/adminFormat.ts'
import {CloseIcon, LogoutIcon, RefreshIcon} from '@/utils/icons.ts'

useHead(() => ({
  title: 'Admin',
  meta: [{name: 'robots', content: 'noindex,nofollow'}],
}))

const router = useRouter()
const {state, site, overview, city, loading, loadError, clock, load, pick, login, logout} = useAdmin()

const token = ref('')
const loginError = ref('')
const scrolled = ref(false)
const cityPart = ref<HTMLElement | null>(null)

const updatedAt = computed(() => site.value ? clockTime(site.value.generated_at) : '')

async function submit() {
  const t = token.value.trim()
  if (!t) return
  loginError.value = await login(t)
  if (!loginError.value) token.value = ''
}

// On one column the city part sits below; bring it up when a city is picked from the list.
async function pickFromList(slug: string) {
  await pick(slug)
  await nextTick()
  const top = cityPart.value?.getBoundingClientRect().top ?? 0
  if (top > window.innerHeight * 0.5) cityPart.value?.scrollIntoView({block: 'start', behavior: 'smooth'})
}

async function onCitiesChanged() {
  await reloadCities()
  void load()
}
</script>

<template>
  <Teleport to="body">
    <div class="admin" @scroll.passive="scrolled = ($event.target as HTMLElement).scrollTop > 0">
      <form v-if="state === 'login'" class="login" @submit.prevent="submit">
        <h1 class="text-headline-small font-bold">Admin</h1>
        <input
          type="text"
          name="username"
          value="admin"
          autocomplete="username"
          readonly
          tabindex="-1"
          aria-hidden="true"
          class="hidden-username"
        />
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
        <header class="bar" :class="{ 'is-scrolled': scrolled }">
          <h1 class="text-title-large flex-1">Admin</h1>
          <button v-if="site" type="button" class="btn btn-text" :disabled="loading" aria-label="Refresh" @click="load">
            <RefreshIcon class="m-icon" :class="{ 'animate-spin motion-reduce:animate-none': loading }"/>{{ updatedAt }}
          </button>
          <button type="button" class="icon-btn" aria-label="Log out" title="Log out" @click="logout">
            <LogoutIcon class="m-icon"/>
          </button>
          <button type="button" class="icon-btn" aria-label="Close" title="Close" @click="router.push(cityPath('/'))">
            <CloseIcon class="m-icon"/>
          </button>
        </header>
        <p v-if="loadError" class="refresh-failed">
          {{ site ? `Refresh failed (${loadError})` : `Could not load (${loadError})` }}
          <button v-if="!site" type="button" class="btn btn-text" @click="load">Try again</button>
        </p>

        <main v-if="site" class="page">
          <AdminSite :site="site" :city="city" :clock="clock" @pick="pickFromList" @changed="onCitiesChanged"/>
          <div ref="cityPart" class="city-part">
            <AdminCity :overview="overview" :cities="site.cities" :city="city" :clock="clock" @pick="pick" @saved="load"/>
          </div>
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

.bar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  gap: 4px;
  height: 64px;
  padding: env(safe-area-inset-top) 4px 0 16px;
  box-sizing: content-box;
  background: var(--md-surface);
  transition: background-color 200ms linear;
}

.bar.is-scrolled {
  background: var(--md-surface-container);
}

.bar .btn-text {
  padding: 0 12px;
  font-variant-numeric: tabular-nums;
}

.refresh-failed {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 16px;
  font-size: 14px;
  line-height: 20px;
  color: var(--md-error);
}

.page {
  max-width: 40rem;
  margin: 0 auto;
  padding: 0 16px max(32px, env(safe-area-inset-bottom));
}

.city-part {
  margin-top: 24px;
  scroll-margin-top: 64px;
}

@media (min-width: 64rem) {
  .page {
    display: grid;
    grid-template-columns: 22rem minmax(0, 1fr);
    align-items: start;
    gap: 24px;
    max-width: 90rem;
  }

  .city-part {
    margin-top: 0;
  }
}
</style>
