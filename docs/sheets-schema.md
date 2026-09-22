# Google Sheet schema & formulas

Spreadsheet name: **Iuran_ClusterN_2026**. Thirteen tabs. Row 1 is always the header.
Formulas are written for row 2 — fill down, or wrap in `ARRAYFORMULA` where noted.

---

## 1. `Rumah` — master data (manual, ~14 rows)

An address has **three parts** — cluster code + block number + house number — and the
joined form `N7-09` is the primary key everything else references. Store the parts
separately so you can group, sort and filter by block; derive the key with a formula.

| Col | Header | Type | Notes |
| --- | --- | --- | --- |
| A | alamat | **formula** | `N7-09` — primary key, referenced by every other tab |
| B | cluster | text | `N` (Cypress) |
| C | blok | text | `7` or `Blvd` — not every block is numeric, so keep it text |
| D | rumah | text | `09` — keep the leading zero (format the column as plain text) |
| E | nama | text | kepala keluarga |
| F | telp | text | 62xxx, for the WhatsApp reminder link |
| G | luas_m2 | **formula** | current land area — see below, not typed directly any more |
| H | tipe | **formula** | current `rumah` or `kavling` — same story as `luas_m2` |
| I | islk | **formula** | |
| J | tarif_bulanan | **formula** | |
| K | pin | **formula**, editable | Warga card PIN — defaults to phone's last 3 digits; admin overwrites the cell to set a custom one |
| L | aktif | boolean, manual | `TRUE` unless the house has been decommissioned — see §12 |

`luas_m2`/`tipe` (G/H) and the RT-wide rate card that turns them into a rupiah amount
(I/J) both change over time — an empty lot gets built on, an owner buys the lot next
door and merges it into one bigger house, the RT eventually raises the flat fee. None
of that is allowed to rewrite what a *past* month's tagihan was, so `G`/`H`/`I`/`J`
are no longer typed values — they're formulas that ask "what's effective **right
now**" against two append-only history tabs, `RumahRiwayat` (§12, luas/tipe per
house) and `TarifVersi` (§13, the RT-wide rate card). Editing a house's physical
spec or the RT's rates means **adding a row** to one of those tabs, never editing
`Rumah!G/H` or a rate directly — see §12/§13 for the exact mechanics and why.

```
A2: =IF($B2="","", $B2 & $C2 & "-" & TEXT($D2,"00"))

G2: =LET(target, YEAR(TODAY())*100+MONTH(TODAY()),
      key, MAXIFS(RumahRiwayat!$F:$F, RumahRiwayat!$A:$A,$A2, RumahRiwayat!$F:$F,"<="&target),
      IFERROR(INDEX(FILTER(RumahRiwayat!$D:$D, RumahRiwayat!$A:$A=$A2, RumahRiwayat!$F:$F=key),1), ""))

H2: =LET(target, YEAR(TODAY())*100+MONTH(TODAY()),
      key, MAXIFS(RumahRiwayat!$F:$F, RumahRiwayat!$A:$A,$A2, RumahRiwayat!$F:$F,"<="&target),
      IFERROR(INDEX(FILTER(RumahRiwayat!$E:$E, RumahRiwayat!$A:$A=$A2, RumahRiwayat!$F:$F=key),1), ""))

I2: =LET(target, YEAR(TODAY())*100+MONTH(TODAY()),
      key, MAXIFS(TarifVersi!$N:$N, TarifVersi!$N:$N,"<="&target),
      row, FILTER(TarifVersi!$A:$N, TarifVersi!$N:$N=key),
      IF($H2="kavling", INDEX(row,1,12)*$G2,
         IFS($G2<INDEX(row,1,3), INDEX(row,1,4),
             $G2<INDEX(row,1,5), INDEX(row,1,6),
             $G2<INDEX(row,1,7), INDEX(row,1,8),
             $G2<INDEX(row,1,9), INDEX(row,1,10),
             TRUE, INDEX(row,1,11))))

J2: =LET(target, YEAR(TODAY())*100+MONTH(TODAY()),
      key, MAXIFS(TarifVersi!$N:$N, TarifVersi!$N:$N,"<="&target),
      row, FILTER(TarifVersi!$A:$N, TarifVersi!$N:$N=key),
      $I2 + INDEX(row,1,13))

K2: =RIGHT($F2,3)
L2: TRUE   (typed, not a formula)
```

