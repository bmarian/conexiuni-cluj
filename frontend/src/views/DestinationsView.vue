<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {type DestinationSummary, fetchDestinations} from '@/utils/destinationPages.ts'
import {cityParams, cityPath} from '@/utils/city.ts'
import {canonicalUrl} from '@/utils/site.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import SectionLabel from '@/components/ui/SectionLabel.vue'
import ListRow from '@/components/ui/ListRow.vue'
import LoadingIndicator from '@/components/LoadingIndicator.vue'

const {t} = useI18n()
const router = useRouter()

const list = ref<DestinationSummary[] | null>(null)
const loadError = ref(false)

async function load() {
  loadError.value = false
  try {
    list.value = await fetchDestinations()
  } catch {
    loadError.value = true
  }
}

onMounted(load)

const groups = computed(() =>
  [
    {label: t('destsDistricts'), areas: (list.value ?? []).filter((d) => d.district)},
    {label: t('destsTowns'), areas: (list.value ?? []).filter((d) => !d.district)},
  ].filter((g) => g.areas.length),
)

useHead(() => {
  if (!list.value) return {}
  const title = t('headDestsTitle', cityParams())
  const description = t('headDestsDesc', cityParams())
  const url = canonicalUrl(cityPath('/destinations'))
  return {
    title,
    meta: [
      {name: 'description', content: description},
      {property: 'og:title', content: title},
      {property: 'og:description', content: description},
      {property: 'og:url', content: url},
      {name: 'twitter:title', content: title},
      {name: 'twitter:description', content: description},
    ],
    link: [{rel: 'canonical', href: url}],
  }
})
</script>

<template>
  <div class="destinations-view">
    <SheetTopBar :title="t('destinations')" />

    <div v-if="loadError" class="flex flex-col items-start gap-3 p-4">
      <p class="text-body-medium text-on-surface-variant">{{ t('loadFailed') }}</p>
      <button type="button" class="btn btn-tonal" @click="load">{{ t('retry') }}</button>
    </div>

    <LoadingIndicator v-else-if="!list" />

    <template v-else>
      <h1 class="text-title-large font-bold px-4 pt-1 pb-2">{{ t('destsIn', cityParams()) }}</h1>
      <section v-for="(group, g) in groups" :key="group.label" :data-kbd-section="`dests-${g}`" :data-kbd-entry="g + 1">
        <SectionLabel :text="group.label" />
        <ListRow
          v-for="area in group.areas"
          :key="area.slug"
          :title="area.name"
          :subtitle="t('destCounts', {stops: area.stops, lines: area.lines})"
          :data-kbd-item="`dest-${area.slug}`"
          :href="cityPath(`/to/${area.slug}`)"
          chevron
          @open="router.push({name: 'destination', params: {slug: area.slug}})"
        />
      </section>
      <div class="h-6"></div>
    </template>
  </div>
</template>
