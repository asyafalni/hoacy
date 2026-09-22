# Google Forms as the write endpoint

Four forms, each wired to one tab of **Iuran_BlokN_2026**.

## A. Form "Catat Pembayaran" → `Pembayaran`

| Question | Type | entry id (yours will differ) |
| --- | --- | --- |
| Alamat (cluster+blok-rumah, mis. N7-09) | short text | `entry.1000001` |
| Bulan | short text (1–12) | `entry.1000002` |
| Tahun | short text | `entry.1000003` |
| Nominal | short text | `entry.1000004` |
| Metode | multiple choice: tunai / transfer | `entry.1000005` |
| Petugas | short text — a name from `Petugas!A` (peran `satpam`), or `Warga` | `entry.1000006` |
| Catatan | short text | `entry.1000007` |
| Bukti transfer | **file upload** | — (Drive; no entry id, cannot be prefilled) |

The file-upload question makes the form require a Google sign-in and blocks the
silent `/formResponse` route — so the **warga** flow always opens `viewform`
(one tab per month), while the **satpam** cash flow keeps the silent submit.

Get the real ids: open the form → ⋮ → **Get pre-filled link**, fill anything,
copy the URL, read the `entry.NNN` keys.

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

`batch_id` prefilled as `SET-260921-1`, `nominal` prefilled with the current
Kas Tunai balance, `oleh` prefilled with the treasurer's name.
After submitting, paste the batch id into `Pembayaran!L` for the banked rows.

## C. Form "Pengeluaran" → `Pengeluaran`

`keterangan`, `nominal`, `sumber` (kas | bank). No prefill needed.

## D. Form "Verifikasi Transfer" → `Verifikasi`

| Question | Type | entry id (yours will differ) |
| --- | --- | --- |
| Alamat | short text | `entry.3000001` |
| Bulan | short text (1–12) | `entry.3000002` |
| Tahun | short text | `entry.3000003` |
| Oleh | short text | `entry.3000004` |

The Kas screen's "Perlu diverifikasi" list comes from a direct read of the
`Pembayaran` tab (not the `API` tab — `usePembayaranLedger.js`), filtered to
`metode="transfer"` and `keabsahan="pending"`. Reading `Pembayaran` directly
(rather than adding yet more columns to `API`) is also how bendahara gets to see
`I` (bukti_url) per pending row — a "Lihat bukti" link opens the Drive photo
before they decide to verify. Tapping **Verifikasi** fires this form silently
(same `/formResponse` no-cors pattern as Form A), prefilled with that house/month
and `oleh` = whichever `Petugas` roster name (peran `bendahara`) they picked at
the "Siapa Anda?" screen (see `Bendahara.vue`). `Pembayaran!K` picks the new row
up via `COUNTIFS(Verifikasi!...)` — §4/§7 of `docs/sheets-schema.md`.

## Verifying a transfer

Two equivalent paths, either one flips `Pembayaran!K` from `pending` to `sah`:
the treasurer ticks `Pembayaran!M` directly in the sheet, or taps **Verifikasi**
in the Kas app screen (Form D above). Both are just inputs to the same formula —
neither is more "official" than the other.
