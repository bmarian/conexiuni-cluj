<script setup lang="ts">
import {ref} from 'vue'
import {useRouter} from 'vue-router'
import '@/components/admin/admin.css'
import {putJSON, SignedOut, writeAuthFlag} from '@/composables/useAdmin.ts'
import {cityPath, currentCity} from '@/utils/city.ts'
import {network, refreshNetwork} from '@/utils/network.ts'

const props = defineProps<{stopId: number}>()

const router = useRouter()
const busy = ref(false)
const saveError = ref('')

async function hide() {
  busy.value = true
  saveError.value = ''
  try {
    const stopIds = (network.value?.platforms(props.stopId) ?? []).map((s) => s.stop_id)
    await putJSON(`/api/admin/hidden-stops?city=${currentCity.value}`, {stop_ids: stopIds, hidden: true})
    await refreshNetwork(true)
    await router.replace(cityPath('/'))
  } catch (err) {
    if (err instanceof SignedOut) writeAuthFlag(false)
    else saveError.value = (err as Error).message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="tt-head">
    <span class="text-title-small text-on-surface-variant">Stop admin</span>
    <span class="divider"></span>
  </div>
  <dl class="admin-rows">
    <div class="has-action">
      <dt>Shown to passengers</dt>
      <dd><button type="button" class="btn btn-text" :disabled="busy" @click="hide">Hide</button></dd>
    </div>
  </dl>
  <p v-if="saveError" class="admin-note admin-bad">{{ saveError }}</p>
</template>