`K` (pin) starts as a formula but is meant to be overwritten: typing a literal value
into that cell (e.g. after a resident asks for a reset) replaces the formula for that
row only, same as `disetor_batch`/`terverifikasi` on `Pembayaran`. It's a deterrent
PIN, not real access control — see the comment above `pendingHouse` in `WargaCard.vue`.

`L` (aktif) is how a decommissioned house (§12 — merged away, address gone) stops
being findable/payable through the app, without deleting anything: `useSheet.js`'s
`rumah` computed filters `aktif !== FALSE`, so once it's flipped, the address drops
out of the Warga login, Pos, and Semua Kartu Rumah lookups, and out of `API!target_bulan_ini`/
`jumlah_rumah` (§10) — but every `Pembayaran`/`RumahRiwayat` row for it stays exactly
where it was, forever, for audit. Flip it only after the house's tunggakan is 0 — the
Sheet can only warn about this (§12), not block the edit; the treasurer is who
actually enforces it.

Tariff table `TarifVersi`'s day-one row encodes (ISLK, per month, by luas tanah):
`<120 → 225.000` · `120–149 → 250.000` · `150–259 → 310.000` ·
`260–399 → 375.000` · `400–499 → 400.000` · kavling kosong → `400 × m²`.
Every house also pays Iuran RT 50.000 — see §13 for how either number changes later.

---

## 2. `Blok` — one row per block, not per house (manual, 10 rows)

| Col | Header | Notes |
| --- | --- | --- |
| A | blok | `Blvd`, `1`, `2`, `3`, `5`, `6`, `7`, `8`, `9`, `10` |
| B | warna | hex, e.g. `#2a78d6` |

```
A2: Blvd   B2: #2a78d6
A3: 1      B3: #9C4A1A
A4: 2      B4: #eb6834
A5: 3      B5: #1baf7a
A6: 5      B6: #eda100
A7: 6      B7: #e87ba4
A8: 7      B8: #008300
A9: 8      B9: #4a3aa7
A10: 9     B10: #e34948
A11: 10    B11: #0F86A3
```

Plain typed values, no formula — each block gets its own color (the Warga card
header, the Pos house-number badge). Admin edits `B` to recolor a block; no deploy
needed. Leave a row blank and the app falls back to its built-in default for that
block (`BLOK_WARNA_DEFAULT` in `src/lib/tariff.js`) — the hex values above are
exactly that default, seeded here so the Sheet and the app agree on day one.

---

## 3. `Petugas` — everyone who's allowed into Pos or Kas (manual, a handful of rows)

| Col | Header | Notes |
| --- | --- | --- |
| A | nama | e.g. `Ujang` |
| B | peran | `satpam` or `bendahara` |

```
A2: Ujang       B2: satpam
A3: Dedi        B3: satpam
A4: Rahmat      B4: satpam
A5: Ibu Siti    B5: bendahara
A6: Pak Joko    B6: bendahara
```

The Pos PIN (`VITE_PIN_POS`) and Kas PIN (`VITE_PIN_KAS`) are each shared by
everyone with that `peran` — they only gate the *screen*. Once in, the app makes
that person pick their own name from the matching rows here before they can do
anything (remembered per device after that), so `Pembayaran!petugas` and
`Verifikasi!oleh` say who actually acted, not a hardcoded name or free text.
Admin adds or removes a row here to add/remove someone; no deploy needed. Satpam
and bendahara never touch this tab or any other — they only ever see the app.

---

## 4. `Pembayaran` — Form responses (append-only ledger)

A–I come from the Form. J onward are formulas or treasurer input.

