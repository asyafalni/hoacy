import { ref, computed } from 'vue';
import { gviz, parse } from './useSheet.js';

// The Kas screen's row-level data, from two small derived tabs
// (docs/sheets-schema.md) instead of the whole payment history:
// - `D-Pending`  — submissions waiting on the bendahara (pending + cek)
// - `D-KasMasuk` — cash received this year and last, by date received
// Both have Pembayaran's column order. This is the ONLY place that knows it —
// everything else reads `entries`. **Never used by /sum** (PDP — see
// RingkasanPublik.vue).
const rows = ref([]);
const loading = ref(false);
let used = false;

async function load() {
  used = true;
  loading.value = true;
  try {
    if (import.meta.env.VITE_SHEET_ID) {
      const tabs = await Promise.all(['D-Pending', 'D-KasMasuk']
        .map((tab) => fetch(gviz(tab)).then((r) => r.text()).then(parse)));
      rows.value = tabs.flat();
    } else {
      // mock: the full ledger — same rows those two tabs would filter out of it
      rows.value = (await import('./mockData.js')).MOCK_PEMBAYARAN_ROWS;
    }
  } finally {
    loading.value = false;
  }
}

// One dues year's rows, from its own `D-Iuran<tahun>` tab (pre-created for ten
// years — docs/sheets-schema.md). Fetched only when the bendahara opens that
// year in "Iuran per tahun"; cached per year for the session.
const perTahun = ref({});   // tahun -> { rows } | { error }
async function loadTahun(tahun) {
  if (perTahun.value[tahun]?.rows) return;
  try {
    let r;
    if (import.meta.env.VITE_SHEET_ID) {
      const res = await fetch(gviz(`D-Iuran${tahun}`));
      r = parse(await res.text());
    } else {
      r = (await import('./mockData.js')).MOCK_PEMBAYARAN_ROWS.filter((x) => Number(x[3]) === tahun);
    }
    perTahun.value = { ...perTahun.value, [tahun]: { rows: r } };
  } catch (e) {
    // gviz answers a missing tab with an error page, not JSON → parse throws
    perTahun.value = { ...perTahun.value, [tahun]: { error: e } };
  }
}

/** Re-fetch only if some screen has actually loaded the ledger (App.vue calls
 *  this on visibilitychange — the public page must never trigger a fetch). */
const refresh = () => (used ? load() : undefined);

// A waktu · B alamat · C bulan · D tahun · E nominal · F metode
// (tunai | transfer | impor) · G petugas · H bukti_url · I keabsahan
// (sah | pending | cek | dobel | tolak). `waktu` is when the money came in
// (the Form timestamp, or Impor's tanggal_bayar) as `yyyy-mm-dd hh:mm:ss` —
// also a submission's id for a Keputusan (see forms.js).
// A Sheet error cell (`#N/A`, `#REF!`…) comes through gviz as its text — never
// treat such a row as a payment.
const isData = (r) => r[1] && !String(r[1]).startsWith('#');
const toEntries = (list) => list.filter(isData).map((r) => ({
  waktu: r[0] != null ? String(r[0]) : '', alamat: String(r[1]),
  bulan: Number(r[2]), tahun: Number(r[3]), nominal: Number(r[4]) || 0,
  metode: r[5], petugas: r[6], buktiUrl: r[7], keabsahan: r[8],
}));

export function usePembayaranLedger() {
  const entries = computed(() => toEntries(rows.value));

  /** A dues year's per-month summary (sah rows only), or null while loading. */
  const ringkasanTahun = (tahun) => {
    const t = perTahun.value[tahun];
    if (!t) return null;
    if (t.error) return { error: true };
    const sah = toEntries(t.rows).filter((e) => e.keabsahan === 'sah');
    const bulan = Array.from({ length: 12 }, (_, i) => {
      const di = sah.filter((e) => e.bulan === i + 1);
      return { bulan: i + 1, total: di.reduce((sum, e) => sum + e.nominal, 0),
               rumah: new Set(di.map((e) => e.alamat)).size };
    });
    const perMetode = {};
    for (const e of sah) perMetode[e.metode] = (perMetode[e.metode] || 0) + e.nominal;
    return { bulan, perMetode, total: sah.reduce((sum, e) => sum + e.nominal, 0) };
  };

  // `waktu` is `yyyy-mm-dd hh:mm:ss` text, so it sorts as a string.
  const terbaru = computed(() => [...entries.value].sort((a, b) => (a.waktu < b.waktu ? 1 : a.waktu > b.waktu ? -1 : 0)));

  // One Form submission covers several months (same waktu + alamat) — Kas
  // verifies/voids/inspects it as one unit with one bukti, not per month.
  const kelompok = (list) => {
    const map = new Map();
    for (const e of list) {
      const key = `${e.waktu}|${e.alamat}`;
      if (!map.has(key)) map.set(key, { key, alamat: e.alamat, waktu: e.waktu, petugas: e.petugas,
                                        buktiUrl: e.buktiUrl, items: [], total: 0 });
      const g = map.get(key);
      g.items.push(e);
      g.total += e.nominal;
    }
    for (const g of map.values()) {
      g.items.sort((a, b) => a.tahun - b.tahun || a.bulan - b.bulan);
      // one verdict per submission, so its months normally share a status;
      // `dobel` can hit only some of them (a month already paid elsewhere)
      g.tolak = g.items.every((e) => e.keabsahan === 'tolak');
      g.dobel = g.items.every((e) => e.keabsahan === 'dobel');
    }
    return [...map.values()];
  };

  const pending = computed(() =>
    kelompok(terbaru.value.filter((e) => e.metode === 'transfer' && e.keabsahan === 'pending')));
  // Rincian didn't add up to the total — never counted; warga must resubmit.
  const cek = computed(() => kelompok(terbaru.value.filter((e) => e.keabsahan === 'cek')));
  // Cash submissions, newest first — dobel/tolak ones stay listed (struck
  // through) for the audit trail. No slice() cap — Bendahara.vue's riwayat
  // sheet lazy-renders this itself.
  const tunai = computed(() => kelompok(terbaru.value.filter((e) => e.metode === 'tunai')));

  return { loading, load, refresh, entries, pending, cek, tunai, loadTahun, ringkasanTahun };
}
