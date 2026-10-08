import { hitungTagihan } from './tagihan.js';
import { luasTipePada } from './tarifHistoris.js';

// D-API's per-house block (docs/sheets-schema.md `D-API`, cols D..M) → one object
// per active house, with status/tunggakan/tarif resolved from RumahRiwayat and
// TarifVersi. Used by the browser (Warga/Pos/Kas screens) and by the Apps Script
// server (public aggregates), so both always agree. No `?.`/`??` — it also runs
// in Apps Script (see tarifHistoris.js).
const periodeSet = (v) => new Set(v ? String(v).split(',').map(Number).filter(Boolean) : []);

/** `hari` = { tahun, bulan } — "today", from the server. */
export function bangunRumah(apiRows, rumahRiwayatRows, tarifVersiRows, hari) {
  const sekarang = hari.tahun * 100 + hari.bulan;
  return apiRows
    .filter((r) => r[3] && String(r[10]).toUpperCase() !== 'FALSE')
    .map((r) => {
      const h = {
        alamat: String(r[3]), nama: r[4], telp: r[5] != null ? String(r[5]) : '',
        cluster: r[6], blok: String(r[7] == null ? '' : r[7]), rumah: String(r[8] == null ? '' : r[8]),
        pin: r[9] != null ? String(r[9]) : '',
        lunas: periodeSet(r[11]),
        pending: periodeSet(r[12]),
      };
      const t = hitungTagihan(h, rumahRiwayatRows, tarifVersiRows, sekarang);
      const lt = luasTipePada(rumahRiwayatRows, h.alamat, hari.tahun, hari.bulan);
      return Object.assign({}, h, t, {
        luas: lt ? lt.luas : null, tipe: lt ? lt.tipe : null,
        // No RumahRiwayat baseline (or no rate card) → never billed as Rp0;
        // every screen shows "Tarif belum diatur" and blocks paying instead.
        tarifDiatur: t.tarif != null,
        status: t.kartuTahun(hari.tahun).status,
      });
    });
}
