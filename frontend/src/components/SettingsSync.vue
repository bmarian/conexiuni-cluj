<script setup lang="ts">
import {computed, defineAsyncComponent, nextTick, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import ListItem from '@/components/ui/ListItem.vue'
import ToggleRow from '@/components/ui/ToggleRow.vue'
import {useKbdEscape, useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import {useSyncPairing} from '@/composables/useSyncPairing.ts'
import {useSyncStore} from '@/stores/sync'
import {cities} from '@/utils/city.ts'
import {canSync} from '@/utils/sync/crypto.ts'
import {CITY_CAP} from '@/utils/sync/records.ts'
import {focusItem} from '@/utils/keyboardFocus.ts'
import {
  AddIcon,
  BrightnessMediumIcon,
  CheckIcon,
  CloseIcon,
  ComputerIcon,
  DeleteIcon,
  DialpadIcon,
  LogoutIcon,
  QrCode2Icon,
  SmartphoneIcon,
  WatchIcon,
} from '@/utils/icons.ts'

const QrCode = defineAsyncComponent(() => import('@/components/QrCode.vue'))

const props = defineProps<{pairCode?: string; pairKey?: string}>()

const {t, locale} = useI18n()
const store = useSyncStore()
const pairing = useSyncPairing()
const {phase, failure, overCap, code, url, check, peer} = pairing

const supported = canSync()
const entering = ref(false)
const typed = ref('')
const codeRef = ref<HTMLInputElement | null>(null)
const asking = ref<{kind: 'remove'; id: string; name: string} | {kind: 'off' | 'delete'} | null>(null)
const failedRemove = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const {keyboardMode} = useKeyboardNav()
let lastAction: 'show' | 'enter' = 'show'

const digits = computed(() => typed.value.replace(/\D/g, ''))
const spaced = (value: string) => `${value.slice(0, 3)} ${value.slice(3)}`
const kindIcon = (kind: string) => (kind === 'android' ? SmartphoneIcon : kind === 'wear' ? WatchIcon : ComputerIcon)
const kindName = (kind: string) => (kind === 'android' ? 'Android' : kind === 'wear' ? 'Wear OS' : t('syncWebDevice'))

const pairingOpen = computed(() => entering.value || phase.value !== 'idle')

function seenText(seen: number): string {
  const day = new Date(seen * 1000)
  const today = new Date()
  if (day.getUTCFullYear() === today.getUTCFullYear() && day.getUTCMonth() === today.getUTCMonth() && day.getUTCDate() === today.getUTCDate()) {
    return t('syncSeenToday')
  }
  return t('syncSeen', {date: day.toLocaleDateString(locale.value, {day: 'numeric', month: 'short', timeZone: 'UTC'})})
}

const statusText = computed(() => {
  if (store.busy) return t('syncSyncing')
  if (store.offline) return t('syncOffline')
  if (!store.lastSynced) return ''
  return t('syncSynced', {time: new Date(store.lastSynced).toLocaleTimeString(locale.value, {hour: '2-digit', minute: '2-digit'})})
})

const problemTexts = computed(() =>
  store.problems.map(
    (p) =>
      ({
        broken: t('syncProblemBroken'),
        update: t('syncProblemUpdate'),
        'over-cap': t('syncProblemOverCap', {n: CITY_CAP}),
        full: t('syncProblemFull'),
        removed: t('syncRemoved'),
      })[p],
  ),
)

const listName = (list: string) =>
  ({stops: t('favoriteStops'), lines: t('favoriteRoutes'), places: t('favoritePlans'), followed: t('followedLines')})[list] ?? list
const cityName = (slug: string) => cities.value.find((c) => c.slug === slug)?.name ?? slug

const failureText = computed(
  () =>
    failure.value &&
    {
      expired: t('syncFailExpired'),
      unknown: t('syncFailUnknown'),
      limited: t('syncFailLimited'),
      broken: t('syncFailBroken'),
      both: t('syncFailBoth'),
      'over-cap': t('syncFailOverCap', {n: CITY_CAP}),
      offline: t('syncFailOffline'),
    }[failure.value],
)

async function bringIntoView() {
  await nextTick()
  rootRef.value?.querySelector('.panel')?.scrollIntoView({block: 'nearest', behavior: 'smooth'})
}

function show() {
  lastAction = 'show'
  entering.value = false
  void pairing.show()
  void bringIntoView()
}

async function openEnter() {
  lastAction = 'enter'
  pairing.cancel()
  entering.value = true
  typed.value = ''
  await bringIntoView()
  codeRef.value?.focus({preventScroll: true})
}

function connect() {
  if (digits.value.length !== 6) return
  entering.value = false
  void pairing.enter(digits.value)
}

function retry() {
  if (lastAction === 'show') show()
  else void openEnter()
}

function close() {
  pairing.cancel()
  entering.value = false
}

// The code from a scanned QR, with its key; scanning was the user's go-ahead.
watch(
  () => props.pairCode,
  (scanned) => {
    if (!scanned || !/^\d{6}$/.test(scanned) || !supported) return
    lastAction = 'enter'
    entering.value = false
    void pairing.enter(scanned, props.pairKey)
    void bringIntoView()
  },
  {immediate: true},
)

watch(phase, (now) => {
  if (now === 'done')
    setTimeout(() => {
      if (phase.value === 'done') close()
    }, 1500)
  if (now === 'confirm' && keyboardMode.value) {
    void nextTick(() => {
      const button = rootRef.value?.querySelector<HTMLElement>('[data-kbd-item="sync-confirm"]')
      if (button) focusItem(button)
    })
  }
})

useKbdEscape(
  () => pairingOpen.value || asking.value !== null,
  () => {
    if (asking.value) asking.value = null
    else close()
  },
)

async function confirmAsk() {
  const ask = asking.value
  if (!ask) return
  asking.value = null
  failedRemove.value = false
  try {
    if (ask.kind === 'remove') await store.removeDevice(ask.id)
    else await store.stop(ask.kind === 'delete')
  } catch {
    failedRemove.value = ask.kind === 'remove'
  }
}
</script>

<template>
  <div ref="rootRef" class="sync">
    <p class="text-body-small text-on-surface-variant py-1">{{ t('syncDesc') }}</p>

    <p v-if="!supported" class="text-body-medium">{{ t('syncNeedsHttps') }}</p>
    <template v-else>
      <p v-if="store.removed && store.status === 'off' && !pairingOpen" class="note">{{ t('syncRemoved') }}</p>

      <template v-if="store.status === 'on'">
        <p v-if="statusText" class="text-body-small text-on-surface-variant">{{ statusText }}</p>
        <p v-for="text in problemTexts" :key="text" class="note">{{ text }}</p>
        <p v-if="failedRemove" class="note">{{ t('syncRemoveFailed') }}</p>
        <div class="devices" data-kbd-section="sync-devices">
          <div v-for="device in store.devices" :key="device.id" class="device">
            <component :is="kindIcon(device.kind)" class="m-icon text-on-surface-variant" />
            <span class="flex-1 min-w-0 flex flex-col">
              <span class="text-body-large text-on-surface">{{ device.label || kindName(device.kind) }}</span>
              <span class="text-body-medium text-on-surface-variant">{{ device.self ? t('syncThisDevice') : seenText(device.seen) }}</span>
            </span>
            <button
              v-if="!device.self"
              type="button"
              class="icon-btn"
              :data-kbd-item="`sync-remove-${device.id}`"
              :aria-label="t('syncRemove')"
              :title="t('syncRemove')"
              :disabled="store.busy"
              @click="asking = {kind: 'remove', id: device.id, name: device.label || kindName(device.kind)}"
            >
              <CloseIcon class="m-icon" />
            </button>
          </div>
        </div>
      </template>

      <div v-if="!pairingOpen && !asking" class="button-row" data-kbd-section="sync-start" data-kbd-axis="x">
        <button type="button" class="btn btn-filled" data-kbd-item="sync-show" @click="show">
          <component :is="store.status === 'on' ? AddIcon : QrCode2Icon" class="m-icon" />
          {{ store.status === 'on' ? t('syncAddDevice') : t('syncShowCode') }}
        </button>
        <button type="button" class="btn btn-tonal" data-kbd-item="sync-enter" @click="openEnter">
          <DialpadIcon class="m-icon" />
          {{ t('syncEnterCode') }}
        </button>
      </div>

      <div v-if="pairingOpen" class="panel" :class="{'items-center text-center': !entering}">
        <template v-if="entering">
          <input
            ref="codeRef"
            v-model="typed"
            class="field code-field"
            data-kbd-section="sync-code"
            data-kbd-item="sync-code"
            type="text"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="7"
            :placeholder="t('syncCodePlaceholder')"
            @keydown.enter.prevent="connect"
          />
        </template>
        <template v-else-if="phase === 'showing'">
          <div class="qr">
            <QrCode :text="url" :label="spaced(code)" />
          </div>
          <p class="text-headline-medium font-extrabold tracking-[2px]">{{ spaced(code) }}</p>
          <p class="text-body-small text-on-surface-variant">{{ t('syncShowHint') }}</p>
        </template>
        <template v-else-if="phase === 'confirm' && peer">
          <p class="text-body-large">{{ t('syncConfirmTitle', {name: peer.label}) }}</p>
          <p class="check" :aria-label="check">{{ spaced(check) }}</p>
          <p class="text-body-small text-on-surface-variant">{{ t('syncConfirmCheck') }}</p>
        </template>
        <p v-else-if="phase === 'done'" class="flex items-center gap-2 text-body-medium"><CheckIcon class="m-icon text-primary" />{{ t('syncDone') }}</p>
        <template v-else-if="phase === 'failed'">
          <p class="text-body-medium">{{ failureText }}</p>
          <ul v-if="failure === 'over-cap'" class="text-body-medium">
            <li v-for="item in overCap" :key="`${item.city}-${item.list}`">
              {{ t('syncOverCapItem', {list: listName(item.list), city: cityName(item.city), count: item.count}) }}
            </li>
          </ul>
        </template>
        <p v-else class="flex items-center gap-2 text-body-medium">
          <span class="spinner"></span>{{ phase === 'waiting' ? t('syncWaiting') : phase === 'starting' ? t('syncStarting') : t('syncConnecting') }}
        </p>

        <div v-if="phase !== 'done'" class="button-row w-full" data-kbd-section="sync-actions" data-kbd-axis="x">
          <button v-if="entering" type="button" class="btn btn-filled" data-kbd-item="sync-connect" :disabled="digits.length !== 6" @click="connect">
            {{ t('syncConnect') }}
          </button>
          <button v-else-if="phase === 'confirm'" type="button" class="btn btn-filled" data-kbd-item="sync-confirm" @click="pairing.confirm()">
            {{ t('syncConfirm') }}
          </button>
          <button
            v-else-if="phase === 'failed' && failure !== 'over-cap' && failure !== 'both'"
            type="button"
            class="btn btn-filled"
            data-kbd-item="sync-retry"
            @click="retry"
          >
            {{ t('syncTryAgain') }}
          </button>
          <button type="button" class="btn btn-text" data-kbd-item="sync-cancel" @click="close"><CloseIcon class="m-icon" />{{ t('cancel') }}</button>
        </div>
      </div>

      <div v-if="asking" class="panel">
        <p class="text-body-medium">
          {{ asking.kind === 'remove' ? t('syncRemoveAsk', {name: asking.name}) : asking.kind === 'off' ? t('syncTurnOffAsk') : t('syncDeleteAsk') }}
        </p>
        <div class="button-row" data-kbd-section="sync-ask" data-kbd-axis="x">
          <button type="button" class="btn btn-filled is-error" data-kbd-item="sync-ask-yes" @click="confirmAsk">
            {{ asking.kind === 'remove' ? t('syncRemove') : asking.kind === 'off' ? t('syncTurnOff') : t('syncDelete') }}
          </button>
          <button type="button" class="btn btn-outlined" data-kbd-item="sync-ask-no" @click="asking = null">{{ t('cancel') }}</button>
        </div>
      </div>

      <div v-if="store.status === 'on' && !pairingOpen && !asking" class="-mx-4" data-kbd-section="sync-options">
        <ToggleRow
          :icon="BrightnessMediumIcon"
          :title="t('syncSettings')"
          :description="t('syncSettingsDesc')"
          :checked="store.withSettings"
          data-kbd-item="sync-settings"
          @change="store.setWithSettings"
        />
        <ListItem :icon="LogoutIcon" :title="t('syncTurnOff')" data-kbd-item="sync-off" @click="asking = {kind: 'off'}" />
        <ListItem :icon="DeleteIcon" :title="t('syncDelete')" data-kbd-item="sync-delete" @click="asking = {kind: 'delete'}" />
      </div>
    </template>
  </div>
</template>

<style scoped>
.sync {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 0 16px 8px;
}

.devices {
  display: flex;
  flex-direction: column;
  margin: 0 -16px;
}

.device {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 56px;
  padding: 8px 12px 8px 20px;
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

.note {
  padding: 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--md-amber) 12%, transparent);
  font-size: 12px;
  line-height: 16px;
  letter-spacing: 0.4px;
  color: var(--md-on-surface);
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

.qr {
  width: 170px;
  height: 170px;
  padding: 10px;
  border-radius: 8px;
  background: #fff;
  box-shadow: inset 0 0 0 1px #e2e8f0;
}

.check {
  font-size: 36px;
  line-height: 44px;
  font-weight: 800;
  letter-spacing: 4px;
  font-variant-numeric: tabular-nums;
}

.btn.is-error {
  background: var(--md-error);
  color: var(--md-on-error);
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid var(--md-primary);
  border-right-color: transparent;
  border-radius: 50%;
  animation: spin 800ms linear infinite;
}

@keyframes spin {
  to {
    rotate: 360deg;
  }
}
</style>
