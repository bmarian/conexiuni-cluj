<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'

const {t} = useI18n()
const settings = useSettingsStore()
const copied = ref(false)
let resetTimer: ReturnType<typeof setTimeout> | null = null

const hoverBg = computed(() => settings.isDark ? '#1e293b' : '#eff6ff')
const hoverColor = computed(() => settings.isDark ? '#94a3b8' : '#3b82f6')
const copiedHoverBg = computed(() => settings.isDark ? '#064e3b' : '#ecfdf5')

async function share() {
  const url = window.location.href
  if (navigator.share) {
    try {
      await navigator.share({url})
    } catch {
      // user cancelled
    }
    return
  }
  try {
    await navigator.clipboard.writeText(url)
    copied.value = true
    if (resetTimer) clearTimeout(resetTimer)
    resetTimer = setTimeout(() => {
      copied.value = false
      resetTimer = null
    }, 2000)
  } catch {
    // clipboard unavailable
  }
}
</script>

<template>
  <button
    type="button"
    class="share-btn"
    :class="{'is-copied': copied}"
    :title="copied ? t('urlCopied') : t('shareUrl')"
    :aria-label="copied ? t('urlCopied') : t('shareUrl')"
    @click="share"
  >
    <template v-if="copied">
      <span v-if="settings.traditionalActive" class="emoji-icon" aria-hidden="true">✅</span>
      <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
      </svg>
    </template>
    <template v-else>
      <span v-if="settings.traditionalActive" class="emoji-icon" aria-hidden="true">🔗</span>
      <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round"
              d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0 0a2.25 2.25 0 1 0 3.935 2.186 2.25 2.25 0 0 0-3.935-2.186zm0-12.814a2.25 2.25 0 1 0 3.933-2.185 2.25 2.25 0 0 0-3.933 2.185z"/>
      </svg>
    </template>
  </button>
</template>

<style scoped>
.share-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 9999px;
  color: #94a3b8;
  background: transparent;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, transform 0.15s;
}

.share-btn:hover {
  background: v-bind(hoverBg);
  color: v-bind(hoverColor);
}

.share-btn:active {
  transform: scale(0.92);
}

.share-btn.is-copied {
  color: #10b981;
}

.share-btn.is-copied:hover {
  background: v-bind(copiedHoverBg);
  color: #10b981;
}
</style>
