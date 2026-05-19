import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'web/src'),
    },
  },
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
  },
})
