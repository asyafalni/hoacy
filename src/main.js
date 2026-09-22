import { createApp, vaporInteropPlugin } from 'vue';
import { createRouter, createWebHashHistory } from 'vue-router';
import './assets/tokens.css';
import './assets/app.css';
import App from './App.vue';

const routes = [
  { path: '/',          name: 'warga',      component: () => import('./views/WargaCard.vue') },
  { path: '/pos',       name: 'pos',        component: () => import('./views/PosSatpam.vue') },
  { path: '/kas',       name: 'bendahara',  component: () => import('./views/Bendahara.vue') },
  { path: '/kas/qr',    name: 'cetak-qr',   component: () => import('./views/CetakQR.vue') },
  { path: '/ringkasan', name: 'ringkasan',  component: () => import('./views/RingkasanPublik.vue') },
];

// Every SFC here is <script setup vapor>, but <router-view> resolves its match
// (a vapor component) from inside vue-router's own vdom component — that's the
// "vapor component found in vdom tree" case the interop plugin is for.
createApp(App)
  .use(vaporInteropPlugin)
  .use(createRouter({ history: createWebHashHistory(), routes }))
  .mount('#app');
