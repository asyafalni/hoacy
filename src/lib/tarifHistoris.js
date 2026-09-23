// Shared "what was true as of month X" lookup — the one place that walks
// RumahRiwayat/TarifVersi (docs/sheets-schema.md). Both are append-only: the
// latest row(s) whose periode <= target win — never edited, only added to.
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

/** TarifVersi row: [tahun_berlaku, bulan_berlaku, komponen, luas_min, nominal].
 *  One rate card = every row sharing the latest (tahun, bulan) not after the
 *  target month, so the number of ISLK tiers is free:
 *  - `islk_rumah`   — luas_min (m²) + monthly ISLK; a house pays the row with
 *                     the largest luas_min <= its luas
 *  - `islk_kavling` — nominal per m²
 *  - `iuran_rt`     — flat, every house
 *  Returns null when no version is effective yet. */
export function rateCardPada(tarifVersiRows, tahun, bulan) {
  const target = periodeOf(tahun, bulan);
  let versi = null;
  for (const r of tarifVersiRows) {
    const p = periodeOf(r[0], r[1]);
    if (p <= target && (versi == null || p > versi)) versi = p;
  }
  if (versi == null) return null;
  const rows = tarifVersiRows.filter((r) => periodeOf(r[0], r[1]) === versi);
  const nominal = (komponen) => {
    const r = rows.find((x) => x[2] === komponen);
    return r ? Number(r[4]) : null;
  };
  return {
    rumah: rows.filter((r) => r[2] === 'islk_rumah')
      .map((r) => [Number(r[3]) || 0, Number(r[4])]).sort((a, b) => a[0] - b[0]),
    kavlingPerM2: nominal('islk_kavling'),
    iuranRt: nominal('iuran_rt') ?? 0,
  };
}

/** ISLK for one house under one rate card — null if the card has no rule for
 *  its tipe/luas (an incomplete TarifVersi row set is never billed as Rp0). */
export function islk({ luas, tipe }, rateCard) {
  if (tipe === 'kavling') return rateCard.kavlingPerM2 == null ? null : rateCard.kavlingPerM2 * luas;
  let tarif = null;
  for (const [luasMin, nominal] of rateCard.rumah) if (luas >= luasMin) tarif = nominal;
  return tarif;
}

/** Full tarif for one house, as of a given (tahun, bulan) — null if the house,
 *  the rate card, or the matching rule isn't there by then. */
export function tarifPada(rumahRiwayatRows, tarifVersiRows, alamatRumah, tahun, bulan) {
  const lt = luasTipePada(rumahRiwayatRows, alamatRumah, tahun, bulan);
  const rc = lt && rateCardPada(tarifVersiRows, tahun, bulan);
  const i = rc && islk(lt, rc);
  return i == null ? null : i + rc.iuranRt;
}
