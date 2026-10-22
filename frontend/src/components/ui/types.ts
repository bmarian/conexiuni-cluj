import type {Component} from 'vue'

export type MenuAction = { label: string, icon: Component, onClick: () => void }

export type ChipAction = {
  label: string
  icon: Component
  primary?: boolean
  selected?: boolean
  tint?: string
  kbd?: string
  disabled?: boolean
  onClick: () => void
}

export type Choice<T extends string> = { value: T, label: string, icon?: Component, disabled?: boolean }
