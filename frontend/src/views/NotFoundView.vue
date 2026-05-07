<script setup lang="ts">
import {onMounted, onUnmounted} from 'vue'
import {useHead} from '@unhead/vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'

const router = useRouter()
const {t} = useI18n()
const settings = useSettingsStore()

useHead(() => ({
  title: t('headNotFoundTitle'),
  meta: [{name: 'robots', content: 'noindex'}],
}))

function activateAndGo() {
  if (!settings.traditionalUnlocked) {
    settings.unlockTraditional()
    settings.activateTraditional()
  }
  router.push({name: 'home'})
}

function onKey() {
  activateAndGo()
}

onMounted(() => {
  window.addEventListener('keydown', onKey)
  window.addEventListener('pointerdown', onKey)
})
onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('pointerdown', onKey)
})
</script>

<template>
  <Teleport to="body">
    <div class="bsod" role="alertdialog" aria-label="STOP error">
      <div class="bsod-inner">
        <div class="bsod-header">
          <span class="bsod-banner">Conexiuni Cluj</span>
        </div>

        <p class="bsod-code">{{ t('bsodCode') }}</p>

        <p>{{ t('bsodMissing') }}</p>

        <p>{{ t('bsodReason') }}</p>

        <p>{{ t('bsodTechHeader') }}</p>

        <p class="bsod-stop">{{ t('bsodStop') }}</p>

        <p class="bsod-prompt">{{ t('bsodPrompt') }}</p>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.bsod {
  position: fixed;
  inset: 0;
  background: #0A246A;
  color: #FFFFFF;
  font-family: 'Lucida Console', 'Consolas', 'Courier New', monospace;
  font-size: clamp(11px, 1.6vw, 15px);
  line-height: 1.45;
  padding: clamp(1rem, 4vh, 3rem) clamp(1rem, 8vw, 6rem);
  overflow-y: auto;
  cursor: pointer;
  z-index: 9999;
  user-select: none;
  -webkit-font-smoothing: none;
  -moz-osx-font-smoothing: auto;
}

.bsod-inner {
  max-width: 70ch;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 1em;
}

.bsod-header {
  display: flex;
  justify-content: center;
  margin-bottom: 0.4em;
}

.bsod-banner {
  background: #BFBFBF;
  color: #0A246A;
  padding: 0.05em 1em;
  font-weight: 700;
  letter-spacing: 0.2em;
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
  font-size: 1.05em;
}

.bsod-lead {
  margin: 0.25em 0;
}

.bsod-code {
  margin: 0.6em 0;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.bsod-stop {
  margin: 0.5em 0;
  word-break: break-all;
}

.bsod-prompt {
  margin-top: 0.75em;
}

.bsod-prompt::after {
  content: '';
  display: inline-block;
  width: 0.6em;
  height: 1em;
  background: #FFFFFF;
  margin-left: 0.15em;
  vertical-align: text-bottom;
  animation: bsod-cursor 1s steps(2) infinite;
}

@keyframes bsod-cursor {
  0%, 50% {
    opacity: 1;
  }
  51%, 100% {
    opacity: 0;
  }
}

.bsod p {
  margin: 0;
}
</style>
