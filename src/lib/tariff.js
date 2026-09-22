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

/** Alamat = kode cluster + nomor blok + nomor rumah  →  "N7-09" */
export const alamat = ({ cluster, blok, rumah }) =>
  `${cluster}${blok}-${String(rumah).padStart(2, '0')}`;

/** "N7-09" | "n7-9" -> { cluster:'N', blok:7, rumah:'09' } | null */
export function parseAlamat(str) {
  const m = String(str).trim().toUpperCase().match(/^([A-Z]+)\s*(\d+)\s*[-\/ ]\s*(\d+)$/);
  return m ? { cluster: m[1], blok: Number(m[2]), rumah: m[3].padStart(2, '0') } : null;
}