| Col | Header | Source |
| --- | --- | --- |
| A | Timestamp | Form |
| B | alamat | Form (prefilled, e.g. `N7-09`) |
| C | bulan | Form, 1–12 |
| D | tahun | Form, prefilled 2026 |
| E | nominal | Form (prefilled with the outstanding amount) |
| F | metode | Form: `tunai` / `transfer` |
| G | petugas | Form: a name from `Petugas!A` (satpam), or `Warga` |
| H | catatan | Form, optional |
| I | bukti_url | Form — the file-upload question's Drive link, blank for `tunai` rows |
| J | tarif | formula |
| K | keabsahan | formula — `sah` / `pending` |
| L | disetor_batch | **treasurer**, blank = still Kas Tunai |
| M | terverifikasi | **treasurer** checkbox (transfers only) — manual fallback, see below |
| N | lokasi_uang | formula — `kas` / `bank` |

`I` isn't a formula you add — it's simply where the Form's file-upload question
lands once the Form is linked to this sheet (Google appends a column per
question, in question order, right after `H`). Miss this and whoever wires up
the Form later finds the bukti link colliding with whatever you put in `I`
instead — put the `tarif` formula in `J`, one column later than you'd guess from
just reading the form fields.

```
J2: =IFERROR(VLOOKUP($B2, Rumah!$A:$L, 10, FALSE), "")   // B = alamat, e.g. N7-09 —
    // today's tarif, just for a quick reference on the row; not what decides
    // keabsahan/status for this specific month (Status!AE:AP, §5, is authoritative)
K2: =IF($F2="transfer",
       IF(OR($M2=TRUE,
             COUNTIFS(Verifikasi!$B:$B,$B2, Verifikasi!$C:$C,$C2, Verifikasi!$D:$D,$D2)>0),
          "sah", "pending"),
       "sah")
N2: =IF($F2="transfer","bank", IF($L2="","kas","bank"))
```

Protect A:I (form range) and leave L:M editable by the treasurer only.

`K2` accepts **either** path to "sah": the treasurer ticking `M` directly in the sheet
(always available, zero setup), or a matching row in the `Verifikasi` tab (§7 —
what the Kas app screen actually does when bendahara taps "Verifikasi"). Neither
is authoritative over the other; whichever happens first wins.

---

## 5. `Status` — the digital card, one row per house

A2:A15 `=Rumah!A2:A15` (the `alamat` keys, `N7-01` … `N8-07`). B1:M1 = 1..12 (month numbers).
Three blocks side by side: **amount received**, **status text**, and (new) **tarif
efektif per bulan**.

`$A$1` holds the active year — put the formula `=YEAR(TODAY())` in it, **not** a typed
number. That's what makes the whole card grid roll over to the new year automatically
every 1 January with zero manual sheet edits; every formula below just reads `$A$1`.

```
Amount received (B2, fill right + down):
=IFERROR(SUM(UNIQUE(FILTER(Pembayaran!$E:$E,
      Pembayaran!$B:$B=$A2, Pembayaran!$C:$C=B$1,
      Pembayaran!$D:$D=$A$1,             // $A$1 = YEAR(TODAY()), see above
      Pembayaran!$K:$K="sah"))), 0)

Tarif efektif per bulan (AE2, fill right through AP2 + down — a house's luas/tipe
and the RT-wide rate card can each change mid-year, so this can no longer be one
VLOOKUP per house; it has to be looked up separately per month, against RumahRiwayat
(§12) and TarifVersi (§13) as of THAT month, not "now"):
=LET(
  target,   $A$1*100 + (COLUMN()-COLUMN($AE2)+1),   // this column's month number
  luasKey,  MAXIFS(RumahRiwayat!$F:$F, RumahRiwayat!$A:$A,$A2, RumahRiwayat!$F:$F,"<="&target),
  luasRow,  FILTER(RumahRiwayat!$A:$E, RumahRiwayat!$A:$A=$A2, RumahRiwayat!$F:$F=luasKey),
  luas,     INDEX(luasRow,1,4),
  tipe,     INDEX(luasRow,1,5),
  tarifKey, MAXIFS(TarifVersi!$N:$N, TarifVersi!$N:$N,"<="&target),
  tRow,     FILTER(TarifVersi!$A:$N, TarifVersi!$N:$N=tarifKey),
  IF(tipe="kavling", INDEX(tRow,1,12)*luas,
     IFS(luas<INDEX(tRow,1,3), INDEX(tRow,1,4),
         luas<INDEX(tRow,1,5), INDEX(tRow,1,6),
         luas<INDEX(tRow,1,7), INDEX(tRow,1,8),
         luas<INDEX(tRow,1,9), INDEX(tRow,1,10),
         TRUE, INDEX(tRow,1,11))) + INDEX(tRow,1,13))

Status text (P2, fill right + down — P1:AA1 also 1..12, one-to-one with AE1:AP1):
=LET(
  tarif,  INDEX($AE2:$AP2, 1, P$1),
  bayar,  INDEX($B2:$M2, 1, P$1),
  pend,   IFERROR(SUM(UNIQUE(FILTER(Pembayaran!$E:$E, Pembayaran!$B:$B=$A2,
                 Pembayaran!$C:$C=P$1, Pembayaran!$K:$K="pending"))), 0),
  IFS( bayar>=tarif, "Lunas",
       bayar>0,      "Sebagian",
       pend>0,       "Pending",
       P$1>MONTH(TODAY()), "-",
       TRUE,         "Belum" ))

Outstanding per house (AC2):
=SUMPRODUCT( (COLUMN($B$1:$M$1)-1 <= MONTH(TODAY())) * MAX(0, $AE2:$AP2 - $B2:$M2) )
```

