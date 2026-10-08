import { handle } from '../server/core.js';
import * as M from '../composables/mockData.js';

// Local stand-in for the Apps Script web app: the very same `handle()` from
// src/server/core.js, fed mock tabs instead of a Sheet. Dev-only PINs:
// Pos 1234 · Kas 5678 · warga = each house's PIN in mockData (default: last 3
// digits of its phone). "Today" is pinned to 2026-09-22 so the fixtures — built
// around September 2026 — stay valid whatever the real date is.
console.warn('[api] VITE_API_URL kosong — memakai data mockup lokal (PIN: pos 1234, kas 5678).');

const HARI = { tahun: 2026, bulan: 9 };
const pembayaran = M.MOCK_PEMBAYARAN_ROWS;
const TABS = {
  'D-API': M.MOCK_ROWS,
  'M-Blok': M.MOCK_BLOK_ROWS,
  'M-Petugas': M.MOCK_PETUGAS_ROWS,
  'M-Opex': M.MOCK_OPEX_ROWS,
  'D-Riwayat': M.MOCK_RIWAYAT_ROWS,
  'M-RumahRiwayat': M.MOCK_RUMAHRIWAYAT_ROWS,
  'M-TarifVersi': M.MOCK_TARIFVERSI_ROWS,
  'D-Pending': pembayaran.filter((r) => r[8] === 'pending' || r[8] === 'cek'),
  'D-KasMasuk': pembayaran.filter((r) => r[5] === 'tunai' && Number(String(r[0]).slice(0, 4)) >= HARI.tahun - 1),
};

const kedaluwarsa = new Map();
const ctx = {
  tab(nama) {
    const iuran = /^D-Iuran(\d{4})$/.exec(nama);
    const rows = TABS[nama] || (iuran && pembayaran.filter((r) => Number(r[3]) === Number(iuran[1])));
    if (!rows) throw new Error('Tab tidak ditemukan: ' + nama);
    return rows;
  },
  prop: (k) => ({ POS_PIN: '1234', KAS_PIN: '5678' })[k],
  cache: {
    get(k) {
      const e = kedaluwarsa.get(k);
      if (e && e.sampai > Date.now()) return e.nilai;
      kedaluwarsa.delete(k);
      return null;
    },
    put(k, nilai, detik) { kedaluwarsa.set(k, { nilai, sampai: Date.now() + detik * 1000 }); },
    remove(k) { kedaluwarsa.delete(k); },
  },
  hari: () => HARI,
};

/** JSON round-trip, like a real network hop — callers can't mutate the fixtures. */
export async function panggilLokal(req) {
  return JSON.parse(JSON.stringify(handle(JSON.parse(JSON.stringify(req)), ctx)));
}
