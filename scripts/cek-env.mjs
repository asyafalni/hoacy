// Refuses a production build that would silently run on mock data or open
// broken Forms: without VITE_API_URL the app falls back to src/lib/apiLokal.js
// (fixtures + dev PINs), and without the Form ids every write goes nowhere.
// Run by the Dockerfile before `npm run build` (docs/deploy.md).
import { readFileSync, existsSync } from 'node:fs';

const WAJIB = [
  'VITE_API_URL',
  'VITE_FORM_TUNAI', 'VITE_E_TUNAI_ALAMAT', 'VITE_E_TUNAI_RINCIAN', 'VITE_E_TUNAI_TOTAL', 'VITE_E_TUNAI_PETUGAS',
  'VITE_FORM_TRANSFER', 'VITE_E_TRANSFER_ALAMAT', 'VITE_E_TRANSFER_RINCIAN', 'VITE_E_TRANSFER_TOTAL',
  'VITE_FORM_SETORAN', 'VITE_E_SETOR_NOMINAL', 'VITE_E_SETOR_OLEH',
  'VITE_FORM_KEPUTUSAN', 'VITE_E_KEP_ALAMAT', 'VITE_E_KEP_WAKTU', 'VITE_E_KEP_KEPUTUSAN', 'VITE_E_KEP_OLEH',
  'VITE_WA_OPERASIONAL',
];

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

const kosong = WAJIB.filter((k) => !env[k]);
if (kosong.length) {
  console.error(`\n✗ Build dibatalkan — variabel ini kosong di .env:\n  ${kosong.join('\n  ')}\n`
    + 'Tanpa VITE_API_URL situs akan jalan dengan data mock; tanpa id Form semua pencatatan hilang.\n'
    + 'Lihat docs/setup.md Tahap 7.\n');
  process.exit(1);
}
if (!/^62\d{8,13}$/.test(env.VITE_WA_OPERASIONAL)) {
  console.error('\n✗ VITE_WA_OPERASIONAL harus nomor WhatsApp berawalan 62, tanpa + atau spasi (mis. 6281234567890)\n');
  process.exit(1);
}
if (!/^https:\/\/script\.google\.com\/macros\/s\/[^/]+\/exec$/.test(env.VITE_API_URL)) {
  console.error('\n✗ VITE_API_URL harus berbentuk https://script.google.com/macros/s/…/exec\n');
  process.exit(1);
}
// Any VITE_* value the code reads is inlined into the public bundle; the day a
// PIN-like variable gets referenced it would ship to anyone. Keep them out entirely.
const bocor = Object.keys(env).filter((k) => /^VITE_.*(PIN|SECRET|TOKEN|PASS)/.test(k));
if (bocor.length) {
  console.error(`\n✗ Build dibatalkan — ${bocor.join(', ')} ada di .env. Semua VITE_* ikut ke JavaScript\n`
    + 'situs yang bisa dibaca siapa pun. PIN Pos/Kas tempatnya di Script Properties Apps Script.\n');
  process.exit(1);
}
console.log('✓ .env lengkap');
