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
