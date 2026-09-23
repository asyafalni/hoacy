// Local-dev fixture only. useSheet.js falls back to this when VITE_SHEET_ID is
// unset, so the app is clickable without a real Google Sheet. Shape matches
// exactly what parse() would produce from the API tab's gviz response — see
// docs/sheets-schema.md `API` for the real column layout (A/B key-value interleaved
// with D..Y per-house columns on the same sheet rows).
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
  // Kavling kosong dibangun jadi rumah Juni 2025 (luas tanahnya sama, cuma tipenya
  // berubah) — demonstrasi RumahRiwayat untuk kasus upgrade paling umum.
  { alamat: 'N9-09', nama: 'Wawan Kurniadi', telp: '6281234500909', luas: 200, tipe: 'rumah',
    tarif: 360000, tunggakan: 360000, cluster: 'N', blok: 9, rumah: '09', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.B,S['-'],S['-'],S['-']] },
  // Rumah lama yang digabung ke tetangga Januari 2026 — tunggakan
  // dilunasi dulu, baru ditandai nonaktif. Tetap ada di Rumah/RumahRiwayat/
  // Pembayaran (audit), tapi harus hilang dari Warga/Pos/Semua Kartu Rumah.
  { alamat: 'N6-07', nama: 'Marto Wijoyo', telp: '6281234500607', luas: 100, tipe: 'rumah',
    tarif: 275000, tunggakan: 0, cluster: 'N', blok: 6, rumah: '07', mukaTahunDepan: '', aktif: false,
    status: [S.L,S.L,S.L,S.L,S.L,S['-'],S['-'],S['-'],S['-'],S['-'],S['-'],S['-']] },
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

// Decommissioned houses (aktif:false) drop out of every total — N6-07 stays in
// `houses` (still rendered into a row, for audit) but useSheet.js filters it out.
const housesAktif = houses.filter((h) => h.aktif !== false);

// What landed for the current month (index 8 = September, matching the status
// patterns above) — seeds Riwayat's current-month row below.
const terkumpulBulanIni = housesAktif.reduce((sum, h) => {
  if (h.status[8] === S.L) return sum + h.tarif;
  if (h.status[8] === S.S) return sum + Math.round(h.tarif * 0.5);
  return sum;
}, 0);

// `Opex` tab (docs/sheets-schema.md `Opex`) — category totals only, e.g. "Gaji Satpam"
// is every satpam's wage combined, never itemized per person. Col D (diperbarui)
// is a plain hand-typed date, updated whenever bendahara touches that row's nominal.
export const MOCK_OPEX_ROWS = [
  ['Gaji Satpam', '🛡️', 2400000, '2026-08-01'],
  ['Kebersihan', '🧹', 300000, '2026-08-01'],
  ['Listrik & Air Pos', '💡', 150000, '2026-09-05'],
  ['Lain-lain', '📋', 100000, '2026-08-01'],
];
// Only what the app can't derive itself — every other cluster total
// (tunggakan, target, jumlah rumah, terkumpul bulan ini, OPEX) is computed
// client-side in useSheet.js's `totals` (docs/sheets-schema.md `API`).
const meta = [
  ['kas_tunai', 1750000],
  ['rekening', 6250000],
  ['updated', '2026-09-22 09:00'],
];

// API per-house block (docs/sheets-schema.md `API`, cols D..Y). No luas/tipe/tarif:
// the app resolves those itself from MOCK_RUMAHRIWAYAT_ROWS/MOCK_TARIFVERSI_ROWS
// below — the fixtures' own `luas/tipe/tarif` fields only exist to generate
// consistent tunggakan/status numbers here, and must agree with what that lookup
// returns for today.
const houseRow = (h) => [
  h.alamat, h.nama, h.telp, h.tunggakan,
  ...h.status, h.cluster, h.blok, h.rumah, h.mukaTahunDepan, h.pin || h.telp.slice(-3),
  h.aktif !== false,
];

// Rows past meta.length still need to exist for houses beyond row 3 — the key/value
// block is short, but the per-house block below it runs the full length of `houses`.
const rowCount = Math.max(meta.length, houses.length);
export const MOCK_ROWS = Array.from({ length: rowCount }, (_, i) => {
  const [k, v] = meta[i] || [null, null];
  return [k, v, null, ...(houses[i] ? houseRow(houses[i]) : Array(22).fill(null))];
});

