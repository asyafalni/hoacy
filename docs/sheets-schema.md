# Google Sheet schema & formulas

Spreadsheet name: **Iuran_ClusterN_2026**. Six tabs. Row 1 is always the header.
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
| G | luas_m2 | number | land area — drives the tariff |
| H | tipe | text | `rumah` or `kavling` |
| I | islk | **formula** | |
| J | iuran_rt | number | 50000 |
| K | tarif_bulanan | **formula** | |
| L | pin | **formula**, editable | Warga card PIN — defaults to phone's last 3 digits; admin overwrites the cell to set a custom one |

```
A2: =IF($B2="","", $B2 & $C2 & "-" & TEXT($D2,"00"))
I2: =IF($H2="kavling", 400*$G2,
      IFS($G2<120,225000, $G2<150,250000, $G2<260,310000, $G2<400,375000, TRUE,400000))
K2: =$I2+$J2
L2: =RIGHT($F2,3)
```

`L` starts as a formula but is meant to be overwritten: typing a literal value into
that cell (e.g. after a resident asks for a reset) replaces the formula for that row
only, same as `disetor_batch`/`terverifikasi` on `Pembayaran`. It's a deterrent PIN,
not real access control — see the comment above `pendingHouse` in `WargaCard.vue`.

Every `VLOOKUP(..., Rumah!$A:$K, n, FALSE)` below keys on `alamat`; the tariff column
is now **11**, not 8. (`L`/pin is looked up separately, by the `API` tab — see §6.)

Tariff table this encodes (ISLK, per month, by luas tanah):
`<120 → 225.000` · `120–149 → 250.000` · `150–259 → 310.000` ·
`260–399 → 375.000` · `400–499 → 400.000` · kavling kosong → `400 × m²`.
Every house also pays Iuran RT 50.000.

---

## 2. `Pembayaran` — Form responses (append-only ledger)

A–H come from the Form. I onward are formulas or treasurer input.

| Col | Header | Source |
| --- | --- | --- |
| A | Timestamp | Form |
| B | alamat | Form (prefilled, e.g. `N7-09`) |
| C | bulan | Form, 1–12 |
| D | tahun | Form, prefilled 2026 |
| E | nominal | Form (prefilled with the outstanding amount) |
| F | metode | Form: `tunai` / `transfer` |
| G | petugas | Form: `Pos Satpam – Ujang` / `Warga` |
| H | catatan | Form, optional |
| I | tarif | formula |
| J | keabsahan | formula — `sah` / `pending` |
| K | disetor_batch | **treasurer**, blank = still Kas Tunai |
| L | terverifikasi | **treasurer** checkbox (transfers only) |
| M | lokasi_uang | formula — `kas` / `bank` |

```
I2: =IFERROR(VLOOKUP($B2, Rumah!$A:$K, 11, FALSE), "")   // B = alamat, e.g. N7-09
J2: =IF($F2="transfer", IF($L2=TRUE,"sah","pending"), "sah")
M2: =IF($F2="transfer","bank", IF($K2="","kas","bank"))
```

Protect A:H (form range) and leave K:L editable by the treasurer only.

---

## 3. `Status` — the digital card, one row per house

A2:A15 `=Rumah!A2:A15` (the `alamat` keys, `N7-01` … `N8-07`). B1:M1 = 1..12 (month numbers).
Two blocks side by side: **amount received** and **status text**.

`$A$1` holds the active year — put the formula `=YEAR(TODAY())` in it, **not** a typed
number. That's what makes the whole card grid roll over to the new year automatically
every 1 January with zero manual sheet edits; every formula below just reads `$A$1`.

```
Amount received (B2, fill right + down):
=SUMIFS(Pembayaran!$E:$E,
        Pembayaran!$B:$B, $A2,
        Pembayaran!$C:$C, B$1,
        Pembayaran!$D:$D, $A$1,           // $A$1 = YEAR(TODAY()), see above
        Pembayaran!$J:$J, "sah")

Status text (P2, fill right + down — P1:AA1 also 1..12):
=LET(
  tarif,  VLOOKUP($A2, Rumah!$A:$K, 11, FALSE),
  bayar,  INDEX($B2:$M2, 1, P$1),
  pend,   SUMIFS(Pembayaran!$E:$E, Pembayaran!$B:$B,$A2,
                 Pembayaran!$C:$C,P$1, Pembayaran!$J:$J,"pending"),
  IFS( bayar>=tarif, "Lunas",
       bayar>0,      "Sebagian",
       pend>0,       "Pending",
       P$1>MONTH(TODAY()), "-",
       TRUE,         "Belum" ))

Outstanding per house (AC2):
=SUMPRODUCT( (COLUMN($B$1:$M$1)-1 <= MONTH(TODAY())) *
             MAX(0, VLOOKUP($A2,Rumah!$A:$K,11,FALSE) - $B2:$M2) )
```

---

## 4. `Setoran` — cash → bank batches (Form responses)

| Col | Header |
| --- | --- |
| A | Timestamp |
| B | batch_id (prefilled: `SET-yymmdd-1`) |
| C | nominal |
| D | oleh |

Depositing = writing the `batch_id` into the `disetor_batch` column of the rows
being banked. Do it with one `Pembayaran!K` fill, or with the helper formula:

