import { ref, computed } from 'vue';
import { gviz, parse } from './useSheet.js';

// Raw Pembayaran rows, fetched separately from the API tab — the Kas screen needs
// row-level detail (who submitted, bukti_url) that the aggregated API tab doesn't
// carry. Columns per docs/sheets-schema.md §4:
// A Timestamp, B alamat, C bulan, D tahun, E nominal, F metode, G petugas,
// H catatan, I bukti_url, J tarif, K keabsahan, L disetor_batch, M terverifikasi,
// N lokasi_uang.
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

  // Pembayaran is append-only, so the last rows are the most recent — no need to
  // parse gviz's date format to sort, just read the tab in reverse.
  const reversed = computed(() => [...rows.value].reverse());

  const pending = computed(() => reversed.value
    .filter((r) => r[5] === 'transfer' && r[10] === 'pending')
    .map((r) => ({
      alamat: r[1], bulan: r[2], tahun: r[3], nominal: r[4],
      catatan: r[7], buktiUrl: r[8], timestamp: r[0],
    })));

  // No slice() cap here — Bendahara.vue's riwayat sheet paginates this itself
  // (lazy-render 10 at a time), so the full history is available to scroll
  // through instead of being cut off at a fixed count.
  const tunai = computed(() => reversed.value
    .filter((r) => r[5] === 'tunai')
    .map((r) => ({
      alamat: r[1], bulan: r[2], tahun: r[3], nominal: r[4],
      petugas: r[6], catatan: r[7], timestamp: r[0],
    })));

  // Raw rows, exposed for anything that needs to aggregate across the whole
  // ledger itself (e.g. RingkasanPublik's terkumpul-vs-target history) rather
  // than the row-shaped views above.
  return { loading, load, pending, tunai, rows };
}
