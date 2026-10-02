// Runs the generated apps-script/Code.gs exactly as Google would — in a vm with
// stubbed SpreadsheetApp/CacheService/… backed by the mock tabs — and checks the
// role rules that keep the Sheet's data private. Run: npm run test:server
import vm from 'node:vm';
import assert from 'node:assert/strict';
import { bangun } from './build-apps-script.mjs';
import * as M from '../src/composables/mockData.js';

const HARI = { tahun: 2026, bulan: 9 };
const pembayaran = M.MOCK_PEMBAYARAN_ROWS;
const TABS = {
  'D-API': M.MOCK_ROWS, 'M-Blok': M.MOCK_BLOK_ROWS, 'M-Petugas': M.MOCK_PETUGAS_ROWS,
  'M-Opex': M.MOCK_OPEX_ROWS, 'D-Riwayat': M.MOCK_RIWAYAT_ROWS,
  'M-RumahRiwayat': M.MOCK_RUMAHRIWAYAT_ROWS, 'M-TarifVersi': M.MOCK_TARIFVERSI_ROWS,
  'D-Pending': pembayaran.filter((r) => r[8] === 'pending' || r[8] === 'cek'),
  'D-KasMasuk': pembayaran.filter((r) => r[5] === 'tunai'),
  'D-Iuran2026': pembayaran.filter((r) => Number(r[3]) === 2026),
};

// A Sheet as Apps Script returns it: a header row first, '' for empty cells.
const grid = (rows) => [['hdr'], ...rows.map((r) => r.map((c) => (c == null ? '' : c)))];
const sheets = Object.fromEntries(Object.entries(TABS).map(([n, rows]) => [n, grid(rows)]));
const cache = new Map();
const sandbox = {
  console,
  SpreadsheetApp: { getActiveSpreadsheet: () => ({
    getSheetByName: (n) => (sheets[n] ? { getDataRange: () => ({ getValues: () => sheets[n] }) } : null),
    getSpreadsheetTimeZone: () => 'Asia/Jakarta',
  }) },
  PropertiesService: { getScriptProperties: () => ({ getProperty: (k) => ({ POS_PIN: '1234', KAS_PIN: '5678' })[k] || null }) },
  CacheService: { getScriptCache: () => ({
    get: (k) => cache.get(k) ?? null, put: (k, v) => cache.set(k, v), remove: (k) => cache.delete(k),
  }) },
  Utilities: { formatDate: (d, tz, f) => (f === 'yyyy-M' ? `${HARI.tahun}-${HARI.bulan}` : '00:00:00') },
  ContentService: { MimeType: { JSON: 'json' },
    createTextOutput: (t) => ({ t, setMimeType() { return this; }, getContent() { return this.t; } }) },
};
vm.createContext(sandbox);
vm.runInContext(bangun(), sandbox, { filename: 'Code.gs' });

const panggil = (req) => JSON.parse(sandbox.doPost({ postData: { contents: JSON.stringify(req) } }).getContent());
let n = 0;
const ok = (nama, fn) => { fn(); n += 1; console.log('  ✓', nama); };
const adaTelpAtauPin = (rows) => rows.some((r) => r[5] != null || r[9] != null);

ok('doGet answers (deployment check)', () => assert.equal(JSON.parse(sandbox.doGet().getContent()).ok, true));

ok('publik: aggregates only — no house rows, names, phones or PINs', () => {
  const r = panggil({ action: 'publik' });
  assert.equal(r.ok, true);
  assert.ok(r.jumlahRumah > 30 && r.target > 0 && r.riwayat.length && r.aging.distribusi.length === 4);
  const teks = JSON.stringify(r);
  for (const rahasia of ['Budi Santoso', 'N7-01', '6281234500001', 'picsum']) assert.ok(!teks.includes(rahasia), rahasia);
});

ok('warga: wrong PIN, unknown house and deactivated house all answer the same', () => {
  const a = panggil({ action: 'warga', alamat: 'N7-01', pin: '000' });
  const b = panggil({ action: 'warga', alamat: 'N0-99', pin: '001' });
  const c = panggil({ action: 'warga', alamat: 'N6-07', pin: '607' });
  assert.deepEqual([a.kode, b.kode, c.kode], ['pin', 'pin', 'pin']);
});

