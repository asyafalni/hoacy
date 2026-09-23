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
 *  treat it as optimistic and confirm on the next fetch (see usePendingSync).
 *  Only works on Forms without a file-upload question (those force sign-in). */
function submit(viewformUrl) {
  return fetch(viewformUrl.replace('/viewform?', '/formResponse?'), { method: 'POST', mode: 'no-cors' });
}

/** One payment covering several months, as the Form's "rincian" answer:
 *  `202607=360000,202609=360000` — periode (tahun*100+bulan) = that month's
 *  tarif. The Sheet splits it back into one Pembayaran row per month and
 *  checks it adds up to `total` (docs/sheets-schema.md `Pembayaran`). */
export const rincianOf = (items) => items.map((i) => `${i.periode}=${i.nominal}`).join(',');
const totalOf = (items) => items.reduce((sum, i) => sum + i.nominal, 0);

/** Form A "Catat Tunai" → Tunai — satpam, silent submit, one per transaction. */
export function urlTunai({ alamat, items, petugas }) {
  return formUrl(E.VITE_FORM_TUNAI, {
    [E.VITE_E_TUNAI_ALAMAT]: alamat, [E.VITE_E_TUNAI_RINCIAN]: rincianOf(items),
    [E.VITE_E_TUNAI_TOTAL]: totalOf(items), [E.VITE_E_TUNAI_PETUGAS]: petugas,
  });
}
export const submitTunai = (rec) => submit(urlTunai(rec));

/** Form B "Konfirmasi Transfer" → Transfer — warga, opened (not silent): its
 *  last question is the bukti upload, which needs a Google sign-in. */
export function urlTransfer({ alamat, items }) {
  return formUrl(E.VITE_FORM_TRANSFER, {
    [E.VITE_E_TRANSFER_ALAMAT]: alamat, [E.VITE_E_TRANSFER_RINCIAN]: rincianOf(items),
    [E.VITE_E_TRANSFER_TOTAL]: totalOf(items),
  });
}

/** Form C "Setor ke Bank" → Setoran — moving cash from kas into the bank. */
export function urlSetoran({ nominal, oleh }) {
  return formUrl(E.VITE_FORM_SETORAN, { [E.VITE_E_SETOR_NOMINAL]: nominal, [E.VITE_E_SETOR_OLEH]: oleh });
}

/** Form E "Keputusan" → Keputusan — bendahara's verdict on one submission
 *  (a Tunai or Transfer response), identified by its alamat + `waktu` (the
 *  Form timestamp as `yyyy-mm-dd hh:mm:ss`, exactly as Pembayaran!A shows it —
 *  the one id a resident can't edit). `sah` verifies a transfer; `tolak`
 *  rejects a transfer or voids a mistaken cash entry. Latest verdict wins. */
export function urlKeputusan({ alamat, waktu, keputusan, oleh }) {
  return formUrl(E.VITE_FORM_KEPUTUSAN, {
    [E.VITE_E_KEP_ALAMAT]: alamat, [E.VITE_E_KEP_WAKTU]: waktu,
    [E.VITE_E_KEP_KEPUTUSAN]: keputusan, [E.VITE_E_KEP_OLEH]: oleh,
  });
}
export const submitKeputusan = (rec) => submit(urlKeputusan(rec));

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

/** `Pembayaran!H` (bukti_url) is a Google Drive **share** link
 *  ("/file/d/<id>/view..." or "open?id=<id>"), not a direct image URL — an
 *  <img> can't render it as-is. Rewrite known Drive share shapes to Drive's
 *  thumbnail endpoint, which does serve the actual bytes; anything that
 *  doesn't match (e.g. the plain image URLs mockData.js uses for local dev)
 *  passes through untouched. */
export function driveImageUrl(url) {
  if (!url) return url;
  const id = url.match(/\/file\/d\/([^/]+)/)?.[1] || url.match(/[?&]id=([^&]+)/)?.[1];
  return id ? `https://drive.google.com/thumbnail?id=${id}&sz=w1000` : url;
}