Why the tarif lookup got pulled into its own `AE:AP` block instead of living inline
in `P2`/`AC2` (which is what a straight port of the old single-VLOOKUP formula would
look like): both `P2:AA2` (12 cells) and `AC2` need "this house's tarif in month N",
and repeating that whole `MAXIFS`+`FILTER` chain 13 times per house instead of once
would be a lot of duplicated formula to keep in sync by hand. Computing it once per
month in `AE:AP` and having `P`/`AC` just read from it keeps the one genuinely
complex piece of logic in one place.

**Why `UNIQUE(FILTER(...))` instead of a plain `SUMIFS`:** a satpam who double-taps "Catat & Kirim" (bad signal at the pos, unsure if the first tap registered) produces two `Pembayaran` rows with the *same* alamat, month, year, status and nominal — a true accidental duplicate. `SUMIFS` would sum both and overstate what was received; collapsing to the set of *distinct* nominal values first makes the duplicate count once. The client also guards against the double-tap itself (`PosSatpam.vue` disables the button while a submission is in flight) — this formula is the second line of defense for whatever gets through anyway (a genuine retry after a real failure, two taps a few seconds apart, etc).

**Accepted tradeoff:** this can't distinguish "the same row twice" from "two genuinely different payments that happen to be for the same exact amount in the same month" — those collapse to one too. That's rare enough (and the wrong-direction way, undercounting rather than a resident being shorted) to accept rather than build real row-level dedup (e.g., a submission nonce column) for.

---

## 6. `Setoran` — cash → bank batches (Form responses)

| Col | Header |
| --- | --- |
| A | Timestamp |
| B | batch_id (prefilled: `SET-yymmdd-1`) |
| C | nominal |
| D | oleh |

Depositing = writing the `batch_id` into the `disetor_batch` column of the rows
being banked. Do it with one `Pembayaran!L` fill, or with the helper formula:

```
Suggested rows to bank (paste in Setoran!F2):
=TEXTJOIN(", ", TRUE, FILTER(Pembayaran!$B:$B, Pembayaran!$N:$N="kas"))
```

---

## 7. `Verifikasi` — transfer sign-off (Form responses, append-only)

