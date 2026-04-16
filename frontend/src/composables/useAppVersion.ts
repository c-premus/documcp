import { ref } from 'vue'

const version = ref('')
let fetched = false

export function useAppVersion() {
  if (!fetched) {
    fetched = true
    fetch('/health')
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data?.version) version.value = data.version
      })
      .catch(() => {})
  }
  return { version }
}
