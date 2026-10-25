<script setup lang="ts">
import {nextTick, onMounted, onUnmounted, ref} from 'vue'

const props = defineProps<{root: HTMLElement | null}>()
const emit = defineEmits<{more: []}>()

const el = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      if (!entries.some((e) => e.isIntersecting)) return
      emit('more')
      // It only reports changes, so look again while still in view.
      void nextTick(() => {
        if (!observer || !el.value) return
        observer.unobserve(el.value)
        observer.observe(el.value)
      })
    },
    {root: props.root, rootMargin: '0px 0px 100% 0px'},
  )
  if (el.value) observer.observe(el.value)
})

onUnmounted(() => observer?.disconnect())
</script>

<template>
  <div ref="el" aria-hidden="true"></div>
</template>
