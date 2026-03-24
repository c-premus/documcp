import { client } from './generated/client.gen'

client.setConfig({
  baseUrl: import.meta.env.VITE_API_BASE_URL || '',
})

client.interceptors.response.use((response: Response) => {
  if (response.status === 401) {
    window.location.href =
      '/auth/login?redirect=' + encodeURIComponent(window.location.pathname)
  }
  if (response.status === 403) {
    window.dispatchEvent(
      new CustomEvent('api:forbidden', {
        detail: { url: response.url },
      }),
    )
  }
  return response
})

export { client }
