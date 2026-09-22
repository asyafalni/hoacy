import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// Vue Vapor: no virtual DOM. Every SFC in this app is <script setup vapor>.
// The plain "vue" package export (vue.runtime.esm-bundler.js) doesn't include
// the vapor runtime at all — createApp() silently falls back to a vdom root and
// warns "vapor-in-vdom interop was not installed". Alias to the build that has it.
export default defineConfig({
  base: process.env.VITE_BASE || '/',
  resolve: {
    alias: { vue: 'vue/dist/vue.runtime-with-vapor.esm-browser.js' },
  },
  plugins: [vue({ features: { optionsAPI: false } })],
});
