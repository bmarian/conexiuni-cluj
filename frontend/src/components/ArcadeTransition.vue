<script setup lang="ts">
import {computed, onUnmounted, ref} from 'vue'
import {useRouter} from 'vue-router'
import {useSettingsStore} from '@/stores/settings'
import ArcadeGhostIcon from './icons/ArcadeGhostIcon.vue'

type Direction = 'ltr' | 'rtl' | 'ttb' | 'btt'

const settings = useSettingsStore()
const router = useRouter()

const visible = ref(false)
const direction = ref<Direction>('ltr')
const hunted = ref(false)
const drawerWidth = ref(300)
const drawerHeight = ref(500)

let timer: ReturnType<typeof setTimeout> | null = null

const HUNTER_GHOSTS = [
  {color: '#60a5fa', pupilColor: '#1e3a8a'},
  {color: '#818cf8', pupilColor: '#312e81'},
  {color: '#a78bfa', pupilColor: '#4c1d95'},
]
const HUNTED_GHOSTS = [
  {color: '#94a3b8', pupilColor: '#334155'},
  {color: '#94a3b8', pupilColor: '#334155'},
  {color: '#94a3b8', pupilColor: '#334155'},
]

const ghosts = computed(() => hunted.value ? HUNTED_GHOSTS : HUNTER_GHOSTS)

const isVertical = computed(() => direction.value === 'ttb' || direction.value === 'btt')

// RTL reverses visual order so arcade needs to be at the back (not front) to stay trailing.
const arcadeFirst = computed(() =>
  direction.value === 'rtl' ? hunted.value : !hunted.value
)

// Pupils look toward whoever is chasing.
const ghostLookRight = computed(() => {
  if (isVertical.value) return false
  return hunted.value === (direction.value === 'ltr')
})

const arcadeRotateClass = computed(() => ({
  'chase-arcade-flip': direction.value === 'rtl',
  'chase-arcade-down': direction.value === 'ttb',
  'chase-arcade-up': direction.value === 'btt',
}))

const removeGuard = router.beforeEach((to, from) => {
  if (from.matched.length === 0) return
  if (!settings.arcadeActive) return

  const dirs: Direction[] = ['ltr', 'rtl', 'ttb', 'btt']
  direction.value = dirs[Math.floor(Math.random() * dirs.length)]!
  hunted.value = Math.random() < 0.2

  const drawerEl = document.querySelector('.app-drawer')
  const rect = drawerEl?.getBoundingClientRect()
  drawerWidth.value = rect?.width ?? 300
  drawerHeight.value = rect?.height ?? 500

  if (timer) clearTimeout(timer)
  visible.value = true
  timer = setTimeout(() => {
    visible.value = false
  }, 1850)
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
      :style="{ '--dw': drawerWidth + 'px', '--dh': drawerHeight + 'px' }"
      aria-hidden="true"
    >
      <div class="chase-row" :class="`chase-${direction}`">
        <template v-if="arcadeFirst">
          <div class="chase-arcade arcade-chomp" :class="arcadeRotateClass"></div>
          <ArcadeGhostIcon
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
          <ArcadeGhostIcon
            v-for="(g, i) in ghosts"
            :key="i"
            class="chase-ghost"
            :style="{ '--gi': i }"
            :color="g.color"
            :pupil-color="g.pupilColor"
            :look-right="ghostLookRight"
          />
          <div class="chase-arcade arcade-chomp" :class="arcadeRotateClass"></div>
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
  display: flex;
}

.chase-ltr,
.chase-rtl {
  top: 42%;
  left: 0;
  flex-direction: row;
  align-items: flex-end;
  gap: 10px;
}

.chase-ltr {
  animation: chase-run-ltr 1.8s linear forwards;
}

.chase-rtl {
  animation: chase-run-rtl 1.8s linear forwards;
}

.chase-ttb,
.chase-btt {
  top: 0;
  left: calc(50% - 17px);
  align-items: center;
  gap: 8px;
}

.chase-ttb {
  flex-direction: column;
  animation: chase-run-ttb 1.8s linear forwards;
}

.chase-btt {
  flex-direction: column-reverse;
  animation: chase-run-btt 1.8s linear forwards;
}

.chase-ghost {
  width: 28px;
  height: 37px;
  flex-shrink: 0;
  filter: drop-shadow(0 2px 3px rgba(0, 0, 0, 0.22));
  animation: ghost-bob 0.4s ease-in-out calc(var(--gi, 0) * 0.13s) infinite alternate;
}

.chase-arcade {
  width: 34px;
  height: 34px;
  background: #facc15;
  border-radius: 50%;
  flex-shrink: 0;
  filter: drop-shadow(0 2px 5px rgba(0, 0, 0, 0.25));
}

.chase-arcade-flip {
  transform: scaleX(-1);
}

.chase-arcade-down {
  transform: rotate(90deg);
}

.chase-arcade-up {
  transform: rotate(-90deg);
}

@keyframes ghost-bob {
  from {
    transform: translateY(0);
  }
  to {
    transform: translateY(-6px);
  }
}

@keyframes chase-run-ltr {
  from {
    transform: translateX(-260px);
  }
  to {
    transform: translateX(calc(var(--dw, 300px) + 20px));
  }
}

@keyframes chase-run-rtl {
  from {
    transform: translateX(calc(var(--dw, 300px) + 20px));
  }
  to {
    transform: translateX(-260px);
  }
}

@keyframes chase-run-ttb {
  from {
    transform: translateY(-260px);
  }
  to {
    transform: translateY(calc(var(--dh, 500px) + 20px));
  }
}

@keyframes chase-run-btt {
  from {
    transform: translateY(calc(var(--dh, 500px) + 20px));
  }
  to {
    transform: translateY(-260px);
  }
}
</style>
