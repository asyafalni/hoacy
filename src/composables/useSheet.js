import { ref, computed } from 'vue';
import { BLOK_WARNA_DEFAULT } from '../lib/tariff.js';
import { periodeOf, rateCardPada, luasTipePada, tarifPada } from '../lib/tarifHistoris.js';
import { hitungTagihan } from '../lib/tagihan.js';

const SHEET = import.meta.env.VITE_SHEET_ID;
/** Exported so other tabs (e.g. usePembayaranLedger's raw Pembayaran fetch) can
 *  reuse the same gviz URL + response parsing instead of duplicating it.
 *  `headers=1` pins row 1 as the header — without it gviz guesses, and a tab
 *  whose first data row looks header-ish silently loses that row. */
export const gviz = (tab) =>
  `https://docs.google.com/spreadsheets/d/${SHEET}/gviz/tq?tqx=out:json&headers=1&sheet=${encodeURIComponent(tab)}`;

/** gviz wraps its JSON in a JS callback — strip it. Two gviz quirks handled
 *  here so no caller has to: dates/datetimes come back as the string
 *  "Date(2026,8,5)" (0-based month) — use the Sheet's own formatted text
 *  (`f`) instead; and a cell whose type differs from its column's majority
 *  comes back with `v: null` but still carries `f`. */
