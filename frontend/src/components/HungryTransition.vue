<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settings'
import HungryGhostIcon from './HungryGhostIcon.vue'

const settings = useSettingsStore()
const router = useRouter()

const visible = ref(false)
const ltr = ref(true)
const hunted = ref(false)
const drawerWidth = ref(300)

let timer: ReturnType<typeof setTimeout> | null = null

// Hunter mode: colored ghosts fleeing. Hunted mode: gray ghosts chasing.
const HUNTER_GHOSTS = [
  { color: '#60a5fa', pupilColor: '#1e3a8a' },
  { color: '#818cf8', pupilColor: '#312e81' },
  { color: '#a78bfa', pupilColor: '#4c1d95' },
]
const HUNTED_GHOSTS = [
  { color: '#94a3b8', pupilColor: '#334155' },
  { color: '#94a3b8', pupilColor: '#334155' },
  { color: '#94a3b8', pupilColor: '#334155' },
]

const ghosts = computed(() => hunted.value ? HUNTED_GHOSTS : HUNTER_GHOSTS)

// Chomper is first in the row when it's the trailing chaser (hunter+ltr or hunted+rtl)
const chomperFirst = computed(() => hunted.value !== ltr.value)

// Ghosts look toward whoever is behind them (the pursuer)
const ghostLookRight = computed(() => hunted.value === ltr.value)

const removeGuard = router.beforeEach((to, from) => {
  if (from.matched.length === 0) return
  if (!settings.easterEggActive) return

  ltr.value = Math.random() > 0.5
  hunted.value = Math.random() < 0.2

  const drawerEl = document.querySelector('.app-drawer')
  drawerWidth.value = drawerEl?.getBoundingClientRect().width ?? 300

  if (timer) clearTimeout(timer)
  visible.value = true
  timer = setTimeout(() => { visible.value = false }, 950)
})

onUnmounted(() => {
  removeGuard()
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <Teleport to=".app-drawer">
    <div
      v-if="visible"
      class="chase-overlay"
      :style="{ '--dw': drawerWidth + 'px' }"
      aria-hidden="true"
    >
      <div class="chase-row" :class="ltr ? 'chase-ltr' : 'chase-rtl'">
        <template v-if="chomperFirst">
          <div
            class="chase-chomper hungry-chomp"
            :class="{ 'chase-chomper-flip': !ltr }"
          ></div>
          <HungryGhostIcon
            v-for="(g, i) in ghosts"
            :key="i"
            class="chase-ghost"
            :style="{ '--gi': i }"
            :color="g.color"
            :pupil-color="g.pupilColor"
            :look-right="ghostLookRight"
          />
        </template>
        <template v-else>
          <HungryGhostIcon
            v-for="(g, i) in ghosts"
            :key="i"
            class="chase-ghost"
            :style="{ '--gi': i }"
            :color="g.color"
            :pupil-color="g.pupilColor"
            :look-right="ghostLookRight"
          />
          <div
            class="chase-chomper hungry-chomp"
            :class="{ 'chase-chomper-flip': !ltr }"
          ></div>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.chase-overlay {
  position: absolute;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
  overflow: hidden;
}

.chase-row {
  position: absolute;
  top: 42%;
  left: 0;
  display: flex;
  align-items: flex-end;
  gap: 10px;
}

.chase-ltr {
  animation: chase-run-ltr 0.9s cubic-bezier(0.4, 0, 0.6, 1) forwards;
}
.chase-rtl {
  animation: chase-run-rtl 0.9s cubic-bezier(0.4, 0, 0.6, 1) forwards;
}

.chase-ghost {
  width: 28px;
  height: 37px;
  flex-shrink: 0;
  filter: drop-shadow(0 2px 3px rgba(0,0,0,0.22));
  animation: ghost-bob 0.24s ease-in-out calc(var(--gi, 0) * 0.08s) infinite alternate;
}

.chase-chomper {
  width: 34px;
  height: 34px;
  background: #facc15;
  border-radius: 50%;
  flex-shrink: 0;
  filter: drop-shadow(0 2px 5px rgba(0,0,0,0.25));
}

.chase-chomper-flip {
  transform: scaleX(-1);
}

@keyframes ghost-bob {
  from { transform: translateY(0); }
  to   { transform: translateY(-6px); }
}

@keyframes chase-run-ltr {
  from { transform: translateX(-260px); }
  to   { transform: translateX(calc(var(--dw, 300px) + 20px)); }
}

@keyframes chase-run-rtl {
  from { transform: translateX(calc(var(--dw, 300px) + 20px)); }
  to   { transform: translateX(-260px); }
}
</style>
