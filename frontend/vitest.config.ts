import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// Constrained CI runners — especially the self-hosted Forgejo act runner, which
// reports the host's full core count while being cgroup-limited — oversubscribe
// vitest's default forks pool (one worker per reported CPU). Workers then fail
// their IPC handshake ("Failed to start forks worker / Timeout waiting for
// worker to respond") and the first test per file blows the 5s default. Cap the
// worker count and add startup headroom on CI only; local dev keeps full
// parallelism. (`maxWorkers` is the Vitest 4 top-level replacement for the
// removed `poolOptions.forks.maxForks`; forks stays the default pool.)
const isCI = !!process.env.CI

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': resolve(__dirname, 'src') },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    ...(isCI
      ? {
          maxWorkers: 2,
          testTimeout: 15000,
          hookTimeout: 15000,
        }
      : {}),
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: ['src/**/*.{ts,vue}'],
      exclude: ['src/**/__tests__/**', 'src/main.ts'],
      thresholds: {
        statements: 47,
        branches: 41,
        functions: 34,
        lines: 47,
      },
    },
  },
})
