<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts";
import {storeToRefs} from "pinia";
import {watch} from "vue";
import {useStopInfoApi} from "@/composables/useStopInfoApi.ts";

const props = defineProps<{
  stopId: string
}>()
const userStore = useUserStore()
const {userLocation, userTime} = storeToRefs(userStore)
const {stopInfo, fetchStopData} = useStopInfoApi()

watch(() => props.stopId, async (newValue) => {
  await fetchStopData(newValue)
}, {immediate: true})
</script>

<template>
  <div>{{ userTime }}</div>
  <div>{{ userLocation?.latitude }} -- {{ userLocation?.longitude }}</div>
  <pre>{{ stopInfo }}</pre>
</template>

<style scoped lang="scss">

</style>
