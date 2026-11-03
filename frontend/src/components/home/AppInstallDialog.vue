<script setup lang="ts">
import {ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useKbdLayer} from '@/composables/useKeyboardNav.ts'
import {useSettingsStore} from '@/stores/settings'
import {PLAY_URL} from '@/utils/androidApp.ts'
import {AlarmIcon, NavigationIcon, NotificationsActiveIcon, WatchIcon} from '@/utils/icons.ts'

const FEATURES = [
  {icon: NavigationIcon, text: 'appFeatureNavigation'},
  {icon: AlarmIcon, text: 'appFeatureReminders'},
  {icon: NotificationsActiveIcon, text: 'appFeatureStopAlert'},
  {icon: WatchIcon, text: 'appFeatureWatch'},
]

const open = defineModel<boolean>('open', {required: true})

const {t} = useI18n()
const settings = useSettingsStore()
const panel = ref<HTMLElement | null>(null)

const close = () => {
  open.value = false
}

useKbdLayer(open, {el: () => panel.value, close})

function dontAskAgain() {
  settings.dismissAppPrompt()
  close()
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="dialog-scrim" @click.self="close">
      <div ref="panel" class="dialog dialog-small" role="dialog" :aria-label="t('appDialogTitle')">
        <h2 class="dialog-title">{{ t('appDialogTitle') }}</h2>
        <ul class="features">
          <li v-for="feature in FEATURES" :key="feature.text" class="feature">
            <component :is="feature.icon" class="m-icon text-on-surface-variant" />
            <span class="text-body-medium">{{ t(feature.text) }}</span>
          </li>
        </ul>
        <div class="dialog-actions">
          <button type="button" class="btn btn-text" @click="dontAskAgain">{{ t('appDontAsk') }}</button>
          <a class="btn btn-filled" :href="PLAY_URL" target="_blank" rel="noopener" @click="close">{{ t('yes') }}</a>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.features {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin: 16px 0 24px;
}

.feature {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>
