<script setup lang="ts">
import {nextTick, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'
import {useFavoritesStore} from '@/stores/favorites'

const {t, locale} = useI18n()
const settings = useSettingsStore()
const favs = useFavoritesStore()

type Mode = 'export' | 'import' | null
const mode = ref<Mode>(null)
const text = ref('')
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const exportDone = ref(false)
const importState = ref<'idle' | 'success' | 'error'>('idle')
let exportTimer: ReturnType<typeof setTimeout> | null = null
let importTimer: ReturnType<typeof setTimeout> | null = null

const canShare = typeof navigator !== 'undefined' && !!navigator.share

function buildJson() {
  return JSON.stringify({
    version: 1,
    settings: {
      theme: settings.theme,
      locale: settings.locale,
      easterEggUnlocked: settings.easterEggUnlocked,
      easterEggActive: settings.easterEggActive,
      traditionalUnlocked: settings.traditionalUnlocked,
      traditionalActive: settings.traditionalActive,
      showWeather: settings.showWeather,
      showNews: settings.showNews,
      autoCenterOnMe: settings.autoCenterOnMe,
      autoFitMap: settings.autoFitMap,
    },
    favorites: {
      routes: favs.favoriteRouteIds,
      stops: favs.favoriteStopIds,
      plans: favs.favoritePlans,
      recentPlans: favs.recentPlans,
    },
  })
}

async function compress(json: string): Promise<string> {
  const stream = new CompressionStream('deflate-raw')
  const writer = stream.writable.getWriter()
  void writer.write(new TextEncoder().encode(json))
  void writer.close()
  const chunks: Uint8Array[] = []
  const reader = stream.readable.getReader()
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    chunks.push(value)
  }
  const buf = new Uint8Array(chunks.reduce((n, c) => n + c.length, 0))
  let off = 0
  for (const chunk of chunks) { buf.set(chunk, off); off += chunk.length }
  return btoa(String.fromCharCode(...buf))
}

async function decompress(b64: string): Promise<string> {
  const bytes = Uint8Array.from(atob(b64), c => c.charCodeAt(0))
  const stream = new DecompressionStream('deflate-raw')
  const writer = stream.writable.getWriter()
  void writer.write(bytes)
  void writer.close()
  const chunks: Uint8Array[] = []
  const reader = stream.readable.getReader()
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    chunks.push(value)
  }
  const buf = new Uint8Array(chunks.reduce((n, c) => n + c.length, 0))
  let off = 0
  for (const chunk of chunks) { buf.set(chunk, off); off += chunk.length }
  return new TextDecoder().decode(buf)
}

async function openExport() {
  mode.value = 'export'
  text.value = await compress(buildJson())
  exportDone.value = false
  await nextTick()
  textareaRef.value?.select()
}

function openImport() {
  mode.value = 'import'
  text.value = ''
  importState.value = 'idle'
}

function cancel() {
  mode.value = null
  exportDone.value = false
  importState.value = 'idle'
}

async function doExportShare() {
  if (canShare) {
    try {
      await navigator.share({text: text.value})
      markExportDone()
    } catch {
      // user cancelled
    }
  } else {
    try {
      await navigator.clipboard.writeText(text.value)
      markExportDone()
    } catch {
      // clipboard unavailable
    }
  }
}

function markExportDone() {
  exportDone.value = true
  if (exportTimer) clearTimeout(exportTimer)
  exportTimer = setTimeout(() => {
    exportDone.value = false
    exportTimer = null
  }, 2000)
}

async function doImport() {
  try {
    let raw = text.value.trim()
    try { raw = await decompress(raw) } catch { /* plain JSON fallback */ }
    const data = JSON.parse(raw)
    if (!data || typeof data !== 'object') throw new Error()
    const s = data.settings ?? {}
    if (s.theme) settings.setTheme(s.theme)
    if (s.locale) {
      settings.setLocale(s.locale)
      locale.value = s.locale
    }
    if (s.easterEggUnlocked) settings.unlockEasterEgg()
    if (s.easterEggActive) settings.activateEasterEgg()
    else settings.deactivateEasterEgg()
    if (s.traditionalUnlocked) settings.unlockTraditional()
    if (s.traditionalActive) settings.activateTraditional()
    else settings.deactivateTraditional()
    if (typeof s.showWeather === 'boolean') settings.setShowWeather(s.showWeather)
    if (typeof s.showNews === 'boolean') settings.setShowNews(s.showNews)
    if (typeof s.autoCenterOnMe === 'boolean') settings.setAutoCenterOnMe(s.autoCenterOnMe)
    if (typeof s.autoFitMap === 'boolean') settings.setAutoFitMap(s.autoFitMap)
    favs.importAll(data.favorites ?? {})
    importState.value = 'success'
    if (importTimer) clearTimeout(importTimer)
    importTimer = setTimeout(cancel, 1500)
  } catch {
    importState.value = 'error'
    if (importTimer) clearTimeout(importTimer)
    importTimer = setTimeout(() => {
      importState.value = 'idle'
      importTimer = null
    }, 2000)
  }
}
</script>

