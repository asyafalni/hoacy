// Local-dev fixture only. useSheet.js falls back to this when VITE_SHEET_ID is
// unset, so the app is clickable without a real Google Sheet. Shape matches
// exactly what parse() would produce from the API tab's gviz response — see
// docs/sheets-schema.md §9 for the real column layout (A/B key-value interleaved
// with D..Z per-house columns on the same sheet rows).
import { islk, IURAN_RT, BLOK_LIST } from '../lib/tariff.js';

const S = { L: 'Lunas', S: 'Sebagian', P: 'Pending', B: 'Belum', '-': '-' };

const houses = [
  { alamat: 'N7-01', nama: 'Budi Santoso',  telp: '6281234500001', luas: 130, tipe: 'rumah',
    tarif: 300000, tunggakan: 300000, cluster: 'N', blok: 7, rumah: '01', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.B,S['-'],S['-'],S['-']] },
  { alamat: 'N7-03', nama: 'Siti Aminah',    telp: '6281234500003', luas: 180, tipe: 'rumah',
    tarif: 360000, tunggakan: 540000, cluster: 'N', blok: 7, rumah: '03', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.S,S.P,S.B,S['-'],S['-'],S['-']] },
  { alamat: 'N7-05', nama: 'Dewi Lestari',   telp: '6281234500005', luas: 230, tipe: 'rumah',
    tarif: 360000, tunggakan: 0, cluster: 'N', blok: 7, rumah: '05', mukaTahunDepan: '1,2',
    pin: '999', // admin override — everyone else defaults to their phone's last 3 digits
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L] },
  { alamat: 'N8-02', nama: 'Agus Wibowo',    telp: '6281234500102', luas: 400, tipe: 'rumah',
    tarif: 450000, tunggakan: 4050000, cluster: 'N', blok: 8, rumah: '02', mukaTahunDepan: '',
    status: [S.B,S.B,S.B,S.B,S.B,S.B,S.B,S.B,S.B,S['-'],S['-'],S['-']] },
  { alamat: 'N8-05', nama: 'Rina Kusuma',    telp: '6281234500105', luas: 50,  tipe: 'kavling',
    tarif: 70000,  tunggakan: 35000, cluster: 'N', blok: 8, rumah: '05', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.S,S['-'],S['-'],S['-']] },
  { alamat: 'N7-09', nama: 'Hendra Saputra', telp: '6281234500009', luas: 260, tipe: 'rumah',
    tarif: 425000, tunggakan: 425000, cluster: 'N', blok: 7, rumah: '09', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.B,S['-'],S['-'],S['-']] },
];

// Bulk-generate more houses across every block so the Pos filter + pagination
// (20/page) actually has something to page through in dev — a real cluster is
// ~200 units, six hand-written fixtures above wouldn't exercise that.
const NAMA_DEPAN = ['Andi','Bambang','Citra','Dedi','Eka','Fitri','Gilang','Hani','Irfan','Joko','Kartika','Lukman'];
const NAMA_BELAKANG = ['Wijaya','Nugroho','Pratama','Hidayat','Kurniawan','Setiawan','Handoko','Wibisono'];
const LUAS_CYCLE = [110, 130, 160, 230, 300, 420];

for (let i = 0; i < 34; i++) {
  const blok = BLOK_LIST[i % BLOK_LIST.length];
  const rumahNo = String(11 + Math.floor(i / BLOK_LIST.length) * 2).padStart(2, '0');
  const luas = LUAS_CYCLE[i % LUAS_CYCLE.length];
  const tipe = i % 9 === 8 ? 'kavling' : 'rumah';
  const tarif = islk({ luas, tipe }) + IURAN_RT;
  const telp = `62812${String(9000000 + i).padStart(7, '0')}`;
  const bulanBelum = i % 4;   // 0..3 bulan tertunggak, cycling for variety
  const status = Array.from({ length: 12 }, (_, m) =>
    m < 9 - bulanBelum ? S.L : m < 9 ? S.B : S['-']);
  houses.push({
    alamat: `N${blok}-${rumahNo}`,
    nama: `${NAMA_DEPAN[i % NAMA_DEPAN.length]} ${NAMA_BELAKANG[i % NAMA_BELAKANG.length]}`,
    telp, luas, tipe, tarif, tunggakan: bulanBelum * tarif,
    cluster: 'N', blok, rumah: rumahNo, mukaTahunDepan: '', status,
  });
}

const meta = [
  ['kas_tunai', 1750000],
  ['rekening', 6250000],
  ['tunggakan_total', houses.reduce((sum, h) => sum + h.tunggakan, 0)],
  ['lunas_bulan_ini', houses.filter((h) => h.status[8] === S.L).length],
  ['jumlah_rumah', houses.length],
  ['updated', '2026-09-22 09:00'],
  ['tahun_aktif', 2026],
];