| Col | Header | Source |
| --- | --- | --- |
| A | Timestamp | Form |
| B | alamat | Form (prefilled from the Kas app's pending-transfer list) |
| C | bulan | Form, 1–12 |
| D | tahun | Form |
| E | oleh | Form — a name from `Petugas!A` where `peran="bendahara"` |

One row per transfer bendahara has checked against the uploaded bukti (`Pembayaran!I`,
the file-upload question's Drive link — the Kas app screen surfaces it as a "Lihat
bukti" link right next to the Verifikasi button, no need to go digging in Drive) and
confirmed. No update to any existing row — this tab exists *because* Forms can only
append, never edit a cell, so "mark this transfer verified" has to be its own ledger
entry rather than flipping `Pembayaran!M`, matching how every other write in this
project works. `Pembayaran!K` reads it (§4). Bendahara can still tick `Pembayaran!M`
by hand instead if they're already in the sheet — both paths lead to "sah".

---

## 8. `Pengeluaran` — spending (Form responses)

`A Timestamp · B keterangan · C nominal · D sumber (kas|bank)`

---

## 9. `Opex` — the cluster's fixed minimum monthly cost, itemized (manual)

| Col | Header | Notes |
| --- | --- | --- |
| A | kategori | text, e.g. `Gaji Satpam` |
| B | ikon | one emoji, e.g. `🛡️` |
| C | nominal | number |
| D | diperbarui | date — bendahara updates this by hand whenever `C` changes |

```
A2: Gaji Satpam        B2: 🛡️   C2: 2400000   D2: 2026-08-01
A3: Kebersihan         B3: 🧹   C3: 300000    D3: 2026-08-01
A4: Listrik & Air Pos  B4: 💡   C4: 150000    D4: 2026-09-05
A5: Lain-lain          B5: 📋   C5: 100000    D5: 2026-08-01
```

Plain typed rows, no formula, same "admin opens the Sheet and edits it directly"
pattern as `Blok`/`Petugas` — add, remove or retotal a line and `API!opex_bulanan`
(§10) picks it up on the next fetch, no deploy, no Form. `Gaji Satpam` is
deliberately the *total* payroll for every satpam combined, not one row per
person — this tab feeds the "Rincian OPEX" bottom sheet on the public `/sum`
page (§10, `RingkasanPublik.vue` — a sheet on the same page, not a separate
route), and a named amount per individual satpam would be exactly the kind of
identifiable personal data that page is built to avoid. `ikon` is a single emoji
bendahara picks from their own keyboard — no icon-name mapping to maintain in
code, it just renders as typed.

`diperbarui` has no formula behind it on purpose — there's no Apps Script trigger
in this project to auto-stamp an edit, so it's a manual "update the date when you
touch the number" habit, same trust level as everything else on this tab. The most
recent of these dates across all rows is surfaced as `API!opex_diperbarui` (§10) —
"OPEX terakhir diperbarui: ..." on the public dashboard, so residents can see at a
glance whether the minimum-cost baseline is stale.

---

## 10. `API` — per-house data + global numbers (one of seven tabs the app reads,
      alongside `Blok`, `Petugas`, `Opex`, `Riwayat`, `RumahRiwayat` and `TarifVersi`)

One flat table the front end parses; keep column order stable.

```
A1: "key"      B1: "value"
A2: "kas_tunai"
B2: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$N:$N,"kas", Pembayaran!$K:$K,"sah")
    -SUMIFS(Pengeluaran!$C:$C, Pengeluaran!$D:$D,"kas")

A3: "rekening"
B3: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$N:$N,"bank", Pembayaran!$K:$K,"sah")
    -SUMIFS(Pengeluaran!$C:$C, Pengeluaran!$D:$D,"bank")

A4: "tunggakan_total"   B4: =SUM(Status!$AC:$AC)
A5: "lunas_bulan_ini"   B5: =COUNTIF(INDEX(Status!$P:$AA,0,MONTH(TODAY())), "Lunas")
A6: "jumlah_rumah"      B6: =COUNTIFS(Rumah!$A$2:$A$1000,"<>", Rumah!$L$2:$L$1000,TRUE)
A7: "updated"           B7: =TEXT(NOW(), "yyyy-mm-dd hh:mm")
A8: "tahun_aktif"       B8: =YEAR(TODAY())

A9: "terkumpul_bulan_ini"
B9: =SUM(INDEX(Status!$B$2:$M$1000, 0, MONTH(TODAY())))

A10: "target_bulan_ini"
B10: =SUMIFS(Rumah!$J$2:$J$1000, Rumah!$L$2:$L$1000, TRUE)

A11: "opex_bulanan"
B11: =SUM(Opex!$C$2:$C$1000)

A12: "opex_diperbarui"
B12: =IFERROR(TEXT(MAX(Opex!$D$2:$D$1000), "dd mmm yyyy"), "")
```

`tahun_aktif` is what the app shows as the card's year and uses as the base for
"bayar di muka" — read it instead of assuming any particular year.

`opex_bulanan` sums `Opex` (§9) rather than being typed here directly — bendahara
edits line items over there (add a category, fix a number) and this total, and
everything downstream of it, updates on the next fetch automatically. It exists
purely so the public dashboard (`RingkasanPublik.vue`) can show a real
surplus/deficit signal (`terkumpul_bulan_ini - opex_bulanan`), not just "did
everyone pay their tariff" — tariff income covering the *tariff target* doesn't
tell anyone whether the tariff itself is still enough to cover what actually gets
spent. It reads `0` (and the dashboard shows no surplus/deficit line) until
`Opex` has at least one row.

Then, starting at D1, a per-house block the app renders directly:

```
D1: "alamat" E1:"nama" F1:"telp" G1:"luas" H1:"tarif" I1:"tunggakan"
J1..U1: 1..12 status text   V1:"cluster" W1:"blok" X1:"rumah"
Y1: "muka_tahun_depan"   Z1:"pin"   AA1:"aktif"
D2: =Rumah!A2   E2: =Rumah!E2  F2: =Rumah!F2  G2: =Rumah!G2
H2: =Rumah!J2   I2: =Status!AC2
V2: =Rumah!B2   W2: =Rumah!C2   X2: =Rumah!D2
J2: =Status!P2  … U2: =Status!AA2
Y2: =TEXTJOIN(",", TRUE, SORT(UNIQUE(FILTER(Pembayaran!$C:$C,
      Pembayaran!$B:$B=D2, Pembayaran!$D:$D=$B$8+1))))
Z2: =Rumah!K2
AA2: =Rumah!L2
```

No `tipe` (Rumah!H) here on purpose — it only feeds Rumah's own tariff formula
(§1, `I2`), already baked into `tarif` by the time it reaches this tab, and the
app never reads a house's `tipe` directly. Dropped in a schema-cleanup pass
(audited via grep across `src/`) rather than carried along unused.

`Z`/pin rides along in the same public, published-to-web feed as everything else
here (see `docs/deploy.md`) — the Warga card checks it client-side, so treat it the
same as the Pos/Kas PIN: a deterrent against a neighbour browsing the app UI, not a
secret that survives someone reading the raw gviz JSON directly.

`Y` is a comma-joined list of month numbers (1–12) that already have a `Pembayaran`
row for **next year** (`$B$8+1` — sah or pending, doesn't matter, a row existing is
enough to avoid double-charging). There's no next-year Status grid — paying ahead
into next year only needs "which months are already spoken for," not a full second
12-month card. The app subtracts this list from 1..12 to build the next-year picker.

Publish the spreadsheet to the web, then the public-facing composable (`useSheet.js`,
used by `/`, `/pos`, `/kas`, `/sum`) reads all seven:

```
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=API
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=Blok
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=Petugas
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=Opex
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=Riwayat
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=RumahRiwayat
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=TarifVersi
```

`RumahRiwayat`/`TarifVersi` are safe on the public page for the same reason the rest
of this list is: neither carries anything more sensitive than what `API`'s own
per-house block already exposes there (§10 above) — `RumahRiwayat` is just
`alamat, luas, tipe` over time (no name, no phone, no payment amount), and
`TarifVersi` has no per-house data at all, just the RT-wide rate card. `RingkasanPublik.vue`
uses them (via `tarifPada()`, `src/lib/tarifHistoris.js`) to get period-correct
tarif for its aging/dibayar-di-muka math instead of assuming today's rate applied
to every month — the one thing that actually stays off that page is `Pembayaran`,
per the note below.

`Pembayaran` itself is an eighth published tab (gviz doesn't support per-tab access
control — see `docs/deploy.md`), but the app only ever fetches it from
`usePembayaranLedger.js`, used by two PIN-gated screens that legitimately need
row-level detail: `Bendahara.vue` (Kas) shows pending transfers with `bukti_url`,
and `WargaCard.vue` uses it to recompute a resident's own card for **years before
`tahun_aktif`** — `Status` only ever holds the current year (§5), so a past year's
12-month grid is reconstructed client-side from the raw ledger, filtered to that
one house's `alamat`, the same `bayar >= tarif` logic as `Status!P2` (tarif itself
also period-correct now, via `tarifPada()` against `RumahRiwayat`/`TarifVersi` — see
§12/§13 — not a flat "today's rate" assumption), just parameterized by year instead
of reading `$A$1`. **`/sum` (no PIN, open to
anyone) must never call `usePembayaranLedger` or fetch `Pembayaran` directly** —
that tab carries `alamat` and a Drive link to each resident's transfer-proof
photo, exactly the kind of per-house, per-payment data the PDP note at the top of
`RingkasanPublik.vue` is built to keep off that page. `Riwayat` (§11) exists
specifically so the public "terkumpul vs target" history can show a month-by-month
trend without the public page ever touching the raw ledger — see that section for
why.