```
Suggested rows to bank (paste in Setoran!F2):
=TEXTJOIN(", ", TRUE, FILTER(Pembayaran!$B:$B, Pembayaran!$M:$M="kas"))
```

---

## 5. `Pengeluaran` — spending (Form responses)

`A Timestamp · B keterangan · C nominal · D sumber (kas|bank)`

---

## 6. `API` — the only tab the website reads

One flat table the front end parses; keep column order stable.

```
A1: "key"      B1: "value"
A2: "kas_tunai"
B2: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$M:$M,"kas", Pembayaran!$J:$J,"sah")
    -SUMIFS(Pengeluaran!$C:$C, Pengeluaran!$D:$D,"kas")

A3: "rekening"
B3: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$M:$M,"bank", Pembayaran!$J:$J,"sah")
    -SUMIFS(Pengeluaran!$C:$C, Pengeluaran!$D:$D,"bank")

A4: "tunggakan_total"   B4: =SUM(Status!$AC:$AC)
A5: "lunas_bulan_ini"   B5: =COUNTIF(INDEX(Status!$P:$AA,0,MONTH(TODAY())), "Lunas")
A6: "jumlah_rumah"      B6: =COUNTA(Rumah!$A2:$A)
A7: "updated"           B7: =TEXT(NOW(), "yyyy-mm-dd hh:mm")
A8: "tahun_aktif"       B8: =YEAR(TODAY())
```

`tahun_aktif` is what the app shows as the card's year and uses as the base for
"bayar di muka" — read it instead of assuming any particular year.

Then, starting at D1, a per-house block the app renders directly:

```
D1: "alamat" E1:"nama" F1:"telp" G1:"luas" H1:"tipe" I1:"tarif" J1:"tunggakan"
K1..V1: 1..12 status text   W1:"cluster" X1:"blok" Y1:"rumah"
Z1: "muka_tahun_depan"   AA1: "pin"
D2: =Rumah!A2   E2: =Rumah!E2  F2: =Rumah!F2  G2: =Rumah!G2
H2: =Rumah!H2   I2: =Rumah!K2  J2: =Status!AC2
W2: =Rumah!B2   X2: =Rumah!C2  Y2: =Rumah!D2
K2: =Status!P2  … V2: =Status!AA2
Z2: =TEXTJOIN(",", TRUE, SORT(UNIQUE(FILTER(Pembayaran!$C:$C,
      Pembayaran!$B:$B=D2, Pembayaran!$D:$D=$B$8+1))))
AA2: =Rumah!L2
```

`AA`/pin rides along in the same public, published-to-web feed as everything else
here (see `docs/deploy.md`) — the Warga card checks it client-side, so treat it the
same as the Pos/Kas PIN: a deterrent against a neighbour browsing the app UI, not a
secret that survives someone reading the raw gviz JSON directly.

`Z` is a comma-joined list of month numbers (1–12) that already have a `Pembayaran`
row for **next year** (`$B$8+1` — sah or pending, doesn't matter, a row existing is
enough to avoid double-charging). There's no next-year Status grid — paying ahead
into next year only needs "which months are already spoken for," not a full second
12-month card. The app subtracts this list from 1..12 to build the next-year picker.

A third, unrelated little table rides in the same tab, starting at `AC1` — one row
per **block**, not per house (10 rows, not ~200):

```
AC1: "blok"   AD1: "warna"
AC2: "Blvd"   AD2: "#2a78d6"
AC3: "1"      AD3: "#9C4A1A"
AC4: "2"      AD4: "#eb6834"
AC5: "3"      AD5: "#1baf7a"
AC6: "5"      AD6: "#eda100"
AC7: "6"      AD7: "#e87ba4"
AC8: "7"      AD8: "#008300"
AC9: "8"      AD9: "#4a3aa7"
AC10: "9"     AD10: "#e34948"
AC11: "10"    AD11: "#0F86A3"
```

Plain typed values, no formula — this is the block color-code feature: each block
gets its own color (the Warga card header, the Pos house-number badge). `AD` is
what the admin edits to recolor a block; no deploy needed. Leave a row blank and
the app falls back to its built-in default for that block
(`BLOK_WARNA_DEFAULT` in `src/lib/tariff.js`) — the hex values above are exactly
that default, seeded here so the Sheet and the app agree on day one. Satpam never
touches this tab or any other — they only ever see the rendered app.

A fourth table, `AE1: "satpam"` / `AE2:AE…`, is just a flat list of names — as many
rows as there are satpam, no fixed count:

```
AE1: "satpam"
AE2: "Ujang"
AE3: "Dedi"
AE4: "Rahmat"
```

The Pos PIN (`VITE_PIN_POS`) is still shared by every satpam — it only gates the
*screen*. Once in, the app makes them pick their name from this list before they
can record anything (remembered per device after that), so the `petugas` column on
`Pembayaran` says who actually took the cash, not a hardcoded name. Admin adds or
removes a name here; no deploy needed, same as the block colors above.

Publish the spreadsheet to the web, then the app reads:

```
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=API
```

Sheet-side caching is ~1–5 minutes. After a write the app optimistically shows the
new row locally and re-fetches; see `src/composables/useSheet.js`.
