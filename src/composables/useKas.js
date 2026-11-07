import { ref, computed } from 'vue';
import { api, ApiError } from '../lib/api.js';
import { BLOK_WARNA_DEFAULT } from '../lib/tariff.js';

// The signed-in pengurus, and the Kas data the server sends them
// (backend/internal/api/kas.go). Everything here is already computed by the
// server — status, tunggakan, tarif per month — the screens only display it.

// ── who is signed in ──
const saya = ref(null);           // { email, nama, peran }
const status = ref('memuat');     // memuat | masuk | perlu-masuk | bukan-pengurus

async function cekSaya() {
  status.value = 'memuat';
  try {
    saya.value = (await api('/api/saya', { pakai: 'pengurus' })).saya;
    status.value = 'masuk';
  } catch (e) {
    saya.value = null;
    status.value = e instanceof ApiError && e.kode === 'bukan-pengurus' ? 'bukan-pengurus' : 'perlu-masuk';
  }
}

export function useSaya() {
  const boleh = (...peran) => !!saya.value && (saya.value.peran === 'admin' || peran.includes(saya.value.peran));
  return { saya, status, cekSaya, boleh };
}

// ── Kas data ──
const data = ref(null);
const loading = ref(false);
let waktuMuat = 0;

async function muat() {
  loading.value = true;
  try {
    data.value = await api('/api/kas', { pakai: 'pengurus' });
    waktuMuat = Date.now();
  } catch (e) {
    if (e instanceof ApiError && (e.status === 401 || e.status === 403)) cekSaya();
    throw e;
  } finally {
    loading.value = false;
  }
}

/** Load unless the data is fresh (moving between Kas pages). */
function pastikan(maksUmurMs = 30_000) {
  if (data.value && Date.now() - waktuMuat < maksUmurMs) return Promise.resolve();
  return muat().catch(() => {});
}

/** Reload if older than a minute — when the tab comes back (App.vue). */
function refresh() {
  if (data.value && Date.now() - waktuMuat > 60_000) muat().catch(() => {});
}

export function useKas() {
  const hari = computed(() => (data.value ? data.value.hari : { tahun: new Date().getFullYear(), bulan: new Date().getMonth() + 1 }));
  const TAHUN = computed(() => hari.value.tahun);
  const sekarang = computed(() => hari.value.tahun * 100 + hari.value.bulan);
  const rumah = computed(() => (data.value ? data.value.rumah : []));
  const saldo = computed(() => (data.value ? data.value.saldo : { kas: 0, rekening: 0 }));
  const blokWarna = computed(() => ({ ...BLOK_WARNA_DEFAULT, ...(data.value ? data.value.warna : {}) }));
  const pending = computed(() => (data.value ? data.value.pending : []));
  const cek = computed(() => (data.value ? data.value.cek : []));
  const tunai = computed(() => (data.value ? data.value.tunai : []));
  const totals = computed(() => ({
    jumlahRumah: rumah.value.length,
    tunggakan: rumah.value.reduce((s, h) => s + h.tunggakan, 0),
    target: rumah.value.reduce((s, h) => s + (h.tarif || 0), 0),
    lunasBulanIni: rumah.value.filter((h) => h.lunasBulanIni).length,
    tarifBelumDiatur: rumah.value.filter((h) => !h.tarifDiatur),
  }));
  return { data, loading, muat, pastikan, refresh, hari, TAHUN, sekarang, rumah, saldo, blokWarna,
           pending, cek, tunai, totals };
}

/** One house's full card (every year) — SemuaKartu's detail, Catat Tunai. */
export async function detailRumah(alamat) {
  return api(`/api/kas/rumah/${encodeURIComponent(alamat)}`, { pakai: 'pengurus' });
}

export const refreshKas = refresh;
