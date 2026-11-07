// Golden data for the Go port of the billing logic (backend/internal/tagihan):
// runs the JavaScript originals in src/lib/ over the mock dataset at a few
// "today"s and writes inputs + outputs to one JSON file. The Go tests replay the
// inputs and must produce exactly the same outputs.
//
//   node scripts/golden-tagihan.mjs
import { writeFileSync, mkdirSync } from 'node:fs';
import * as M from '../src/composables/mockData.js';
import { bangunRumah } from '../src/lib/rumah.js';
import { ringkasanPublik, ringkasTahun } from '../src/lib/ringkasan.js';
import { tahunOf } from '../src/lib/tagihan.js';

const HARI = [{ tahun: 2026, bulan: 9 }, { tahun: 2027, bulan: 1 }, { tahun: 2025, bulan: 12 }, { tahun: 2023, bulan: 6 }];

const rumah = M.MOCK_ROWS.filter((r) => r[3]).map((r) => ({
  alamat: String(r[3]), aktif: String(r[10]).toUpperCase() !== 'FALSE',
  lunas: r[11] ? String(r[11]).split(',').map(Number) : [],
  pending: r[12] ? String(r[12]).split(',').map(Number) : [],
}));

const kasus = HARI.map((hari) => {
  const houses = bangunRumah(M.MOCK_ROWS, M.MOCK_RUMAHRIWAYAT_ROWS, M.MOCK_TARIFVERSI_ROWS, hari);
  return {
    hari,
    rumah: houses.map((h) => {
      const kartu = [];
      if (h.mulai != null) for (let y = tahunOf(h.mulai); y <= hari.tahun + 1; y += 1) kartu.push({ tahun: y, ...h.kartuTahun(y) });
      return {
        alamat: h.alamat, mulai: h.mulai, tarif: h.tarif, tarifDiatur: h.tarifDiatur,
        luas: h.luas, tipe: h.tipe, tunggakan: h.tunggakan, tertua: h.tertua,
        tunggakanList: h.tunggakanList, kartu,
      };
    }),
    ringkasan: ringkasanPublik({
      api: M.MOCK_ROWS, rumahRiwayat: M.MOCK_RUMAHRIWAYAT_ROWS, tarifVersi: M.MOCK_TARIFVERSI_ROWS,
      riwayat: M.MOCK_RIWAYAT_ROWS, opex: M.MOCK_OPEX_ROWS,
    }, hari),
  };
});

const meta = {};
M.MOCK_ROWS.forEach((r) => { if (r[0]) meta[r[0]] = r[1]; });


mkdirSync('backend/internal/tagihan/testdata', { recursive: true });
writeFileSync('backend/internal/tagihan/testdata/golden.json', JSON.stringify({
  masukan: {
    rumah,
    riwayat: M.MOCK_RUMAHRIWAYAT_ROWS.map((r) => ({ alamat: r[0], berlaku: r[1] * 100 + r[2], luas: Number(r[3]), tipe: r[4] })),
    tarif: M.MOCK_TARIFVERSI_ROWS.map((r) => ({ berlaku: r[0] * 100 + r[1], komponen: r[2], luasMin: Number(r[3]) || 0, nominal: Number(r[4]) })),
    terkumpul: M.MOCK_RIWAYAT_ROWS.map((r) => ({ tahun: r[0], bulan: r[1], terkumpul: r[2] })),
    opex: M.MOCK_OPEX_ROWS.map((r) => ({ kategori: r[0], ikon: r[1], nominal: r[2], diperbarui: r[3] })),
    kas: Number(meta.kas_tunai || 0), rekening: Number(meta.rekening || 0),
    pembayaran: M.MOCK_PEMBAYARAN_ROWS,
  },
  kasus,
  ringkasTahun: [2025, 2026].map((t) => ({ tahun: t, hasil: ringkasTahun(M.MOCK_PEMBAYARAN_ROWS.filter((r) => Number(r[3]) === t)) })),

}, null, 1));
console.log('golden:', kasus.length, 'hari ×', kasus[0].rumah.length, 'rumah →', 'backend/internal/tagihan/testdata/golden.json');
