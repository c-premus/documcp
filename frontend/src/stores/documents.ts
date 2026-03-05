import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useDocumentsStore = defineStore('documents', () => {
  const loading = ref(false)

  return { loading }
})
