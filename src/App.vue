<script setup vapor>
import { onMounted } from 'vue';
import { useSheet } from './composables/useSheet';

const { load, error } = useSheet();
onMounted(load);
</script>

<template>
  <div class="app">
    <header class="row" style="padding:var(--space-4)">
      <div style="width:40px;height:40px;border-radius:50%;background:var(--color-accent);
                  color:var(--color-bg);display:flex;align-items:center;justify-content:center;
                  font-family:var(--font-heading)">N</div>
      <div class="grow">
        <div style="font-family:var(--font-heading);font-size:17px">Iuran Cluster N</div>
        <div class="text-muted" style="font-size:11.5px">Cypress · Blok 7 &amp; 8 · ISLK &amp; RT 03/14</div>
      </div>
    </header>

    <p v-if="error" class="scr text-muted" style="font-size:12.5px">
      Gagal memuat data dari Sheet. Periksa koneksi, lalu tarik untuk menyegarkan.
    </p>

    <router-view />
  </div>
</template>
