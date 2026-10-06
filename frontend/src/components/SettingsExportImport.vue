<script setup lang="ts">
import {computed, defineAsyncComponent, nextTick, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {useSettingsStore} from '@/stores/settings'
import {useKbdEscape, useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import {describeExport, useSettingsTransfer} from '@/composables/useSettingsTransfer.ts'
import {useTransferReceiver} from '@/composables/useTransferReceiver.ts'
import {focusItem} from '@/utils/keyboardFocus.ts'
import {sendToTransferLink, TRANSFER_QUERY} from '@/utils/transferLink.ts'

const QrCode = defineAsyncComponent(() => import('@/components/QrCode.vue'))

const props = defineProps<{ sendCode?: string; sendKey?: string }>()

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const settings = useSettingsStore()
const {buildExport, applyExport} = useSettingsTransfer()

type Mode = 'export' | 'import' | 'send' | 'receive' | null
const mode = ref<Mode>(null)
const text = ref('')
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const exportDone = ref(false)
const importState = ref<'idle' | 'success' | 'error'>('idle')
let exportTimer: ReturnType<typeof setTimeout> | null = null
let importTimer: ReturnType<typeof setTimeout> | null = null

const code = ref('')
const codeRef = ref<HTMLInputElement | null>(null)
const sendState = ref<'idle' | 'busy' | 'sent' | 'unknown' | 'limited' | 'failed'>('idle')
let sendTimer: ReturnType<typeof setTimeout> | null = null
const codeDigits = computed(() => code.value.replace(/\D/g, ''))
// The key from a scanned code only belongs to that code.
let scanned: { code: string; key?: string } | null = null
const canSend = computed(() => codeDigits.value.length === 6 && sendState.value === 'idle')

let receiveTimer: ReturnType<typeof setTimeout> | null = null
const {
  state: receiveState,
  url: receiveUrl,
  spacedCode: receiveCode,
  summary: receivedSummary,
  start: startReceive,
  stop: stopReceive,
  accept: acceptReceived,
} = useTransferReceiver(describeExport, applyExport)

const summaryRows = computed(() => {
  const summary = receivedSummary.value
  if (!summary) return []
  return [
    {label: t('favoriteStops'), count: summary.stops},
    {label: t('favoriteRoutes'), count: summary.lines},
    {label: t('favoritePlans'), count: summary.places},
    {label: t('followedLines'), count: summary.followed},
  ]
})

function replaceWithReceived() {
  acceptReceived()
  if (receiveState.value === 'done') receiveTimer = setTimeout(cancel, 1500)
}

watch(mode, (_now, before) => {
  if (before === 'receive') stopReceive()
})

// The code, and later what arrived, make the panel taller, so bring it all into view then.
watch(receiveState, async (state) => {
  if ((state !== 'waiting' && state !== 'review') || mode.value !== 'receive') return
  await nextTick()
  rootRef.value?.querySelector('[data-kbd-section="ei-receive-actions"]')?.scrollIntoView({block: 'nearest', behavior: 'smooth'})
})

const rootRef = ref<HTMLElement | null>(null)
const {keyboardMode} = useKeyboardNav()

useKbdEscape(() => mode.value !== null, () => {
  const opener = rootRef.value?.querySelector<HTMLElement>(`[data-kbd-item="ei-${mode.value}"]`)
  cancel()
  if (opener) focusItem(opener)
})

const canShare = typeof navigator !== 'undefined' && !!navigator.share

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
  text.value = await compress(JSON.stringify(buildExport()))
  exportDone.value = false
  await nextTick()
  textareaRef.value?.select()
}

function openImport() {
  mode.value = 'import'
  text.value = ''
  importState.value = 'idle'
  if (keyboardMode.value) void nextTick(() => textareaRef.value?.focus())
}

function cancel() {
  mode.value = null
  exportDone.value = false
  importState.value = 'idle'
  sendState.value = 'idle'
  if (sendTimer) clearTimeout(sendTimer)
  sendTimer = null
  if (receiveTimer) clearTimeout(receiveTimer)
  receiveTimer = null
  if (route.query[TRANSFER_QUERY] !== undefined) void router.replace({query: {}})
}

async function openSend(prefill = '', key?: string) {
  mode.value = 'send'
  code.value = prefill
  scanned = prefill ? {code: prefill, key} : null
  sendState.value = 'idle'
  await nextTick()
  if (prefill) {
    // A scanned code opens a fresh page that is still laying out, which cuts a smooth scroll short.
    await document.fonts.ready
    await new Promise(requestAnimationFrame)
    await new Promise(requestAnimationFrame)
    rootRef.value?.scrollIntoView({block: 'center'})
    if (keyboardMode.value) {
      const confirm = rootRef.value?.querySelector<HTMLElement>('[data-kbd-item="ei-send-confirm"]')
      if (confirm) focusItem(confirm)
    }
  } else {
    codeRef.value?.focus({preventScroll: true})
    rootRef.value?.scrollIntoView({block: 'nearest', behavior: 'smooth'})
  }
}

// Scanning the code on the other device opens Settings with it filled in.
watch(() => props.sendCode, (prefill) => {
  if (prefill && /^\d{6}$/.test(prefill)) void openSend(prefill, props.sendKey)
}, {immediate: true})

async function doSend() {
  if (!canSend.value) return
  sendState.value = 'busy'
  const key = scanned?.code === codeDigits.value ? scanned.key : undefined
  const result = await sendToTransferLink(codeDigits.value, buildExport(), key)
  sendState.value = result
  if (sendTimer) clearTimeout(sendTimer)
  sendTimer = setTimeout(() => {
    sendTimer = null
    if (result === 'sent') cancel()
    else sendState.value = 'idle'
  }, result === 'sent' ? 1500 : 2000)
}

async function openReceive() {
  mode.value = 'receive'
  void startReceive()
  await nextTick()
  rootRef.value?.scrollIntoView({block: 'nearest', behavior: 'smooth'})
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
    applyExport(JSON.parse(raw))
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
  <div ref="rootRef" class="ei-root" :class="{ 'is-dark': settings.isDark, 'is-arcade': settings.arcadeActive, 'is-legacy-blue': settings.legacyBlueActive, 'is-paper': settings.paperActive }">
    <div class="ei-group" data-kbd-section="ei-transfer" data-kbd-axis="x">
      <button type="button" class="ei-btn ei-btn-primary" data-kbd-item="ei-send" @click="openSend()">
        <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">📤</span>
        <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
             stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
             width="13" height="13" aria-hidden="true">
          <path d="M22 2 11 13"/>
          <path d="M22 2 15 22l-4-9-9-4 20-7z"/>
        </svg>
        {{ t('transferSend') }}
      </button>
      <button type="button" class="ei-btn ei-btn-primary" data-kbd-item="ei-receive" @click="openReceive">
        <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">📥</span>
        <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
             stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
             width="13" height="13" aria-hidden="true">
          <rect x="3" y="3" width="7" height="7" rx="1"/>
          <rect x="14" y="3" width="7" height="7" rx="1"/>
          <rect x="3" y="14" width="7" height="7" rx="1"/>
          <path d="M14 14h3v3h-3zM20 14v.01M14 20v.01M17 20h4v-3"/>
        </svg>
        {{ t('transferReceive') }}
      </button>
    </div>

    <template v-if="mode === 'send'">
      <input
        ref="codeRef"
        v-model="code"
        class="ei-textarea ei-code"
        data-kbd-section="ei-code"
        data-kbd-item="ei-code"
        type="text"
        inputmode="numeric"
        autocomplete="one-time-code"
        maxlength="7"
        :placeholder="t('sendCodePlaceholder')"
        @keydown.enter="doSend"
      />
      <div class="ei-actions" data-kbd-section="ei-send-actions" data-kbd-axis="x">
        <button
          type="button"
          class="ei-btn ei-btn-primary"
          data-kbd-item="ei-send-confirm"
          :class="{'ei-btn-success': sendState === 'sent', 'ei-btn-error': sendState === 'unknown' || sendState === 'limited' || sendState === 'failed'}"
          :disabled="!canSend"
          @click="doSend"
        >
          <template v-if="sendState === 'sent'">
            <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">✅</span>
            <svg v-else width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
            {{ t('sendDone') }}
          </template>
          <template v-else-if="sendState === 'unknown' || sendState === 'limited' || sendState === 'failed'">
            <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">❌</span>
            <svg v-else width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
            </svg>
            {{ sendState === 'unknown' ? t('sendUnknownCode') : sendState === 'limited' ? t('sendTooMany') : t('sendFailed') }}
          </template>
          <template v-else>
            {{ t('sendConfirm') }}
          </template>
        </button>
        <button type="button" class="ei-btn" data-kbd-item="ei-cancel" @click="cancel">{{ t('cancel') }}</button>
      </div>
    </template>

    <template v-else-if="mode === 'receive'">
      <div v-if="receiveState !== 'done'" class="ei-receive">
        <template v-if="receiveState === 'waiting'">
          <div class="ei-qr">
            <QrCode :text="receiveUrl" :label="receiveCode"/>
          </div>
          <p class="ei-receive-code">{{ receiveCode }}</p>
          <p class="ei-note setting-desc">{{ t('transferHint') }}</p>
        </template>
        <template v-else-if="receiveState === 'review'">
          <p class="ei-note setting-desc">{{ t('receiveReview') }}</p>
          <dl class="ei-summary">
            <div v-for="row in summaryRows" :key="row.label" class="ei-summary-row">
              <dt>{{ row.label }}</dt>
              <dd>{{ row.count }}</dd>
            </div>
          </dl>
        </template>
        <p v-else-if="receiveState === 'expired' || receiveState === 'failed' || receiveState === 'broken'"
           class="ei-note setting-desc">
          {{ t({expired: 'transferExpired', failed: 'transferFailed', broken: 'transferBroken'}[receiveState]) }}
        </p>
        <p v-else class="ei-note setting-desc">{{ t('transferLoading') }}</p>
      </div>
      <div class="ei-actions" data-kbd-section="ei-receive-actions" data-kbd-axis="x">
        <button v-if="receiveState === 'done'" type="button" class="ei-btn ei-btn-primary ei-btn-success" disabled>
          <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">✅</span>
          <svg v-else width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
          </svg>
          {{ t('receiveDone') }}
        </button>
        <template v-else>
          <button v-if="receiveState === 'review'" type="button"
                  class="ei-btn ei-btn-primary" data-kbd-item="ei-receive-confirm" @click="replaceWithReceived">
            {{ t('receiveReplace') }}
          </button>
          <button v-else-if="receiveState === 'expired' || receiveState === 'failed' || receiveState === 'broken'"
                  type="button" class="ei-btn ei-btn-primary" data-kbd-item="ei-new-code" @click="startReceive">
            {{ t('transferNewCode') }}
          </button>
          <button type="button" class="ei-btn" data-kbd-item="ei-cancel" @click="cancel">{{ t('cancel') }}</button>
        </template>
      </div>
    </template>

    <div class="ei-group ei-group-text" data-kbd-section="ei" data-kbd-axis="x">
      <button type="button" class="ei-btn" data-kbd-item="ei-export" @click="openExport">
        <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">📋</span>
        <svg v-else xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
             stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
             width="13" height="13" aria-hidden="true">
          <rect x="9" y="9" width="13" height="13" rx="2"/>
          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
        </svg>
        {{ t('exportSettings') }}
      </button>
      <button type="button" class="ei-btn" data-kbd-item="ei-import" @click="openImport">
        <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">📂</span>
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

    <template v-if="mode === 'export' || mode === 'import'">
      <textarea
        ref="textareaRef"
        v-model="text"
        class="ei-textarea"
        data-kbd-section="ei-text"
        data-kbd-item="ei-text"
        :readonly="mode === 'export'"
        :placeholder="mode === 'import' ? t('importPastePlaceholder') : ''"
        rows="7"
        spellcheck="false"
        autocomplete="off"
        autocorrect="off"
        autocapitalize="off"
      />
      <div class="ei-actions" data-kbd-section="ei-actions" data-kbd-axis="x">
        <button
          v-if="mode === 'export'"
          type="button"
          class="ei-btn ei-btn-primary"
          data-kbd-item="ei-share"
          :class="{'ei-btn-success': exportDone}"
          :disabled="exportDone"
          @click="doExportShare"
        >
          <template v-if="exportDone">
            <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">✅</span>
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
          data-kbd-item="ei-confirm"
          :class="{'ei-btn-success': importState === 'success', 'ei-btn-error': importState === 'error'}"
          :disabled="!text.trim() || importState !== 'idle'"
          @click="doImport"
        >
          <template v-if="importState === 'success'">
            <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">✅</span>
            <svg v-else width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
            </svg>
            {{ t('importSuccess') }}
          </template>
          <template v-else-if="importState === 'error'">
            <span v-if="settings.legacyBlueActive" class="emoji-icon-sm" aria-hidden="true">❌</span>
            <svg v-else width="13" height="13" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
            </svg>
            {{ t('importError') }}
          </template>
          <template v-else>
            {{ t('importConfirm') }}
          </template>
        </button>
        <button type="button" class="ei-btn" data-kbd-item="ei-cancel" @click="cancel">{{ t('cancel') }}</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.ei-root {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

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

/* arcade light */
.ei-root.is-arcade .ei-btn { color: #92400e; }
.ei-root.is-arcade .ei-btn:hover { background: #fef9c3; color: #78350f; }
.ei-root.is-arcade .ei-btn.ei-btn-primary { background: #fef9c3; color: #92400e; border-color: #fcd34d; }
.ei-root.is-arcade .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #fef08a; color: #78350f; border-color: #f59e0b; }

/* arcade dark */
.ei-root.is-arcade.is-dark .ei-btn { color: #d97706; }
.ei-root.is-arcade.is-dark .ei-btn:hover { background: #422006; color: #fde68a; }
.ei-root.is-arcade.is-dark .ei-btn.ei-btn-primary { background: #422006; color: #fde68a; border-color: #d97706; }
.ei-root.is-arcade.is-dark .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #5c2d06; color: #fef08a; border-color: #f59e0b; }

/* legacy blue light */
.ei-root.is-legacy-blue .ei-btn { color: #4A6FA5; }
.ei-root.is-legacy-blue .ei-btn:hover { background: #EEF3FF; color: #1A3A8C; }
.ei-root.is-legacy-blue .ei-btn.ei-btn-primary { background: #EEF3FF; color: #1A3A8C; border-color: #245EDC; }
.ei-root.is-legacy-blue .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #D8E4FF; color: #0F2870; border-color: #1A3A8C; }
.ei-root.is-legacy-blue .ei-btn.ei-btn-success { background: #EEF8EE; color: #1A6A2A; border-color: #4CAF50; }
.ei-root.is-legacy-blue .ei-btn.ei-btn-error { background: #FEF0EE; color: #8A1A1A; border-color: #DC4444; }

/* legacy blue dark */
.ei-root.is-legacy-blue.is-dark .ei-btn { color: #90B4E0; }
.ei-root.is-legacy-blue.is-dark .ei-btn:hover { background: #172040; color: #B8D4F0; }
.ei-root.is-legacy-blue.is-dark .ei-btn.ei-btn-primary { background: #10193A; color: #90B4E0; border-color: #2A508C; }
.ei-root.is-legacy-blue.is-dark .ei-btn.ei-btn-primary:hover:not(:disabled) { background: #1a2d5a; color: #B8D4F0; border-color: #3d70c0; }
.ei-root.is-legacy-blue.is-dark .ei-btn.ei-btn-success { background: #0A2010; color: #4ADE80; border-color: #1A6A2A; }
.ei-root.is-legacy-blue.is-dark .ei-btn.ei-btn-error { background: #200A0A; color: #F87171; border-color: #6A1A1A; }

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

/* arcade light */
.ei-root.is-arcade .ei-textarea { border-color: #fcd34d; background: #fefce8; color: #78350f; }
.ei-root.is-arcade .ei-textarea:focus { border-color: #f59e0b; background: #fefce8; }

/* arcade dark */
.ei-root.is-arcade.is-dark .ei-textarea { border-color: #d97706; background: #1c0a00; color: #fde68a; }
.ei-root.is-arcade.is-dark .ei-textarea:focus { border-color: #f59e0b; background: #1c0a00; }

/* legacy blue light */
.ei-root.is-legacy-blue .ei-textarea { border-color: #B8CAEE; background: #F5F8FF; color: #1A3A8C; }
.ei-root.is-legacy-blue .ei-textarea:focus { border-color: #245EDC; background: #fff; }

/* legacy blue dark */
.ei-root.is-legacy-blue.is-dark .ei-textarea { border-color: #2A508C; background: #0A1020; color: #90B4E0; }
.ei-root.is-legacy-blue.is-dark .ei-textarea:focus { border-color: #3d70c0; background: #0A1020; }

.ei-code {
  font-size: 1.125rem;
  letter-spacing: 0.2em;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.ei-code::placeholder {
  font-size: 0.75rem;
  letter-spacing: normal;
}

.ei-note {
  font-size: 0.75rem;
  line-height: 1.45;
  color: #64748b;
}

.ei-root.is-dark .ei-note { color: #94a3b8; }

.ei-group-text {
  margin-top: 0.25rem;
}

.ei-receive {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0 0.25rem;
}

/* Dark on white in every theme: that's what cameras read. */
.ei-qr {
  width: 10rem;
  height: 10rem;
  padding: 0.625rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  background: #fff;
}

.ei-qr :deep(svg) {
  display: block;
  width: 100%;
  height: 100%;
}

.ei-root.is-dark .ei-qr { border-color: transparent; }

.ei-receive-code {
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  font-variant-numeric: tabular-nums;
}

.ei-summary {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  width: 100%;
  max-width: 16rem;
  font-size: 0.8125rem;
}

.ei-summary-row {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.ei-summary dd {
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.ei-receive .ei-note {
  max-width: 18rem;
  text-align: center;
}

/* ── actions row ── */
.ei-actions {
  display: flex;
  gap: 0.25rem;
}
</style>