const houseRow = (h) => [
  h.alamat, h.nama, h.telp, h.luas, h.tipe, h.tarif, h.tunggakan,
  ...h.status, h.cluster, h.blok, h.rumah, h.mukaTahunDepan, h.pin || h.telp.slice(-3),
];

// Rows past meta.length still need to exist for houses beyond row 7 — the key/value
// block is short, but the per-house block below it runs the full length of `houses`.
const rowCount = Math.max(meta.length, houses.length);
export const MOCK_ROWS = Array.from({ length: rowCount }, (_, i) => {
  const [k, v] = meta[i] || [null, null];
  return [k, v, null, ...(houses[i] ? houseRow(houses[i]) : Array(24).fill(null))];
});

// `Blok` tab (docs/sheets-schema.md §2) — one row per block. Blok 7 is overridden
// here on purpose, different from BLOK_WARNA_DEFAULT in tariff.js — proves the
// "admin edits the Sheet, app picks it up" path actually works.
export const MOCK_BLOK_ROWS = [
  ['Blvd', '#2a78d6'], ['1', '#9C4A1A'], ['2', '#eb6834'], ['3', '#1baf7a'], ['5', '#eda100'],
  ['6', '#e87ba4'], ['7', '#7C3AED'], ['8', '#4a3aa7'], ['9', '#e34948'], ['10', '#0F86A3'],
];

// `Petugas` tab (docs/sheets-schema.md §3) — one row per person, peran satpam|bendahara.
export const MOCK_PETUGAS_ROWS = [
  ['Ujang', 'satpam'], ['Dedi', 'satpam'], ['Rahmat', 'satpam'],
  ['Ibu Siti', 'bendahara'], ['Pak Joko', 'bendahara'],
];

// Raw `Pembayaran` ledger — usePembayaranLedger.js falls back to this the same way
// useSheet.js falls back to MOCK_ROWS. Columns: Timestamp, alamat, bulan, tahun,
// nominal, metode, petugas, catatan, bukti_url, then J..N (tarif/keabsahan/
// disetor_batch/terverifikasi/lokasi_uang) are formula/treasurer columns the app
// never reads except keabsahan (index 10), set directly here since there's no
// live formula engine in mock mode.
const pembayaranRow = ({ alamat, bulan, tahun = 2026, nominal, metode, petugas, catatan = '',
                          buktiUrl = '', keabsahan = 'sah' }, ts) => [
  ts, alamat, bulan, tahun, nominal, metode, petugas, catatan, buktiUrl,
  null, keabsahan, null, null, null,
];

export const MOCK_PEMBAYARAN_ROWS = [
  pembayaranRow({ alamat: 'N7-01', bulan: 8, nominal: 300000, metode: 'tunai', petugas: 'Ujang' }, '2026-08-03 08:12:00'),
  pembayaranRow({ alamat: 'N8-02', bulan: 7, nominal: 450000, metode: 'tunai', petugas: 'Ujang' }, '2026-08-03 08:20:00'),
  pembayaranRow({ alamat: 'N7-03', bulan: 7, nominal: 180000, metode: 'transfer', petugas: 'Warga',
    catatan: 'bukti: transfer-juli.jpg', buktiUrl: 'https://picsum.photos/seed/N7-03-jul/500/700', keabsahan: 'pending' },
    '2026-08-05 19:41:00'),
  pembayaranRow({ alamat: 'N1-13', bulan: 8, nominal: 385000, metode: 'tunai', petugas: 'Dedi' }, '2026-08-10 09:03:00'),
  pembayaranRow({ alamat: 'N3-15', bulan: 8, nominal: 385000, metode: 'tunai', petugas: 'Dedi' }, '2026-08-10 09:11:00'),
  pembayaranRow({ alamat: 'N8-05', bulan: 9, nominal: 35000, metode: 'transfer', petugas: 'Warga',
    catatan: 'bukti: bukti_sept.png', buktiUrl: 'https://picsum.photos/seed/N8-05-sep/500/700', keabsahan: 'pending' },
    '2026-09-14 21:02:00'),
  pembayaranRow({ alamat: 'N6-13', bulan: 9, nominal: 295000, metode: 'tunai', petugas: 'Rahmat' }, '2026-09-15 10:22:00'),
  pembayaranRow({ alamat: 'N2-11', bulan: 9, nominal: 310000, metode: 'tunai', petugas: 'Rahmat' }, '2026-09-15 10:30:00'),
  pembayaranRow({ alamat: 'N7-09', bulan: 9, nominal: 425000, metode: 'transfer', petugas: 'Warga',
    catatan: 'bukti: sept_hendra.jpg', buktiUrl: 'https://picsum.photos/seed/N7-09-sep/500/700', keabsahan: 'pending' },
    '2026-09-16 07:55:00'),
  pembayaranRow({ alamat: 'N5-11', bulan: 9, nominal: 320000, metode: 'tunai', petugas: 'Ujang' }, '2026-09-18 16:40:00'),
];