<template>
  <div class="ei-root" :class="{ 'is-dark': settings.isDark, 'is-chomper': settings.easterEggActive, 'is-traditional': settings.traditionalActive }">
    <p class="ei-label">{{ t('exportImport') }}</p>
    <div class="ei-group">
      <button type="button" class="ei-btn" @click="openExport">
        <span v-if="settings.traditionalActive" class="emoji-icon-sm" aria-hidden="true">📋</span>
        <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
             stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
             width="13" height="13" aria-hidden="true">
          <rect x="9" y="9" width="13" height="13" rx="2"/>
          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
        </svg>
        {{ t('exportSettings') }}
      </button>
      <button type="button" class="ei-btn" @click="openImport">
        <span v-if="settings.traditionalActive" class="emoji-icon-sm" aria-hidden="true">📂</span>
        <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
             stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
             width="13" height="13" aria-hidden="true">
          <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/>
          <polyline points="17 8 12 3 7 8"/>
          <line x1="12" y1="3" x2="12" y2="15"/>
        </svg>
        {{ t('importSettings') }}
      </button>
    </div>

    <template v-if="mode">
      <textarea
        ref="textareaRef"
        v-model="text"
        class="ei-textarea"
        :readonly="mode === 'export'"
        :placeholder="mode === 'import' ? t('importPastePlaceholder') : ''"
        rows="7"
        spellcheck="false"
        autocomplete="off"
        autocorrect="off"
        autocapitalize="off"
      />
      <div class="ei-actions">
        <button
          v-if="mode === 'export'"
          type="button"
          class="ei-btn ei-btn-primary"
          :class="{'ei-btn-success': exportDone}"
          :disabled="exportDone"
          @click="doExportShare"
        >
          <template v-if="exportDone">
            <span v-if="settings.traditionalActive" class="emoji-icon-sm" aria-hidden="true">✅</span>
            <svg v-else width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
            {{ t('exportCopied') }}
          </template>
          <template v-else>
            {{ canShare ? t('share') : t('exportCopyBtn') }}
          </template>
        </button>
        <button
          v-if="mode === 'import'"
          type="button"
          class="ei-btn ei-btn-primary"
          :class="{'ei-btn-success': importState === 'success', 'ei-btn-error': importState === 'error'}"
          :disabled="!text.trim() || importState !== 'idle'"
          @click="doImport"
        >
          <template v-if="importState === 'success'">
            <span v-if="settings.traditionalActive" class="emoji-icon-sm" aria-hidden="true">✅</span>
            <svg v-else width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
            {{ t('importSuccess') }}
          </template>
          <template v-else-if="importState === 'error'">
            <span v-if="settings.traditionalActive" class="emoji-icon-sm" aria-hidden="true">❌</span>
            <svg v-else width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
            </svg>
            {{ t('importError') }}
          </template>
          <template v-else>
            {{ t('importConfirm') }}
          </template>
        </button>
        <button type="button" class="ei-btn" @click="cancel">{{ t('cancel') }}</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.ei-root {
  margin-top: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

/* ── label ── */
.ei-label {
  margin: 0 0 0.4rem 0;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: #94a3b8;
}

.ei-root.is-dark .ei-label { color: #64748b; }
.ei-root.is-chomper .ei-label { color: #b45309; }
.ei-root.is-chomper.is-dark .ei-label { color: #d97706; }

/* ── button group ── */
.ei-group {
  display: flex;
  gap: 0.25rem;
}

/* ── buttons (base) ── */
.ei-btn {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.3rem;
  padding: 0.4rem 0.25rem;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: transparent;
  color: #64748b;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 120ms ease, color 120ms ease, border-color 120ms ease;
  white-space: nowrap;
}

.ei-btn:hover { background: #f1f5f9; color: #0f172a; }
.ei-btn:disabled { opacity: 0.4; cursor: default; }

.ei-btn.ei-btn-primary {
  background: #eff6ff;
  color: #1d4ed8;
  border-color: #bfdbfe;
}
.ei-btn.ei-btn-primary:hover:not(:disabled) {
  background: #dbeafe;
  color: #1e40af;
  border-color: #93c5fd;
}

.ei-btn.ei-btn-success {
  background: #f0fdf4;
  color: #16a34a;
  border-color: #bbf7d0;
}

.ei-btn.ei-btn-error {
  background: #fef2f2;
  color: #dc2626;
  border-color: #fecaca;
}

/* default dark */
.ei-root.is-dark .ei-btn { color: #64748b; }
.ei-root.is-dark .ei-btn:hover { background: #334155; color: #e2e8f0; }
.ei-root.is-dark .ei-btn.ei-btn-primary { background: #1e3a5f; color: #93c5fd; border-color: #1d4ed8; }
.ei-root.is-dark .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #1e40af; color: #bfdbfe; border-color: #3b82f6; }
.ei-root.is-dark .ei-btn.ei-btn-success { background: #052e16; color: #4ade80; border-color: #14532d; }
.ei-root.is-dark .ei-btn.ei-btn-error { background: #450a0a; color: #f87171; border-color: #7f1d1d; }

/* chomper light */
.ei-root.is-chomper .ei-btn { color: #92400e; }
.ei-root.is-chomper .ei-btn:hover { background: #fef9c3; color: #78350f; }
.ei-root.is-chomper .ei-btn.ei-btn-primary { background: #fef9c3; color: #92400e; border-color: #fcd34d; }
.ei-root.is-chomper .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #fef08a; color: #78350f; border-color: #f59e0b; }

/* chomper dark */
.ei-root.is-chomper.is-dark .ei-btn { color: #d97706; }
.ei-root.is-chomper.is-dark .ei-btn:hover { background: #422006; color: #fde68a; }
.ei-root.is-chomper.is-dark .ei-btn.ei-btn-primary { background: #422006; color: #fde68a; border-color: #d97706; }
.ei-root.is-chomper.is-dark .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #5c2d06; color: #fef08a; border-color: #f59e0b; }

/* traditional light */
.ei-root.is-traditional .ei-btn { color: #4A6FA5; }
.ei-root.is-traditional .ei-btn:hover { background: #EEF3FF; color: #1A3A8C; }
.ei-root.is-traditional .ei-btn.ei-btn-primary { background: #EEF3FF; color: #1A3A8C; border-color: #245EDC; }
.ei-root.is-traditional .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #D8E4FF; color: #0F2870; border-color: #1A3A8C; }
.ei-root.is-traditional .ei-btn.ei-btn-success { background: #EEF8EE; color: #1A6A2A; border-color: #4CAF50; }
.ei-root.is-traditional .ei-btn.ei-btn-error { background: #FEF0EE; color: #8A1A1A; border-color: #DC4444; }

/* traditional dark */
.ei-root.is-traditional.is-dark .ei-btn { color: #90B4E0; }
.ei-root.is-traditional.is-dark .ei-btn:hover { background: #172040; color: #B8D4F0; }
.ei-root.is-traditional.is-dark .ei-btn.ei-btn-primary { background: #10193A; color: #90B4E0; border-color: #2A508C; }
.ei-root.is-traditional.is-dark .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #1a2d5a; color: #B8D4F0; border-color: #3d70c0; }
.ei-root.is-traditional.is-dark .ei-btn.ei-btn-success { background: #0A2010; color: #4ADE80; border-color: #1A6A2A; }
.ei-root.is-traditional.is-dark .ei-btn.ei-btn-error { background: #200A0A; color: #F87171; border-color: #6A1A1A; }

/* ── textarea ── */
.ei-textarea {
  width: 100%;
  padding: 0.5rem 0.625rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  background: #f8fafc;
  color: #334155;
  font-size: 0.6875rem;
  font-family: ui-monospace, monospace;
  line-height: 1.5;
  resize: none;
  outline: none;
  box-sizing: border-box;
  transition: border-color 120ms ease, background 120ms ease;
}

.ei-textarea:focus { border-color: #93c5fd; background: #fff; }
.ei-textarea[readonly] { cursor: text; }

/* default dark */
.ei-root.is-dark .ei-textarea { border-color: #334155; background: #0f172a; color: #cbd5e1; }
.ei-root.is-dark .ei-textarea:focus { border-color: #3b82f6; background: #0f172a; }

/* chomper light */
.ei-root.is-chomper .ei-textarea { border-color: #fcd34d; background: #fefce8; color: #78350f; }
.ei-root.is-chomper .ei-textarea:focus { border-color: #f59e0b; background: #fefce8; }

/* chomper dark */
.ei-root.is-chomper.is-dark .ei-textarea { border-color: #d97706; background: #1c0a00; color: #fde68a; }
.ei-root.is-chomper.is-dark .ei-textarea:focus { border-color: #f59e0b; background: #1c0a00; }

/* traditional light */
.ei-root.is-traditional .ei-textarea { border-color: #B8CAEE; background: #F5F8FF; color: #1A3A8C; }
.ei-root.is-traditional .ei-textarea:focus { border-color: #245EDC; background: #fff; }

/* traditional dark */
.ei-root.is-traditional.is-dark .ei-textarea { border-color: #2A508C; background: #0A1020; color: #90B4E0; }
.ei-root.is-traditional.is-dark .ei-textarea:focus { border-color: #3d70c0; background: #0A1020; }

/* ── actions row ── */
.ei-actions {
  display: flex;
  gap: 0.25rem;
}
</style>
