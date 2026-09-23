import { ref, computed } from 'vue';
import { gviz, parse } from './useSheet.js';

// Raw Pembayaran rows, fetched separately from the API tab — the Kas screen and
// a resident's past-year cards need row-level detail (who submitted, bukti_url,
// per-month amounts) that the aggregated API tab doesn't carry.
// This is the ONLY place that knows Pembayaran's column order
// (docs/sheets-schema.md `Pembayaran`) — everything else reads `entries`.
const rows = ref([]);
const loading = ref(false);

export function usePembayaranLedger() {
  async function load() {
    loading.value = true;
    try {
      if (import.meta.env.VITE_SHEET_ID) {
        const res = await fetch(gviz('Pembayaran'));
        rows.value = parse(await res.text());
      } else {
        rows.value = (await import('./mockData.js')).MOCK_PEMBAYARAN_ROWS;
      }
    } finally {
      loading.value = false;
    }
  }

  // A Timestamp · B alamat · C bulan · D tahun · E nominal · F metode ·
  // G petugas · H catatan · I bukti_url · J keabsahan
  const entries = computed(() => rows.value.filter((r) => r[1]).map((r) => ({
    timestamp: r[0], alamat: r[1], bulan: Number(r[2]), tahun: Number(r[3]),
    nominal: Number(r[4]) || 0, metode: r[5], petugas: r[6], catatan: r[7],
    buktiUrl: r[8], keabsahan: r[9],
  })));

  // Pembayaran is append-only, so the last rows are the most recent — no need to
  // parse gviz's date format to sort, just read the tab in reverse.
  const terbaru = computed(() => [...entries.value].reverse());
  const pending = computed(() =>
    terbaru.value.filter((e) => e.metode === 'transfer' && e.keabsahan === 'pending'));
  // No slice() cap — Bendahara.vue's riwayat sheet lazy-renders this itself.
  const tunai = computed(() => terbaru.value.filter((e) => e.metode === 'tunai'));

  return { loading, load, entries, pending, tunai };
}
