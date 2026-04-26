<script setup lang="ts">
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'

const router = useRouter()
const {t} = useI18n()
</script>

<template>
  <div class="not-found-container bg-white dark:bg-[#0f172a] flex flex-col items-center justify-center gap-6">
    <!--
      Horse-drawn wagon (căruță) facing left.
      Clicking anywhere navigates home.
      The horse is a single closed silhouette path for clear recognition.
    -->
    <svg
      @click="router.push({ name: 'home' })"
      class="not-found-horse-wagon text-slate-600 dark:text-slate-400"
      viewBox="0 0 270 118"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <!-- ── CART ── -->

      <!-- Back wheel (large) -->
      <circle cx="232" cy="90" r="24" stroke="currentColor" stroke-width="2.5"/>
      <circle cx="232" cy="90" r="3" fill="currentColor"/>
      <line x1="232" y1="66" x2="232" y2="114" stroke="currentColor" stroke-width="1.5"/>
      <line x1="208" y1="90" x2="256" y2="90" stroke="currentColor" stroke-width="1.5"/>
      <line x1="215" y1="73" x2="249" y2="107" stroke="currentColor" stroke-width="1.5"/>
      <line x1="249" y1="73" x2="215" y2="107" stroke="currentColor" stroke-width="1.5"/>

      <!-- Front wheel (smaller) -->
      <circle cx="162" cy="92" r="20" stroke="currentColor" stroke-width="2.5"/>
      <circle cx="162" cy="92" r="2.5" fill="currentColor"/>
      <line x1="162" y1="72" x2="162" y2="112" stroke="currentColor" stroke-width="1.5"/>
      <line x1="142" y1="92" x2="182" y2="92" stroke="currentColor" stroke-width="1.5"/>
      <line x1="148" y1="78" x2="176" y2="106" stroke="currentColor" stroke-width="1.5"/>
      <line x1="176" y1="78" x2="148" y2="106" stroke="currentColor" stroke-width="1.5"/>

      <!-- Cart bed -->
      <rect x="142" y="28" width="114" height="44" rx="2" stroke="currentColor" stroke-width="2.5"/>
      <line x1="162" y1="28" x2="162" y2="72" stroke="currentColor" stroke-width="1"/>
      <line x1="182" y1="28" x2="182" y2="72" stroke="currentColor" stroke-width="1"/>
      <line x1="202" y1="28" x2="202" y2="72" stroke="currentColor" stroke-width="1"/>
      <line x1="222" y1="28" x2="222" y2="72" stroke="currentColor" stroke-width="1"/>
      <line x1="242" y1="28" x2="242" y2="72" stroke="currentColor" stroke-width="1"/>

      <!-- Shaft (traces from harness area to cart front) -->
      <line x1="88" y1="64" x2="144" y2="62" stroke="currentColor" stroke-width="2"/>

      <!-- ── HORSE (facing left) ── -->

      <!--
        Single closed silhouette path, traced clockwise starting at the tail base:
        top line (rump → back → withers → neck crest → poll) → ear → face → muzzle
        → jaw → throat → chest → belly → hindquarters → back to tail base.
      -->
      <path
        d="M 94 48
           C 96 44 96 40 94 36
           C 90 32 82 30 72 30
           C 62 30 54 30 50 28
           C 46 26 42 22 38 16
           C 34 12 30 10 26 10
           L 22 6
           L 18 10
           C 14 14 10 18 6 24
           C 3 28 3 33 6 36
           L 10 40
           L 20 44
           C 26 46 30 50 32 56
           C 36 62 40 64 44 66
           C 56 70 70 70 82 68
           C 88 66 92 62 94 58
           C 96 54 96 50 94 48
           Z"
        stroke="currentColor" stroke-width="2.5"
      />

      <!-- Eye -->
      <circle cx="10" cy="22" r="2" fill="currentColor"/>

      <!-- Nostril -->
      <ellipse cx="4" cy="32" rx="1.5" ry="2" fill="currentColor"/>

      <!-- Mane (thicker accent along neck crest) -->
      <path d="M 26 10 C 32 16 38 22 44 28" stroke="currentColor" stroke-width="5" stroke-linecap="round" opacity="0.35"/>

      <!-- Front legs (with knee joint) -->
      <line x1="40" y1="66" x2="38" y2="84" stroke="currentColor" stroke-width="3" stroke-linecap="round"/>
      <line x1="38" y1="84" x2="36" y2="106" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
      <line x1="48" y1="66" x2="46" y2="84" stroke="currentColor" stroke-width="3" stroke-linecap="round"/>
      <line x1="46" y1="84" x2="44" y2="106" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>

      <!-- Back legs (with hock joint angled back then forward) -->
      <line x1="78" y1="68" x2="82" y2="86" stroke="currentColor" stroke-width="3" stroke-linecap="round"/>
      <line x1="82" y1="86" x2="78" y2="106" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
      <line x1="87" y1="68" x2="91" y2="86" stroke="currentColor" stroke-width="3" stroke-linecap="round"/>
      <line x1="91" y1="86" x2="87" y2="106" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>

      <!-- Tail (two flowing strands curving right from the rump) -->
      <path d="M 94 48 C 106 44 112 36 109 26" stroke="currentColor" stroke-width="3" stroke-linecap="round"/>
      <path d="M 94 52 C 108 48 114 40 111 30" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
    </svg>

    <p class="not-found-text text-base font-bold text-slate-700 dark:text-slate-300">{{ t('notFoundFunny') }}</p>
  </div>
</template>

<style scoped>
.not-found-container {
  padding: 2rem 1.5rem;
  height: 100%;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

@keyframes wagon-bounce {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-5px); }
}

.not-found-horse-wagon {
  width: 16rem;
  height: 7.5rem;
  animation: wagon-bounce 2s ease-in-out infinite;
  cursor: pointer;
  transition: opacity 0.2s ease;
}

.not-found-horse-wagon:hover {
  animation: none;
  transform: scale(1.08);
  opacity: 0.85;
}
</style>
