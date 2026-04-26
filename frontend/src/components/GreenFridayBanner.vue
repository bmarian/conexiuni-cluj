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

    <!--
      5 s loop:
        0–25 %   rest
        25–44%   card glides to terminal
        44–49%   bounce (overshoot → settle)
        49–62%   hold at terminal, NFC arcs pulse
        62–74%   card retreats
        74–100%  NO stamp slams in, holds, fades
    -->
    <div class="gf-icon" aria-hidden="true">
      <svg class="gf-svg" viewBox="0 0 115 75" fill="none" overflow="visible">
        <defs>
          <linearGradient id="gf-card-grad" x1="0%" y1="0%" x2="70%" y2="100%">
            <stop offset="0%" stop-color="#a5b4fc"/>
            <stop offset="100%" stop-color="#3730a3"/>
          </linearGradient>
          <linearGradient id="gf-chip-grad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stop-color="#fde68a"/>
            <stop offset="100%" stop-color="#b45309"/>
          </linearGradient>
        </defs>

        <!-- ── Payment terminal ─────────────────────────── -->
        <rect x="4"  y="5"  width="30" height="65" rx="5"   fill="#1e293b"/>
        <rect x="4"  y="5"  width="5"  height="65" rx="4"   fill="rgba(255,255,255,0.055)"/>
        <!-- Screen -->
        <rect x="7"  y="8"  width="24" height="18" rx="2.5" fill="#0f172a"/>
        <rect x="10" y="11" width="9"  height="2"  rx="1"   fill="#22c55e" opacity="0.9"/>
        <rect x="10" y="15" width="16" height="2"  rx="1"   fill="#22c55e" opacity="0.5"/>
        <rect x="10" y="19" width="12" height="2"  rx="1"   fill="#22c55e" opacity="0.3"/>
        <!-- Divider -->
        <line x1="4" y1="29" x2="34" y2="29" stroke="#334155" stroke-width="0.75"/>
        <!-- NFC zone panel -->
        <rect x="7"  y="31" width="24" height="26" rx="2.5" fill="#0f172a" opacity="0.4"/>
        <!-- NFC symbol: dot + expanding arcs -->
        <g class="gf-nfc">
          <circle cx="13" cy="44" r="2.5" fill="#22c55e"/>
          <path d="M17.5 40.5 Q20.5 44 17.5 47.5" stroke="#22c55e" fill="none" stroke-width="2.25" stroke-linecap="round"/>
          <path d="M21.5 37.5 Q26   44 21.5 50.5" stroke="#22c55e" fill="none" stroke-width="2"    stroke-linecap="round"/>
          <path d="M25.5 34.5 Q31.5 44 25.5 53.5" stroke="#22c55e" fill="none" stroke-width="1.5"  stroke-linecap="round"/>
        </g>
        <!-- Status LED -->
        <circle cx="19" cy="64" r="2.5" fill="#22c55e" opacity="0.85"/>
        <circle cx="19" cy="64" r="1.5" fill="#4ade80"/>

        <!-- ── Credit card (animates) ───────────────────── -->
        <g class="gf-card-grp">
          <!-- Card body (shorter — no logo area) -->
          <rect x="75" y="22" width="27" height="42" rx="4" fill="url(#gf-card-grad)"/>
          <!-- Thin top shine -->
          <rect x="75" y="22" width="27" height="10" rx="4" fill="rgba(255,255,255,0.1)"/>
          <!-- Edge stroke -->
          <rect x="75" y="22" width="27" height="42" rx="4" fill="none" stroke="rgba(255,255,255,0.18)" stroke-width="0.75"/>
          <!-- EMV chip body -->
          <rect x="79" y="32" width="12" height="10" rx="2"   fill="url(#gf-chip-grad)"/>
          <!-- Chip contact pads (2×2 grid) -->
          <rect x="80" y="33" width="4"  height="3"  rx="0.75" fill="rgba(180,130,0,0.6)"/>
          <rect x="86" y="33" width="4"  height="3"  rx="0.75" fill="rgba(180,130,0,0.6)"/>
          <rect x="80" y="37" width="4"  height="3"  rx="0.75" fill="rgba(180,130,0,0.6)"/>
          <rect x="86" y="37" width="4"  height="3"  rx="0.75" fill="rgba(180,130,0,0.6)"/>
          <!-- Contactless symbol right of chip -->
          <circle cx="95" cy="37" r="1.5" fill="rgba(255,255,255,0.7)"/>
          <path d="M97.5 34.5 Q99.5 37 97.5 39.5" stroke="rgba(255,255,255,0.7)" fill="none" stroke-width="1.3" stroke-linecap="round"/>
          <path d="M100 32   Q103  37 100  42"    stroke="rgba(255,255,255,0.7)" fill="none" stroke-width="1.3" stroke-linecap="round"/>
          <!-- Embossed number row (4 groups of dots) -->
          <circle cx="79" cy="48" r="1.1" fill="rgba(255,255,255,0.55)"/>
          <circle cx="81.5" cy="48" r="1.1" fill="rgba(255,255,255,0.55)"/>
          <circle cx="84"   cy="48" r="1.1" fill="rgba(255,255,255,0.55)"/>
          <circle cx="86.5" cy="48" r="1.1" fill="rgba(255,255,255,0.55)"/>
          <circle cx="90"   cy="48" r="1.1" fill="rgba(255,255,255,0.45)"/>
          <circle cx="92.5" cy="48" r="1.1" fill="rgba(255,255,255,0.45)"/>
          <circle cx="95"   cy="48" r="1.1" fill="rgba(255,255,255,0.45)"/>
          <circle cx="97.5" cy="48" r="1.1" fill="rgba(255,255,255,0.45)"/>
          <!-- Name strip -->
          <rect x="79" y="54" width="17" height="2.5" rx="1.25" fill="rgba(255,255,255,0.3)"/>
          <rect x="79" y="58" width="11" height="2"   rx="1"    fill="rgba(255,255,255,0.18)"/>
        </g>

      </svg>

      <!-- NO stamp — positioned over the scene, separate from SVG for clean animation -->
      <div class="gf-stamp">
        <svg viewBox="0 0 100 100" fill="none">
          <circle cx="50" cy="50" r="46" fill="rgba(220,38,38,0.93)"/>
          <circle cx="50" cy="50" r="46" fill="none" stroke="white" stroke-width="7"/>
          <line x1="17" y1="17" x2="83" y2="83" stroke="white" stroke-width="14" stroke-linecap="round"/>
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

