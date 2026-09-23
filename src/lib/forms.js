const E = import.meta.env;

// Every Form's entry ids come from .env (docs/form-mapping.md) — they're
// generated per Form by Google, so the real ones never match anything that
// could be hardcoded here.
function formUrl(formId, fields) {
  const p = new URLSearchParams({ usp: 'pp_url' });
  for (const [k, v] of Object.entries(fields)) if (k !== 'undefined' && v !== undefined && v !== '') p.set(k, v);
  return `https://docs.google.com/forms/d/e/${formId}/viewform?${p}`;
}

/** Fire-and-forget silent submit. The response is opaque (no-cors), so callers
 *  treat it as optimistic and confirm on the next fetch (see usePendingSync). */
function submit(viewformUrl) {
  return fetch(viewformUrl.replace('/viewform?', '/formResponse?'), { method: 'POST', mode: 'no-cors' });
}

/** Form A "Catat Pembayaran" → Pembayaran — one call per month being paid. */
export function urlPembayaran({ noRumah, bulan, tahun = new Date().getFullYear(),
                                 nominal, metode, petugas, catatan }) {
  return formUrl(E.VITE_FORM_PEMBAYARAN, {
    [E.VITE_E_RUMAH]: noRumah, [E.VITE_E_BULAN]: bulan, [E.VITE_E_TAHUN]: tahun,
    [E.VITE_E_NOMINAL]: nominal, [E.VITE_E_METODE]: metode, [E.VITE_E_PETUGAS]: petugas,
    [E.VITE_E_CATATAN]: catatan,
  });
}
export const submitPembayaran = (rec) => submit(urlPembayaran(rec));

/** Form B "Setor ke Bank" → Setoran — moving cash from kas into the bank. */
export function urlSetoran({ nominal, oleh }) {
  return formUrl(E.VITE_FORM_SETORAN, { [E.VITE_E_SETOR_NOMINAL]: nominal, [E.VITE_E_SETOR_OLEH]: oleh });
}

/** Form D "Verifikasi Transfer" → Verifikasi — flips one house/month's transfer
 *  from pending to sah via an append-only row, never by editing Pembayaran. */
export function urlVerifikasi({ alamat, bulan, tahun = new Date().getFullYear(), oleh }) {
  return formUrl(E.VITE_FORM_VERIFIKASI, {
    [E.VITE_E_VERIF_ALAMAT]: alamat, [E.VITE_E_VERIF_BULAN]: bulan,
    [E.VITE_E_VERIF_TAHUN]: tahun, [E.VITE_E_VERIF_OLEH]: oleh,
  });
}
export const submitVerifikasi = (rec) => submit(urlVerifikasi(rec));

/** A resident's own card, deep-linked — used by CetakQR's QR codes and the
 *  "kirim via WhatsApp" button in SemuaKartu.vue. `?alamat` must sit before the
 *  `#` (WargaCard reads it off location.search; with hash routing anything after
 *  `#` is location.hash, invisible to that) — this is the one place that builds
 *  the URL, so that mistake can't happen twice. */
export function urlKartu(alamat) {
  const base = import.meta.env.BASE_URL;
  return `${location.origin}${base}?alamat=${encodeURIComponent(alamat)}#/`;
}

/** wa.me deep link, prefilled with the card link — telp is already 62xxx
 *  (docs/sheets-schema.md `Rumah`) so it can go straight into the wa.me path. */
export function urlWhatsapp({ nama, telp, alamat }) {
  const pesan = `Halo ${nama}, ini link kartu iuran untuk rumah ${alamat}:\n${urlKartu(alamat)}\n\n`
    + `Buka linknya lalu masukkan PIN (default: 3 digit terakhir no. HP Anda) untuk lihat status & bayar iuran.`;
  return `https://wa.me/${telp}?text=${encodeURIComponent(pesan)}`;
}

/** `Pembayaran!I` (bukti_url) is a Google Drive **share** link
 *  ("/file/d/<id>/view..."), not a direct image URL — an <img> can't render
 *  it as-is. Rewrite known Drive share shapes to Drive's thumbnail endpoint,
 *  which does serve the actual bytes; anything that doesn't match (e.g. the
 *  plain image URLs mockData.js uses for local dev) passes through untouched,
 *  so the same <img :src> works in both dev and production. */
export function driveImageUrl(url) {
  if (!url) return url;
  const id = url.match(/\/file\/d\/([^/]+)/)?.[1] || url.match(/[?&]id=([^&]+)/)?.[1];
  return id ? `https://drive.google.com/thumbnail?id=${id}&sz=w1000` : url;
}
