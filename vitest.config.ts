import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// Vite plugin: resolve static asset references (e.g., /logo.png) to the
// actual file in the assets directory so they can be found during tests.
// In production Vite serves these from publicDir, but during unit tests
// the module resolver needs an explicit alias.
function staticAssetResolver(): import('vite').Plugin {
  return {
    name: 'static-asset-resolver',
    resolveId(source) {
      if (source === '/logo.png') {
        return resolve(__dirname, 'assets/logo.png')
      }
    },
  }
}

export default defineConfig({
  plugins: [vue(), staticAssetResolver()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'web/src'),
    },
  },
  publicDir: resolve(__dirname, 'assets'),
  test: {
    environment: 'jsdom',
    css: true,
    exclude: [
      '**/.worktrees/**',
      '**/node_modules/**',
      '**/dist/**',
      '**/cypress/**',
      '**/.{idea,git,cache,output,temp}/**',
    ],
    coverage: {
      reporter: ['text', 'json', 'json-summary'],
    },
    setupFiles: [resolve(__dirname, 'web/src/test-setup.ts')],
  },
})