Sheet-side caching is ~1–5 minutes. After a write the app optimistically shows the
new row locally and re-fetches; see `src/composables/useSheet.js`.

---

## 11. `Riwayat` — monthly terkumpul history, pre-aggregated (formula)

| Col | Header | Notes |
| --- | --- | --- |
| A | tahun | **formula** |
| B | bulan | **formula**, 1–12 |
| C | terkumpul | **formula** — `sah` `Pembayaran` for that (tahun, bulan) |

Row 2 is the current month; each row below it steps one month further back, so the
whole tab reads newest-to-oldest and rolls forward automatically — same spirit as
`Status!$A$1`'s year rollover, just monthly instead of yearly:

```
A2: =YEAR(TODAY())              B2: =MONTH(TODAY())
C2: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$C:$C,B2, Pembayaran!$D:$D,A2, Pembayaran!$K:$K,"sah")

A3: =IF(B2=1, A2-1, A2)         B3: =IF(B2=1, 12, B2-1)
C3: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$C:$C,B3, Pembayaran!$D:$D,A3, Pembayaran!$K:$K,"sah")
```

Fill A/B/C down as many rows as months of history the public dashboard should be
able to show — 36 rows (3 years) is a reasonable starting depth; extending it later
is just filling more rows, never a breaking change. `RingkasanPublik.vue`'s "Riwayat
Terkumpul vs Target" bottom sheet (opened by tapping the "Terkumpul bulan ini" card)
reads this tab, groups it by the requested duration (3/6/12 bulan or semua = every
row present), and pairs each month's `terkumpul` against the *current*
`API!target_bulanan` as a constant reference line — this tab has no `target` column
of its own because the Sheet doesn't keep a historical tariff snapshot per month;
see the caveat text already on that bottom sheet.

