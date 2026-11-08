<script setup lang="ts">
import {computed} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {cities, type City} from '@/utils/city.ts'
import {canonicalUrl, pageHead} from '@/utils/site.ts'
import {OpenInNewIcon} from '@/utils/icons.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ListRow from '@/components/ui/ListRow.vue'

const {t, locale} = useI18n()

const list = computed(() => [...cities.value].sort((a, b) => a.name.localeCompare(b.name, locale.value)))

const credits = (city: City) => [city.operator, ...(city.credits ?? []).map((c) => c.name)].join(' · ')

const open = (url?: string) => {
  if (url) window.open(url, '_blank', 'noopener')
}

useHead(() => {
  const title = t('headSourcesTitle')
  const description = t('sourcesIntro')
  const url = canonicalUrl('/surse')
  return pageHead(title, description, url)
})
</script>

<template>
  <div class="sources-view">
    <SheetTopBar :title="t('sources')" />
    <h1 class="text-title-large font-bold px-4 pt-1 pb-2">{{ t('sources') }}</h1>
    <p class="text-body-medium text-on-surface-variant px-4 pb-3">{{ t('sourcesIntro') }}</p>
    <section data-kbd-section="sources" :data-kbd-entry="1">
      <ListRow
        v-for="city in list"
        :key="city.slug"
        :title="city.name"
        :subtitle="credits(city)"
        :href="city.operatorUrl"
        :data-kbd-item="`source-${city.slug}`"
        @open="open(city.operatorUrl)"
      >
        <template #trailing><OpenInNewIcon class="m-icon size-[18px] text-outline mr-3" /></template>
      </ListRow>
    </section>
    <p class="text-body-small text-on-surface-variant px-4 pt-3">{{ t('sourcesMap') }}</p>
    <div class="h-6"></div>
  </div>
</template>
