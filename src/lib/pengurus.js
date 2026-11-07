import { ref } from 'vue';

// Who is signed in as pengurus (admin / bendahara / komite). Production signs
// in with Clerk (VITE_CLERK_PUBLISHABLE_KEY); the server checks the session
// token and looks the email up in its own pengurus table, so nothing here
// grants rights by itself. Without a key — local development only — the email
// typed on the sign-in screen is sent as X-Dev-Email, which the backend only
// honours with APP_ENV=development.

const KUNCI = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY || '';
export const modeDev = !KUNCI;
const DEV_EMAIL = 'iuran.dev.email';

let clerk = null;
let memuat = null;

/** Load Clerk once (lazily: only /kas pages pay for it). */
export function muatClerk() {
  if (modeDev) return Promise.resolve(null);
  if (!memuat) {
    memuat = Promise.all([import('@clerk/clerk-js'), import('@clerk/localizations')]).then(async ([{ Clerk }, { idID }]) => {
      clerk = new Clerk(KUNCI);
      await clerk.load({ localization: idID });
      return clerk;
    });
  }
  return memuat;
}

export const emailDev = ref((() => { try { return localStorage.getItem(DEV_EMAIL) || ''; } catch { return ''; } })());
export function setEmailDev(e) {
  emailDev.value = e;
  try { e ? localStorage.setItem(DEV_EMAIL, e) : localStorage.removeItem(DEV_EMAIL); } catch { /* private mode */ }
}

export async function headerPengurus() {
  if (modeDev) return emailDev.value ? { 'X-Dev-Email': emailDev.value } : {};
  const c = await muatClerk();
  const token = c && c.session ? await c.session.getToken() : null;
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export async function keluarPengurus() {
  if (modeDev) { setEmailDev(''); return; }
  const c = await muatClerk();
  if (c) await c.signOut();
}
