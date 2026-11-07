// Links the app hands out: a house's own card (QR codes, WhatsApp) and the
// pengurus' operational WhatsApp number.

/** A resident's own card, deep-linked — used by CetakQR's QR codes and the
 *  "kirim via WhatsApp" button in SemuaKartu.vue. `?alamat` must sit before the
 *  `#` (WargaCard reads it off location.search; with hash routing anything after
 *  `#` is location.hash, invisible to that) — this is the one place that builds
 *  the URL, so that mistake can't happen twice. */
export function urlKartu(alamat) {
  const base = import.meta.env.BASE_URL;
  return `${location.origin}${base}?alamat=${encodeURIComponent(alamat)}#/`;
}

/** wa.me deep link to a resident, prefilled with their card link — telp is
 *  already 62xxx so it goes straight into the wa.me path. */
export function urlWhatsapp({ nama, telp, alamat }) {
  const pesan = `Halo ${nama}, ini link kartu iuran untuk rumah ${alamat}:\n${urlKartu(alamat)}\n\n`
    + 'Buka linknya lalu masukkan PIN (bawaan: 6 digit terakhir no. HP Anda) untuk lihat status & bayar iuran. '
    + 'Setelah masuk, ganti PIN Anda dari kartu.';
  return `https://wa.me/${telp}?text=${encodeURIComponent(pesan)}`;
}

/** wa.me link to the pengurus' operational number (VITE_WA_OPERASIONAL, 62xxx),
 *  prefilled with who is writing and about what — the "ada kendala?" contact
 *  under the transfer dialog. */
export function urlWaOperasional(pesan) {
  return `https://wa.me/${import.meta.env.VITE_WA_OPERASIONAL}?text=${encodeURIComponent(pesan)}`;
}
