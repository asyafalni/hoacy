# Google Forms as the write endpoint

Three forms, each wired to one tab of **Iuran_BlokN_2026**.

## A. Form "Catat Pembayaran" → `Pembayaran`

| Question | Type | entry id (yours will differ) |
| --- | --- | --- |
| Alamat (cluster+blok-rumah, mis. N7-09) | short text | `entry.1000001` |
| Bulan | short text (1–12) | `entry.1000002` |
| Tahun | short text | `entry.1000003` |
| Nominal | short text | `entry.1000004` |
| Metode | multiple choice: tunai / transfer | `entry.1000005` |
| Petugas | short text | `entry.1000006` |
| Catatan | short text | `entry.1000007` |
| Bukti transfer | **file upload** | — (Drive; no entry id, cannot be prefilled) |

The file-upload question makes the form require a Google sign-in and blocks the
silent `/formResponse` route — so the **warga** flow always opens `viewform`
(one tab per month), while the **satpam** cash flow keeps the silent submit.

Get the real ids: open the form → ⋮ → **Get pre-filled link**, fill anything,
copy the URL, read the `entry.NNN` keys.

### Prefill URL (what every button in the app builds)

```
https://docs.google.com/forms/d/e/<FORM_ID>/viewform?usp=pp_url
  &entry.1000001=N7-03
  &entry.1000002=9
  &entry.1000003=2026
  &entry.1000004=360000
  &entry.1000005=tunai
  &entry.1000006=Pos+Satpam+-+Ujang
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
After submitting, paste the batch id into `Pembayaran!K` for the banked rows.

## C. Form "Pengeluaran" → `Pengeluaran`

`keterangan`, `nominal`, `sumber` (kas | bank). No prefill needed.

## Verifying a transfer

No form — the treasurer ticks `Pembayaran!L` (terverifikasi) in the sheet, or you
add a fourth form keyed by row id. The `Pending → Lunas` transition is pure formula.
