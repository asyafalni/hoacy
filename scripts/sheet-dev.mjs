// The mock dataset (src/composables/mockData.js) written out as the Google
// Sheet's tabs, one CSV per tab in backend/dev/sheet/. The backend's Sheet sync
// reads them (SHEET_CSV_DIR) exactly like the real spreadsheet, so a local
// database is seeded through the same path a real cutover takes:
//
//   node scripts/sheet-dev.mjs
//   cd backend && go run ./cmd/sinkron -csv dev/sheet -terapkan
import { mkdirSync, writeFileSync } from 'node:fs';
import * as M from '../src/composables/mockData.js';

const DIR = 'backend/dev/sheet';
mkdirSync(DIR, { recursive: true });
const csv = (v) => (v == null ? '' : /[",\n]/.test(String(v)) ? `"${String(v).replace(/"/g, '""')}"` : String(v));
const tulis = (tab, header, rows) => {
  writeFileSync(`${DIR}/${tab}.csv`, [header, ...rows].map((r) => r.map(csv).join(',')).join('\n') + '\n');
  console.log(`${tab}: ${rows.length}`);
};

tulis('M-Blok', ['blok', 'warna'], M.MOCK_BLOK_ROWS);

const rumah = M.MOCK_ROWS.filter((r) => r[3]);
tulis('M-Rumah', ['alamat', 'cluster', 'blok', 'rumah', 'nama', 'telp', 'pin', 'konfirmasi_nonaktif',
  'dinonaktifkan_oleh', 'tanggal_nonaktif', 'aktif', 'peringatan'], rumah.map((r) => {
  const aktif = r[10] !== false;
  return [r[3], r[6], r[7], r[8], r[4], r[5], r[9], aktif ? '' : r[3], aktif ? '' : 'Ibu Siti',
    aktif ? '' : '2026-01-31', aktif ? 'TRUE' : 'FALSE', ''];
}));

tulis('M-RumahRiwayat', ['alamat', 'tahun_berlaku', 'bulan_berlaku', 'luas', 'tipe'], M.MOCK_RUMAHRIWAYAT_ROWS);
tulis('M-TarifVersi', ['tahun_berlaku', 'bulan_berlaku', 'komponen', 'luas_min', 'nominal'], M.MOCK_TARIFVERSI_ROWS);

// D-Pembayaran rows back into the submissions that produced them
const kiriman = new Map();
for (const r of M.MOCK_PEMBAYARAN_ROWS) {
  const [waktu, alamat, bulan, tahun, nominal, metode, petugas, bukti, keabsahan] = r;
  const k = `${metode}|${waktu}|${alamat}`;
  if (!kiriman.has(k)) kiriman.set(k, { waktu, alamat, metode, petugas, bukti, keabsahan, bulan: [] });
  kiriman.get(k).bulan.push([tahun * 100 + bulan, nominal]);
}
const impor = {};
const tunai = [], transfer = [], keputusan = [];
const rincian = (b) => b.map(([p, n]) => `${p}=${n}`).join(',');
const jumlah = (b) => b.reduce((s, [, n]) => s + n, 0);
let kasTunai = 0, masukTransfer = 0;
for (const k of kiriman.values()) {
  if (k.metode === 'impor') {
    for (const [p, n] of k.bulan) {
      const t = Math.floor(p / 100);
      (impor[t] ||= []).push([k.alamat, p % 100, n, k.waktu.slice(0, 10)]);
    }
    continue;
  }
  const total = jumlah(k.bulan) + (k.keabsahan === 'cek' ? 1 : 0);   // `cek` = total doesn't add up
  if (k.metode === 'tunai') {
    tunai.push([k.waktu, k.alamat, rincian(k.bulan), total, k.petugas]);
    if (k.keabsahan === 'sah') kasTunai += jumlah(k.bulan);
  } else {
    transfer.push([k.waktu, k.alamat, rincian(k.bulan), total, k.bukti]);
    if (k.keabsahan === 'sah') masukTransfer += jumlah(k.bulan);
  }
  if (k.keabsahan === 'sah' && k.metode === 'transfer') keputusan.push([k.waktu, k.alamat, k.waktu, 'sah', 'Ibu Siti']);
  if (k.keabsahan === 'tolak') keputusan.push([k.waktu, k.alamat, k.waktu, 'tolak', 'Ibu Siti']);
}
for (const [t, rows] of Object.entries(impor)) tulis(`M-Impor${t}`, ['alamat', 'bulan', 'nominal', 'tanggal_bayar'], rows);
tulis('L-Tunai', ['Timestamp', 'alamat', 'rincian', 'total', 'petugas'], tunai);
tulis('L-Transfer', ['Timestamp', 'alamat', 'rincian', 'total', 'bukti'], transfer);
tulis('L-Keputusan', ['Timestamp', 'alamat', 'waktu', 'keputusan', 'oleh'], keputusan);
tulis('L-Setoran', ['Timestamp', 'nominal', 'oleh'], []);
tulis('L-Pengeluaran', ['Timestamp', 'keterangan', 'nominal', 'sumber'], []);
tulis('M-Opex', ['kategori', 'ikon', 'nominal', 'diperbarui'], M.MOCK_OPEX_ROWS);

// opening balances chosen so the computed balances land on the mock's
const meta = Object.fromEntries(M.MOCK_ROWS.filter((r) => r[0]).map((r) => [r[0], r[1]]));
tulis('M-SaldoAwal', ['tanggal', 'kas', 'rekening'],
  [['2026-01-01', Math.max(0, meta.kas_tunai - kasTunai), Math.max(0, meta.rekening - masukTransfer)]]);
