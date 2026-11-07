import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { VitePWA } from 'vite-plugin-pwa';

// Vue Vapor: no virtual DOM. Every SFC in this app is <script setup vapor>.
// The plain "vue" package export (vue.runtime.esm-bundler.js) doesn't include
// the vapor runtime at all — createApp() silently falls back to a vdom root and
// warns "vapor-in-vdom interop was not installed". Alias to the build that has it.
export default defineConfig({
  base: process.env.VITE_BASE || '/',
  resolve: {
    alias: { vue: 'vue/dist/vue.runtime-with-vapor.esm-browser.js' },
  },
  // `npm run dev` talks to the Go backend (backend/, `go run ./cmd/server`)
  server: { proxy: { '/api': process.env.API_DEV || 'http://localhost:8080' } },
  plugins: [
    vue({ features: { optionsAPI: false } }),
    // Residents and pengurus open this from their phones, often — installable
    // + a cached app shell behaves like an app, not just a tab. API calls are
    // never cached (the default precache is build assets only, no
    // runtimeCaching rules), so data is never served stale, and /api is kept
    // out of the navigation fallback.
    VitePWA({
      registerType: 'autoUpdate',
      workbox: { navigateFallbackDenylist: [/^\/api\//] },
      manifest: {
        name: 'Iuran Cluster the Cypress',
        short_name: 'Iuran Cypress',
        description: 'ISLK & Iuran RT 03/14 — Cluster N (Cypress)',
        theme_color: '#f5ead8',
        background_color: '#f5ead8',
        display: 'standalone',
        start_url: '.',
        icons: [
          { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: 'maskable-192.png', sizes: '192x192', type: 'image/png', purpose: 'maskable' },
          { src: 'maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
    }),
  ],
});
