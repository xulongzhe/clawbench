import { defineConfig, Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'
import { cpSync, existsSync, mkdirSync, readdirSync } from 'fs'

const __dirname = dirname(fileURLToPath(import.meta.url))
const publicDir = resolve(__dirname, 'public')
const srcAssets = resolve(__dirname, 'assets')

// Ensure public/ exists
if (!existsSync(publicDir)) mkdirSync(publicDir, { recursive: true })

// Copy logo files to public/ so they are served at /assets/*
if (existsSync(srcAssets)) {
  // Ensure public/assets directory exists
  const publicAssets = resolve(publicDir, 'assets')
  if (!existsSync(publicAssets)) mkdirSync(publicAssets, { recursive: true })

  for (const f of readdirSync(srcAssets)) {
    cpSync(resolve(srcAssets, f), resolve(publicAssets, f), { force: true })
  }
}

// Vite plugin: wrap highlight.js theme CSS with attribute selectors
// so light/dark themes can coexist without conflict.
function hljsThemeWrapper(): Plugin {
  return {
    name: 'hljs-theme-wrapper',
    transform(code: string, id: string) {
      if (!id.includes('highlight.js/styles/')) return null
      const theme = id.endsWith('github-dark.css') ? 'dark' : 'light'
      // Wrap all top-level .hljs-* rules with [data-hljs-theme="..."]
      const wrapped = code.replace(
        /^(\.[a-z-]+\s*\{)/gm,
        `[data-hljs-theme="${theme}"] $1`
      )
      return { code: wrapped, map: null }
    },
  }
}

export default defineConfig({
  plugins: [vue(), hljsThemeWrapper()],
  root: 'web',
  publicDir: srcAssets,
  server: {
    host: '0.0.0.0',
    allowedHosts: ['xulongzhe.top', 'your-domain.com', 'localhost', '127.0.0.1'],
    port: 20001,
    proxy: {
      '/api': {
        target: `http://localhost:${process.env.VITE_BACKEND_PORT || 20000}`,
        // Don't buffer SSE responses - needed for streaming chat
        configure: (proxy) => {
          proxy.on('proxyRes', (proxyRes) => {
            if (proxyRes.headers['content-type'] === 'text/event-stream') {
              proxyRes.headers['cache-control'] = 'no-cache'
              proxyRes.headers['x-accel-buffering'] = 'no'
            }
          })
        },
      },
      '/login': `http://localhost:${process.env.VITE_BACKEND_PORT || 20000}`,
      '/dialog': `http://localhost:${process.env.VITE_BACKEND_PORT || 20000}`,
      '/assets': `http://localhost:${process.env.VITE_BACKEND_PORT || 20000}`,
    },
  },
  build: {
    outDir: publicDir,
    emptyOutDir: false,
    assetsDir: '.',
    rollupOptions: {
      input: resolve(__dirname, 'web/index.html'),
    },
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'web/src'),
    },
  },
})
