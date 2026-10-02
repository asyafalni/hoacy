import { tarifPada } from './tarifHistoris.js';
import { tahunOf, selisihBulan } from './tagihan.js';
import { bangunRumah } from './rumah.js';

// Everything the public /sum page shows — cluster-wide aggregates only, computed
// on the server (Apps Script) so per-house rows never leave the private Sheet
// (PDP — see RingkasanPublik.vue). Pure functions; also bundled into
// apps-script/Code.gs, so no `?.`/`??` (see tarifHistoris.js).

// Aging buckets, counted only for houses already ≥ 3 months behind. Colours live
// in the view (index-matched); everything here is numbers.
const BUCKET = [
  { label: '3–6 bulan', min: 3, max: 6 },
  { label: '6–12 bulan', min: 6, max: 12 },
  { label: '1–3 tahun', min: 12, max: 36 },
  { label: '> 3 tahun', min: 36, max: Infinity },
];

const persen = (bagian, total) => (total ? Math.round((bagian / total) * 100) : 0);

/** Tabs are the raw tab rows (header row already dropped); `hari` = { tahun, bulan }. */
export function ringkasanPublik(tabs, hari) {
  const houses = bangunRumah(tabs.api, tabs.rumahRiwayat, tabs.tarifVersi, hari);
  const sekarang = hari.tahun * 100 + hari.bulan;
  const meta = {};
  tabs.api.forEach((r) => { if (r[0]) meta[r[0]] = r[1]; });

  // Terkumpul per month from D-Riwayat (newest row first → oldest first), leading
  // all-zero months trimmed; each month's target = every house's tarif that month.
  const bulanan = tabs.riwayat
    .filter((r) => r[0] && r[1])
    .map((r) => ({ tahun: Number(r[0]), bulan: Number(r[1]), terkumpul: Number(r[2]) || 0 }))
    .reverse();
  const nolAwal = bulanan.findIndex((b) => b.terkumpul > 0);
  const riwayat = (nolAwal <= 0 ? bulanan : bulanan.slice(nolAwal)).map((b) => ({
    tahun: b.tahun, bulan: b.bulan, terkumpul: b.terkumpul,
    target: houses.reduce((sum, h) => sum + (tarifPada(tabs.rumahRiwayat, tabs.tarifVersi, h.alamat, b.tahun, b.bulan) || 0), 0),
  }));
  const iniBulan = riwayat.find((b) => b.tahun === hari.tahun && b.bulan === hari.bulan);

  // Opex — category totals only.
  const opexList = tabs.opex.filter((r) => r[0]).map((r) => ({
    kategori: r[0], ikon: r[1] || '📋', nominal: Number(r[2]) || 0, diperbarui: r[3] ? String(r[3]) : '',
  }));
  const tglOpex = opexList.map((o) => o.diperbarui).filter(Boolean).sort();

  // Paid ahead: every sah month not yet due, at the tarif in force that month.
  const muka = { rumah: 0, bulanTahunIni: 0, nominalTahunIni: 0, bulanTahunDepan: 0, nominalTahunDepan: 0 };
  houses.forEach((h) => {
    const depan = Array.from(h.lunas).filter((p) => p > sekarang);
    if (depan.length) muka.rumah += 1;
    depan.forEach((p) => {
      const nominal = h.tarifPer(p) || 0;
      if (tahunOf(p) === hari.tahun) { muka.bulanTahunIni += 1; muka.nominalTahunIni += nominal; }
      else { muka.bulanTahunDepan += 1; muka.nominalTahunDepan += nominal; }
    });
  });
  muka.total = muka.nominalTahunIni + muka.nominalTahunDepan;

  // Aging: age runs from the oldest unpaid month (across years) to this month.
  const nunggak = houses.filter((h) => h.tertua).map((h) => ({
    umur: selisihBulan(h.tertua, sekarang) + 1, nominal: h.tunggakan,
  }));
  const aging = nunggak.filter((h) => h.umur >= 3);
  const nominalAging = aging.reduce((sum, h) => sum + h.nominal, 0);

  return {
    jumlahRumah: houses.length,
    tunggakan: houses.reduce((sum, h) => sum + h.tunggakan, 0),
    target: houses.reduce((sum, h) => sum + (h.tarif || 0), 0),
    lunasBulanIni: houses.filter((h) => h.lunas.has(sekarang)).length,
    terkumpulBulanIni: iniBulan ? iniBulan.terkumpul : 0,
    kas: Number(meta.kas_tunai || 0),
    rekening: Number(meta.rekening || 0),
    opex: opexList.reduce((sum, o) => sum + o.nominal, 0),
    opexDiperbarui: tglOpex.length ? tglOpex[tglOpex.length - 1] : '',
    opexList,
    riwayat,
    dibayarDimuka: muka,
    aging: {
      jumlahRumah: nunggak.length,
      belumDianggap: nunggak.length - aging.length,
      jumlahAging: aging.length,
      nominalAging,
      distribusi: BUCKET.map((b) => {
        const di = aging.filter((h) => h.umur >= b.min && h.umur < b.max);
        const nominal = di.reduce((sum, h) => sum + h.nominal, 0);
        return {
          label: b.label, jumlah: di.length, nominal,
          persenRumah: persen(di.length, aging.length), persenNominal: persen(nominal, nominalAging),
        };
      }),
    },
  };
}

/** One dues year's summary (sah rows only) from a D-Iuran<tahun> tab's rows
 *  (`D-Pembayaran` column order): per-month totals + per-metode totals. */
export function ringkasTahun(rows) {
  const sah = rows.filter((r) => r[1] && String(r[1]).charAt(0) !== '#' && r[8] === 'sah');
  const perMetode = {};
  sah.forEach((r) => { perMetode[r[5]] = (perMetode[r[5]] || 0) + (Number(r[4]) || 0); });
  return {
    bulan: Array.from({ length: 12 }, (_, i) => {
      const di = sah.filter((r) => Number(r[2]) === i + 1);
      const rumah = {};
      di.forEach((r) => { rumah[r[1]] = true; });
      return { bulan: i + 1, total: di.reduce((sum, r) => sum + (Number(r[4]) || 0), 0),
               rumah: Object.keys(rumah).length };
    }),
    perMetode,
    total: sah.reduce((sum, r) => sum + (Number(r[4]) || 0), 0),
  };
}
