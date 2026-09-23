# Google Sheet schema & formulas

Spreadsheet name: **Iuran_ClusterN**. Thirteen tabs in three groups. Row 1 is
always the header; formulas are written for row 2 — fill down unless noted.

| Group | Tabs | Who writes |
| --- | --- | --- |
| **Master** | `Rumah`, `RumahRiwayat`, `TarifVersi`, `Blok`, `Petugas`, `Opex` | admin types directly in the Sheet (rare, deliberate changes — no Form) |
| **Ledger** | `Pembayaran`, `Verifikasi`, `Setoran`, `Pengeluaran` | Google Forms only, append-only — nobody edits a row after it lands |
| **Derived** | `Status`, `Riwayat`, `API` | formulas only, never typed into |

Two rules that hold across every tab:

- **Nothing is ever edited to change history.** A house getting bigger, a rate
  going up, a transfer getting verified — each is a *new row* somewhere, and
  every formula looks up "what was true in that month", not "what's true now".
- **A total the app can compute from rows it already fetches doesn't get its own
  cell.** `API` only carries what the app genuinely can't derive (kas/rekening
  balances, which need the ledgers the public page must never fetch).

The app fetches seven tabs (see [`API`](#api) for the list and why `Pembayaran`
isn't one of the public ones). Column order is load-bearing for every tab the app
reads — `useSheet.js`/`usePembayaranLedger.js` read by position.

---

# Master tabs

## `Rumah` — one row per address

| Col | Header | Type | Notes |
| --- | --- | --- | --- |
| A | alamat | **formula** | `N7-09` — primary key every other tab references |
| B | cluster | text | `N` (Cypress) |
| C | blok | text | `7` or `Blvd` — not every block is numeric, keep it text |
| D | rumah | text | `09` — keep the leading zero (format column as plain text) |
| E | nama | text | kepala keluarga |
| F | telp | text | `62xxx` — WhatsApp link + default PIN |
| G | pin | **formula**, overridable | Warga card PIN |
| H | aktif | boolean | `TRUE` unless decommissioned (see [`RumahRiwayat`](#rumahriwayat)) |

```
A2: =IF($B2="","", $B2 & $C2 & "-" & TEXT($D2,"00"))
G2: =RIGHT($F2,3)
H2: TRUE
```

`Rumah` is identity only. A house's **luas/tipe** live in `RumahRiwayat` and the
**rates** in `TarifVersi` — both change over time, `Rumah` doesn't.

`G` (pin) starts as a formula but is meant to be overwritten: type a literal value
into one cell (e.g. a resident asks for a reset) and it replaces the formula for
that row only. It's a deterrent, not access control — it travels in the same
public gviz feed as everything else (`docs/deploy.md`).

`H` (aktif) is how an address stops being findable/payable in the app without
deleting anything: `useSheet.js` drops `FALSE` rows, so they vanish from the Warga
login, Pos, Semua Kartu Rumah and every cluster total, while every
`Pembayaran`/`RumahRiwayat` row for them stays put for audit.

---

## `RumahRiwayat` — luas/tipe per house over time (append-only)

| Col | Header | Type | Notes |
| --- | --- | --- | --- |
| A | alamat | text | matches `Rumah!A` |
| B | tahun_berlaku | number | year this luas/tipe took effect |
| C | bulan_berlaku | number | 1–12 |
| D | luas | number | m² from that month onward |
| E | tipe | text | `rumah` or `kavling` |
| F | periode | **formula** | sortable key — `202603` for March 2026 |

```
F2: =B2*100+C2
```

A house's physical spec changing means **adding a row**, never editing one.
"The luas in month X" is always the row with the largest `periode <= X` for that
`alamat` — so a past month keeps whatever was true then, permanently.

**Every house needs a baseline row** (its luas/tipe as of whenever tracking starts;
backfill real history as far back as you have it). Without one there's nothing to
resolve: the app shows that month as `—` (unknown), never as a silent `0` that would
read as "paid".

Two real-world changes, two mechanics:

- **Upgrade in place** — a kavling gets built on, or the owner absorbs the lot next
  door and keeps living at the same address: add a row here. Nothing else changes.
- **An address disappears** (a merger where only one address survives): settle that
  address's tunggakan first, then set `Rumah!H` (aktif) to `FALSE`. The surviving
  house is just a normal `Rumah` row (plus its own baseline here) — it inherits
  nothing. The Sheet can only *warn* about unpaid tunggakan, not block the edit
  (no Apps Script), so put this next to `Rumah` as a guard:

```
Rumah!I2 (warning column, optional):
=IF(AND($H2=FALSE, IFERROR(VLOOKUP($A2, Status!$A:$AC, 29, FALSE),0)>0), "⚠️ tunggakan belum lunas", "")
```

---

## `TarifVersi` — the RT-wide rate card over time (append-only)

| Col | Header | Notes |
| --- | --- | --- |
| A | tahun_berlaku | |
| B | bulan_berlaku | 1–12 |
| C, D | tier1_maks, tier1_tarif | `120`, `225000` — ISLK for luas < 120 |
| E, F | tier2_maks, tier2_tarif | `150`, `250000` |
| G, H | tier3_maks, tier3_tarif | `260`, `310000` |
| I, J | tier4_maks, tier4_tarif | `400`, `375000` |
| K | tier5_tarif | `400000` — luas ≥ tier4_maks |
| L | kavling_per_m2 | `400` — empty lots pay per m² instead of a tier |
| M | iuran_rt | `50000` — flat, every house |
| N | periode | **formula** `=A2*100+B2` |

One row = one complete rate card, so "the rates in month X" is always exactly one
row (largest `periode <= X`). A price change — ISLK tiers, `iuran_rt`, or both — is
a new row with the new numbers and the month it starts; everything before keeps
using the old row. Seed the first row with day-one rates; they match
`TARIF_DEFAULT` in `src/lib/tariff.js`, which the app uses for its local-dev mock:

```
A2:2023 B2:1 C2:120 D2:225000 E2:150 F2:250000 G2:260 H2:310000 I2:400 J2:375000 K2:400000 L2:400 M2:50000
```

A house's tarif for month X = ISLK from this row (by the luas/tipe `RumahRiwayat`
gives for month X) + `iuran_rt`. The Sheet computes that in `Status!AE:AP`; the app
computes the identical thing in `tarifPada()` (`src/lib/tarifHistoris.js`, reusing
`islk()` from `src/lib/tariff.js`) for current tarif, past-year cards, and paying a
specific month.

---

## `Blok` — one color per block

| Col | Header | Notes |
| --- | --- | --- |
| A | blok | `Blvd`, `1`, `2`, `3`, `5`, `6`, `7`, `8`, `9`, `10` |
| B | warna | hex, e.g. `#2a78d6` |

```
A2: Blvd  B2: #2a78d6     A7: 6   B7: #e87ba4
A3: 1     B3: #9C4A1A     A8: 7   B8: #008300
A4: 2     B4: #eb6834     A9: 8   B9: #4a3aa7
A5: 3     B5: #1baf7a     A10: 9  B10: #e34948
A6: 5     B6: #eda100     A11: 10 B11: #0F86A3
```

Colors the Warga card header and the Pos/Semua Kartu house badges. A missing row
falls back to `BLOK_WARNA_DEFAULT` in `src/lib/tariff.js` — the values above are
exactly that default.

---

## `Petugas` — who may use Pos or Kas

| Col | Header | Notes |
| --- | --- | --- |
| A | nama | e.g. `Ujang` |
| B | peran | `satpam` or `bendahara` |

The Pos/Kas PINs (`VITE_PIN_POS`/`VITE_PIN_KAS`) are shared per role and only gate
the screen; the app then makes each person pick their own name from here (once per
device), so `Pembayaran!petugas`, `Verifikasi!oleh` and `Setoran!oleh` record who
actually acted.

---

## `Opex` — fixed minimum monthly cost, itemized

| Col | Header | Notes |
| --- | --- | --- |
| A | kategori | e.g. `Gaji Satpam` |
| B | ikon | one emoji, e.g. `🛡️` |
| C | nominal | number |
| D | diperbarui | date — update by hand whenever `C` changes |

```
A2: Gaji Satpam        B2: 🛡️  C2: 2400000  D2: 2026-08-01
A3: Kebersihan         B3: 🧹  C3: 300000   D3: 2026-08-01
A4: Listrik & Air Pos  B4: 💡  C4: 150000   D4: 2026-09-05
A5: Lain-lain          B5: 📋  C5: 100000   D5: 2026-08-01
```

Feeds the public `/sum` dashboard: total OPEX (runway), the "Rincian OPEX" sheet,
and "diperbarui <latest D>". `Gaji Satpam` is deliberately **one combined row**,
never one per person — an individual's wage on a public page would be exactly the
personal data that page exists to avoid.

---

# Ledger tabs (Google Forms, append-only)

Form wiring and entry ids: `docs/form-mapping.md`.

## `Pembayaran` — every payment, one row per house per month

| Col | Header | Source |
| --- | --- | --- |
| A | Timestamp | Form |
| B | alamat | Form (prefilled, e.g. `N7-09`) |
| C | bulan | Form, 1–12 |
| D | tahun | Form |
| E | nominal | Form (prefilled with that month's tarif) |
| F | metode | Form: `tunai` / `transfer` |
| G | petugas | Form: a `Petugas` name (satpam), or `Warga` |
| H | catatan | Form, optional free text |
| I | bukti_url | Form — the file-upload question's Drive link (transfers) |
| J | keabsahan | **formula** — `sah` / `pending` |

`I` is where Google puts the file-upload answer once the Form is linked — Forms
append one column per question, in question order, so the upload question must stay
**last** in the Form and the formula goes in `J`.

```
J2: =IF($F2="transfer",
       IF(COUNTIFS(Verifikasi!$B:$B,$B2, Verifikasi!$C:$C,$C2, Verifikasi!$D:$D,$D2)>0, "sah", "pending"),
       "sah")
```

Cash (`tunai`) is `sah` immediately — the satpam took it in hand. A transfer is
`pending` until bendahara verifies it, which means a matching `Verifikasi` row;
there's no checkbox to tick here, because the only way anything in this project
changes state is a new ledger row.

---

## `Verifikasi` — transfer sign-off

| Col | Header | Source |
| --- | --- | --- |
| A | Timestamp | Form |
| B | alamat | Form |
| C | bulan | Form, 1–12 |
| D | tahun | Form |
| E | oleh | Form — a `Petugas` name with peran `bendahara` |

One row per transfer bendahara checked against its bukti and confirmed. The Kas
screen does this in one tap ("Verifikasi" next to "Lihat bukti"); if the app is
unavailable, submitting Form D directly does the same.

---

## `Setoran` — cash moved from kas into the bank

| Col | Header | Source |
| --- | --- | --- |
| A | Timestamp | Form |
| B | nominal | Form (prefilled with the current kas balance) |
| C | oleh | Form — the bendahara's name |

That's the whole deposit flow: submit one row, and `API!kas_tunai` drops by
`nominal` while `API!rekening` rises by it. No marking of individual `Pembayaran`
rows — the balances are computed from totals, so there's nothing to keep in sync.

---

## `Pengeluaran` — money leaving the cluster

| Col | Header |
| --- | --- |
| A | Timestamp |
| B | keterangan |
| C | nominal |
| D | sumber — `kas` or `bank` |

Entered through Form C by bendahara; the app has no screen for it.

---

# Derived tabs (formulas only)

## `Status` — each house's 12-month card for the current year

`A1: =YEAR(TODAY())` (the year this grid shows — rolls over by itself every
1 January). `A2`: `=Rumah!A2`, fill down. Three blocks, each with its header row
holding month numbers 1..12: `B1:M1`, `P1:AA1`, `AE1:AP1`.

```
Amount received — B2, fill right to M, and down:
=IFERROR(SUM(UNIQUE(FILTER(Pembayaran!$E:$E,
    Pembayaran!$B:$B=$A2, Pembayaran!$C:$C=B$1, Pembayaran!$D:$D=$A$1,
    Pembayaran!$J:$J="sah"))), 0)

Tarif that month — AE2, fill right to AP, and down:
=LET(
  target,   $A$1*100 + AE$1,
  luasKey,  MAXIFS(RumahRiwayat!$F:$F, RumahRiwayat!$A:$A,$A2, RumahRiwayat!$F:$F,"<="&target),
  luasRow,  FILTER(RumahRiwayat!$A:$E, RumahRiwayat!$A:$A=$A2, RumahRiwayat!$F:$F=luasKey),
  luas,     INDEX(luasRow,1,4),
  tipe,     INDEX(luasRow,1,5),
  tarifKey, MAXIFS(TarifVersi!$N:$N, TarifVersi!$N:$N,"<="&target),
  r,        FILTER(TarifVersi!$A:$N, TarifVersi!$N:$N=tarifKey),
  IF(tipe="kavling", INDEX(r,1,12)*luas,
     IFS(luas<INDEX(r,1,3), INDEX(r,1,4),  luas<INDEX(r,1,5), INDEX(r,1,6),
         luas<INDEX(r,1,7), INDEX(r,1,8),  luas<INDEX(r,1,9), INDEX(r,1,10),
         TRUE, INDEX(r,1,11))) + INDEX(r,1,13))

Status text — P2, fill right to AA, and down:
=LET(
  tarif, INDEX($AE2:$AP2, 1, P$1),
  bayar, INDEX($B2:$M2, 1, P$1),
  pend,  IFERROR(SUM(UNIQUE(FILTER(Pembayaran!$E:$E, Pembayaran!$B:$B=$A2,
             Pembayaran!$C:$C=P$1, Pembayaran!$D:$D=$A$1, Pembayaran!$J:$J="pending"))), 0),
  IFS(bayar>=tarif, "Lunas", bayar>0, "Sebagian", pend>0, "Pending",
      P$1>MONTH(TODAY()), "-", TRUE, "Belum"))

Tunggakan (due months only) — AC2, fill down:
=SUMPRODUCT((COLUMN($B$1:$M$1)-1 <= MONTH(TODAY())) * MAX(0, $AE2:$AP2 - $B2:$M2))
```

The tarif lookup lives once, in `AE:AP`, and both the status text and tunggakan read
from it — it's the one complex formula in the Sheet, so it's written in one place.

**Why `UNIQUE(FILTER(...))`, not `SUMIFS`:** a double-tapped submit produces two
identical rows (same house, month, year, nominal). Summing *distinct* amounts counts
it once. Accepted tradeoff: two genuinely separate payments of the exact same amount
for the same month also collapse to one — rare, and it errs toward undercounting, not
overcharging a resident. (`PosSatpam.vue` also disables its button mid-submit.)

`Status` only ever holds the current year. Past years are reconstructed by the app
from `Pembayaran` + `RumahRiwayat`/`TarifVersi` (`WargaCard.vue`), same logic.

---

## `Riwayat` — collected per month, for the public trend chart

| Col | Header |
| --- | --- |
| A | tahun (formula) |
| B | bulan (formula) |
| C | terkumpul (formula) |

Row 2 is the current month; each row below steps one month back:

```
A2: =YEAR(TODAY())         B2: =MONTH(TODAY())
A3: =IF(B2=1, A2-1, A2)    B3: =IF(B2=1, 12, B2-1)
C2: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$C:$C,B2, Pembayaran!$D:$D,A2, Pembayaran!$J:$J,"sah")
```

Fill A3:B3 and C2 down for as many months as the dashboard should reach (36 = three
years; extending later is just filling more rows). This tab exists so the public
`/sum` page can show a trend **without ever fetching `Pembayaran`** (see below); it's
also where the app reads "terkumpul bulan ini" (row 2).

---

## `API` — the one tab shaped for the app

Two side-by-side blocks on the same rows. **Keep column positions stable.**

Balances (A/B, key-value) — only what the app can't compute itself:

```
A1: key          B1: value
A2: kas_tunai    B2: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$F:$F,"tunai", Pembayaran!$J:$J,"sah")
                      - SUM(Setoran!$B:$B) - SUMIFS(Pengeluaran!$C:$C, Pengeluaran!$D:$D,"kas")
A3: rekening     B3: =SUMIFS(Pembayaran!$E:$E, Pembayaran!$F:$F,"transfer", Pembayaran!$J:$J,"sah")
                      + SUM(Setoran!$B:$B) - SUMIFS(Pengeluaran!$C:$C, Pengeluaran!$D:$D,"bank")
A4: updated      B4: =TEXT(NOW(), "yyyy-mm-dd hh:mm")
```

Every other cluster number — jumlah rumah, target, tunggakan, lunas bulan ini,
terkumpul bulan ini, total OPEX, OPEX diperbarui — is computed by the app
(`totals` in `useSheet.js`) from rows it already fetches, so there's no second copy
here that could drift.

Per-house block (from D), one row per `Rumah` row:

| Col | Header | Formula (row 2) |
| --- | --- | --- |
| D | alamat | `=Rumah!A2` |
| E | nama | `=Rumah!E2` |
| F | telp | `=Rumah!F2` |
| G | tunggakan | `=Status!AC2` |
| H..S | status bulan 1..12 | `=Status!P2` … `=Status!AA2` |
| T | cluster | `=Rumah!B2` |
| U | blok | `=Rumah!C2` |
| V | rumah | `=Rumah!D2` |
| W | muka_tahun_depan | see below |
| X | pin | `=Rumah!G2` |
| Y | aktif | `=Rumah!H2` |

```
W2: =TEXTJOIN(",", TRUE, SORT(UNIQUE(FILTER(Pembayaran!$C:$C,
      Pembayaran!$B:$B=D2, Pembayaran!$D:$D=YEAR(TODAY())+1))))
```

`W` lists the months of **next year** that already have a `Pembayaran` row (sah or
pending — a row existing is enough to not charge it twice). The app offers the rest
as "bayar di muka".

No luas/tipe/tarif columns: the app resolves those from `RumahRiwayat`/`TarifVersi`
itself — one definition of "this house's tarif in month X", used everywhere.

### What gets published and fetched

Publish the whole spreadsheet to the web. `useSheet.js` (every screen, including the
public `/sum`) fetches seven tabs:

```
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&sheet=<TAB>
TAB = API, Blok, Petugas, Opex, Riwayat, RumahRiwayat, TarifVersi
```

`Pembayaran` is fetched only by `usePembayaranLedger.js`, used only by PIN-gated
screens that need row detail: Kas (pending transfers with `bukti_url`) and a
resident's own card (past years). **`/sum` must never fetch `Pembayaran`** — it holds
addresses and links to transfer-proof photos, the per-house data the public page is
built to keep off. Everything the public page shows is an aggregate: `Riwayat`
(monthly sums), `Opex` (category totals), and the per-house block summed client-side
and never rendered per house. gviz can't restrict individual tabs, so this is a
convention the code keeps, not something the Sheet enforces (`docs/deploy.md`).

Sheet-side caching is ~1–5 minutes; after a write the app keeps a local pending
entry until a re-fetch confirms it (`usePendingSync.js`).
