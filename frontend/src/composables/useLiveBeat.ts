import {inject, provide, ref, type InjectionKey, type Ref} from 'vue'

const BEAT: InjectionKey<Ref<number | null>> = Symbol('liveBeat')

export const provideLiveBeat = (beat: Ref<number | null>) => provide(BEAT, beat)

export const useLiveBeat = (): Ref<number | null> => inject(BEAT, ref(null))
