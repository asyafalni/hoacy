// Local-dev fixture only. useSheet.js / usePembayaranLedger.js fall back to this
// when VITE_SHEET_ID is unset, so the app is clickable without a real Google
// Sheet. Every export has exactly the shape parse() would produce from that
// tab's gviz response (docs/sheets-schema.md) — and the derived tabs (Pembayaran's
// keabsahan, API's lunas/pending lists, Riwayat's monthly sums) are computed
// here from the same source the real Sheet formulas would use, so the numbers
// on every screen agree with each other.
import { BLOK_LIST } from '../lib/tariff.js';
import { tarifPada } from '../lib/tarifHistoris.js';
import { geserPeriode, tahunOf, bulanOf } from '../lib/tagihan.js';

const rentang = (dari, sampai) => {
  const out = [];
  for (let p = dari; p <= sampai; p = geserPeriode(p, 1)) out.push(p);
  return out;
};

// One entry per house. `riwayat` = its RumahRiwayat rows [periode, luas, tipe]
// (omitted → one baseline row at `mulai`); months from `mulai` to `bayarSampai`
// are paid in cash unless listed in `belum` (unpaid) or `pending` (transfer
// waiting for verification).
const houses = [
  // 90m² since 2023, absorbed the strip next door → 130m² from March 2025
  // (upgrade in place). Nov–Dec 2025 + Sep 2026 unpaid: cross-year arrears.
  { alamat: 'N7-01', nama: 'Budi Santoso', telp: '6281234500001', blok: '7', rumah: '01',
    riwayat: [[202301, 90, 'rumah'], [202503, 130, 'rumah']], mulai: 202301, bayarSampai: 202608,
    belum: [202511, 202512] },
  { alamat: 'N7-03', nama: 'Siti Aminah', telp: '6281234500003', blok: '7', rumah: '03',
    luas: 180, tipe: 'rumah', mulai: 202601, bayarSampai: 202608, belum: [202607], pending: [202608] },
  // Paid the whole year + two months of next year by transfer, all verified.
  { alamat: 'N7-05', nama: 'Dewi Lestari', telp: '6281234500005', blok: '7', rumah: '05',
    pin: '999', // admin override — everyone else defaults to their phone's last 3 digits
    luas: 230, tipe: 'rumah', mulai: 202601, bayarSampai: 202702, metode: 'transfer' },
  { alamat: 'N8-02', nama: 'Agus Wibowo', telp: '6281234500102', blok: '8', rumah: '02',
    luas: 400, tipe: 'rumah', mulai: 202601, bayarSampai: 202607, belum: rentang(202601, 202606) },
  { alamat: 'N8-05', nama: 'Rina Kusuma', telp: '6281234500105', blok: '8', rumah: '05',
    luas: 50, tipe: 'kavling', mulai: 202601, bayarSampai: 202608 },
  { alamat: 'N7-09', nama: 'Hendra Saputra', telp: '6281234500009', blok: '7', rumah: '09',
    luas: 260, tipe: 'rumah', mulai: 202601, bayarSampai: 202608 },
  // Empty 200m² kavling since 2023, built into a rumah from June 2025 (same
  // land) — the "tipe" upgrade. Sep–Dec 2025 unpaid.
  { alamat: 'N9-09', nama: 'Wawan Kurniadi', telp: '6281234500909', blok: '9', rumah: '09',
    riwayat: [[202301, 200, 'kavling'], [202506, 200, 'rumah']], mulai: 202301, bayarSampai: 202608,
    belum: rentang(202509, 202512) },
  // Merged into the neighbour Jan 2026 and deactivated ('M-Rumah'!K = FALSE) —
  // stays in every tab for audit but must vanish from Warga/Pos/Kas/totals.
  { alamat: 'N6-07', nama: 'Marto Wijoyo', telp: '6281234500607', blok: '6', rumah: '07',
    luas: 100, tipe: 'rumah', mulai: 202301, bayarSampai: 202512, aktif: false },
  // New house the admin hasn't given a RumahRiwayat baseline yet → the app must
  // show "Tarif belum diatur" and refuse payment, never bill it as Rp0.
  { alamat: 'N10-01', nama: 'Yusuf Maulana', telp: '6281234501001', blok: '10', rumah: '01',
    tanpaBaseline: true },
];

