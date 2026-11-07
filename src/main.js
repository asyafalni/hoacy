import { createApp, vaporInteropPlugin } from 'vue';
import { createRouter, createWebHashHistory } from 'vue-router';
import './assets/tokens.css';
import './assets/app.css';
import App from './App.vue';

const routes = [
  { path: '/',          name: 'warga',      component: () => import('./views/WargaCard.vue') },
  // the satpam's Pos screen is gone (cash is recorded by the bendahara on
  // /kas/tunai) — an old bookmark lands on the warga page instead of a blank one
  { path: '/pos',       redirect: '/' },
  { path: '/kas',       name: 'bendahara',  component: () => import('./views/Bendahara.vue') },
  { path: '/kas/tunai', name: 'catat-tunai', component: () => import('./views/CatatTunai.vue') },
  { path: '/kas/qr',    name: 'cetak-qr',   component: () => import('./views/CetakQR.vue') },
  { path: '/kas/rumah', name: 'semua-kartu', component: () => import('./views/SemuaKartu.vue') },
  { path: '/kas/admin', name: 'admin',      component: () => import('./views/Admin.vue') },
  { path: '/sum',       name: 'ringkasan',  component: () => import('./views/RingkasanPublik.vue') },
];

// Every SFC here is <script setup vapor>, but <router-view> resolves its match
// (a vapor component) from inside vue-router's own vdom component — that's the
// "vapor component found in vdom tree" case the interop plugin is for.
createApp(App)
  .use(vaporInteropPlugin)
  .use(createRouter({ history: createWebHashHistory(), routes }))
  .mount('#app');
