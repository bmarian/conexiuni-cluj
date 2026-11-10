import {inject, provide, ref, type InjectionKey, type Ref} from 'vue'

const SHEET: InjectionKey<Ref<boolean>> = Symbol('phoneSheet')

export const providePhoneSheet = (sheet: Ref<boolean>) => provide(SHEET, sheet)

export const usePhoneSheet = (): Ref<boolean> => inject(SHEET, ref(false))