// Bulk-generate more houses across every block so the Pos filter + pagination
// actually has something to page through in dev.
const NAMA_DEPAN = ['Andi','Bambang','Citra','Dedi','Eka','Fitri','Gilang','Hani','Irfan','Joko','Kartika','Lukman'];
const NAMA_BELAKANG = ['Wijaya','Nugroho','Pratama','Hidayat','Kurniawan','Setiawan','Handoko','Wibisono'];
const LUAS_CYCLE = [110, 130, 160, 230, 300, 420];
for (let i = 0; i < 34; i++) {
  const blok = BLOK_LIST[i % BLOK_LIST.length];
  const rumah = String(11 + Math.floor(i / BLOK_LIST.length) * 2).padStart(2, '0');
  const bulanBelum = i % 4;   // 0..3 months behind, cycling for variety
  houses.push({
    alamat: `N${blok}-${rumah}`,
    nama: `${NAMA_DEPAN[i % NAMA_DEPAN.length]} ${NAMA_BELAKANG[i % NAMA_BELAKANG.length]}`,
    telp: `62812${String(9000000 + i).padStart(7, '0')}`, blok, rumah,
    luas: LUAS_CYCLE[i % LUAS_CYCLE.length], tipe: i % 9 === 8 ? 'kavling' : 'rumah',
    mulai: 202601, bayarSampai: 202609 - bulanBelum,
  });
}

// `M-RumahRiwayat` tab (docs/sheets-schema.md `M-RumahRiwayat`) — append-only.
export const MOCK_RUMAHRIWAYAT_ROWS = houses.filter((h) => !h.tanpaBaseline).flatMap((h) =>
  (h.riwayat || [[h.mulai, h.luas, h.tipe]]).map(([p, luas, tipe]) =>
    [h.alamat, tahunOf(p), bulanOf(p), luas, tipe]));

// `M-TarifVersi` tab (docs/sheets-schema.md `M-TarifVersi`) — append-only; one rate
// card = every row sharing a (tahun, bulan). Three versions:
// - 2021: older tier-2 ISLK (240rb) + iuran_rt 35rb — ISLK amounts change too.
// - 2023: tier 2 at today's 250rb, iuran_rt still 40rb.
// - 2024 onward (today): iuran_rt 50rb.
const rateCard = (tahun, tier2, iuranRt) => [
  [tahun, 1, 'islk_rumah', 0, 225000],
  [tahun, 1, 'islk_rumah', 120, tier2],
  [tahun, 1, 'islk_rumah', 150, 310000],
  [tahun, 1, 'islk_rumah', 260, 375000],
  [tahun, 1, 'islk_rumah', 400, 400000],
  [tahun, 1, 'islk_kavling', '', 400],
  [tahun, 1, 'iuran_rt', '', iuranRt],
];
export const MOCK_TARIFVERSI_ROWS = [
  ...rateCard(2021, 240000, 35000),
  ...rateCard(2023, 250000, 40000),
  ...rateCard(2024, 250000, 50000),
];

const tarif = (alamat, p) =>
  tarifPada(MOCK_RUMAHRIWAYAT_ROWS, MOCK_TARIFVERSI_ROWS, alamat, tahunOf(p), bulanOf(p));

// `D-Pembayaran` tab (docs/sheets-schema.md `D-Pembayaran`) — the Sheet's derived
// one-row-per-month view of the Tunai + Transfer + Impor tabs:
// A waktu · B alamat · C bulan · D tahun · E nominal · F metode ·
// G petugas · H bukti_url · I keabsahan
// The app "went live" January 2026 — every month before that was typed into
// `M-Impor<tahun>` by the committee (waktu = its tanggal_bayar, no petugas,
// never touches kas balances).
const PETUGAS = ['Ujang', 'Dedi', 'Rahmat'];
const bukti = (seed) => `https://picsum.photos/seed/${seed}/500/700`;
const row = (ts, alamat, p, nominal, metode, petugas, buktiUrl, keabsahan) =>
  [ts, alamat, bulanOf(p), tahunOf(p), nominal, metode, petugas, buktiUrl, keabsahan];
const tsOf = (p, jam = '08:00:00') =>
  `${tahunOf(p)}-${String(bulanOf(p)).padStart(2, '0')}-05 ${jam}`;

