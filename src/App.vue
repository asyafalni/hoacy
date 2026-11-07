<script setup vapor>
import { onMounted, onUnmounted } from 'vue';
import { refreshKas } from './composables/useKas';
import { apiError } from './lib/api';
import cypressLogo from './assets/logos/the-cypress.png';

// Each screen loads its own data (the resident's card, Kas, or the public
// summary). All App.vue does is refresh the Kas data when the tab comes back to
// the foreground, so a phone left on Kas overnight doesn't keep showing
// yesterday's status (the warga card refreshes itself the same way).
function onVisible() {
  if (document.visibilityState === 'visible') refreshKas();
}
onMounted(() => document.addEventListener('visibilitychange', onVisible));
onUnmounted(() => document.removeEventListener('visibilitychange', onVisible));
</script>

<template>
  <div class="app">
    <header class="row" style="padding:var(--space-4)">
      <img :src="cypressLogo" alt="The Cypress" style="width:40px;height:40px;flex:none;object-fit:contain">
      <div class="grow">
        <div style="font-family:var(--font-heading);font-size:17px">Iuran Cluster the Cypress</div>
        <div class="text-muted" style="font-size:11.5px">Blok N · ISLK &amp; RT 03/14</div>
      </div>
    </header>

    <p v-if="apiError" class="scr text-muted" style="font-size:12.5px">
      Gagal memuat data dari server. Periksa koneksi, lalu muat ulang halaman.
    </p>

    <router-view />
  </div>
</template>