This is the only reason `Riwayat` exists: a plain `SUMIFS` per month, published
alongside `API`/`Blok`/`Petugas`/`Opex`, so the public page's fetch list never has to
include the raw `Pembayaran` ledger to show a trend line.

---

## 12. `RumahRiwayat` — per-house luas/tipe history (manual, append-only)

| Col | Header | Type | Notes |
| --- | --- | --- | --- |
| A | alamat | text | matches `Rumah!A` |
| B | tahun_berlaku | number | year this luas/tipe took effect |
| C | bulan_berlaku | number | 1–12, month it took effect |
| D | luas | number | m² effective from that (tahun,bulan) onward |
| E | tipe | text | `rumah` or `kavling`, same |
| F | periode | **formula** | `=B2*100+C2` — a sortable key, `202603` for March 2026 |

```
F2: =B2*100+C2
```

Append-only, never edited: a house's physical spec changing (an empty `kavling`
gets built on, an owner buys the lot next door and it becomes one bigger house)
means **adding a row**, not touching an old one. `Rumah!G/H` (§1) always resolve
to whichever row has the largest `periode <= today` for that `alamat`; `Status!AE:AP`
(§5) does the same lookup per month when building a house's 12-month card, so a
past month's tagihan keeps using whatever was true *then*, permanently, even after
a later row is added.

**Give every house a baseline row** (its `luas`/`tipe` as of whenever it started
being tracked — realistically, whenever this feature goes live and history is
backfilled) — without one, `tarifPada()` (`src/lib/tarifHistoris.js`) and the
`Status!AE:AP` lookup have nothing to resolve for that house and return blank/null,
which the app treats as "no data" (a `-` on the card), not a `0`, specifically so
it's never silently mistaken for "fully paid."

