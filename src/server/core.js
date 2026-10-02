import { ringkasanPublik, ringkasTahun } from '../lib/ringkasan.js';

// The read gateway between the app and the private Sheet. One pure function,
// `handle(req, ctx)`, run by Google Apps Script in production (bundled into
// apps-script/Code.gs, with src/server/gas.js as the thin doPost wrapper) and by
// the browser's local mock mode (src/lib/apiLokal.js) — so what the tests see is
// what runs for real. Also bundled, so no `?.`/`??` (see lib/tarifHistoris.js).
//
// ctx = {
//   tab(name)            → rows of a tab, header row dropped, '' → null
//   prop(name)           → a Script Property (POS_PIN, KAS_PIN)
//   cache                → { get(key), put(key, value, ttlSeconds), remove(key) }
//   hari()               → { tahun, bulan } — "today" in the Sheet's time zone
// }
//
// Who gets what (the point of the whole thing — the Sheet itself is private):
//   publik  nobody's PIN   aggregates only, no house rows (PDP)
//   warga   alamat + PIN   that one house's rows + its own RumahRiwayat
//   pos     POS_PIN        every active house, without phone numbers or PINs
//   kas     KAS_PIN        + phone numbers, balances, pending/cash ledgers
// Rows are returned raw (the app's own lib/tagihan.js turns them into status).

const GAGAL_MAKS_WARGA = 5;      // wrong PINs per house before it locks…
const GAGAL_MAKS_PETUGAS = 10;   // …and per petugas role (a global counter)
const KUNCI_DETIK = 900;         // …for 15 minutes
const PUBLIK_DETIK = 60;         // public aggregates are cached this long

const gagal = (kode, pesan) => ({ ok: false, kode, pesan: pesan || '' });

// ── PIN checks, with a lock-out so 3 digits can't simply be brute-forced ───
// (cache keys are capped at 250 chars; a made-up giant alamat must not throw)
const kunciGagal = (kunci) => 'gagal:' + String(kunci).slice(0, 60);
function terkunci(ctx, kunci, maks) {
  return Number(ctx.cache.get(kunciGagal(kunci)) || 0) >= maks;
}
function catatGagal(ctx, kunci) {
  ctx.cache.put(kunciGagal(kunci), String(Number(ctx.cache.get(kunciGagal(kunci)) || 0) + 1), KUNCI_DETIK);
}
function bersihkanGagal(ctx, kunci) {
  ctx.cache.remove(kunciGagal(kunci));
}

/** → null when OK, else the failure response. */
function cekPetugas(ctx, role, pin) {
  if (role !== 'pos' && role !== 'kas') return gagal('role');
  const benar = ctx.prop(role === 'pos' ? 'POS_PIN' : 'KAS_PIN');
  if (!benar) return gagal('config', 'PIN belum diatur di Script Properties (POS_PIN / KAS_PIN).');
  if (terkunci(ctx, role, GAGAL_MAKS_PETUGAS)) return gagal('terkunci');
  if (String(pin == null ? '' : pin) !== String(benar)) {
    catatGagal(ctx, role);
    return gagal('pin');
  }
  bersihkanGagal(ctx, role);
  return null;
}

/** Unknown house, deactivated house and wrong PIN all answer the same 'pin', so
 *  addresses can't be enumerated. → { gagal } or { baris }. */
function cekWarga(ctx, alamat, pin) {
  const id = String(alamat == null ? '' : alamat);
  if (terkunci(ctx, id, GAGAL_MAKS_WARGA)) return { gagal: gagal('terkunci') };
  const baris = ctx.tab('D-API').find((r) =>
    r[3] && String(r[3]) === id && String(r[10]).toUpperCase() !== 'FALSE');
  const benar = baris && baris[9] != null ? String(baris[9]) : '';
  if (!benar || String(pin == null ? '' : pin) !== benar) {
    catatGagal(ctx, id);
    return { gagal: gagal('pin') };
  }
  bersihkanGagal(ctx, id);
  return { baris };
}

// ── Shaping rows per role ──────────────────────────────────────────────────
// D-API row: 0 key · 1 value · 3 alamat · 4 nama · 5 telp · 9 pin · 10 aktif …
const aktif = (r) => r[3] && String(r[10]).toUpperCase() !== 'FALSE';

