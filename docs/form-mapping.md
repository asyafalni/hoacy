# Google Forms as the write endpoint

Four forms, each linked to one ledger tab of **Iuran_ClusterN**
(`docs/sheets-schema.md`). Every entry id is read from `.env` — Google generates
them per Form, so copy the real ones from each Form: ⋮ → **Get pre-filled link**,
fill anything, copy the URL, read the `entry.NNN` keys.

## A. Form "Catat Pembayaran" → `Pembayaran`

| Question | Type | `.env` key |
| --- | --- | --- |
| Alamat (mis. N7-09) | short text | `VITE_E_RUMAH` |
| Bulan | short text (1–12) | `VITE_E_BULAN` |
| Tahun | short text | `VITE_E_TAHUN` |
| Nominal | short text | `VITE_E_NOMINAL` |
| Metode | multiple choice: tunai / transfer | `VITE_E_METODE` |
| Petugas | short text — a `Petugas` name (peran `satpam`), or `Warga` | `VITE_E_PETUGAS` |
| Catatan | short text, optional | `VITE_E_CATATAN` |
| Bukti transfer | **file upload — must be the last question** | — (Drive; can't be prefilled) |

Form id: `VITE_FORM_PEMBAYARAN`.

The file-upload question makes the form require a Google sign-in and blocks the
silent `/formResponse` route — so the **warga** flow always opens `viewform`
(one tab per month), while the **satpam** cash flow keeps the silent submit.

**File-upload question settings** (question ⋮ → the file-upload block itself):
allowed file types → check **Image** only (uncheck the rest, or add PDF if you
also want screenshots-as-PDF), max number of files → **1**, max file size →
**10 MB** is plenty for a phone photo. Restricting the type to Image is what
makes mobile browsers default straight to the camera when the app's "Ambil
foto" button opens this input — Forms' file-upload question is just an
`<input type=file>` under the hood, same as `WargaCard.vue`'s own picker.

### Prefill URL (what every button in the app builds)

```
https://docs.google.com/forms/d/e/<FORM_ID>/viewform?usp=pp_url
  &entry.1000001=N7-03
  &entry.1000002=9
  &entry.1000003=2026
  &entry.1000004=360000
  &entry.1000005=tunai
  &entry.1000006=Ujang
```

The satpam only reviews and taps **Kirim** — nothing to type. One request per month
being paid (loop the months; they are separate rows so partial payments stay
auditable).

### Optional: silent submit (no form UI)

```js
// same params, /formResponse instead of /viewform — fire-and-forget
fetch(\`https://docs.google.com/forms/d/e/\${FORM_ID}/formResponse?\${params}\`,
      { method: 'POST', mode: 'no-cors' })
```

Response is opaque (no-cors), so you cannot read success — show an optimistic row
and confirm on the next fetch. Keep the `viewform` route as the fallback when the
network is bad; the satpam then has a visible receipt screen.

## B. Form "Setor ke Bank" → `Setoran`

| Question | Type | `.env` key |
| --- | --- | --- |
| Nominal | short text | `VITE_E_SETOR_NOMINAL` |
| Oleh | short text — the bendahara's `Petugas` name | `VITE_E_SETOR_OLEH` |

Form id: `VITE_FORM_SETORAN`. The Kas screen's "Setor ke Bank" button opens it
prefilled with the current kas balance and the signed-in bendahara. Submitting is
the whole deposit — `API!kas_tunai`/`rekening` move the amount by formula.

## C. Form "Pengeluaran" → `Pengeluaran`

`keterangan`, `nominal`, `sumber` (kas | bank). No prefill, no app screen —
bendahara opens it directly.

## D. Form "Verifikasi Transfer" → `Verifikasi`

| Question | Type | `.env` key |
| --- | --- | --- |
| Alamat | short text | `VITE_E_VERIF_ALAMAT` |
| Bulan | short text (1–12) | `VITE_E_VERIF_BULAN` |
| Tahun | short text | `VITE_E_VERIF_TAHUN` |
| Oleh | short text | `VITE_E_VERIF_OLEH` |

Form id: `VITE_FORM_VERIFIKASI`. The Kas screen lists pending transfers (read
straight from `Pembayaran` via `usePembayaranLedger.js`), shows each bukti photo
inline ("Lihat bukti"), and "Verifikasi" submits this form silently with that
house/month and the signed-in bendahara. That new row is the **only** way a transfer
becomes `sah` (`Pembayaran!J`); if the app is down, submitting this Form directly
does the same.