**Two different real-world changes, two different mechanics:**

- **Upgrade in place** (same address, e.g. a kavling gets built on, or the owner
  absorbs the empty lot next door and it's still lived in at the same address):
  just add a row here for that `alamat`. Nothing else changes.
- **Merger where an address itself disappears** (two houses become one and only
  one address survives): the address that's going away gets marked `aktif = FALSE`
  on `Rumah!L` (§1) — **only after its `tunggakan` is 0**. The surviving/new house
  is a plain new row in `Rumah`, exactly like any new resident joining the
  cluster — it does not inherit the retired address's `RumahRiwayat`/`Pembayaran`
  history, and doesn't need to; that history stays exactly where it is, findable
  by its own (now inactive) `alamat` forever, just not reachable through the app's
  normal house lookup any more.

There's no way for the Sheet to *block* setting `aktif = FALSE` while `tunggakan >
0` — no Apps Script in this project, so the closest available guard is a plain
warning formula next to it:

```
M2 (put anywhere convenient, e.g. Rumah!M — a scratch/warning column):
=IF(AND($L2=FALSE, Status!AC2>0), "⚠️ tunggakan belum lunas", "")
```

That's a nudge, not enforcement — same trust level as the Pos/Kas PIN elsewhere in
this app (a deterrent, not a lock). The treasurer is who actually holds the line.

---

## 13. `TarifVersi` — RT-wide rate card history (manual, append-only)

| Col | Header | Type | Notes |
| --- | --- | --- | --- |
| A | tahun_berlaku | number | year this rate card took effect |
| B | bulan_berlaku | number | 1–12 |
| C | tier1_maks | number | `120` — ISLK tier 1 upper bound (m²) |
| D | tier1_tarif | number | `225000` |
| E | tier2_maks | number | `150` |
| F | tier2_tarif | number | `250000` |
| G | tier3_maks | number | `260` |
| H | tier3_tarif | number | `310000` |
| I | tier4_maks | number | `400` |
| J | tier4_tarif | number | `375000` |
| K | tier5_tarif | number | `400000` — no upper bound (luas ≥ `tier4_maks`) |
| L | kavling_per_m2 | number | `400` |
| M | iuran_rt | number | `50000` — the flat RT fee component |
| N | periode | **formula** | `=A2*100+B2`, same sortable key as `RumahRiwayat!F` |

```
N2: =A2*100+B2
```

One row = one complete rate card, all thresholds and amounts bundled together so a
lookup for "what were the rates as of month X" is always exactly one row, never a
join across several. Append-only, same discipline as `RumahRiwayat`: a price change
(ISLK tiers, the flat `iuran_rt`, or both) is a **new row**, never an edit to an
existing one — `Rumah!I/J` (§1) and `Status!AE:AP` (§5) both resolve "the rate card
effective right now / effective that month" the same way `RumahRiwayat` resolves
luas/tipe (largest `periode <= target`).

Seed the first row with day-one rates (matching `TARIF_DEFAULT` in
`src/lib/tariff.js`, so the app's local-dev fallback and the real Sheet agree before
any change is ever made):

```
A2:2023  B2:1  C2:120  D2:225000  E2:150  F2:250000  G2:260  H2:310000
I2:400  J2:375000  K2:400000  L2:400  M2:50000
```

A rate hike later is a second row with the new numbers and whatever `(tahun,bulan)`
it starts from — everything before that month keeps computing off the old row,
forever.

`src/lib/tarifHistoris.js` is where the client mirrors this: `rateCardPada()` picks
the right row (given raw `TarifVersi` rows + a target year/month), `tarifPada()`
combines it with `luasTipePada()` (against `RumahRiwayat`) and reuses
`islk()`/`tarifBulanan()` from `src/lib/tariff.js` — same tier logic the Sheet
formulas encode, just parameterized instead of hardcoded, so there's exactly one
place (per side, Sheet and client) that knows how ISLK tiers turn into a rupiah
amount.
