import {ref, watch, type Ref} from 'vue'
import {searchPlaces, type Place} from '@/utils/places.ts'

export type SearchField = 'origin' | 'destination'

export function usePlaceSearch(isOnline: Ref<boolean>) {
  const activeSearchField = ref<SearchField | null>(null)
  const searchQuery = ref('')
  const searchResults = ref<Place[]>([])
  const isSearching = ref(false)
  let searchRequest: AbortController | null = null
  let searchTimeout: ReturnType<typeof setTimeout> | null = null

  async function performSearch() {
    if (!isOnline.value && activeSearchField.value === 'destination') {
      searchResults.value = []
      isSearching.value = false
      return
    }
    const q = searchQuery.value.trim()
    searchRequest?.abort()
    searchRequest = null
    if (q.length < 3) {
      searchResults.value = []
      isSearching.value = false
      return
    }
    const request = new AbortController()
    searchRequest = request
    isSearching.value = true
    try {
      const results = await searchPlaces(q, 5, request.signal)
      if (!request.signal.aborted) searchResults.value = results
    } finally {
      if (searchRequest === request) {
        searchRequest = null
        isSearching.value = false
      }
    }
  }

  watch(searchQuery, () => {
    if (searchTimeout) clearTimeout(searchTimeout)
    searchTimeout = setTimeout(performSearch, 150)
  })

  async function searchNow() {
    if (searchTimeout) clearTimeout(searchTimeout)
    await performSearch()
  }

  function openSearchField(field: SearchField) {
    if (field === 'destination' && !isOnline.value) return
    activeSearchField.value = field
    searchResults.value = []
    searchQuery.value = ''
  }

  function closeSearchField() {
    activeSearchField.value = null
    searchQuery.value = ''
    searchResults.value = []
  }

  return {activeSearchField, searchQuery, searchResults, isSearching, searchNow, openSearchField, closeSearchField}
}
