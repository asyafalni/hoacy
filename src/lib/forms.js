const E = import.meta.env;

const params = (obj) => {
  const p = new URLSearchParams({ usp: 'pp_url' });
  for (const [k, v] of Object.entries(obj)) if (v !== undefined && v !== '') p.set(k, v);
  return p;
};

/** Prefilled "Catat Pembayaran" URL — one call per month being paid. */
export function urlPembayaran({ noRumah, bulan, tahun = new Date().getFullYear(),
                                 nominal, metode, petugas, catatan }) {
  const p = params({
    [E.VITE_E_RUMAH]: noRumah,
    [E.VITE_E_BULAN]: bulan,
    [E.VITE_E_TAHUN]: tahun,
    [E.VITE_E_NOMINAL]: nominal,
    [E.VITE_E_METODE]: metode,
    [E.VITE_E_PETUGAS]: petugas,
    [E.VITE_E_CATATAN]: catatan,
  });
  return `https://docs.google.com/forms/d/e/${E.VITE_FORM_PEMBAYARAN}/viewform?${p}`;
}

/** Fire-and-forget submit. Opaque response: treat as optimistic, confirm on refetch. */
export function submitPembayaran(rec) {
  const url = urlPembayaran(rec).replace('/viewform?', '/formResponse?');
  return fetch(url, { method: 'POST', mode: 'no-cors' });
}

export function urlSetoran({ batchId, nominal, oleh }) {
  const p = params({ 'entry.2000001': batchId, 'entry.2000002': nominal, 'entry.2000003': oleh });
  return `https://docs.google.com/forms/d/e/${E.VITE_FORM_SETORAN}/viewform?${p}`;
}

export const batchId = () => {
  const d = new Date(), z = (n) => String(n).padStart(2, '0');
  return `SET-${String(d.getFullYear()).slice(2)}${z(d.getMonth() + 1)}${z(d.getDate())}-1`;
};

/** "Verifikasi Transfer" — flips Pembayaran!J pending -> sah for one house/month
 *  via Verifikasi (append-only), never by editing the original row directly. */
export function urlVerifikasi({ alamat, bulan, tahun = new Date().getFullYear(), oleh }) {
  const p = params({
    'entry.3000001': alamat, 'entry.3000002': bulan, 'entry.3000003': tahun, 'entry.3000004': oleh,
  });
  return `https://docs.google.com/forms/d/e/${E.VITE_FORM_VERIFIKASI}/viewform?${p}`;
}

/** Fire-and-forget, same opaque/optimistic pattern as submitPembayaran. */
export function submitVerifikasi(rec) {
  const url = urlVerifikasi(rec).replace('/viewform?', '/formResponse?');
  return fetch(url, { method: 'POST', mode: 'no-cors' });
}

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
 *  (docs/sheets-schema.md §1) so it can go straight into the wa.me path. */
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
