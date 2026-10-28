<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import FieldButton from '@/components/ui/FieldButton.vue'
import SelectField from '@/components/ui/SelectField.vue'
import TimePickerDialog from '@/components/ui/TimePickerDialog.vue'
import {cityDate} from '@/utils/cityClock.ts'
import {planDayLabel, planDays} from '@/utils/planDays.ts'
import {ScheduleIcon} from '@/utils/icons.ts'

const OTHER_DAY = 'other'

const props = defineProps<{now: Date}>()
const value = defineModel<string>({required: true})

const {t, locale} = useI18n()
const picking = ref(false)
const datePicker = ref<HTMLInputElement | null>(null)

const today = computed(() => cityDate(props.now))
const day = computed(() => value.value.slice(0, 10))
const time = computed(() => value.value.slice(11, 16))
const days = computed(() => [
  ...planDays(today.value, day.value).map((d) => ({value: d, label: planDayLabel(d, today.value, locale.value, t)})),
  {value: OTHER_DAY, label: t('planOtherDay')},
])

function setDay(next: string) {
  if (next === OTHER_DAY) return datePicker.value?.showPicker()
  value.value = `${next}T${time.value}`
}

function setTime(next: string) {
  value.value = `${day.value}T${next}`
}

function onDatePicked() {
  if (datePicker.value?.value) setDay(datePicker.value.value)
}
</script>

<template>
  <div class="plan-time">
    <FieldButton :label="t('planTime')" :value="time" :icon="ScheduleIcon" aria-haspopup="dialog" data-kbd-item="time-clock" @click="picking = true" />
    <div class="day">
      <SelectField :model-value="day" :label="t('planDay')" :options="days" kbd="time-day" @update:model-value="setDay" />
      <input ref="datePicker" class="date-picker" type="date" tabindex="-1" aria-hidden="true" :min="today" :value="day" @change="onDatePicked" />
    </div>
    <TimePickerDialog v-model:open="picking" :time="time" @pick="setTime" />
  </div>
</template>

<style scoped>
.plan-time {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
  gap: 8px;
}

.day {
  position: relative;
  min-width: 0;
}

.date-picker {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 0;
  padding: 0;
  border: 0;
  opacity: 0;
  pointer-events: none;
}
</style>
