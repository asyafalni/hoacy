import { islk } from './tariff.js';

// Shared "what was true as of month X" lookup — reused by WargaCard.vue's past-
// year cards and RingkasanPublik.vue's aging/dibayar-di-muka math, so there's
// one place that knows how to walk RumahRiwayat/TarifVersi instead of two
// slightly-different approximations. Mirrors the Sheet-side design in
// docs/sheets-schema.md §12/§13: both are append-only, "latest row whose
// periode <= target" wins — never edited, only added to.
export const periodeOf = (tahun, bulan) => Number(tahun) * 100 + Number(bulan);

/** RumahRiwayat row: [alamat, tahun_berlaku, bulan_berlaku, luas, tipe] */
export function luasTipePada(rumahRiwayatRows, alamatRumah, tahun, bulan) {
  const target = periodeOf(tahun, bulan);
  let best = null;
  for (const r of rumahRiwayatRows) {
    if (r[0] !== alamatRumah) continue;
    const p = periodeOf(r[1], r[2]);
    if (p <= target && (!best || p > best.p)) best = { p, luas: Number(r[3]), tipe: r[4] };
  }
  return best ? { luas: best.luas, tipe: best.tipe } : null;
}

/** TarifVersi row: [tahun_berlaku, bulan_berlaku, t1_maks, t1_tarif, t2_maks,
 *  t2_tarif, t3_maks, t3_tarif, t4_maks, t4_tarif, t5_tarif, kavling_per_m2,
 *  iuran_rt] — see docs/sheets-schema.md §13. */
export function rateCardPada(tarifVersiRows, tahun, bulan) {
  const target = periodeOf(tahun, bulan);
  let best = null;
  for (const r of tarifVersiRows) {
    const p = periodeOf(r[0], r[1]);
    if (p <= target && (!best || p > best.p)) best = { p, row: r };
  }
  if (!best) return null;
  const [, , t1m, t1, t2m, t2, t3m, t3, t4m, t4, t5, kavlingPerM2, iuranRt] = best.row.map(Number);
  return { tier: [[t1m, t1], [t2m, t2], [t3m, t3], [t4m, t4]], tarifMax: t5, kavlingPerM2, iuranRt };
}

/** Full tarif for one house, as of a given (tahun, bulan) — null if either the
 *  house or the rate card has no row effective by then. */
export function tarifPada(rumahRiwayatRows, tarifVersiRows, alamatRumah, tahun, bulan) {
  const lt = luasTipePada(rumahRiwayatRows, alamatRumah, tahun, bulan);
  const rc = rateCardPada(tarifVersiRows, tahun, bulan);
  return lt && rc ? islk(lt, rc) + rc.iuranRt : null;
}
