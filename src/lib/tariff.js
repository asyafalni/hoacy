// Day-one default rate card — mirrors TarifVersi's first seeded row
// (docs/sheets-schema.md `TarifVersi`). The Sheet is authoritative for anything beyond
// "right now, before the next fetch lands": once TarifVersi has more than one
// row, `islk()/tarifBulanan()` should be called with a rate card looked up
// from it (see src/lib/tarifHistoris.js), not this constant — this exists so
// mockData.js and a first prefill have *something* to compute with, and so
// there's a single place the tier logic itself lives (both the "current"
// and "as of some past month" paths reuse the same function, just with a
// different rate card).
export const TARIF_DEFAULT = {
  tier: [[120, 225000], [150, 250000], [260, 310000], [400, 375000]],
  tarifMax: 400000,   // 5th tier, no upper bound (luas >= 400)
  kavlingPerM2: 400,
  iuranRt: 50000,
};
export const IURAN_RT = TARIF_DEFAULT.iuranRt;

// Single source for the destination account shown on Kartu/Pos/Kas — change it here.
export const REKENING = { bank: 'BRI', nomor: '038401002159562', nama: 'Cluster The Cypress' };

export function islk({ luas, tipe }, rateCard = TARIF_DEFAULT) {
  if (tipe === 'kavling') return rateCard.kavlingPerM2 * luas;
  for (const [maks, tarif] of rateCard.tier) if (luas < maks) return tarif;
  return rateCard.tarifMax;
}

export const tarifBulanan = (h, rateCard = TARIF_DEFAULT) => islk(h, rateCard) + rateCard.iuranRt;

export const rupiah = (n) =>
  'Rp ' + Math.round(n || 0).toLocaleString('id-ID');

export const rupiahPendek = (n) => {
  const a = Math.abs(n);
  if (a >= 1e6) return 'Rp ' + (n / 1e6).toFixed(1).replace('.', ',') + ' jt';
  if (a >= 1e3) return 'Rp ' + Math.round(n / 1e3) + ' rb';
  return 'Rp ' + Math.round(n);
};

export const BULAN = ['Januari','Februari','Maret','April','Mei','Juni','Juli',
  'Agustus','September','Oktober','November','Desember'];

// Every block in Cluster N. Shared by the warga login picker and the Pos blok filter.
export const BLOK_LIST = ['Blvd', '1', '2', '3', '5', '6', '7', '8', '9', '10'];

// Fallback only — the real source is the Blok tab (docs/sheets-schema.md `Blok`),
// editable by the admin straight in the Sheet, no deploy needed. This just covers local dev
// (mockData.js) and any block the admin hasn't set yet. Order matches BLOK_LIST;
// all 10 hues pass the dataviz-skill categorical checks (lightness/chroma/CVD/
// normal-vision separation) against this app's cream surface (#f5ead8).
export const BLOK_WARNA_DEFAULT = {
  Blvd: '#2a78d6', 1: '#9C4A1A', 2: '#eb6834', 3: '#1baf7a', 5: '#eda100',
  6: '#e87ba4', 7: '#008300', 8: '#4a3aa7', 9: '#e34948', 10: '#0F86A3',
};

/** Alamat = kode cluster + nomor blok + nomor rumah  →  "N7-09" */
export const alamat = ({ cluster, blok, rumah }) =>
  `${cluster}${blok}-${String(rumah).padStart(2, '0')}`;
