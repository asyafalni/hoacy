import { ref } from 'vue';
import { headerPengurus } from './pengurus.js';

// The one way the app talks to the backend (backend/internal/api). Same origin
// in production — the Go server serves this app too; `npm run dev` proxies
// /api to it (vite.config.js).
//
// Every answer is JSON {ok, …}; a refusal is {ok:false, kode, pesan} where
// `pesan` is meant for the user as is. Network trouble goes to `apiError`
// (the banner in App.vue); a refusal is thrown as ApiError for the screen to show.

export const apiError = ref('');

export class ApiError extends Error {
  constructor(kode, pesan, status) {
    super(pesan || kode);
    this.kode = kode;
    this.status = status;
  }
}

const SESI = 'iuran.sesi';   // { alamat, token } — the resident signed in on this device
export function sesiWarga() {
  try { return JSON.parse(localStorage.getItem(SESI) || 'null'); } catch { return null; }
}
export function simpanSesiWarga(s) {
  try { s ? localStorage.setItem(SESI, JSON.stringify(s)) : localStorage.removeItem(SESI); } catch { /* private mode */ }
}

/**
 * `pakai`: 'warga' sends the resident's session token, 'pengurus' the signed-in
 * pengurus (Clerk), nothing otherwise. `body` is JSON; `form` a FormData upload.
 */
export async function api(path, { method = 'GET', body, form, pakai } = {}) {
  const headers = {};
  if (pakai === 'warga') {
    const s = sesiWarga();
    if (s) headers.Authorization = `Bearer ${s.token}`;
  } else if (pakai === 'pengurus') {
    Object.assign(headers, await headerPengurus());
  }
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  let res;
  try {
    res = await fetch(path, {
      method, headers,
      body: form || (body !== undefined ? JSON.stringify(body) : undefined),
    });
  } catch {
    apiError.value = 'Tidak bisa menghubungi server — periksa koneksi lalu coba lagi.';
    throw new ApiError('jaringan', apiError.value, 0);
  }
  let data = null;
  try { data = await res.json(); } catch { /* not JSON */ }
  if (res.status >= 500 && !(data && data.pesan)) {
    apiError.value = 'Server bermasalah — coba lagi sebentar lagi.';
  } else {
    apiError.value = '';
  }
  if (!res.ok || !data || data.ok === false) {
    throw new ApiError((data && data.kode) || 'server', (data && data.pesan) || 'Server bermasalah.', res.status);
  }
  return data;
}

/** A proof image/PDF served only to pengurus → a local object URL for <img>/<a>. */
export async function urlBerkas(path) {
  const res = await fetch(path, { headers: await headerPengurus() });
  if (!res.ok) throw new ApiError('berkas', 'Berkas tidak bisa dibuka.', res.status);
  return URL.createObjectURL(await res.blob());
}
