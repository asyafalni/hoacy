import { ref, computed } from 'vue';
import { api, ApiError } from '../lib/api.js';
import { ledgerRows } from './useSheet.js';
import { useAuth } from './useAuth.js';

// Row-level payment data for the Kas screen only. The rows ride along with the
// Kas payload (useSheet.muat('kas') → D-Pending + D-KasMasuk); this is the
// ONLY place that knows their column order — everything else reads `entries`.
// Never reachable from /sum or any other role (the server decides — PDP).
//
// A waktu · B alamat · C bulan · D tahun · E nominal · F metode
// (tunai | transfer | impor) · G petugas · H bukti_url · I keabsahan
// (sah | pending | cek | dobel | tolak). `waktu` is when the money came in
// (the Form timestamp, or Impor's tanggal_bayar) as `yyyy-mm-dd hh:mm:ss` —
// also a submission's id for a Keputusan (see forms.js).
// A Sheet error cell (`#N/A`, `#REF!`…) arrives as its text — never treat such
// a row as a payment.
const isData = (r) => r[1] && !String(r[1]).startsWith('#');
const toEntries = (list) => list.filter(isData).map((r) => ({
  waktu: r[0] != null ? String(r[0]) : '', alamat: String(r[1]),
  bulan: Number(r[2]), tahun: Number(r[3]), nominal: Number(r[4]) || 0,
  metode: r[5], petugas: r[6], buktiUrl: r[7], keabsahan: r[8],
}));

// One dues year's summary, summed by the server from that year's own
// D-Iuran<tahun> tab — fetched whenever the bendahara opens or switches year
// (always fresh; a payment verified a minute ago shows up).
const perTahun = ref({});   // tahun -> { ringkasan } | { error }
async function loadTahun(tahun) {
  try {
    const res = await api('kasTahun', { pin: useAuth().pins.value.kas, tahun });
    perTahun.value = { ...perTahun.value, [tahun]: { ringkasan: res.ringkasan } };
  } catch (e) {
    if (e instanceof ApiError && e.kode === 'pin') useAuth().clearPin('kas', true);
    perTahun.value = { ...perTahun.value, [tahun]: { error: true } };
  }
}

export function usePembayaranLedger() {
  const entries = computed(() => toEntries([...ledgerRows.value.pending, ...ledgerRows.value.kasMasuk]));

  /** A dues year's per-month summary, null while loading, { error } if the tab's missing. */
  const ringkasanTahun = (tahun) => {
    const t = perTahun.value[tahun];
    if (!t) return null;
    return t.error ? { error: true } : t.ringkasan;
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

  return { entries, pending, cek, tunai, loadTahun, ringkasanTahun };
}
