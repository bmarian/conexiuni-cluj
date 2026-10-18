export type RegionSettingKey = 'showGreenFriday'

export type RegionSetting = {
  key: RegionSettingKey
  cities: string[]
  defaultOn: boolean
  titleKey: string
  descKey: string
}

export const REGION_SETTINGS: RegionSetting[] = [
  {key: 'showGreenFriday', cities: ['cluj'], defaultOn: true, titleKey: 'greenFridayTitle', descKey: 'greenFridaySettingDesc'},
]

export const regionSettingsFor = (slug: string) => REGION_SETTINGS.filter((s) => s.cities.includes(slug))

export const regionSettingStorageKey = (slug: string, key: RegionSettingKey) => `${slug}:settings.${key}`
