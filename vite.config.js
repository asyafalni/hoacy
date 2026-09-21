import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// Vue Vapor: no virtual DOM. Every SFC in this app is <script setup vapor>.
export default defineConfig({
  base: process.env.VITE_BASE || '/',
  plugins: [vue({ features: { optionsAPI: false } })],
});
