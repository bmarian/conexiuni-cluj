import {describe, expect, it} from 'vitest'
import {placeIcon} from '@/utils/placeLabel.ts'

describe('placeIcon', () => {
  it('takes a leading emoji word as the icon', () => {
    expect(placeIcon('🏡 Home')).toEqual({icon: '🏡', name: 'Home'})
    expect(placeIcon('👩‍🎓 Facultate de Litere')).toEqual({icon: '👩‍🎓', name: 'Facultate de Litere'})
    expect(placeIcon('🇷🇴 Acasă')).toEqual({icon: '🇷🇴', name: 'Acasă'})
  })

  it('keeps names without one', () => {
    expect(placeIcon('Iulius Mall')).toEqual({name: 'Iulius Mall'})
    expect(placeIcon('1 Mai')).toEqual({name: '1 Mai'})
    expect(placeIcon('🏢')).toEqual({name: '🏢'})
    expect(placeIcon('🏢Work')).toEqual({name: '🏢Work'})
  })
})
