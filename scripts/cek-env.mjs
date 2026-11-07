// Refuses a production build with a missing or dangerous setting. Run by the
// Dockerfile before `npm run build` (docs/deploy.md). Everything VITE_* is
// public — it ends up in the JavaScript every visitor downloads — so secrets
// (Clerk's sk_…, database URLs, tokens) must never be named VITE_*.
import { readFileSync, existsSync } from 'node:fs';

const WAJIB = ['VITE_CLERK_PUBLISHABLE_KEY', 'VITE_WA_OPERASIONAL'];

const env = { ...process.env };
if (existsSync('.env')) {
  for (const baris of readFileSync('.env', 'utf8').split('\n')) {
    const m = /^\s*([A-Z0-9_]+)\s*=\s*(.*?)\s*$/.exec(baris);
    if (!m || m[1] in process.env) continue;
    // same rules as Vite's dotenv: quoted values verbatim, otherwise ` # …` is a comment
    const v = /^["']/.test(m[2]) ? m[2].replace(/^["']|["']$/g, '') : m[2].replace(/(^|\s+)#.*$/, '');
    env[m[1]] = v.trim();
  }
}

const gagal = (pesan) => { console.error(`\n✗ Build dibatalkan — ${pesan}\n`); process.exit(1); };

const kosong = WAJIB.filter((k) => !env[k]);
if (kosong.length) gagal(`variabel ini kosong:\n  ${kosong.join('\n  ')}\nLihat .env.example dan docs/setup.md.`);
if (!/^pk_(test|live)_/.test(env.VITE_CLERK_PUBLISHABLE_KEY)) gagal('VITE_CLERK_PUBLISHABLE_KEY harus kunci publik Clerk (pk_test_… / pk_live_…), BUKAN secret key sk_…');
if (!/^62\d{8,13}$/.test(env.VITE_WA_OPERASIONAL)) gagal('VITE_WA_OPERASIONAL harus nomor WhatsApp berawalan 62, tanpa + atau spasi');
const bocor = Object.entries(env).filter(([k, v]) =>
  /^VITE_/.test(k) && (/(SECRET|TOKEN|PASS|PIN|DATABASE)/.test(k) || /^(sk_|postgres)/.test(String(v))));
if (bocor.length) gagal(`variabel ini terlihat rahasia, padahal VITE_* ikut ke JavaScript publik:\n  ${bocor.map(([k]) => k).join('\n  ')}`);
console.log('✓ .env frontend lengkap');