function sanitasi(r, opsi) {
  const c = r.slice();
  if (!aktif(r)) for (let i = 2; i < c.length; i += 1) c[i] = null;   // a deactivated house sends nothing
  c[9] = null;                       // nobody gets a PIN back
  if (!opsi.telp) c[5] = null;
  if (!opsi.meta) { c[0] = null; c[1] = null; }
  return c;
}

/** D-API rows for a role: active houses (plus the key/value balance rows for Kas). */
function barisApi(ctx, opsi) {
  return ctx.tab('D-API')
    .filter((r) => aktif(r) || (opsi.meta && r[0]))
    .map((r) => sanitasi(r, opsi));
}

const dasar = (ctx, hari) => ({
  ok: true,
  hari: { tahun: hari.tahun, bulan: hari.bulan },
  tarifVersi: ctx.tab('M-TarifVersi'),
  blok: ctx.tab('M-Blok'),
});

function publik(ctx, hari) {
  const hit = ctx.cache.get('publik');
  if (hit) return JSON.parse(hit);
  const hasil = Object.assign({ ok: true, hari: { tahun: hari.tahun, bulan: hari.bulan } },
    ringkasanPublik({
      api: ctx.tab('D-API'), rumahRiwayat: ctx.tab('M-RumahRiwayat'), tarifVersi: ctx.tab('M-TarifVersi'),
      opex: ctx.tab('M-Opex'), riwayat: ctx.tab('D-Riwayat'),
    }, hari));
  ctx.cache.put('publik', JSON.stringify(hasil), PUBLIK_DETIK);
  return hasil;
}

function warga(ctx, hari, req) {
  const cek = cekWarga(ctx, req.alamat, req.pin);
  if (cek.gagal) return cek.gagal;
  const id = String(cek.baris[3]);
  const p = dasar(ctx, hari);
  p.api = [sanitasi(cek.baris, {})];   // this house only
  p.rumahRiwayat = ctx.tab('M-RumahRiwayat').filter((r) => String(r[0]) === id);
  return p;
}

function pos(ctx, hari, req) {
  const g = cekPetugas(ctx, 'pos', req.pin);
  if (g) return g;
  const p = dasar(ctx, hari);
  p.api = barisApi(ctx, {});
  p.rumahRiwayat = ctx.tab('M-RumahRiwayat');
  p.petugas = ctx.tab('M-Petugas').filter((r) => r[1] === 'satpam');
  return p;
}

function kas(ctx, hari, req) {
  const g = cekPetugas(ctx, 'kas', req.pin);
  if (g) return g;
  const p = dasar(ctx, hari);
  p.api = barisApi(ctx, { meta: true, telp: true });
  p.rumahRiwayat = ctx.tab('M-RumahRiwayat');
  p.petugas = ctx.tab('M-Petugas').filter((r) => r[1] === 'bendahara');
  p.pending = ctx.tab('D-Pending');
  p.kasMasuk = ctx.tab('D-KasMasuk');
  return p;
}

function kasTahun(ctx, hari, req) {
  const g = cekPetugas(ctx, 'kas', req.pin);
  if (g) return g;
  const tahun = Math.floor(Number(req.tahun));
  if (!(tahun >= 2000 && tahun <= 2100)) return gagal('tahun');
  let rows;
  try { rows = ctx.tab('D-Iuran' + tahun); } catch (e) { return gagal('tab', 'Tab D-Iuran' + tahun + ' belum ada.'); }
  return { ok: true, hari: { tahun: hari.tahun, bulan: hari.bulan }, tahun, ringkasan: ringkasTahun(rows) };
}

export function handle(req, ctx) {
  try {
    const hari = ctx.hari();
    switch (req && req.action) {
      case 'publik': return publik(ctx, hari);
      case 'login': {
        const g = cekPetugas(ctx, req.role, req.pin);
        return g || { ok: true };
      }
      case 'warga': return warga(ctx, hari, req);
      case 'pos': return pos(ctx, hari, req);
      case 'kas': return kas(ctx, hari, req);
      case 'kasTahun': return kasTahun(ctx, hari, req);
      default: return gagal('action');
    }
  } catch (e) {
    return gagal('server', String(e && e.message ? e.message : e));
  }
}
