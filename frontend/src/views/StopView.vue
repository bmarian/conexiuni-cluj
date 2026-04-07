<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts";
import {storeToRefs} from "pinia";
import {onMounted, ref} from "vue";
import type {StopInfo} from "@/types/tranzy.ts";

const props = defineProps<{
  stopId: string
}>()
const userStore = useUserStore()
const {userLocation, userTime} = storeToRefs(userStore)
const stopInfo = ref<StopInfo>()

onMounted(async () => {
  const response = await fetch(`/api/stop_info?stop_id=${props.stopId}`)
  if (response.ok) {
    stopInfo.value = await response.json()
  }
})
</script>

<template>
  <div>{{ userTime }}</div>
  <div>{{ userLocation?.latitude }} -- {{ userLocation?.longitude }}</div>
  <pre>{{ stopInfo }}</pre>
</template>

<style scoped lang="scss">

</style>
