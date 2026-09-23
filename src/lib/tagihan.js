import { periodeOf, tarifPada } from './tarifHistoris.js';

// The one place that turns "which months has this house paid" into status,
// tunggakan and aging — for every screen and every year. The Sheet only says
// which periodes are sah/pending per house (API!L/M, docs/sheets-schema.md
// `API`); everything that needs a tarif is resolved here with tarifPada(), the
// same lookup the rest of the app uses, so there's no second copy of the rate
// logic living in a Sheet formula.
//
// A month is always paid in full (no partial payments — docs/sheets-schema.md
// `Pembayaran`), so a month is simply Lunas, Pending, Belum, or '-' (not billed:
// before the house's first RumahRiwayat row, not due yet, or no rate card).

export const tahunOf = (p) => Math.floor(p / 100);
export const bulanOf = (p) => p % 100;

/** Periode `n` months after `p` (n may be negative). */
export function geserPeriode(p, n) {
  const idx = tahunOf(p) * 12 + (bulanOf(p) - 1) + n;
  return Math.floor(idx / 12) * 100 + (idx % 12) + 1;
}

/** Whole months from `a` to `b` (b - a). */
export const selisihBulan = (a, b) =>
  (tahunOf(b) - tahunOf(a)) * 12 + (bulanOf(b) - bulanOf(a));

/** "Sep" for the current year, "Sep '25" for any other. */
export function labelBulan(p, tahunIni, BULAN) {
  const nama = BULAN[bulanOf(p) - 1].slice(0, 3);
  return tahunOf(p) === tahunIni ? nama : `${nama} '${String(tahunOf(p)).slice(2)}`;
}

/** Earliest RumahRiwayat periode for a house — billing starts here. */
export function mulaiTagih(rumahRiwayatRows, alamat) {
  let min = null;
  for (const r of rumahRiwayatRows) {
    if (r[0] !== alamat) continue;
    const p = periodeOf(r[1], r[2]);
    if (min == null || p < min) min = p;
  }
  return min;
}

/**
 * Everything the app shows about one house's dues, as of `sekarang` (a periode).
 * `h` needs { alamat, lunas:Set<periode>, pending:Set<periode> }.
 */
export function hitungTagihan(h, rumahRiwayatRows, tarifVersiRows, sekarang) {
  const tarifPer = (p) => tarifPada(rumahRiwayatRows, tarifVersiRows, h.alamat, tahunOf(p), bulanOf(p));
  const mulai = mulaiTagih(rumahRiwayatRows, h.alamat);

  function statusPada(p) {
    if (h.lunas.has(p)) return 'Lunas';
    if (h.pending.has(p)) return 'Pending';
    if (mulai == null || p < mulai || p > sekarang || tarifPer(p) == null) return '-';
    return 'Belum';
  }

  // Every due, unpaid, not-pending month since billing started, oldest first.
  // Pending transfers aren't owed any more from the resident's side — bendahara
  // clears them from the Kas screen.
  const tunggakanList = [];
  if (mulai != null) {
    for (let p = mulai; p <= sekarang; p = geserPeriode(p, 1)) {
      if (statusPada(p) === 'Belum') tunggakanList.push({ periode: p, tarif: tarifPer(p) });
    }
  }

  /** 12-month card for any year: status + that month's own tarif. */
  const kartuTahun = (tahun) => {
    const bulan = Array.from({ length: 12 }, (_, i) => periodeOf(tahun, i + 1));
    return { status: bulan.map(statusPada), tarifBulan: bulan.map(tarifPer) };
  };

  return {
    mulai,
    tarif: tarifPer(sekarang),
    tunggakanList,
    tunggakan: tunggakanList.reduce((sum, t) => sum + t.tarif, 0),
    tertua: tunggakanList[0]?.periode ?? null,
    statusPada,
    tarifPer,
    kartuTahun,
  };
}
