<script setup lang="ts">
import { onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settings'

const settings = useSettingsStore()
const router = useRouter()

const visible = ref(false)
const goRight = ref(true)
const drawerWidth = ref(300)
const drawerHeight = ref(500)

const PAPER_COUNT = 8

function paperStyle(i: number) {
  // spread papers vertically, stagger their launch times and rotations
  const yPct = 5 + ((i - 1) / (PAPER_COUNT - 1)) * 82
  const delay = ((i - 1) * 0.1).toFixed(2)
  const rStart = (-28 + ((i * 37) % 60)).toFixed(0)
  const rEnd = goRight.value
    ? (10 + ((i * 23) % 25)).toFixed(0)
    : (-10 - ((i * 23) % 25)).toFixed(0)
  const dur = (1.25 + (i % 3) * 0.18).toFixed(2)

  return {
    top: `${yPct}%`,
    animationDelay: `${delay}s`,
    animationName: goRight.value ? 'wind-blow-right' : 'wind-blow-left',
    animationDuration: `${dur}s`,
    animationTimingFunction: 'ease-in',
    animationFillMode: 'both',
    '--r-start': `${rStart}deg`,
    '--r-end': `${rEnd}deg`,
  }
}

let timer: ReturnType<typeof setTimeout> | null = null

const removeGuard = router.beforeEach((to, from) => {
  if (from.matched.length === 0) return
  if (!settings.traditionalActive) return

  goRight.value = Math.random() < 0.5

  const drawerEl = document.querySelector('.app-drawer')
  const rect = drawerEl?.getBoundingClientRect()
  drawerWidth.value = rect?.width ?? 300
  drawerHeight.value = rect?.height ?? 500

  if (timer) clearTimeout(timer)
  visible.value = true
  timer = setTimeout(() => { visible.value = false }, 1900)
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
      class="wind-overlay"
      :style="{ '--dw': drawerWidth + 'px', '--dh': drawerHeight + 'px' }"
      aria-hidden="true"
    >
      <div
        v-for="i in PAPER_COUNT"
        :key="i"
        class="wind-paper"
        :style="paperStyle(i)"
      >
        <!-- Page with dog-eared corner, a few ruled lines, and a red decorative border -->
        <svg viewBox="0 0 22 28" width="22" height="28" xmlns="http://www.w3.org/2000/svg" fill="none">
          <path d="M0 0 L17 0 L22 5 L22 28 L0 28 Z" fill="#FDF5E6" stroke="#C41E3A" stroke-width="1.2"/>
          <!-- folded corner -->
          <path d="M17 0 L17 5 L22 5" fill="#F0DFC0" stroke="#C41E3A" stroke-width="1"/>
          <!-- ruled lines -->
          <line x1="3" y1="10" x2="19" y2="10" stroke="#D4A017" stroke-width="0.7" opacity="0.6"/>
          <line x1="3" y1="14" x2="19" y2="14" stroke="#D4A017" stroke-width="0.7" opacity="0.6"/>
          <line x1="3" y1="18" x2="14" y2="18" stroke="#D4A017" stroke-width="0.7" opacity="0.6"/>
          <!-- small red diamond ornament -->
          <path d="M11 21 L13 23 L11 25 L9 23 Z" fill="#C41E3A" opacity="0.5"/>
        </svg>
      </div>
    </div>
  </Teleport>
</template>

<style>
@keyframes wind-blow-right {
  0% {
    transform: translateX(-80px) rotate(var(--r-start));
    opacity: 1;
  }
  80% { opacity: 1; }
  100% {
    transform: translateX(calc(var(--dw, 300px) + 80px)) rotate(var(--r-end));
    opacity: 0;
  }
}

@keyframes wind-blow-left {
  0% {
    transform: translateX(calc(var(--dw, 300px) + 80px)) rotate(var(--r-start));
    opacity: 1;
  }
  80% { opacity: 1; }
  100% {
    transform: translateX(-80px) rotate(var(--r-end));
    opacity: 0;
  }
}
</style>

<style scoped>
.wind-overlay {
  position: absolute;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
  overflow: hidden;
}

.wind-paper {
  position: absolute;
  filter: drop-shadow(2px 3px 5px rgba(0, 0, 0, 0.2));
}
</style>
