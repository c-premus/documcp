import { client } from './generated/client.gen'

client.setConfig({
  baseUrl: import.meta.env.VITE_API_BASE_URL || '',
})

client.interceptors.response.use((response) => {
  if (response.status === 401) {
    window.location.href =
      '/auth/login?redirect=' + encodeURIComponent(window.location.pathname)
  }
  return response
})

export { client }