// `Blok` tab (docs/sheets-schema.md `Blok`) — one row per block. Blok 7 is overridden
// here on purpose, different from BLOK_WARNA_DEFAULT in tariff.js — proves the
// "admin edits the Sheet, app picks it up" path actually works.
export const MOCK_BLOK_ROWS = [
  ['Blvd', '#2a78d6'], ['1', '#9C4A1A'], ['2', '#eb6834'], ['3', '#1baf7a'], ['5', '#eda100'],
  ['6', '#e87ba4'], ['7', '#7C3AED'], ['8', '#4a3aa7'], ['9', '#e34948'], ['10', '#0F86A3'],
];

// `Petugas` tab (docs/sheets-schema.md `Petugas`) — one row per person, peran satpam|bendahara.
export const MOCK_PETUGAS_ROWS = [
  ['Ujang', 'satpam'], ['Dedi', 'satpam'], ['Rahmat', 'satpam'],
  ['Ibu Siti', 'bendahara'], ['Pak Joko', 'bendahara'],
];

// Raw `Pembayaran` ledger (docs/sheets-schema.md `Pembayaran`) — usePembayaranLedger.js
// falls back to this the same way useSheet.js falls back to MOCK_ROWS. A–I are
// the Form's columns, J (keabsahan) is the Sheet's formula — set directly here
// since there's no formula engine in mock mode.
const pembayaranRow = ({ alamat, bulan, tahun = 2026, nominal, metode, petugas, catatan = '',
                          buktiUrl = '', keabsahan = 'sah' }, ts) => [
  ts, alamat, bulan, tahun, nominal, metode, petugas, catatan, buktiUrl, keabsahan,
];

export const MOCK_PEMBAYARAN_ROWS = [
  pembayaranRow({ alamat: 'N7-01', bulan: 8, nominal: 300000, metode: 'tunai', petugas: 'Ujang' }, '2026-08-03 08:12:00'),

  // N7-01 (Budi Santoso), tahun 2025: lunas Jan-Okt, tapi Nov & Des belum — buat
  // nguji "kartu tahun sebelumnya" (WargaCard.vue): rumah ini sekarang juga
  // nunggak bulan berjalan (status generator di atas), jadi bisa sekalian nguji
  // tunggakan lintas tahun (2025 + 2026) digabung dalam satu konfirmasi transfer.
  ...Array.from({ length: 10 }, (_, i) => pembayaranRow(
    { alamat: 'N7-01', bulan: i + 1, tahun: 2025, nominal: 300000, metode: 'tunai', petugas: 'Ujang' },
    `2025-${String(i + 1).padStart(2, '0')}-05 08:00:00`)),
  pembayaranRow({ alamat: 'N8-02', bulan: 7, nominal: 450000, metode: 'tunai', petugas: 'Ujang' }, '2026-08-03 08:20:00'),
  pembayaranRow({ alamat: 'N7-03', bulan: 7, nominal: 180000, metode: 'transfer', petugas: 'Warga',
    buktiUrl: 'https://picsum.photos/seed/N7-03-jul/500/700', keabsahan: 'pending' },
    '2026-08-05 19:41:00'),
  pembayaranRow({ alamat: 'N1-13', bulan: 8, nominal: 385000, metode: 'tunai', petugas: 'Dedi' }, '2026-08-10 09:03:00'),
  pembayaranRow({ alamat: 'N3-15', bulan: 8, nominal: 385000, metode: 'tunai', petugas: 'Dedi' }, '2026-08-10 09:11:00'),
  pembayaranRow({ alamat: 'N8-05', bulan: 9, nominal: 35000, metode: 'transfer', petugas: 'Warga',
    buktiUrl: 'https://picsum.photos/seed/N8-05-sep/500/700', keabsahan: 'pending' },
    '2026-09-14 21:02:00'),
  pembayaranRow({ alamat: 'N6-13', bulan: 9, nominal: 295000, metode: 'tunai', petugas: 'Rahmat' }, '2026-09-15 10:22:00'),
  pembayaranRow({ alamat: 'N2-11', bulan: 9, nominal: 310000, metode: 'tunai', petugas: 'Rahmat' }, '2026-09-15 10:30:00'),
  pembayaranRow({ alamat: 'N7-09', bulan: 9, nominal: 425000, metode: 'transfer', petugas: 'Warga',
    buktiUrl: 'https://picsum.photos/seed/N7-09-sep/500/700', keabsahan: 'pending' },
    '2026-09-16 07:55:00'),
  pembayaranRow({ alamat: 'N5-11', bulan: 9, nominal: 320000, metode: 'tunai', petugas: 'Ujang' }, '2026-09-18 16:40:00'),

  // N9-09 (Wawan Kurniadi), tahun 2025: bayar tarif kavling (Rp130.000, 400/m² +
  // iuran RT) Jan-Mei, lalu lompat ke tarif rumah (Rp360.000) begitu bangunannya
  // jadi Juni 2025 — RumahRiwayat mencatat konversi itu di bulan yang sama.
  // Sept-Des 2025 sengaja dibiarkan belum bayar buat nguji tunggakan lintas tahun.
  ...Array.from({ length: 5 }, (_, i) => pembayaranRow(
    { alamat: 'N9-09', bulan: i + 1, tahun: 2025, nominal: 130000, metode: 'tunai', petugas: 'Dedi' },
    `2025-${String(i + 1).padStart(2, '0')}-05 08:00:00`)),
  ...Array.from({ length: 3 }, (_, i) => pembayaranRow(
    { alamat: 'N9-09', bulan: i + 6, tahun: 2025, nominal: 360000, metode: 'tunai', petugas: 'Dedi' },
    `2025-${String(i + 6).padStart(2, '0')}-05 08:00:00`)),
];

