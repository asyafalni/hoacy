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

const rows = ref([]);        // API tab — key/value block + per-house block
const blokRows = ref([]);    // Blok tab — one row per block
const petugasRows = ref([]); // Petugas tab — one row per satpam/bendahara
const loading = ref(true);
const error = ref(null);
const optimistic = ref([]);   // rows submitted this session, not yet in the sheet

export function useSheet() {
  async function load() {
    loading.value = true;
    try {
      if (SHEET) {
        const [api, blok, petugas] = await Promise.all(
          ['API', 'Blok', 'Petugas'].map((tab) => fetch(gviz(tab)).then((r) => r.text())));
        rows.value = parse(api);
        blokRows.value = parse(blok);
        petugasRows.value = parse(petugas);
      } else {
        // Local dev only — no real Sheet configured, use the fixtures instead
        // of hitting gviz with an undefined id. See src/composables/mockData.js.
        const mock = await import('./mockData.js');
        rows.value = mock.MOCK_ROWS;
        blokRows.value = mock.MOCK_BLOK_ROWS;
        petugasRows.value = mock.MOCK_PETUGAS_ROWS;
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
      // months (1-12) of next year already submitted — see docs/sheets-schema.md §9
      mukaTahunDepan: r[25] ? String(r[25]).split(',').map(Number).filter(Boolean) : [],
      // per-house PIN — deterrent gate on the Warga card, see WargaCard.vue
      pin: r[26] != null ? String(r[26]) : '',
    })));

  // per-block color (its own Blok tab, docs/sheets-schema.md §2), admin-editable
  // straight in the Sheet, no code deploy needed — falls back to
  // BLOK_WARNA_DEFAULT for any block not set yet.
  const blokWarna = computed(() => ({
    ...BLOK_WARNA_DEFAULT,
    ...Object.fromEntries(blokRows.value.filter((r) => r[0]).map((r) => [String(r[0]), r[1]])),
  }));

  // Petugas tab (docs/sheets-schema.md §3): one row per person, `peran` says which
  // screen they belong to. The shared Pos/Kas PIN gets everyone with that peran
  // into the screen; the app then makes them pick their own name from here (see
  // petugasAktif in PosSatpam.vue, bendaharaNama in Bendahara.vue) so writes say
  // who actually acted, not a hardcoded name or free text.
  const satpamList = computed(() =>
    petugasRows.value.filter((r) => r[1] === 'satpam').map((r) => String(r[0])));
  const bendaharaList = computed(() =>
    petugasRows.value.filter((r) => r[1] === 'bendahara').map((r) => String(r[0])));

  /** Local echo so the satpam sees the row immediately after submitting. */
  function echo(rec) { optimistic.value.push({ ...rec, at: Date.now() }); }

  return { load, rows, loading, error, meta, rumah, blokWarna, satpamList, bendaharaList, optimistic, echo };
}
