import { ref, computed } from 'vue';
import { BLOK_WARNA_DEFAULT } from '../lib/tariff.js';

const SHEET = import.meta.env.VITE_SHEET_ID;
/** Exported so other tabs (e.g. usePembayaranLedger's raw Pembayaran fetch) can
 *  reuse the same gviz URL + response parsing instead of duplicating it. */
export const gviz = (tab) =>
  `https://docs.google.com/spreadsheets/d/${SHEET}/gviz/tq?tqx=out:json&sheet=${tab}`;

/** gviz wraps its JSON in a JS callback — strip it. */
export function parse(text) {
  const json = JSON.parse(text.slice(text.indexOf('{'), text.lastIndexOf('}') + 1));
  return json.table.rows.map((r) => (r.c || []).map((c) => (c ? c.v : null)));
}

const rows = ref([]);
const loading = ref(true);
const error = ref(null);
const optimistic = ref([]);   // rows submitted this session, not yet in the sheet

export function useSheet() {
  async function load() {
    loading.value = true;
    try {
      if (SHEET) {
        const res = await fetch(gviz('API'));
        rows.value = parse(await res.text());
      } else {
        // Local dev only — no real Sheet configured, use the fixture instead
        // of hitting gviz with an undefined id. See src/composables/mockData.js.
        rows.value = (await import('./mockData.js')).MOCK_ROWS;
        console.warn('[useSheet] VITE_SHEET_ID kosong — memakai data mockup lokal.');
      }
      error.value = null;
    } catch (e) {
      error.value = e;
    } finally {
      loading.value = false;
    }
  }

  // key/value block (cols A,B) -> { kas_tunai, rekening, tunggakan_total, ... }
  const meta = computed(() =>
    Object.fromEntries(rows.value.filter((r) => r[0]).map((r) => [r[0], r[1]])));

  // per-house block (cols D..AA)
  const rumah = computed(() =>
    rows.value.filter((r) => r[3]).map((r) => ({
      // alamat = cluster + blok + '-' + rumah, e.g. N7-09
      alamat: r[3], nama: r[4], telp: r[5], luas: r[6], tipe: r[7],
      tarif: r[8], tunggakan: r[9],
      status: r.slice(10, 22),   // 12 months of "Lunas|Sebagian|Pending|Belum|-"
      cluster: r[22], blok: r[23], rumah: r[24],
      // months (1-12) of next year already submitted — see docs/sheets-schema.md §7
      mukaTahunDepan: r[25] ? String(r[25]).split(',').map(Number).filter(Boolean) : [],
      // per-house PIN — deterrent gate on the Warga card, see WargaCard.vue
      pin: r[26] != null ? String(r[26]) : '',
    })));

  // per-block color, admin-editable straight in the Sheet (API!AC:AD, no code
  // deploy needed) — falls back to BLOK_WARNA_DEFAULT for any block not set yet.
  const blokWarna = computed(() => ({
    ...BLOK_WARNA_DEFAULT,
    ...Object.fromEntries(rows.value.filter((r) => r[28]).map((r) => [String(r[28]), r[29]])),
  }));

  // roster of satpam names (API!AE), admin-editable — the shared Pos PIN gets
  // everyone into the screen, but each payment records which one of these names
  // actually took the cash (see petugasAktif in PosSatpam.vue).
  const satpamList = computed(() =>
    rows.value.filter((r) => r[30]).map((r) => String(r[30])));

  // same idea for Kas (API!AF) — bendahara/admin/komite roster, so a verification
  // is signed by a name from this list, not free text (see bendaharaAktif in
  // Bendahara.vue).
  const bendaharaList = computed(() =>
    rows.value.filter((r) => r[31]).map((r) => String(r[31])));

  /** Local echo so the satpam sees the row immediately after submitting. */
  function echo(rec) { optimistic.value.push({ ...rec, at: Date.now() }); }

  return { load, rows, loading, error, meta, rumah, blokWarna, satpamList, bendaharaList, optimistic, echo };
}
