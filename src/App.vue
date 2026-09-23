<script setup vapor>
import { onMounted, onUnmounted } from 'vue';
import { useSheet } from './composables/useSheet';
import { usePembayaranLedger } from './composables/usePembayaranLedger';
import cypressLogo from './assets/logos/the-cypress.png';

// The single place data gets loaded — views never call load() on mount
// themselves (that used to fetch everything twice on /sum). Coming back to
// the tab refreshes it too, so a phone left on Pos/Kas overnight doesn't keep
// showing yesterday's status, or yesterday's "bulan ini".
const { load, refresh, error } = useSheet();
const { refresh: refreshLedger } = usePembayaranLedger();
function onVisible() {
  if (document.visibilityState === 'visible') { refresh(); refreshLedger(); }
}
onMounted(() => { load(); document.addEventListener('visibilitychange', onVisible); });
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

    <p v-if="error" class="scr text-muted" style="font-size:12.5px">
      Gagal memuat data dari Sheet. Periksa koneksi, lalu tarik untuk menyegarkan.
    </p>

    <router-view />
  </div>
</template>
