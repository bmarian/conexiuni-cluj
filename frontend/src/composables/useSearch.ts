import {ref} from 'vue'

const open = ref(false)
const query = ref('')

export function useSearch() {
  return {open, query}
}
