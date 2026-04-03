import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

// Vite rewrites all "/" paths in HTML to "/admin/" due to the base config.
// Favicons and icons must be served at the root (unauthenticated), so this
// plugin rewrites them back after Vite's built-in HTML transform.
function rootFavicons() {
  const rootPaths = [
    '/favicon.ico',
    '/favicon-16x16.png',
    '/favicon-32x32.png',
    '/apple-touch-icon.png',
  ]
  return {
    name: 'root-favicons',
    transformIndexHtml: {
      order: 'post' as const,
      handler(html: string) {
        for (const p of rootPaths) {
          html = html.replaceAll(`/admin${p}`, p)
        }
        return html
      },
    },
  }
}

export default defineConfig({
  plugins: [vue(), tailwindcss(), rootFavicons()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      '@docs': resolve(__dirname, '../docs'),
    },
  },
  base: '/admin/',
  build: {
    outDir: '../web/frontend/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/oauth': 'http://localhost:8080',
    },
  },
})
