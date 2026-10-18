// Settings that only make sense in some cities. Each is kept per city and read for the city of the page.

export type RegionSettingKey = 'showGreenFriday'

export type RegionSetting = {
  key: RegionSettingKey
  // Where the setting exists at all.
  cities: string[]
  defaultOn: boolean
  titleKey: string
  descKey: string
}

export const REGION_SETTINGS: RegionSetting[] = [
  // CTP Cluj runs free on Fridays.
  {key: 'showGreenFriday', cities: ['cluj'], defaultOn: true, titleKey: 'greenFridayTitle', descKey: 'greenFridaySettingDesc'},
]

export const regionSettingsFor = (slug: string) => REGION_SETTINGS.filter((s) => s.cities.includes(slug))

export const regionSettingStorageKey = (slug: string, key: RegionSettingKey) => `${slug}:settings.${key}`
