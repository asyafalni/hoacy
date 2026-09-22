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
  plugins: [
    vue({ features: { optionsAPI: false } }),
    // Satpam/bendahara open this from one device, every day, via a bookmarked
    // hidden URL — installable + a cached app shell means it survives a flaky
    // pos connection and behaves like an app, not just a tab. gviz/Forms calls
    // are never cached (generateSW's default precache is build assets only, no
    // runtimeCaching rules added here), so data is never served stale.
    VitePWA({
      registerType: 'autoUpdate',
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
