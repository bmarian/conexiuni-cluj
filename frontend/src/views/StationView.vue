<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts";
import {storeToRefs} from "pinia";

const props = defineProps<{
  stationId: string
}>()
const userStore = useUserStore()
const {userLocation, userTime} = storeToRefs(userStore)

</script>

<template>
<div>{{ stationId }}</div>
<div>{{ userTime }}</div>
<div>{{ userLocation?.latitude }} -- {{ userLocation?.longitude }}</div>
</template>

<style scoped lang="scss">

</style>