/* ── Icon container ─────────────────────────────────────────── */
.gf-icon {
  position: relative;
  flex-shrink: 0;
  width: 5.75rem;  /* 92px — enough to show both objects clearly */
  height: 4rem;    /* 64px */
}

.gf-svg {
  width: 100%;
  height: 100%;
  display: block;
}

/* ── Card tap animation ─────────────────────────────────────── */
/* Glides toward terminal, bounces on contact, retreats */
.gf-card-grp {
  transform-box: fill-box;
  transform-origin: center;
  animation: gf-tap 5s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}

@keyframes gf-tap {
  0%   { transform: translateX(0)     rotate(0deg);   }  /* rest        */
  25%  { transform: translateX(0)     rotate(0deg);   }  /* rest        */
  44%  { transform: translateX(-32px) rotate(-12deg); }  /* at terminal */
  48%  { transform: translateX(-26px) rotate(-7deg);  }  /* bounce back */
  53%  { transform: translateX(-30px) rotate(-9deg);  }  /* settle      */
  62%  { transform: translateX(-30px) rotate(-9deg);  }  /* hold        */
  74%  { transform: translateX(0)     rotate(0deg);   }  /* retreat     */
  100% { transform: translateX(0)     rotate(0deg);   }  /* rest        */
}

/* ── NFC arcs pulse at moment of contact ───────────────────── */
.gf-nfc {
  animation: gf-nfc-pulse 5s ease-in-out infinite;
}

@keyframes gf-nfc-pulse {
  0%   { opacity: 0.7; }
  43%  { opacity: 0.7; }
  46%  { opacity: 1;   filter: drop-shadow(0 0 3px #22c55e); }
  52%  { opacity: 1;   filter: drop-shadow(0 0 3px #22c55e); }
  56%  { opacity: 0.7; filter: none; }
  100% { opacity: 0.7; }
}

/* ── NO stamp ───────────────────────────────────────────────── */
.gf-stamp {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 4.25rem;
  height: 4.25rem;
  pointer-events: none;
  animation: gf-stamp 5s ease-in-out infinite;
}

.gf-stamp svg {
  width: 100%;
  height: 100%;
  display: block;
}

@keyframes gf-stamp {
  0%   { transform: translate(-50%, -70%) scale(0.2); opacity: 0; } /* hidden  */
  74%  { transform: translate(-50%, -70%) scale(0.2); opacity: 0; } /* hidden  */
  82%  { transform: translate(-50%, -50%) scale(1.25); opacity: 1; } /* slam in */
  87%  { transform: translate(-50%, -50%) scale(1);    opacity: 1; } /* settle  */
  93%  { transform: translate(-50%, -50%) scale(1);    opacity: 1; } /* hold    */
  99%  { transform: translate(-50%, -50%) scale(0.9);  opacity: 0; } /* fade    */
  100% { transform: translate(-50%, -70%) scale(0.2);  opacity: 0; } /* reset   */
}

/* ── Text ───────────────────────────────────────────────────── */
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

/* ── Close button ───────────────────────────────────────────── */
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
