import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': resolve(__dirname, 'src') },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: ['src/**/*.{ts,vue}'],
      exclude: [
        'src/api/sdk/**',
        'src/api/generated/**',
        'src/**/__tests__/**',
        'src/main.ts',
      ],
      // Ratcheted after WP2 test sprint. Target: 70/60/60/70.
      thresholds: {
        statements: 39,
        branches: 33,
        functions: 33,
        lines: 39,
      },
    },
  },
})
