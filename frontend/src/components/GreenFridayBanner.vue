<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const isFriday = computed(() => new Date().getDay() === 5)
const todayKey = new Date().toISOString().slice(0, 10)
const dismissed = ref(localStorage.getItem('greenFridayDismissed') === todayKey)

function dismiss() {
  localStorage.setItem('greenFridayDismissed', todayKey)
  dismissed.value = true
}
</script>

<template>
  <div v-if="isFriday && !dismissed" class="green-friday-banner">

    <div class="gf-icon" aria-hidden="true">
      <svg class="gf-svg" viewBox="0 0 110 75" fill="none">

        <!-- ── Terminal ─────────────────────────────────── -->
        <rect x="5" y="6" width="30" height="63" rx="5" fill="#1e293b"/>
        <!-- 3-D left-edge highlight -->
        <rect x="5" y="6" width="4" height="63" rx="3" fill="rgba(255,255,255,0.06)"/>
        <!-- Screen bezel -->
        <rect x="8"  y="9"  width="24" height="18" rx="2.5" fill="#0f172a"/>
        <!-- Green display lines -->
        <rect x="11" y="12" width="9"  height="2" rx="1" fill="#22c55e" opacity="0.85"/>
        <rect x="11" y="16" width="17" height="2" rx="1" fill="#22c55e" opacity="0.5"/>
        <rect x="11" y="20" width="13" height="2" rx="1" fill="#22c55e" opacity="0.3"/>
        <!-- Divider -->
        <line x1="5" y1="30" x2="35" y2="30" stroke="#334155" stroke-width="0.75"/>
        <!-- NFC zone background -->
        <rect x="8" y="32" width="24" height="26" rx="2.5" fill="#0f172a" opacity="0.35"/>
        <!-- NFC symbol: dot + 3 arcs -->
        <circle cx="14" cy="45" r="2.5" fill="#22c55e"/>
        <path d="M18.5 41.5 Q21.5 45 18.5 48.5" stroke="#22c55e" fill="none" stroke-width="2.25" stroke-linecap="round"/>
        <path d="M22 38.5 Q26.5 45 22 51.5"     stroke="#22c55e" fill="none" stroke-width="2"    stroke-linecap="round"/>
        <path d="M25.5 35.5 Q31.5 45 25.5 54.5" stroke="#22c55e" fill="none" stroke-width="1.5"  stroke-linecap="round"/>
        <!-- Status LED -->
        <circle cx="20" cy="63.5" r="2.5" fill="#22c55e" opacity="0.9"/>
        <circle cx="20" cy="63.5" r="1.5" fill="#4ade80"/>

        <!-- ── Vertical card (taps toward terminal) ───── -->
        <g class="gf-card-grp">
          <!-- Card body -->
          <rect x="72" y="12" width="24" height="51" rx="3.5" fill="#6366f1" stroke="#4338ca" stroke-width="1"/>
          <!-- Dark mag-stripe band -->
          <rect x="72" y="20" width="24" height="6" fill="rgba(0,0,0,0.22)"/>
          <!-- Chip -->
          <rect x="76" y="31" width="11" height="8" rx="1.5" fill="#fbbf24"/>
          <line x1="81" y1="31" x2="81" y2="39" stroke="#d97706" stroke-width="0.8"/>
          <line x1="76" y1="35" x2="87" y2="35" stroke="#d97706" stroke-width="0.8"/>
          <!-- Contactless waves (right of chip) -->
          <path d="M90 33 Q92 35 90 37"   stroke="rgba(255,255,255,0.85)" fill="none" stroke-width="1.4" stroke-linecap="round"/>
          <path d="M93 30 Q96 35 93 40"   stroke="rgba(255,255,255,0.85)" fill="none" stroke-width="1.4" stroke-linecap="round"/>
          <!-- Number strips -->
          <rect x="76" y="44" width="16" height="2.5" rx="1.25" fill="rgba(255,255,255,0.3)"/>
          <rect x="76" y="49" width="12" height="2.5" rx="1.25" fill="rgba(255,255,255,0.2)"/>
          <rect x="76" y="54" width="9"  height="2"   rx="1"    fill="rgba(255,255,255,0.13)"/>
        </g>

      </svg>

      <!-- NO stamp (div overlay — slams in, holds, fades, loops) -->
      <div class="gf-stamp">
        <svg viewBox="0 0 100 100" fill="none">
          <circle cx="50" cy="50" r="46" fill="rgba(239,68,68,0.92)"/>
          <circle cx="50" cy="50" r="46" fill="none" stroke="white" stroke-width="7"/>
          <line x1="18" y1="18" x2="82" y2="82" stroke="white" stroke-width="13" stroke-linecap="round"/>
        </svg>
      </div>
    </div>

    <div class="green-friday-text">
      <p class="green-friday-title">{{ t('greenFridayTitle') }}</p>
      <p class="green-friday-desc">{{ t('greenFridayDesc') }}</p>
    </div>

    <button type="button" class="green-friday-close" :aria-label="t('dismiss')" @click="dismiss">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>
</template>

<style scoped>
.green-friday-banner {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 1rem;
  background: linear-gradient(135deg, #dcfce7, #d1fae5);
  border-bottom: 1px solid #a7f3d0;
}

/* Icon area — wide+tall enough to show the scene properly */
.gf-icon {
  position: relative;
  flex-shrink: 0;
  width: 5.5rem;   /* 88px */
  height: 3.75rem; /* 60px */
}

.gf-svg {
  width: 100%;
  height: 100%;
  display: block;
  overflow: visible;
}

/* Card group: slides left toward terminal, slight tilt, then retreats */
.gf-card-grp {
  animation: gf-tap 4.5s ease-in-out infinite;
}

@keyframes gf-tap {
  0%   { transform: translateX(0px); }
  20%  { transform: translateX(-29px); }
  38%  { transform: translateX(-29px); }
  55%  { transform: translateX(0px); }
  100% { transform: translateX(0px); }
}

/* NO stamp: positioned dead-center over the scene */
.gf-stamp {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 4rem;
  height: 4rem;
  pointer-events: none;
  animation: gf-stamp 4.5s ease-in-out infinite;
}

.gf-stamp svg {
  width: 100%;
  height: 100%;
  display: block;
}

@keyframes gf-stamp {
  /* hidden while card is tapping */
  0%   { transform: translate(-50%, -70%) scale(0.25); opacity: 0; }
  54%  { transform: translate(-50%, -70%) scale(0.25); opacity: 0; }
  /* slam in */
  66%  { transform: translate(-50%, -50%) scale(1.22);  opacity: 1; }
  73%  { transform: translate(-50%, -50%) scale(1);     opacity: 1; }
  /* hold */
  84%  { transform: translate(-50%, -50%) scale(1);     opacity: 1; }
  /* fade out */
  96%  { transform: translate(-50%, -50%) scale(0.88);  opacity: 0; }
  100% { transform: translate(-50%, -70%) scale(0.25);  opacity: 0; }
}

/* Text */
.green-friday-text { flex: 1; min-width: 0; }

.green-friday-title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 800;
  color: #065f46;
}

.green-friday-desc {
  margin: 0.1rem 0 0;
  font-size: 0.75rem;
  font-weight: 500;
  color: #047857;
}

/* Close button */
.green-friday-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
  border: 0;
  border-radius: 9999px;
  background: transparent;
  color: #059669;
  cursor: pointer;
  transition: background 120ms ease;
}
.green-friday-close:hover { background: rgb(16 185 129 / 0.15); }
.green-friday-close svg { width: 0.75rem; height: 0.75rem; }
</style>
