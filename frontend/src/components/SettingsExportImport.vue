<script setup lang="ts">
import {computed, defineAsyncComponent, nextTick, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {useKbdEscape, useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import {describeExport, useSettingsTransfer} from '@/composables/useSettingsTransfer.ts'
import {useTransferReceiver} from '@/composables/useTransferReceiver.ts'
import {focusItem} from '@/utils/keyboardFocus.ts'
import {sendToTransferLink, TRANSFER_QUERY} from '@/utils/transferLink.ts'
import {CheckIcon, CloseIcon, ContentCopyIcon, FileUploadIcon, QrCode2Icon, SendIcon} from '@/utils/icons.ts'

const QrCode = defineAsyncComponent(() => import('@/components/QrCode.vue'))

const props = defineProps<{ sendCode?: string; sendKey?: string }>()

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
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
  <div ref="rootRef" class="transfer">
    <div class="button-row" data-kbd-section="ei-transfer" data-kbd-axis="x">
      <button type="button" class="btn btn-filled" data-kbd-item="ei-send" @click="openSend()">
        <SendIcon class="m-icon"/>
        {{ t('transferSend') }}
      </button>
      <button type="button" class="btn btn-filled" data-kbd-item="ei-receive" @click="openReceive">
        <QrCode2Icon class="m-icon"/>
        {{ t('transferReceive') }}
      </button>
    </div>

    <div v-if="mode === 'send'" class="panel">
      <input
        ref="codeRef"
        v-model="code"
        class="field code-field"
        data-kbd-section="ei-code"
        data-kbd-item="ei-code"
        type="text"
        inputmode="numeric"
        autocomplete="one-time-code"
        maxlength="7"
        :placeholder="t('sendCodePlaceholder')"
        @keydown.enter="doSend"
      />
      <div class="button-row" data-kbd-section="ei-send-actions" data-kbd-axis="x">
        <button
          type="button"
          class="btn btn-filled"
          data-kbd-item="ei-send-confirm"
          :class="{ 'is-success': sendState === 'sent', 'is-error': sendState === 'unknown' || sendState === 'limited' || sendState === 'failed' }"
          :disabled="!canSend && sendState === 'idle'"
          @click="doSend"
        >
          <span v-if="sendState === 'busy'" class="spinner"></span>
          <template v-else-if="sendState === 'sent'">✓ {{ t('sendDone') }}</template>
          <template v-else-if="sendState === 'unknown' || sendState === 'limited' || sendState === 'failed'">
            ✕ {{ sendState === 'unknown' ? t('sendUnknownCode') : sendState === 'limited' ? t('sendTooMany') : t('sendFailed') }}
          </template>
          <template v-else>{{ t('sendConfirm') }}</template>
        </button>
        <button type="button" class="btn btn-outlined" data-kbd-item="ei-cancel" @click="cancel">{{ t('cancel') }}</button>
      </div>
    </div>

    <div v-else-if="mode === 'receive'" class="panel items-center text-center">
      <template v-if="receiveState === 'waiting'">
        <div class="qr">
          <QrCode :text="receiveUrl" :label="receiveCode"/>
        </div>
        <p class="text-headline-medium font-extrabold tracking-[2px]">{{ receiveCode }}</p>
        <p class="text-body-small text-on-surface-variant">{{ t('transferHint') }}</p>
      </template>
      <template v-else-if="receiveState === 'review' || receiveState === 'done'">
        <p class="text-body-medium">{{ t('receiveReview') }}</p>
        <dl class="summary">
          <div v-for="row in summaryRows" :key="row.label" class="flex justify-between">
            <dt class="text-body-medium">{{ row.label }}</dt>
            <dd class="text-body-medium font-bold">{{ row.count }}</dd>
          </div>
        </dl>
      </template>
      <p v-else-if="receiveState === 'expired' || receiveState === 'failed' || receiveState === 'broken'" class="text-body-medium">
        {{ t({expired: 'transferExpired', failed: 'transferFailed', broken: 'transferBroken'}[receiveState]) }}
      </p>
      <p v-else class="flex items-center gap-2 text-body-medium">
        <span class="spinner is-primary"></span>{{ t('transferLoading') }}
      </p>
      <div class="button-row w-full" data-kbd-section="ei-receive-actions" data-kbd-axis="x">
        <button v-if="receiveState === 'done'" type="button" class="btn btn-filled" disabled>✓ {{ t('receiveDone') }}</button>
        <template v-else>
          <button v-if="receiveState === 'review'" type="button" class="btn btn-filled" data-kbd-item="ei-receive-confirm" @click="replaceWithReceived">
            {{ t('receiveReplace') }}
          </button>
          <button
            v-else-if="receiveState === 'expired' || receiveState === 'failed' || receiveState === 'broken'"
            type="button"
            class="btn btn-filled"
            data-kbd-item="ei-new-code"
            @click="startReceive"
          >{{ t('transferNewCode') }}</button>
          <button type="button" class="btn btn-text" data-kbd-item="ei-cancel" @click="cancel">
            <CloseIcon class="m-icon"/>{{ t('cancel') }}
          </button>
        </template>
      </div>
    </div>

    <div class="button-row" data-kbd-section="ei" data-kbd-axis="x">
      <button type="button" class="btn btn-tonal" data-kbd-item="ei-export" @click="openExport">
        <ContentCopyIcon class="m-icon"/>
        {{ t('exportSettings') }}
      </button>
      <button type="button" class="btn btn-tonal" data-kbd-item="ei-import" @click="openImport">
        <FileUploadIcon class="m-icon"/>
        {{ t('importSettings') }}
      </button>
    </div>

    <template v-if="mode === 'export' || mode === 'import'">
      <textarea
        ref="textareaRef"
        v-model="text"
        class="field text-field"
        data-kbd-section="ei-text"
        data-kbd-item="ei-text"
        :readonly="mode === 'export'"
        :placeholder="mode === 'import' ? t('importPastePlaceholder') : ''"
        rows="6"
        spellcheck="false"
        autocomplete="off"
        autocorrect="off"
        autocapitalize="off"
      />
      <div class="button-row" data-kbd-section="ei-actions" data-kbd-axis="x">
        <button
          v-if="mode === 'export'"
          type="button"
          class="btn btn-filled"
          data-kbd-item="ei-share"
          :disabled="exportDone"
          @click="doExportShare"
        >
          <CheckIcon v-if="exportDone" class="m-icon"/>
          {{ exportDone ? t('exportCopied') : canShare ? t('share') : t('exportCopyBtn') }}
        </button>
        <button
          v-if="mode === 'import'"
          type="button"
          class="btn btn-filled"
          data-kbd-item="ei-confirm"
          :class="{ 'is-error': importState === 'error' }"
          :disabled="!text.trim() || importState !== 'idle'"
          @click="doImport"
        >
          {{ importState === 'success' ? `✓ ${t('importSuccess')}` : importState === 'error' ? `✕ ${t('importError')}` : t('importConfirm') }}
        </button>
        <button type="button" class="btn btn-outlined" data-kbd-item="ei-cancel" @click="cancel">{{ t('cancel') }}</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.transfer {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 16px;
}

.button-row {
  display: flex;
  gap: 8px;
}

.button-row > .btn {
  flex: 1;
  min-width: 0;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border-radius: 16px;
  background: var(--md-surface-container);
}

.field {
  width: 100%;
  padding: 16px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--md-on-surface);
  box-shadow: inset 0 0 0 1px var(--md-outline);
  outline: 0;
  caret-color: var(--md-primary);
}

.field:focus {
  box-shadow: inset 0 0 0 2px var(--md-primary);
}

.field::placeholder {
  color: var(--md-on-surface-variant);
  letter-spacing: normal;
  font-weight: 400;
  font-size: 16px;
}

.code-field {
  font-size: 20px;
  line-height: 28px;
  font-weight: 700;
  letter-spacing: 4px;
  text-align: center;
}

.text-field {
  resize: vertical;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 16px;
}

.qr {
  width: 170px;
  height: 170px;
  padding: 10px;
  border-radius: 8px;
  background: #fff;
  box-shadow: inset 0 0 0 1px #e2e8f0;
}

.summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}

.btn.is-success {
  background: var(--md-tertiary);
  color: var(--md-on-tertiary);
}

.btn.is-error {
  background: var(--md-error);
  color: var(--md-on-error);
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: spin 800ms linear infinite;
}

.spinner.is-primary {
  color: var(--md-primary);
}

@keyframes spin {
  to {
    rotate: 360deg;
  }
}
</style>
