// Local-dev fixture only. useSheet.js falls back to this when VITE_SHEET_ID is
// unset, so the app is clickable without a real Google Sheet. Shape matches
// exactly what parse() would produce from the API tab's gviz response — see
// docs/sheets-schema.md §10 for the real column layout (A/B key-value interleaved
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

// Mirrors the real B9/B10 formulas (docs/sheets-schema.md §10): sum of whatever
// landed for the current month (index 8 = September, matching the status
// patterns generated above) across every house, vs. sum of everyone's tarif.
const terkumpulBulanIni = houses.reduce((sum, h) => {
  if (h.status[8] === S.L) return sum + h.tarif;
  if (h.status[8] === S.S) return sum + Math.round(h.tarif * 0.5);
  return sum;
}, 0);
const targetBulanIni = houses.reduce((sum, h) => sum + h.tarif, 0);

// `Opex` tab (docs/sheets-schema.md §9) — category totals only, e.g. "Gaji Satpam"
// is every satpam's wage combined, never itemized per person. Col D (diperbarui)
// is a plain hand-typed date, updated whenever bendahara touches that row's nominal.
export const MOCK_OPEX_ROWS = [
  ['Gaji Satpam', '🛡️', 2400000, '2026-08-01'],
  ['Kebersihan', '🧹', 300000, '2026-08-01'],
  ['Listrik & Air Pos', '💡', 150000, '2026-09-05'],
  ['Lain-lain', '📋', 100000, '2026-08-01'],
];
// opex_bulanan mirrors =SUM(Opex!$C:$C) — computed from the rows above, same as
// the real sheet, instead of a second hand-typed number that could drift from them.
const opexBulanan = MOCK_OPEX_ROWS.reduce((sum, [, , nominal]) => sum + nominal, 0);
// opex_diperbarui mirrors =TEXT(MAX(Opex!$D:$D), "dd mmm yyyy") — the most recent
// of the per-row dates above, so residents can see at a glance if the baseline is stale.
const opexDiperbarui = MOCK_OPEX_ROWS.map(([, , , d]) => d).sort().at(-1);

const meta = [
  ['kas_tunai', 1750000],
  ['rekening', 6250000],
  ['tunggakan_total', houses.reduce((sum, h) => sum + h.tunggakan, 0)],
  ['lunas_bulan_ini', houses.filter((h) => h.status[8] === S.L).length],
  ['jumlah_rumah', houses.length],
  ['updated', '2026-09-22 09:00'],
  ['tahun_aktif', 2026],
  ['terkumpul_bulan_ini', terkumpulBulanIni],
  ['target_bulan_ini', targetBulanIni],
  ['opex_bulanan', opexBulanan],
  ['opex_diperbarui', opexDiperbarui],
];

// `tipe` (rumah|kavling) feeds the tariff formula server-side (Rumah!I) but the
// API tab's per-house block never carries it through — nothing in the app reads
// it, see useSheet.js's `rumah` computed. Kept on the fixture objects above only
// because tarifBulanan()/islk() need it as an input.
// `aktif` (docs/sheets-schema.md §1/§12): none of these fixtures are
// decommissioned, so it's always TRUE here — the column exists so useSheet.js's
// filter has something real to read.
const houseRow = (h) => [
  h.alamat, h.nama, h.telp, h.luas, h.tarif, h.tunggakan,
  ...h.status, h.cluster, h.blok, h.rumah, h.mukaTahunDepan, h.pin || h.telp.slice(-3),
  h.aktif !== false,
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

  // N7-01 (Budi Santoso), tahun 2025: lunas Jan-Okt, tapi Nov & Des belum — buat
  // nguji "kartu tahun sebelumnya" (WargaCard.vue): rumah ini sekarang juga
  // nunggak bulan berjalan (status generator di atas), jadi bisa sekalian nguji
  // tunggakan lintas tahun (2025 + 2026) digabung dalam satu konfirmasi transfer.
  ...Array.from({ length: 10 }, (_, i) => pembayaranRow(
    { alamat: 'N7-01', bulan: i + 1, tahun: 2025, nominal: 300000, metode: 'tunai', petugas: 'Ujang' },
    `2025-${String(i + 1).padStart(2, '0')}-05 08:00:00`)),
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

// `Riwayat` tab (docs/sheets-schema.md §11) — pre-aggregated per month, newest
// first, independent of MOCK_PEMBAYARAN_ROWS above (same relationship as the
// real Sheet: a SUMIFS formula over Pembayaran, not something the client
// derives itself — RingkasanPublik.vue never fetches the raw ledger). Row 2's
// nominal mirrors terkumpul_bulan_ini so the "Terkumpul bulan ini" card and
// the riwayat chart's current-month bar agree.
export const MOCK_RIWAYAT_ROWS = [
  [2026, 9, terkumpulBulanIni],
  [2026, 8, 10600000], [2026, 7, 9800000], [2026, 6, 12100000], [2026, 5, 9950000],
  [2026, 4, 11200000], [2026, 3, 10450000], [2026, 2, 8800000], [2026, 1, 9100000],
  [2025, 12, 9600000], [2025, 11, 8200000],
];

// `RumahRiwayat` tab (docs/sheets-schema.md §12) — append-only, one row per
// luas/tipe change. Every house gets a 2023-01 baseline row matching its
// CURRENT luas/tipe (so tarifPada() can resolve any month this app can
// currently show, not just the ones after this feature shipped) — real data
// entry only needs to add a row when something actually changes, same as here:
// N7-01 (Budi Santoso) is the one house that demonstrates an actual upgrade —
// 90m² since 2023, absorbed the strip next door and grew to its current 130m²
// starting March 2025 (after which its 2025 Pembayaran rows above already
// assume the bigger tarif).
export const MOCK_RUMAHRIWAYAT_ROWS = [
  ...houses.map((h) => [h.alamat, 2023, 1, h.alamat === 'N7-01' ? 90 : h.luas, h.tipe]),
  ['N7-01', 2025, 3, 130, 'rumah'],
];

// `TarifVersi` tab (docs/sheets-schema.md §13) — append-only rate cards, one
// row per RT-wide price change. `iuran_rt` was 40.000 before 2024, raised to
// today's 50.000 starting Jan 2024 — everything from 2024 onward (including
// "today") matches TARIF_DEFAULT in tariff.js exactly, so the hand-written
// fixtures above (which hardcode each house's CURRENT tarif) stay correct;
// only months in 2023 compute a lower tarif via tarifPada().
export const MOCK_TARIFVERSI_ROWS = [
  [2023, 1, 120, 225000, 150, 250000, 260, 310000, 400, 375000, 400000, 400, 40000],
  [2024, 1, 120, 225000, 150, 250000, 260, 310000, 400, 375000, 400000, 400, 50000],
];
