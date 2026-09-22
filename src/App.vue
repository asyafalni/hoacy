<script setup vapor>
import { onMounted } from 'vue';
import { useSheet } from './composables/useSheet';
import cypressLogo from './assets/logos/the-cypress.png';

const { load, error } = useSheet();
onMounted(load);
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
