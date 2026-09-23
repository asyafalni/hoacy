import { ref, computed } from 'vue';
import { BLOK_WARNA_DEFAULT } from '../lib/tariff.js';
import { rateCardPada, luasTipePada, tarifPada } from '../lib/tarifHistoris.js';

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
const opexRows = ref([]);    // Opex tab — one row per cost category
const riwayatRows = ref([]); // Riwayat tab — one row per month, newest first
const rumahRiwayatRows = ref([]); // RumahRiwayat tab — one row per house per luas/tipe change
const tarifVersiRows = ref([]);   // TarifVersi tab — one row per rate-card change
const loading = ref(true);
const error = ref(null);

export function useSheet() {
  async function load() {
    loading.value = true;
    try {
      if (SHEET) {
        const [api, blok, petugas, opex, riwayat, rumahRiwayat, tarifVersi] = await Promise.all(
          ['API', 'Blok', 'Petugas', 'Opex', 'Riwayat', 'RumahRiwayat', 'TarifVersi']
            .map((tab) => fetch(gviz(tab)).then((r) => r.text())));
        rows.value = parse(api);
        blokRows.value = parse(blok);
        petugasRows.value = parse(petugas);
        opexRows.value = parse(opex);
        riwayatRows.value = parse(riwayat);
        rumahRiwayatRows.value = parse(rumahRiwayat);
        tarifVersiRows.value = parse(tarifVersi);
      } else {
        // Local dev only — no real Sheet configured, use the fixtures instead
        // of hitting gviz with an undefined id. See src/composables/mockData.js.
        const mock = await import('./mockData.js');
        rows.value = mock.MOCK_ROWS;
        blokRows.value = mock.MOCK_BLOK_ROWS;
        petugasRows.value = mock.MOCK_PETUGAS_ROWS;
        opexRows.value = mock.MOCK_OPEX_ROWS;
        riwayatRows.value = mock.MOCK_RIWAYAT_ROWS;
        rumahRiwayatRows.value = mock.MOCK_RUMAHRIWAYAT_ROWS;
        tarifVersiRows.value = mock.MOCK_TARIFVERSI_ROWS;
        console.warn('[useSheet] VITE_SHEET_ID kosong — memakai data mockup lokal.');
      }
      error.value = null;
    } catch (e) {
      error.value = e;
    } finally {
      loading.value = false;
    }
  }

  // "Today" per the device — the same date the Sheet's own TODAY() formulas use.
  const now = new Date();
  const TAHUN = now.getFullYear();
  const BULAN_INI = now.getMonth() + 1;

  // key/value block (cols A,B) -> { kas_tunai, rekening, updated } — only what
  // the app can't derive itself; every other total is computed from the
  // per-house block / Riwayat / Opex below (docs/sheets-schema.md `API`).
  const meta = computed(() =>
    Object.fromEntries(rows.value.filter((r) => r[0]).map((r) => [r[0], r[1]])));

  // per-house block (cols D..Y, docs/sheets-schema.md `API`). `luas/tipe/tarif`
  // aren't columns any more — they're resolved here from RumahRiwayat/TarifVersi
  // (same lookup the Sheet's Status!AE:AP uses), so there's one definition of
  // "a house's tarif this month" instead of a Sheet copy and a client copy.
  // `aktif` (Y) drops decommissioned houses (`Rumah/RumahRiwayat`) out of every lookup.
  const rumah = computed(() =>
    rows.value.filter((r) => r[3] && String(r[24]).toUpperCase() !== 'FALSE').map((r) => {
      const alamat = r[3];
      const lt = luasTipePada(rumahRiwayatRows.value, alamat, TAHUN, BULAN_INI);
      return {
        alamat, nama: r[4], telp: r[5],
        luas: lt?.luas ?? null, tipe: lt?.tipe ?? null,
        tarif: tarifPada(rumahRiwayatRows.value, tarifVersiRows.value, alamat, TAHUN, BULAN_INI) || 0,
        tunggakan: Number(r[6]) || 0,
        status: r.slice(7, 19),   // 12 months of "Lunas|Sebagian|Pending|Belum|-"
        cluster: r[19], blok: r[20], rumah: r[21],
        // months (1-12) of next year already submitted
        mukaTahunDepan: r[22] ? String(r[22]).split(',').map(Number).filter(Boolean) : [],
        // per-house PIN — deterrent gate on the Warga card, see WargaCard.vue
        pin: r[23] != null ? String(r[23]) : '',
      };
    }));

  // per-block color (its own Blok tab, docs/sheets-schema.md `Blok`), admin-editable
  // straight in the Sheet, no code deploy needed — falls back to
  // BLOK_WARNA_DEFAULT for any block not set yet.
  const blokWarna = computed(() => ({
    ...BLOK_WARNA_DEFAULT,
    ...Object.fromEntries(blokRows.value.filter((r) => r[0]).map((r) => [String(r[0]), r[1]])),
  }));

  // Petugas tab (docs/sheets-schema.md `Petugas`): one row per person, `peran` says which
  // screen they belong to. The shared Pos/Kas PIN gets everyone with that peran
  // into the screen; the app then makes them pick their own name from here (see
  // petugasAktif in PosSatpam.vue, bendaharaNama in Bendahara.vue) so writes say
  // who actually acted, not a hardcoded name or free text.
  const satpamList = computed(() =>
    petugasRows.value.filter((r) => r[1] === 'satpam').map((r) => String(r[0])));
  const bendaharaList = computed(() =>
    petugasRows.value.filter((r) => r[1] === 'bendahara').map((r) => String(r[0])));

  // Opex tab (docs/sheets-schema.md `Opex`): itemized fixed monthly cost, category
  // totals only — e.g. "Gaji Satpam" is every satpam's wage summed into one row,
  // never one row per person. Feeds the public /sum "Rincian OPEX" sheet.
  const opexList = computed(() =>
    opexRows.value.filter((r) => r[0]).map((r) => ({
      kategori: r[0], ikon: r[1] || '📋', nominal: Number(r[2]) || 0, diperbarui: r[3] || '',
    })));

  // Rate card effective right now — used where the app needs to split a
  // total tarif back into its ISLK/Iuran RT parts (WargaCard's "Tagihan
  // berjalan" breakdown). Historical (past-month) lookups go through
  // tarifPada()/rateCardPada() in src/lib/tarifHistoris.js directly against
  // rumahRiwayatRows/tarifVersiRows instead — this computed is only "today".
  const rateCardAktif = computed(() => rateCardPada(tarifVersiRows.value, TAHUN, BULAN_INI));

  /** A house's tarif for one specific month — what Pos/WargaCard charge when
   *  paying that month, so an old or pre-upgrade month is billed at its own rate. */
  const tarifRumah = (alamat, tahun, bulan) =>
    tarifPada(rumahRiwayatRows.value, tarifVersiRows.value, alamat, tahun, bulan);

  // Riwayat tab (docs/sheets-schema.md `Riwayat`): pre-aggregated per month, newest
  // row first (mirrors the Sheet's own fill-down order). Reversed to
  // chronological order here, and any leading (oldest) all-zero months are
  // trimmed so "semua data" doesn't open on a flat run of empty bars from
  // before the cluster had any payments — this tab is filled with a fixed
  // number of rows (e.g. 36), not dynamically sized to actual history.
  const riwayat = computed(() => {
    const bulanan = riwayatRows.value
      .filter((r) => r[0] && r[1])
      .map((r) => ({ tahun: Number(r[0]), bulan: Number(r[1]), terkumpul: Number(r[2]) || 0 }))
      .reverse();
    const firstNonZero = bulanan.findIndex((b) => b.terkumpul > 0);
    return firstNonZero <= 0 ? bulanan : bulanan.slice(firstNonZero);
  });

  // Cluster-wide totals — all derivable from data already fetched above, so
  // they no longer exist as separate API!meta rows that could drift from it.
  const totals = computed(() => {
    const iniBulan = riwayat.value.find((b) => b.tahun === TAHUN && b.bulan === BULAN_INI);
    const diperbarui = opexList.value.map((o) => String(o.diperbarui)).filter(Boolean).sort();
    return {
      jumlahRumah: rumah.value.length,
      tunggakan: rumah.value.reduce((sum, h) => sum + h.tunggakan, 0),
      target: rumah.value.reduce((sum, h) => sum + h.tarif, 0),
      lunasBulanIni: rumah.value.filter((h) => h.status[BULAN_INI - 1] === 'Lunas').length,
      terkumpulBulanIni: iniBulan?.terkumpul || 0,
      opex: opexList.value.reduce((sum, o) => sum + o.nominal, 0),
      opexDiperbarui: diperbarui.at(-1) || '',
    };
  });

  return {
    load, loading, error, meta, rumah, totals, blokWarna, satpamList, bendaharaList, opexList, riwayat,
    rateCardAktif, tarifRumah, TAHUN, BULAN_INI,
  };
}