export function parse(text) {
  const json = JSON.parse(text.slice(text.indexOf('{'), text.lastIndexOf('}') + 1));
  return json.table.rows.map((r) => (r.c || []).map((c) => {
    if (!c) return null;
    if (typeof c.v === 'string' && c.v.startsWith('Date(')) return c.f ?? c.v;
    return c.v ?? c.f ?? null;
  }));
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
// "Today" per the device, refreshed on every load — a tab left open across
// midnight or a month boundary catches up on its next (visibility) reload.
const today = ref(new Date());
let lastLoad = 0;

async function load() {
  loading.value = true;
  try {
    if (SHEET) {
      const [api, blok, petugas, opex, riwayat, rumahRiwayat, tarifVersi] = await Promise.all(
        ['D-API', 'M-Blok', 'M-Petugas', 'M-Opex', 'D-Riwayat', 'M-RumahRiwayat', 'M-TarifVersi']
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
    today.value = new Date();
    lastLoad = Date.now();
    error.value = null;
  } catch (e) {
    error.value = e;
  } finally {
    loading.value = false;
  }
}

/** Reload only if the data is older than `maxAgeMs` — used when the tab
 *  becomes visible again (App.vue), so switching apps back and forth doesn't
 *  hammer gviz but a phone left on the Pos screen overnight isn't stale. */
function refresh(maxAgeMs = 60_000) {
  if (Date.now() - lastLoad > maxAgeMs) return load();
}

const periodeSet = (v) => new Set(v ? String(v).split(',').map(Number).filter(Boolean) : []);

export function useSheet() {
  const TAHUN = computed(() => today.value.getFullYear());
  const BULAN_INI = computed(() => today.value.getMonth() + 1);
  const sekarang = computed(() => periodeOf(TAHUN.value, BULAN_INI.value));

  // key/value block (cols A,B) -> { kas_tunai, rekening, updated } — only what
  // the app can't derive itself (docs/sheets-schema.md `D-API`).
  const meta = computed(() =>
    Object.fromEntries(rows.value.filter((r) => r[0]).map((r) => [r[0], r[1]])));
  // `updated` is a yyyymmddhhmm number (keeps column B all-numeric for gviz).
  const diperbarui = computed(() => {
    const s = String(meta.value.updated || '');
    return s.length === 12 ? `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)} ${s.slice(8, 10)}:${s.slice(10)}` : s;
  });

  // per-house block (cols D..M, docs/sheets-schema.md `D-API`). The Sheet only
  // says which periodes are sah (L) / pending (M); status, tunggakan, aging and
  // tarif are all resolved here (src/lib/tagihan.js) from RumahRiwayat/TarifVersi.
  // `aktif` (K) drops decommissioned houses out of every lookup.
  const rumah = computed(() =>
    rows.value.filter((r) => r[3] && String(r[10]).toUpperCase() !== 'FALSE').map((r) => {
      const h = {
        alamat: String(r[3]), nama: r[4], telp: r[5] != null ? String(r[5]) : '',
        cluster: r[6], blok: String(r[7] ?? ''), rumah: String(r[8] ?? ''),
        // per-house PIN — deterrent gate on the Warga card, see WargaCard.vue
        pin: r[9] != null ? String(r[9]) : '',
        lunas: periodeSet(r[11]),
        pending: periodeSet(r[12]),
      };
      const t = hitungTagihan(h, rumahRiwayatRows.value, tarifVersiRows.value, sekarang.value);
      const lt = luasTipePada(rumahRiwayatRows.value, h.alamat, TAHUN.value, BULAN_INI.value);
      return {
        ...h, ...t,
        luas: lt?.luas ?? null, tipe: lt?.tipe ?? null,
        // No RumahRiwayat baseline (or no rate card) → never billed as Rp0;
        // every screen shows "Tarif belum diatur" and blocks paying instead.
        tarifDiatur: t.tarif != null,
        status: t.kartuTahun(TAHUN.value).status,
      };
    }));

  // per-block color (its own Blok tab, docs/sheets-schema.md `M-Blok`), admin-editable
  // straight in the Sheet, no code deploy needed — falls back to
  // BLOK_WARNA_DEFAULT for any block not set yet.
  const blokWarna = computed(() => ({
    ...BLOK_WARNA_DEFAULT,
    ...Object.fromEntries(blokRows.value.filter((r) => r[0]).map((r) => [String(r[0]), r[1]])),
  }));

  // Petugas tab (docs/sheets-schema.md `M-Petugas`): one row per person, `peran` says which
  // screen they belong to. The shared Pos/Kas PIN gets everyone with that peran
  // into the screen; the app then makes them pick their own name from here (see
  // petugasAktif in PosSatpam.vue, bendaharaNama in Bendahara.vue) so writes say
  // who actually acted, not a hardcoded name or free text.
  const satpamList = computed(() =>
    petugasRows.value.filter((r) => r[1] === 'satpam').map((r) => String(r[0])));
  const bendaharaList = computed(() =>
    petugasRows.value.filter((r) => r[1] === 'bendahara').map((r) => String(r[0])));

  // Opex tab (docs/sheets-schema.md `M-Opex`): itemized fixed monthly cost, category
  // totals only — e.g. "Gaji Satpam" is every satpam's wage summed into one row,
  // never one row per person. Feeds the public /sum "Rincian OPEX" sheet.
  const opexList = computed(() =>
    opexRows.value.filter((r) => r[0]).map((r) => ({
      kategori: r[0], ikon: r[1] || '📋', nominal: Number(r[2]) || 0, diperbarui: r[3] || '',
    })));

  // Rate card effective right now — used where the app needs to split a
  // total tarif back into its ISLK/Iuran RT parts (WargaCard's "Tagihan
  // berjalan" breakdown).
  const rateCardAktif = computed(() => rateCardPada(tarifVersiRows.value, TAHUN.value, BULAN_INI.value));

  /** A house's tarif for one specific month — null when RumahRiwayat/TarifVersi
   *  have nothing effective by then. */
  const tarifRumah = (alamat, tahun, bulan) =>
    tarifPada(rumahRiwayatRows.value, tarifVersiRows.value, alamat, tahun, bulan);

  /** Cluster target for one month: every active house's tarif as of that month
   *  (houses without a baseline yet add nothing). */
  const targetPada = (tahun, bulan) =>
    rumah.value.reduce((sum, h) => sum + (tarifRumah(h.alamat, tahun, bulan) || 0), 0);

  // Riwayat tab (docs/sheets-schema.md `D-Riwayat`): pre-aggregated per month, newest
  // row first (mirrors the Sheet's own fill-down order). Reversed to
  // chronological order here, and any leading (oldest) all-zero months are
  // trimmed so "semua data" doesn't open on a flat run of empty bars from
  // before the cluster had any payments.
  const riwayat = computed(() => {
    const bulanan = riwayatRows.value
      .filter((r) => r[0] && r[1])
      .map((r) => ({ tahun: Number(r[0]), bulan: Number(r[1]), terkumpul: Number(r[2]) || 0 }))
      .reverse();
    const firstNonZero = bulanan.findIndex((b) => b.terkumpul > 0);
    return firstNonZero <= 0 ? bulanan : bulanan.slice(firstNonZero);
  });

  // Cluster-wide totals — all derivable from data already fetched above.
  const totals = computed(() => {
    const iniBulan = riwayat.value.find((b) => b.tahun === TAHUN.value && b.bulan === BULAN_INI.value);
    const opexDiperbarui = opexList.value.map((o) => String(o.diperbarui)).filter(Boolean).sort();
    return {
      jumlahRumah: rumah.value.length,
      tunggakan: rumah.value.reduce((sum, h) => sum + h.tunggakan, 0),
      target: rumah.value.reduce((sum, h) => sum + (h.tarif || 0), 0),
      lunasBulanIni: rumah.value.filter((h) => h.lunas.has(sekarang.value)).length,
      tarifBelumDiatur: rumah.value.filter((h) => !h.tarifDiatur),
      terkumpulBulanIni: iniBulan?.terkumpul || 0,
      opex: opexList.value.reduce((sum, o) => sum + o.nominal, 0),
      opexDiperbarui: opexDiperbarui.at(-1) || '',
    };
  });

  return {
    load, refresh, loading, error, meta, diperbarui, rumah, totals, blokWarna, satpamList,
    bendaharaList, opexList, riwayat, rateCardAktif, tarifRumah, targetPada,
    TAHUN, BULAN_INI, sekarang,
  };
}
