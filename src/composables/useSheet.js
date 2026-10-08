import { ref, computed } from 'vue';
import { BLOK_WARNA_DEFAULT } from '../lib/tariff.js';
import { rateCardPada, tarifPada } from '../lib/tarifHistoris.js';
import { bangunRumah } from '../lib/rumah.js';
import { api, ApiError } from '../lib/api.js';
import { useAuth } from './useAuth.js';

// The signed-in screens' data (Warga card, Pos, Kas). It comes from the Apps
// Script gateway already trimmed to what that role may see (src/server/core.js):
// raw tab rows, which the app turns into houses/status/tunggakan itself
// (src/lib/rumah.js + tagihan.js — the same code the server uses). The public
// /sum page has its own store, usePublik.js, and never sees a house row.
const rows = ref([]);              // D-API — key/value block (Kas only) + per-house block
const blokRows = ref([]);          // M-Blok
const petugasRows = ref([]);       // M-Petugas — the role's own names only
const rumahRiwayatRows = ref([]);  // M-RumahRiwayat — this house's only, for a resident
const tarifVersiRows = ref([]);    // M-TarifVersi
/** Kas only: D-Pending + D-KasMasuk rows, read by usePembayaranLedger. */
export const ledgerRows = ref({ pending: [], kasMasuk: [] });
const loading = ref(false);

// "Today" comes from the server, in the Sheet's time zone — the same day its
// TODAY() formulas use — so a wrong phone clock can't shift "this month".
// Until the first response it falls back to the device date.
const sekarangDevice = new Date();
const hari = ref({ tahun: sekarangDevice.getFullYear(), bulan: sekarangDevice.getMonth() + 1 });
let terakhir = null;       // [role, creds] of the last load, for refresh()
let waktuMuat = 0;

/** Load a role's data. Resolves to null on success, else the failure kode
 *  ('pin', 'terkunci', 'server', 'jaringan' …) — never throws. A petugas PIN the
 *  server refuses is forgotten here, which sends Pos/Kas back to the PIN screen. */
async function muat(role, creds = {}) {
  const auth = useAuth();
  terakhir = [role, creds];
  loading.value = true;
  try {
    const res = await api(role, role === 'warga' ? creds : { pin: auth.pins.value[role] });
    rows.value = res.api;
    blokRows.value = res.blok || [];
    petugasRows.value = res.petugas || [];
    rumahRiwayatRows.value = res.rumahRiwayat || [];
    tarifVersiRows.value = res.tarifVersi || [];
    ledgerRows.value = { pending: res.pending || [], kasMasuk: res.kasMasuk || [] };
    hari.value = res.hari;
    waktuMuat = Date.now();
    return null;
  } catch (e) {
    if (e instanceof ApiError) {
      if (e.kode === 'pin' && role !== 'warga') auth.clearPin(role, true);
      return e.kode;
    }
    return 'jaringan';   // api() already put it in apiError (the banner)
  } finally {
    loading.value = false;
  }
}

/** Load unless this role's data is already fresh (moving between Kas pages). */
function pastikan(role, maksUmurMs = 30_000) {
  if (terakhir && terakhir[0] === role && Date.now() - waktuMuat < maksUmurMs) return Promise.resolve(null);
  return muat(role);
}

/** Reload the same data if it's older than `maksUmurMs` — called when the tab
 *  becomes visible again (App.vue), so a phone left open overnight catches up. */
function refresh(maksUmurMs = 60_000) {
  if (terakhir && Date.now() - waktuMuat > maksUmurMs) return muat(terakhir[0], terakhir[1]);
}

/** Forget everything in memory (a resident hitting "Ganti rumah"). */
function bersihkan() {
  rows.value = []; blokRows.value = []; petugasRows.value = [];
  rumahRiwayatRows.value = []; ledgerRows.value = { pending: [], kasMasuk: [] };
  terakhir = null;
}

export function useSheet() {
  const TAHUN = computed(() => hari.value.tahun);
  const BULAN_INI = computed(() => hari.value.bulan);
  const sekarang = computed(() => hari.value.tahun * 100 + hari.value.bulan);

  // D-API's key/value block (cols A,B) — only the Kas payload carries it.
  const meta = computed(() => {
    const m = {};
    rows.value.forEach((r) => { if (r[0]) m[r[0]] = r[1]; });
    return m;
  });
  // `updated` is a yyyymmddhhmm number (D-API!B4).
  const diperbarui = computed(() => {
    const s = String(meta.value.updated || '');
    return s.length === 12 ? `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)} ${s.slice(8, 10)}:${s.slice(10)}` : s;
  });

  const rumah = computed(() => bangunRumah(rows.value, rumahRiwayatRows.value, tarifVersiRows.value, hari.value));

  // per-block color (M-Blok, admin-editable in the Sheet) over the built-in defaults
  const blokWarna = computed(() => ({
    ...BLOK_WARNA_DEFAULT,
    ...Object.fromEntries(blokRows.value.filter((r) => r[0]).map((r) => [String(r[0]), r[1]])),
  }));

  // M-Petugas — the server only sends the signed-in role's own names
  const namaPeran = (peran) => petugasRows.value.filter((r) => r[1] === peran).map((r) => String(r[0]));
  const satpamList = computed(() => namaPeran('satpam'));
  const bendaharaList = computed(() => namaPeran('bendahara'));

  // Rate card effective right now — Warga's "Tagihan berjalan" ISLK/Iuran RT split.
  const rateCardAktif = computed(() => rateCardPada(tarifVersiRows.value, TAHUN.value, BULAN_INI.value));

  /** A house's tarif for one specific month — null when RumahRiwayat/TarifVersi
   *  have nothing effective by then. */
  const tarifRumah = (alamat, tahun, bulan) =>
    tarifPada(rumahRiwayatRows.value, tarifVersiRows.value, alamat, tahun, bulan);

  // Cluster numbers Kas shows (the full set for /sum comes from the server).
  const totals = computed(() => ({
    jumlahRumah: rumah.value.length,
    tunggakan: rumah.value.reduce((sum, h) => sum + h.tunggakan, 0),
    target: rumah.value.reduce((sum, h) => sum + (h.tarif || 0), 0),
    lunasBulanIni: rumah.value.filter((h) => h.lunas.has(sekarang.value)).length,
    tarifBelumDiatur: rumah.value.filter((h) => !h.tarifDiatur),
  }));

  return {
    muat, pastikan, refresh, bersihkan, loading, meta, diperbarui, rumah, totals, blokWarna,
    satpamList, bendaharaList, rateCardAktif, tarifRumah, TAHUN, BULAN_INI, sekarang,
  };
}