export const MOCK_PEMBAYARAN_ROWS = [];
houses.forEach((h, n) => {
  if (h.tanpaBaseline) return;
  for (const p of rentang(h.mulai, h.bayarSampai)) {
    if (h.belum?.includes(p)) continue;
    const nominal = tarif(h.alamat, p);
    if (p < 202601) {
      MOCK_PEMBAYARAN_ROWS.push(row(tsOf(p, '00:00:00'), h.alamat, p, nominal, 'impor', '', '', 'sah'));
    } else if (h.pending?.includes(p)) {
      MOCK_PEMBAYARAN_ROWS.push(row(tsOf(p, '19:41:00'), h.alamat, p, nominal, 'transfer', 'Warga',
        bukti(`${h.alamat}-${p}`), 'pending'));
    } else if (h.metode === 'transfer') {
      MOCK_PEMBAYARAN_ROWS.push(row(tsOf(p, '20:15:00'), h.alamat, p, nominal, 'transfer', 'Warga',
        bukti(`${h.alamat}-${p}`), 'sah'));
    } else {
      MOCK_PEMBAYARAN_ROWS.push(row(tsOf(p), h.alamat, p, nominal, 'tunai', PETUGAS[n % 3], '', 'sah'));
    }
  }
});
MOCK_PEMBAYARAN_ROWS.push(
  // One transfer covering two months (Sep + Oct) — one bukti, one Kas card.
  row('2026-09-16 07:55:00', 'N7-09', 202609, 425000, 'transfer', 'Warga', bukti('N7-09-sep'), 'pending'),
  row('2026-09-16 07:55:00', 'N7-09', 202610, 425000, 'transfer', 'Warga', bukti('N7-09-sep'), 'pending'),
  // Rincian didn't add up to the Form's total → `cek`, never counted.
  row('2026-09-14 21:02:00', 'N8-05', 202609, 35000, 'transfer', 'Warga', bukti('N8-05-sep'), 'cek'),
  // Satpam recorded N8-02's Aug cash under the wrong house; bendahara voided
  // it (Keputusan `tolak`) — listed struck-through, counted nowhere.
  row('2026-09-10 09:12:00', 'N8-11', 202608, 450000, 'tunai', 'Dedi', '', 'tolak'),
  // Satpam double-tapped → second copy is `dobel`, counted once.
  row('2026-09-05 08:00:10', 'N5-11', 202609, tarif('N5-11', 202609), 'tunai', 'Ujang', '', 'dobel'),
);

// 'D-API'!L/M — what the real Sheet's TEXTJOIN(FILTER(...)) would produce.
const periodeList = (alamat, keabsahan) => [...new Set(MOCK_PEMBAYARAN_ROWS
  .filter((r) => r[1] === alamat && r[8] === keabsahan)
  .map((r) => r[3] * 100 + r[2]))].sort((a, b) => a - b).join(',');

// Only what the app can't derive itself — every other cluster total is
// computed client-side in useSheet.js (docs/sheets-schema.md `D-API`).
const meta = [
  ['kas_tunai', 1750000],
  ['rekening', 6250000],
  ['updated', 202609220900],
];

// API per-house block (docs/sheets-schema.md `D-API`, cols D..M).
const houseRow = (h) => [
  h.alamat, h.nama, h.telp, 'N', h.blok, h.rumah, h.pin || h.telp.slice(-3), h.aktif !== false,
  periodeList(h.alamat, 'sah'), periodeList(h.alamat, 'pending'),
];
const rowCount = Math.max(meta.length, houses.length);
export const MOCK_ROWS = Array.from({ length: rowCount }, (_, i) => {
  const [k, v] = meta[i] || [null, null];
  return [k, v, null, ...(houses[i] ? houseRow(houses[i]) : Array(10).fill(null))];
});

// `D-Riwayat` tab (docs/sheets-schema.md `D-Riwayat`) — sah nominal per month, newest
// first, 36 rows back from the current month (same as the Sheet's SUMIFS).
export const MOCK_RIWAYAT_ROWS = rentang(geserPeriode(202609, -35), 202609).reverse().map((p) => [
  tahunOf(p), bulanOf(p),
  MOCK_PEMBAYARAN_ROWS.filter((r) => r[3] * 100 + r[2] === p && r[8] === 'sah')
    .reduce((sum, r) => sum + r[4], 0),
]);

// `M-Opex` tab (docs/sheets-schema.md `M-Opex`) — category totals only.
export const MOCK_OPEX_ROWS = [
  ['Gaji Satpam', '🛡️', 2400000, '2026-08-01'],
  ['Kebersihan', '🧹', 300000, '2026-08-01'],
  ['Listrik & Air Pos', '💡', 150000, '2026-09-05'],
  ['Lain-lain', '📋', 100000, '2026-08-01'],
];

// `M-Blok` tab (docs/sheets-schema.md `M-Blok`) — Blok 7 deliberately differs from
// BLOK_WARNA_DEFAULT, proving the "admin edits the Sheet, app picks it up" path.
export const MOCK_BLOK_ROWS = [
  ['Blvd', '#2a78d6'], ['1', '#9C4A1A'], ['2', '#eb6834'], ['3', '#1baf7a'], ['5', '#eda100'],
  ['6', '#e87ba4'], ['7', '#7C3AED'], ['8', '#4a3aa7'], ['9', '#e34948'], ['10', '#0F86A3'],
];

// `M-Petugas` tab (docs/sheets-schema.md `M-Petugas`) — one row per person.
export const MOCK_PETUGAS_ROWS = [
  ['Ujang', 'satpam'], ['Dedi', 'satpam'], ['Rahmat', 'satpam'],
  ['Ibu Siti', 'bendahara'], ['Pak Joko', 'bendahara'],
];
