// Display helpers for periodes (yyyymm). The billing itself — status,
// tunggakan, each month's tarif — is computed by the server
// (backend/internal/tagihan); the app only shows it.

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

/** A short label for a set of months: full years as `2016` (consecutive full
 *  years as `2016–2018`), consecutive months within a year as `Jun–Sep '16`,
 *  lone months as `Nov '21`. The current year's months drop the `'yy`, as in
 *  labelBulan. Input: yyyymm numbers, any order. */
export function ringkasPeriode(periodes, tahunIni, BULAN) {
  const perTahun = new Map();
  for (const p of [...periodes].sort((a, b) => a - b)) {
    const y = tahunOf(p);
    if (!perTahun.has(y)) perTahun.set(y, []);
    perTahun.get(y).push(bulanOf(p));
  }
  const nama = (b) => BULAN[b - 1].slice(0, 3);
  const bagian = [];
  let penuh = null;   // [first, last] run of consecutive full years
  const tutupPenuh = () => {
    if (penuh) bagian.push(penuh[0] === penuh[1] ? String(penuh[0]) : `${penuh[0]}–${penuh[1]}`);
    penuh = null;
  };
  for (const [y, bulan] of perTahun) {
    if (bulan.length === 12) {
      if (penuh && penuh[1] === y - 1) penuh[1] = y;
      else { tutupPenuh(); penuh = [y, y]; }
      continue;
    }
    tutupPenuh();
    const thn = y === tahunIni ? '' : ` '${String(y).slice(2)}`;
    for (let i = 0; i < bulan.length; ) {
      let j = i;
      while (j + 1 < bulan.length && bulan[j + 1] === bulan[j] + 1) j += 1;
      bagian.push((j > i ? `${nama(bulan[i])}–${nama(bulan[j])}` : nama(bulan[i])) + thn);
      i = j + 1;
    }
  }
  tutupPenuh();
  return bagian.join(', ');
}

/** A server timestamp (ISO) as "2026-09-16 07:55", in WIB. */
export function waktuTeks(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  const f = new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Jakarta', year: 'numeric', month: '2-digit',
    day: '2-digit', hour: '2-digit', minute: '2-digit' });
  return f.format(d);
}