// `Riwayat` tab (docs/sheets-schema.md `Riwayat`) — pre-aggregated per month, newest
// first, independent of MOCK_PEMBAYARAN_ROWS above (same relationship as the
// real Sheet: a SUMIFS formula over Pembayaran, not something the client
// derives itself — RingkasanPublik.vue never fetches the raw ledger). Row 2's
// nominal matches terkumpulBulanIni above, so the "Terkumpul bulan ini" card and
// the riwayat chart's current-month bar agree.
export const MOCK_RIWAYAT_ROWS = [
  [2026, 9, terkumpulBulanIni],
  [2026, 8, 10600000], [2026, 7, 9800000], [2026, 6, 12100000], [2026, 5, 9950000],
  [2026, 4, 11200000], [2026, 3, 10450000], [2026, 2, 8800000], [2026, 1, 9100000],
  [2025, 12, 9600000], [2025, 11, 8200000],
];

// `RumahRiwayat` tab (docs/sheets-schema.md `RumahRiwayat`) — append-only, one row per
// luas/tipe change. Every house gets a 2023-01 baseline row (its luas/tipe
// *before* any of the changes below, so tarifPada() has something to resolve
// for any month this app can show) — real data entry only needs to add a row
// when something actually changes. Two houses demonstrate that:
// - N7-01 (Budi Santoso): 90m² since 2023, absorbed the strip next door and
//   grew to its current 130m² starting March 2025 — upgrade in place, same
//   address.
// - N9-09 (Wawan Kurniadi): empty 200m² kavling since 2023, built into a
//   rumah (same land, luas unchanged) starting June 2025 — the "type" upgrade
//   specifically, not just a size change.
// N6-07 (decommissioned, see `houses` above) never changed shape — it's here
// purely to prove `aktif:false` excludes it from every app lookup, not to
// exercise RumahRiwayat, so it only needs the baseline row every house gets.
const RIWAYAT_AWAL = { 'N7-01': [90, 'rumah'], 'N9-09': [200, 'kavling'] };
export const MOCK_RUMAHRIWAYAT_ROWS = [
  ...houses.map((h) => {
    const [luas, tipe] = RIWAYAT_AWAL[h.alamat] || [h.luas, h.tipe];
    return [h.alamat, 2023, 1, luas, tipe];
  }),
  ['N7-01', 2025, 3, 130, 'rumah'],
  ['N9-09', 2025, 6, 200, 'rumah'],
];

// `TarifVersi` tab (docs/sheets-schema.md `TarifVersi`) — append-only rate cards, one
// row per RT-wide price change. Three eras, oldest first:
// - before 2023: an even older tier 2 amount (240rb, not 250rb) + iuran_rt
//   35rb — demonstrates that ISLK tier AMOUNTS can change too, not just
//   iuran_rt.
// - 2023: tier 2 already at today's 250rb, but iuran_rt still 40rb.
// - 2024 onward (today): iuran_rt raised to 50rb — matches TARIF_DEFAULT in
//   tariff.js exactly, so the hand-written fixtures above (which hardcode
//   each house's CURRENT tarif) stay correct without any client-side derivation.
export const MOCK_TARIFVERSI_ROWS = [
  [2021, 1, 120, 225000, 150, 240000, 260, 310000, 400, 375000, 400000, 400, 35000],
  [2023, 1, 120, 225000, 150, 250000, 260, 310000, 400, 375000, 400000, 400, 40000],
  [2024, 1, 120, 225000, 150, 250000, 260, 310000, 400, 375000, 400000, 400, 50000],
];
