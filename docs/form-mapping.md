# Google Forms as the write endpoint

Five forms, each linked to one ledger tab of **Iuran_ClusterN**
(`docs/sheets-schema.md`). Every entry id is read from `.env` — Google generates
them per Form, so copy the real ones from each Form: ⋮ → **Get pre-filled link**,
fill anything, copy the URL, read the `entry.NNN` keys.

Settings that apply to **every** Form:

- *Settings → Responses → Collect email addresses* → **Do not collect**. It would
  insert an extra column and shift every column the Sheet formulas read.
- *Link to Sheets* → pick the existing spreadsheet, then **rename** the new
  response tab to the tab name below. Never type into or add formulas to a
  response tab (`docs/sheets-schema.md` — ledger tabs hold no formulas).
- Keep questions in the order listed: Forms writes one column per question, in
  question order.
- Number questions: *Response validation → Number → Is number* (and the ranges
  noted below), so a stray letter can't turn a column into text.

## A. Form "Catat Tunai" → `Tunai`

| Question | Type | `.env` key |
| --- | --- | --- |
| Alamat | short answer | `VITE_E_TUNAI_ALAMAT` |
| Rincian | short answer | `VITE_E_TUNAI_RINCIAN` |
| Total | short answer, *Number* | `VITE_E_TUNAI_TOTAL` |
| Petugas | short answer | `VITE_E_TUNAI_PETUGAS` |

Form id: `VITE_FORM_TUNAI`. **No file-upload question and "Require sign in" off**
(*Settings → Responses*) — that's what lets the Pos screen submit it silently in
one tap:

```js
// src/lib/forms.js — same params, /formResponse instead of /viewform
fetch(`https://docs.google.com/forms/d/e/${FORM_ID}/formResponse?${params}`,
      { method: 'POST', mode: 'no-cors' })
```

The response is opaque (no-cors), so the Pos screen keeps each submission as a
local "belum tersinkron" entry until a re-fetch shows its months as paid
(`usePendingSync.js`). One submission = one payment, however many months it
covers.

## B. Form "Konfirmasi Transfer" → `Transfer`

| Question | Type | `.env` key |
| --- | --- | --- |
| Alamat | short answer | `VITE_E_TRANSFER_ALAMAT` |
| Rincian | short answer | `VITE_E_TRANSFER_RINCIAN` |
| Total | short answer, *Number* | `VITE_E_TRANSFER_TOTAL` |
| Bukti transfer | **file upload — must be the last question** | — (Drive; can't be prefilled) |

Form id: `VITE_FORM_TRANSFER`. The file-upload question forces a Google sign-in,
so this Form is always **opened** (never submitted silently): the warga card picks
the months, then opens one prefilled Form for all of them — one transfer, one bukti.

**File-upload settings** (the question's own options): allowed types → **Image**
(add PDF if screenshots-as-PDF are fine), max files → **1**, max size → **10 MB**.
Restricting to Image makes mobile browsers offer the camera straight away.

### Prefill URL (what the app builds)

```
https://docs.google.com/forms/d/e/<FORM_ID>/viewform?usp=pp_url
  &entry.<ALAMAT>=N7-01
  &entry.<RINCIAN>=202512%3D300000%2C202609%3D300000
  &entry.<TOTAL>=600000
```

`Rincian` is `periode=nominal` pairs, comma-separated (`202512=300000,202609=300000`,
`periode = tahun*100 + bulan`). The resident only attaches the bukti and taps
**Kirim**. If they edit the rincian so it no longer adds up to the total, the
Sheet marks it `cek` and doesn't count it (`docs/sheets-schema.md` `Pembayaran`).

## C. Form "Setor ke Bank" → `Setoran`

| Question | Type | `.env` key |
| --- | --- | --- |
| Nominal | short answer, *Number* | `VITE_E_SETOR_NOMINAL` |
| Oleh | short answer — the bendahara's `Petugas` name | `VITE_E_SETOR_OLEH` |

Form id: `VITE_FORM_SETORAN`. The Kas screen's "Setor ke Bank" button opens it
prefilled with the current kas balance and the signed-in bendahara. Submitting is
the whole deposit — `API!kas_tunai`/`rekening` move the amount by formula.
To record a **withdrawal** from the bank into kas, open the Form directly and
enter a negative nominal (e.g. `-500000`) — keep the Number validation at *Is
number*, which accepts negatives.

## D. Form "Pengeluaran" → `Pengeluaran`

`keterangan`, `nominal` (*Number*), `sumber` (multiple choice: kas | bank). No
prefill, no app screen — bendahara opens it directly.

## E. Form "Keputusan" → `Keputusan`

| Question | Type | `.env` key |
| --- | --- | --- |
| Alamat | short answer | `VITE_E_KEP_ALAMAT` |
| Waktu kiriman | short answer — `yyyy-mm-dd hh:mm:ss`, as `Pembayaran!A` shows it | `VITE_E_KEP_WAKTU` |
| Keputusan | multiple choice: `sah` / `tolak` | `VITE_E_KEP_KEPUTUSAN` |
| Oleh | short answer | `VITE_E_KEP_OLEH` |

Form id: `VITE_FORM_KEPUTUSAN`. No file upload, sign-in off (silent submit).
The Kas screen writes it:

- **Verifikasi** (`sah`) / **Tolak** (`tolak`) on each pending transfer — one card
  per submission, bukti shown inline, any month claimed below its tarif flagged.
- **Batalkan** (`tolak`) on a cash entry in *Riwayat Kas Masuk* — for a satpam's
  wrong house/month; the satpam then records it again correctly.

One row decides a whole submission (all its months). The latest decision wins.
If the app is down, submitting this Form directly does the same — copy `waktu`
and `alamat` from the `Pembayaran` tab.
