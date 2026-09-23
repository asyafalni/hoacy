<script setup vapor>
import { ref, watch } from 'vue';
import QRCode from 'qrcode';
import { useSheet } from '../composables/useSheet';
import { urlKartu } from '../lib/forms';
import PinGate from '../components/PinGate.vue';

// Rollout tool for bendahara: one printable QR per house, stuck on a door/mailbox,
// so a resident doesn't have to type blok+rumah on their own phone. The QR only
// pre-fills the address (?alamat=N7-09) — it still goes through the PIN screen,
// same as typing it in by hand, so a printed sticker being seen by a neighbour
// doesn't grant them anything.
const PIN = import.meta.env.VITE_PIN_KAS || '';
const { rumah } = useSheet();

// `window` isn't reachable from a template expression — call it from here.
const cetak = () => window.print();

const qrSvg = ref({});   // alamat -> inline <svg> markup
watch(rumah, async (list) => {
  for (const h of list) {
    if (qrSvg.value[h.alamat]) continue;
    qrSvg.value[h.alamat] = await QRCode.toString(urlKartu(h.alamat), { type: 'svg', margin: 1, width: 132 });
  }
}, { immediate: true });
</script>

<template>
 <PinGate :pin="PIN" storage-key="kas" title="Kas Bendahara" env-var="VITE_PIN_KAS">
  <section class="scr col" style="gap:var(--space-3)">
    <div class="row no-print" style="align-items:flex-start;gap:var(--space-3)">
      <div class="grow">
        <h4 style="margin:0">Cetak QR per rumah</h4>
        <div class="text-muted" style="font-size:11.5px">
          {{ rumah.length }} rumah · tempel di pintu/kotak surat masing-masing
        </div>
      </div>
      <button class="btn btn-primary" @click="cetak" title="Cetak"
              style="flex:none;padding:10px;border-radius:999px">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M6 9V2h12v7"></path>
          <path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"></path>
          <rect x="6" y="14" width="12" height="8"></rect>
        </svg>
      </button>
      <a href="#/kas" class="btn btn-ghost" style="font-size:12px;flex:none;margin-top:2px">← Kas</a>
    </div>

    <div class="qr-grid">
      <div v-for="h in rumah" :key="h.alamat" class="qr-card">
        <div v-html="qrSvg[h.alamat]"></div>
        <div class="num" style="font-weight:700;font-size:13px">{{ h.alamat }}</div>
        <div class="truncate" style="font-size:11px;color:#555">{{ h.nama }}</div>
      </div>
    </div>
  </section>
 </PinGate>
</template>
