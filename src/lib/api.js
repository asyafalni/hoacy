import { ref } from 'vue';

// The only door to the data. In production every call is one POST to the Apps
// Script web app (VITE_API_URL — docs/setup.md, Tahap 6); the Sheet behind it is
// private, and the server decides what each caller may see. Without
// VITE_API_URL the same server code runs locally against src/composables/
// mockData.js (see apiLokal.js), so `npm run dev` works with no Google setup.
/** The last network/server failure (null while calls succeed) — App.vue shows a
 *  banner from it. A wrong PIN is not a failure and never lands here. */
export const apiError = ref(null);

export class ApiError extends Error {
  constructor(kode, pesan) {
    super(pesan || kode);
    this.kode = kode;   // pin | terkunci | config | tab | server …
  }
}

export async function api(action, params = {}) {
  let res;
  try {
    // The condition reads import.meta.env directly (not through a variable) so the
    // build can see it's constant and drop the mock branch — and apiLokal.js, with
    // its dev PINs and fixtures — from a production bundle.
    if (import.meta.env.VITE_API_URL) {
      // No Content-Type header → text/plain: a "simple" request, so the browser
      // sends no CORS preflight (Apps Script web apps can't answer one).
      const r = await fetch(import.meta.env.VITE_API_URL, { method: 'POST', body: JSON.stringify({ action, ...params }) });
      res = await r.json();
    } else {
      res = await (await import('./apiLokal.js')).panggilLokal({ action, ...params });
    }
  } catch (e) {
    apiError.value = e;
    throw e;
  }
  apiError.value = null;
  if (!res.ok) {
    if (res.kode === 'server' || res.kode === 'config') apiError.value = new Error(res.pesan || res.kode);
    throw new ApiError(res.kode, res.pesan);
  }
  return res;
}
