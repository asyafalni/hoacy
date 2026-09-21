import { createApp } from 'vue';
import { createRouter, createWebHashHistory } from 'vue-router';
import './assets/tokens.css';
import './assets/app.css';
import App from './App.vue';

const routes = [
  { path: '/',    name: 'warga',     component: () => import('./views/WargaCard.vue') },
  { path: '/pos', name: 'pos',       component: () => import('./views/PosSatpam.vue') },
  { path: '/kas', name: 'bendahara', component: () => import('./views/Bendahara.vue') },
];

createApp(App)
  .use(createRouter({ history: createWebHashHistory(), routes }))
  .mount('#app');
