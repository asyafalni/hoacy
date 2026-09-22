<script setup vapor>
import { ref, watch } from 'vue';
import QRCode from 'qrcode';
import { useSheet } from '../composables/useSheet';
import PinGate from '../components/PinGate.vue';

// Rollout tool for bendahara: one printable QR per house, stuck on a door/mailbox,
// so a resident doesn't have to type blok+rumah on their own phone. The QR only
// pre-fills the address (?alamat=N7-09) — it still goes through the PIN screen,
// same as typing it in by hand, so a printed sticker being seen by a neighbour
// doesn't grant them anything.
const PIN = import.meta.env.VITE_PIN_KAS || '';
const { rumah } = useSheet();

const qrSvg = ref({});   // alamat -> inline <svg> markup
watch(rumah, async (list) => {
  const base = import.meta.env.BASE_URL;
  for (const h of list) {
    if (qrSvg.value[h.alamat]) continue;
    // ?alamat must sit before the #, not after — WargaCard reads it off
    // location.search, and with hash-history routing only the part before the
    // hash is ever in .search (the part after # is .hash, invisible to it).
    const link = `${location.origin}${base}?alamat=${encodeURIComponent(h.alamat)}#/`;
    qrSvg.value[h.alamat] = await QRCode.toString(link, { type: 'svg', margin: 1, width: 132 });
  }
}, { immediate: true });
</script>

<template>
 <PinGate :pin="PIN" storage-key="kas" title="Kas Bendahara" env-var="VITE_PIN_KAS">
  <section class="scr col" style="gap:var(--space-3)">
    <div class="spread no-print">
      <div>
        <h4 style="margin:0">Cetak QR per rumah</h4>
        <div class="text-muted" style="font-size:11.5px">
          {{ rumah.length }} rumah · tempel di pintu/kotak surat masing-masing
        </div>
      </div>
      <button class="btn btn-primary" @click="window.print()">Cetak</button>
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
