<script setup lang="ts">
import {ref} from 'vue'
import {useRouter} from 'vue-router'
import '@/components/admin/admin.css'
import {putJSON, SignedOut, writeAuthFlag} from '@/composables/useAdmin.ts'
import {cityPath, currentCity} from '@/utils/city.ts'
import {refreshNetwork} from '@/utils/network.ts'

const props = defineProps<{title: string; list: 'hidden-stops' | 'hidden-lines'; body: Record<string, unknown>}>()

const router = useRouter()
const busy = ref(false)
const saveError = ref('')

async function hide() {
  busy.value = true
  saveError.value = ''
  try {
    await putJSON(`/api/admin/${props.list}?city=${currentCity.value}`, {...props.body, hidden: true})
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
    <span class="text-title-small text-on-surface-variant">{{ title }}</span>
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