ok('warga: right PIN → this house only, no phone/PIN/balances', () => {
  const r = panggil({ action: 'warga', alamat: 'N7-01', pin: '001' });
  assert.equal(r.ok, true);
  assert.equal(r.api.length, 1);
  assert.equal(r.api[0][3], 'N7-01');
  assert.ok(!adaTelpAtauPin(r.api));
  assert.ok(r.rumahRiwayat.every((x) => x[0] === 'N7-01') && r.rumahRiwayat.length === 2);
  assert.ok(!('petugas' in r) && !('pending' in r));
});

ok('warga: 5 wrong PINs lock the house, even for the right PIN', () => {
  for (let i = 0; i < 5; i += 1) assert.equal(panggil({ action: 'warga', alamat: 'N7-03', pin: '999' }).kode, 'pin');
  assert.equal(panggil({ action: 'warga', alamat: 'N7-03', pin: '003' }).kode, 'terkunci');
  assert.equal(panggil({ action: 'warga', alamat: 'N7-01', pin: '001' }).ok, true);   // other houses unaffected
});

ok('pos: needs POS_PIN; all active houses, no phones, PINs or balances', () => {
  assert.equal(panggil({ action: 'pos', pin: '5678' }).kode, 'pin');
  const r = panggil({ action: 'pos', pin: '1234' });
  assert.equal(r.ok, true);
  assert.ok(r.api.length > 30 && !adaTelpAtauPin(r.api));
  assert.ok(r.api.every((x) => x[0] == null && x[1] == null));
  assert.ok(!r.api.some((x) => x[3] === 'N6-07'));   // deactivated: never sent
  assert.ok(r.petugas.every((x) => x[1] === 'satpam') && r.petugas.length === 3);
  assert.ok(!('pending' in r));
});

ok('kas: needs KAS_PIN; phones + balances + ledgers, still no PINs', () => {
  assert.equal(panggil({ action: 'kas', pin: '1234' }).kode, 'pin');
  const r = panggil({ action: 'kas', pin: '5678' });
  assert.equal(r.ok, true);
  assert.ok(r.api.some((x) => x[5] != null) && r.api.every((x) => x[9] == null));
  assert.ok(r.api.some((x) => x[0] === 'kas_tunai'));
  assert.ok(r.pending.length > 0 && r.kasMasuk.length > 0);
  assert.ok(r.petugas.every((x) => x[1] === 'bendahara'));
});

ok('kasTahun: PIN-gated per-year summary; missing tab is a clean error', () => {
  assert.equal(panggil({ action: 'kasTahun', pin: '1234', tahun: 2026 }).kode, 'pin');
  const r = panggil({ action: 'kasTahun', pin: '5678', tahun: 2026 });
  assert.equal(r.ringkasan.bulan.length, 12);
  assert.ok(r.ringkasan.total > 0);
  assert.equal(panggil({ action: 'kasTahun', pin: '5678', tahun: 2031 }).kode, 'tab');
  assert.equal(panggil({ action: 'kasTahun', pin: '5678', tahun: 'x' }).kode, 'tahun');
});

ok('login: petugas PIN check + lock-out after 10 misses', () => {
  assert.equal(panggil({ action: 'login', role: 'pos', pin: '1234' }).ok, true);
  for (let i = 0; i < 10; i += 1) panggil({ action: 'login', role: 'kas', pin: '0000' });
  assert.equal(panggil({ action: 'login', role: 'kas', pin: '5678' }).kode, 'terkunci');
  assert.equal(panggil({ action: 'login', role: 'admin', pin: '1' }).kode, 'role');
});

ok('garbage input never throws', () => {
  assert.equal(JSON.parse(sandbox.doPost({ postData: { contents: 'not json' } }).getContent()).kode, 'request');
  assert.equal(panggil({ action: 'nope' }).kode, 'action');
  assert.equal(panggil({}).kode, 'action');
});

console.log(`\n${n} server checks passed`);
