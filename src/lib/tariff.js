// Mirrors Rumah!F and Rumah!H. The Sheet is authoritative — this exists only so the
// app can prefill a nominal before the next fetch lands.
export const IURAN_RT = 50000;

// Single source for the destination account shown on Kartu/Pos/Kas — change it here.
export const REKENING = { bank: 'BRI', nomor: '038401002159562', nama: 'Cluster The Cypress' };

export function islk({ luas, tipe }) {
  if (tipe === 'kavling') return 400 * luas;
  if (luas < 120) return 225000;
  if (luas < 150) return 250000;
  if (luas < 260) return 310000;
  if (luas < 400) return 375000;
  return 400000;
}

export const tarifBulanan = (h) => islk(h) + IURAN_RT;

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

// Fallback only — the real source is the Blok tab (docs/sheets-schema.md §2),
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
