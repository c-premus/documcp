import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'
import { copyFileSync } from 'fs'

// Vite rewrites all "/" paths in HTML to "/admin/" due to the base config.
// Favicons, icons, and the web app manifest must be served at the root
// (unauthenticated), so this plugin rewrites them back after Vite's built-in
// HTML transform.
function rootAssets() {
  const rootPaths = [
    '/favicon.ico',
    '/favicon.svg',
    '/favicon-96x96.png',
    '/apple-touch-icon.png',
    '/site.webmanifest',
  ]
  return {
    name: 'root-assets',
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

// copyOpenAPISpec copies the canonical OpenAPI spec into public/ so Vite
// bundles it into dist/. The Go server then embeds dist/ and serves it at
// /openapi.yaml (see web/frontend/handler.go rootAssetAllowlist). Keeping the
// canonical source at docs/contracts/openapi.yaml and deriving the build copy
// avoids a committed duplicate drifting from the contract.
function copyOpenAPISpec() {
  const src = resolve(__dirname, '../docs/contracts/openapi.yaml')
  const dest = resolve(__dirname, 'public/openapi.yaml')
  return {
    name: 'copy-openapi-spec',
    buildStart() {
      copyFileSync(src, dest)
    },
    configureServer() {
      copyFileSync(src, dest)
    },
  }
}

export default defineConfig({
  plugins: [vue(), tailwindcss(), rootAssets(), copyOpenAPISpec()],
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
    chunkSizeWarningLimit: 1400,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/oauth': 'http://localhost:8080',
      '/health': 'http://localhost:8080',
    },
  },
})
