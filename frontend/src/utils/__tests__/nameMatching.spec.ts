import {describe, expect, it} from 'vitest'
import {editDistance, matching, nameMatches} from '@/utils/stopSearch.ts'
import {fold} from '@/utils/text.ts'

const matches = (name: string, query: string, typos = false) => nameMatches(fold(name), fold(query), typos)

describe('nameMatches', () => {
  it('takes the words in any order', () => {
    expect(matches('Bulevardul 21 Decembrie 1989', 'decembrie 21')).toBe(true)
  })

  it('reads shortened words as what they stand for', () => {
    expect(matches('P-ta M. Viteazu', 'piata mihai viteazu')).toBe(true)
    expect(matches('P-ța Unirii', 'piata unirii')).toBe(true)
    expect(matches('C-tin Brâncoveanu Est', 'constantin brancoveanu')).toBe(true)
    expect(matches('B-dul Muncii', 'bulevardul muncii')).toBe(true)
    expect(matches('M.Viteazu', 'mihai viteazu')).toBe(true)
    expect(matches('Str. Unirii', 'strada unirii')).toBe(true)
  })

  it('finds a word after the metropolitan mark', () => {
    expect(matches('M-Școala Gimnazială', 'scoala')).toBe(true)
  })

  it('needs no kind of street the name does not have', () => {
    expect(matches('Horea', 'strada horea')).toBe(true)
    expect(matches('Horea', 'bd horea')).toBe(true)
  })

  it('does not take an initial alone for a word', () => {
    expect(matches('P-ta M. Viteazu', 'marasti')).toBe(false)
    expect(matches('Str. Unirii', 'strazi')).toBe(false)
  })

  it('needs every word', () => {
    expect(matches('Iulius Mall Est', 'iulius vest')).toBe(false)
  })

  it('forgives a typo only when asked', () => {
    expect(matches('Iulius Mall Est', 'iulus')).toBe(false)
    expect(matches('Iulius Mall Est', 'iulus', true)).toBe(true)
    expect(matches('Memorandumului Sud', 'memorandomulu', true)).toBe(true)
  })

  it('keeps short words and numbers exact', () => {
    expect(matches('Gara Mare', 'gar', true)).toBe(true)
    expect(matches('Mare', 'mar', true)).toBe(true)
    expect(matches('Bar', 'car', true)).toBe(false)
    expect(matches('Linia 25', '24', true)).toBe(false)
    expect(matches('Scoala 2019', '2018', true)).toBe(false)
  })
})

describe('editDistance', () => {
  it('counts a swap as one', () => {
    expect(editDistance('lidl', 'lild', 2)).toBe(1)
  })

  it('gives up past the limit', () => {
    expect(editDistance('kaufland', 'lidl', 1)).toBe(2)
  })
})

describe('matching', () => {
  const name = (s: string) => fold(s)

  it('ranks what the name holds whole before what its words match', () => {
    const found = matching(['P-ta M. Viteazu', 'Mihai Viteazu'], name, 'mihai viteazu')
    expect(found.map((m) => m.rank)).toEqual([3, 0])
  })

  it('tries typos only with no other match', () => {
    expect(matching(['Iulius Mall', 'Iulis'], name, 'iulis').map((m) => m.item)).toEqual(['Iulis'])
    expect(matching(['Iulius Mall'], name, 'iulis').map((m) => m.item)).toEqual(['Iulius Mall'])
  })
})
