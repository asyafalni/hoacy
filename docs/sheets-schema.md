# Google Sheet schema & formulas

Spreadsheet name: **Iuran_ClusterN**. Tabs in three groups. Row 1 is always the
header; formulas are written for row 2 — fill down unless noted.

| Group | Tabs | Who writes |
| --- | --- | --- |
| **Master** | `M-Rumah`, `M-RumahRiwayat`, `M-TarifVersi`, `M-Blok`, `M-Petugas`, `M-Opex`, `M-SaldoAwal`, `M-Impor<tahun>` | admin/komite type directly in the Sheet (no Form) |
| **Ledger** | `L-Tunai`, `L-Transfer`, `L-Keputusan`, `L-Setoran`, `L-Pengeluaran` | Google Forms only, append-only — nobody edits a row after it lands |
| **Derived** | `D-Pembayaran`, `D-Iuran<tahun>`, `D-Pending`, `D-KasMasuk`, `D-Riwayat`, `D-API` | formulas only, never typed into |

Tab names carry their group as a prefix — `M-` master, `L-` ledger, `D-`
derived — and must match exactly: the app and the formulas address tabs by name.
Inside a formula a prefixed name is always quoted: `'M-Rumah'!A2`. A `Daftar Isi`
tab at the far left maps every tab for humans (not read by the app; content in
`docs/setup.md`).

**Why this split matters:** only master and ledger tabs hold data, and they only
hold raw facts (who paid, when, for which month, how much). All logic lives in
the derived tabs' formulas and in the app's code — so changing a rule later means
editing one formula or redeploying the app, never migrating thousands of rows.
Adding a new column at the **right end** of a tab is always safe; never reorder or
repurpose an existing column once data is in it.

Rules that hold across every tab:

- **Nothing is ever edited to change history.** A house getting bigger, a rate
  going up, a transfer getting verified — each is a *new row* somewhere, and
  every lookup asks "what was true in that month", not "what's true now".
- **A month is paid in full or not at all.** The app only ever records a whole
  month at that month's tarif. Partial payments don't exist in the system — a
  shortfall the pengurus decide to absorb is settled outside it.
- **Ledger tabs hold no formulas.** Google Forms inserts a new row per
  response, so a fill-down formula next to Form columns silently stops at the
  last row it was filled to. Everything computed lives in the derived tabs.
- **The Sheet stores facts, the app computes.** Status, tunggakan (all years),
  aging and each month's tarif are computed in the app (`src/lib/tagihan.js`,
  `src/lib/tarifHistoris.js`) from `D-API`'s lists of paid months plus
  `M-RumahRiwayat`/`M-TarifVersi`. The Sheet only computes what the app can't:
  splitting Form answers into months, cash balances, and monthly sums for the
  public page.

The app reads by column **position**, so column order is load-bearing on every
tab it fetches (`useSheet.js`, `usePembayaranLedger.js`).

## Dates, times and periods — one reference

The spreadsheet's locale is **United States** and time zone **Asia/Jakarta**
(`docs/setup.md`, Tahap 0). Type dates as `yyyy-mm-dd` (ISO) — Sheets reads that
unambiguously in any locale — and give every date column the display format
*Format → Number → Custom date and time* → `yyyy-mm-dd`.

| Where | Kind | Format | Example | Who writes it |
| --- | --- | --- | --- | --- |
| `M-Rumah!J` tanggal_nonaktif | date | `yyyy-mm-dd` | `2026-11-15` | bendahara/admin |
| `M-RumahRiwayat!B:C` tahun_berlaku, bulan_berlaku | two **numbers** (not a date) | `yyyy`, `1`–`12` | `2025`, `6` | admin |
| `M-TarifVersi!A:B` tahun_berlaku, bulan_berlaku | two **numbers** | `yyyy`, `1`–`12` | `2027`, `1` | admin |
| `M-Opex!D` diperbarui | date — **must** display as `yyyy-mm-dd` (the app sorts its text) | `yyyy-mm-dd` | `2026-09-05` | bendahara |
| `M-SaldoAwal!A` tanggal | date | `yyyy-mm-dd` | `2026-10-01` | bendahara |
| `M-Impor<tahun>!B` bulan | **number** | `1`–`12` | `3` | komite |
| `M-Impor<tahun>!D` tanggal_bayar | date, optional | `yyyy-mm-dd` (a time is allowed: `yyyy-mm-dd hh:mm`) | `2025-03-05` | komite |
| `L-*!A` Timestamp | date-time, set by Google Forms | any display format | shown as `9/16/2026 7:55:00` in US locale | Google |
| `L-Tunai!C`, `L-Transfer!C` rincian | text; periode = **tahun × 100 + bulan** (`yyyymm`) | `yyyymm=nominal,…` | `202512=300000,202609=300000` | the app |
| `L-Keputusan!C` waktu | text, **exactly** `'D-Pembayaran'!A` of the submission, seconds included | `yyyy-mm-dd hh:mm:ss` (24-hour) | `2026-09-16 07:55:00` | the app (Kas screen) |
| `D-Pembayaran!A` waktu | text built by the formula — a Form timestamp, an Impor `tanggal_bayar` (time `00:00:00`), or empty for an Impor row without one | `yyyy-mm-dd hh:mm:ss` | `2026-09-16 07:55:00`, `2025-03-05 00:00:00` | formula |
| `D-Pembayaran!C:D` bulan, tahun | numbers (the **dues** month, not the day paid) | `1`–`12`, `yyyy` | `12`, `2025` | formula |
| `D-Riwayat!A:B` tahun, bulan | numbers | `yyyy`, `1`–`12` | `2026`, `9` | formula |
| `D-API!B4` updated | **number** (keeps column B all-numeric for gviz) | `yyyymmddhhmm` | `202609221430` → shown as `2026-09-22 14:30` | formula |

Why `waktu` is text, not a date: it is the id a `L-Keputusan` row points back to,
and text compares exactly — a date cell would round-trip through gviz and the
Form in locale-dependent shapes. The formula formats both sides with the same
`TEXT(…, "yyyy-mm-dd hh:mm:ss")`, so it matches whether Sheets stored the
Keputusan answer as text or converted it to a date-time.

A **periode** (`yyyymm`, e.g. `202609`) is the one way a month is written
wherever a single number must name it: in rincian, `D-API!L:M`, and the app.

---

# Master tabs

## `M-Rumah` — one row per address

| Col | Header | Type | Notes |
| --- | --- | --- | --- |
| A | alamat | **formula** | `N7-09` — primary key every other tab references |
| B | cluster | text | `N` (Cypress) |
| C | blok | text | `7` or `Blvd` — format the column as **Plain text** |
| D | rumah | text | `09` — **Plain text**, keeps the leading zero |
| E | nama | text | kepala keluarga |
| F | telp | text | `62xxx` — WhatsApp link + default PIN (**Plain text**) |
| G | pin | **formula**, overridable | Warga card PIN |
| H | konfirmasi_nonaktif | text | empty = active. See [Menonaktifkan rumah](#menonaktifkan-rumah) |
| I | dinonaktifkan_oleh | text | pengurus name |
| J | tanggal_nonaktif | date | |
| K | aktif | **formula** | read by `D-API` |
| L | peringatan | **formula** | guidance while H:J are being filled |

```
A2: =IF($B2="","", $B2 & $C2 & "-" & TEXT($D2,"00"))
G2: =RIGHT($F2,3)
K2: =NOT(AND($H2=$A2, $I2<>"", $J2<>""))
L2: =IF(AND($H2<>"", $K2), "⚠️ nonaktif belum lengkap — isi alamat persis, oleh, dan tanggal", "")
```

`M-Rumah` is identity only. A house's **luas/tipe** live in `M-RumahRiwayat` and the
**rates** in `M-TarifVersi` — both change over time, `M-Rumah` doesn't.

`G` (pin) starts as a formula but is meant to be overwritten: type a literal value
into one cell (e.g. a resident asks for a reset) and it replaces the formula for
that row only. It's a deterrent, not access control — it travels in the same
public gviz feed as everything else (`docs/deploy.md`).

### Menonaktifkan rumah

An inactive house vanishes from the Warga login, Pos, Kas, Semua Kartu and every
cluster total — while all its rows in every other tab stay for audit. Only two
situations call for it:

- **Merger** — two addresses become one house (with a different tarif). The old
  address is deactivated; the merged house is a normal new `M-Rumah` row with its
  own `M-RumahRiwayat` baseline. It inherits nothing.
- **Uncollectable** — the pengurus have confirmed, through their own checks,
  that nothing more can ever be collected (e.g. an empty kavling whose owner
  has died). Its remaining tunggakan simply stops being counted; the reasons
  are recorded **outside** this Sheet — never in it, because every tab is
  readable through the public gviz feed.

Deactivating is deliberately hard. Set up these three layers once:

1. **Only bendahara/admin can type in `H`.** Select `'M-Rumah'!H2:H` → *Data →
   Protect sheets and ranges* → *Set permissions* → **Restrict who can edit this
   range** → only the bendahara/admin accounts.
2. **`H` only accepts the house's own address.** Select `'M-Rumah'!H2:H` → *Data →
   Data validation* → *Add rule* → criteria **Custom formula is** `=H2=$A2` →
   *Advanced options* → **Reject the input**, help text *"Ketik alamat rumah ini
   persis (mis. N6-07) untuk menonaktifkan."* A typo or any other value is
   refused.
3. **"Are you sure?" on `I:J`.** Select `'M-Rumah'!I2:J` → *Protect range* → **Show
   a warning when editing this range**. (Google allows either a warning *or*
   restricted editors on one range, not both — that's why they're split across
   H and I:J.) Also give `J` a *Data validation → Is valid date* rule.

`K` flips to `FALSE` only when all three of H (exact alamat), I and J are
filled; until then `L` says what's missing. Before starting, open the house in
the app (Kas → Lihat semua kartu rumah) and check its tunggakan with the
pengurus.

To reactivate, clear `H`.

---

## `M-RumahRiwayat` — luas/tipe per house over time (append-only)

| Col | Header | Type | Notes |
| --- | --- | --- | --- |
| A | alamat | text | matches `'M-Rumah'!A` |
| B | tahun_berlaku | number | year this luas/tipe took effect |
| C | bulan_berlaku | number | 1–12 |
| D | luas | number | m² from that month onward |
| E | tipe | text | `rumah` or `kavling` |

A house's physical spec changing means **adding a row**, never editing one.
"The luas in month X" is always the row with the latest (tahun, bulan) not after X
for that `alamat` — so a past month keeps whatever was true then, permanently.

**A house's first row is when billing starts.** Every month from that row to
today is owed until paid, so when backfilling history make sure the payments for
that period are in the `M-Impor<tahun>` tabs too — otherwise those months show as
tunggakan.
A house with **no row at all** is shown everywhere as *"Tarif belum diatur"*:
it can't be paid for, isn't in the target, and Kas lists it as a warning. It is
never billed as Rp0.

Growing in place — a kavling gets built on, or the owner absorbs the lot next
door and keeps the same address — is just a new row here.

---

## `M-TarifVersi` — the RT-wide rate card over time (append-only)

One row per rule, so the number of ISLK tiers is free:

| Col | Header | Notes |
| --- | --- | --- |
| A | tahun_berlaku | |
| B | bulan_berlaku | 1–12 |
| C | komponen | `islk_rumah`, `islk_kavling` or `iuran_rt` |
| D | luas_min | `islk_rumah` only — m², the tier's lower bound (first tier `0`) |
| E | nominal | `islk_rumah`: ISLK per month · `islk_kavling`: per m² · `iuran_rt`: flat per month |

**A rate card = every row sharing one (tahun, bulan).** The card for month X is
the one with the latest date not after X. A house pays the `islk_rumah` row with
the largest `luas_min` ≤ its luas (or `luas × islk_kavling` if it's a kavling),
plus `iuran_rt`. A price change — any tier, the kavling rate, `iuran_rt` — is a
**complete new set of rows** with the new date, including the unchanged rules;
older months keep using the older set. Day-one card:

```
tahun  bulan  komponen      luas_min  nominal
2023   1      islk_rumah    0         225000
2023   1      islk_rumah    120       250000
2023   1      islk_rumah    150       310000
2023   1      islk_rumah    260       375000
2023   1      islk_rumah    400       400000
2023   1      islk_kavling            400
2023   1      iuran_rt                50000
```

Computed by `tarifPada()` in `src/lib/tarifHistoris.js`. A card missing the rule a
house needs (e.g. no `islk_kavling` row) gives that house *"Tarif belum diatur"*,
never Rp0.

---

## `M-Blok` — one color per block

| Col | Header | Notes |
| --- | --- | --- |
| A | blok | `Blvd`, `1`, `2`, `3`, `5`, `6`, `7`, `8`, `9`, `10` — **Plain text** |
| B | warna | hex, e.g. `#2a78d6` |

```
A2: Blvd  B2: #2a78d6     A7: 6   B7: #e87ba4
A3: 1     B3: #9C4A1A     A8: 7   B8: #008300
A4: 2     B4: #eb6834     A9: 8   B9: #4a3aa7
A5: 3     B5: #1baf7a     A10: 9  B10: #e34948
A6: 5     B6: #eda100     A11: 10 B11: #0F86A3
```

Colors the Warga card header and the Pos/Semua Kartu house badges. A missing row
falls back to `BLOK_WARNA_DEFAULT` in `src/lib/tariff.js`. Column A must be plain
text: gviz drops cells whose type differs from the rest of their column, so a
numeric `7` next to a text `Blvd` would vanish.

---

## `M-Petugas` — who may use Pos or Kas

| Col | Header | Notes |
| --- | --- | --- |
| A | nama | e.g. `Ujang` |
| B | peran | `satpam` or `bendahara` |

The Pos/Kas PINs (`VITE_PIN_POS`/`VITE_PIN_KAS`) are shared per role and only gate
the screen; the app then makes each person pick their own name from here (once per
device), so `'L-Tunai'!petugas`, `'L-Keputusan'!oleh` and `'L-Setoran'!oleh` record who
actually acted.

---

## `M-Opex` — fixed minimum monthly cost, itemized

| Col | Header | Notes |
| --- | --- | --- |
| A | kategori | e.g. `Gaji Satpam` |
| B | ikon | one emoji, e.g. `🛡️` |
| C | nominal | number |
| D | diperbarui | date, formatted `yyyy-mm-dd` (*Format → Number → Custom date and time*) — update by hand whenever `C` changes |

```
A2: Gaji Satpam        B2: 🛡️  C2: 2400000  D2: 2026-08-01
A3: Kebersihan         B3: 🧹  C3: 300000   D3: 2026-08-01
A4: Listrik & Air Pos  B4: 💡  C4: 150000   D4: 2026-09-05
A5: Lain-lain          B5: 📋  C5: 100000   D5: 2026-08-01
```

Feeds the public `/sum` dashboard: total OPEX (runway), the "Rincian OPEX" sheet,
and "diperbarui <latest D>" (the app reads the Sheet's formatted date and sorts it
as text, hence `yyyy-mm-dd`). `Gaji Satpam` is deliberately **one combined row**,
never one per person — an individual's wage on a public page would be exactly the
personal data that page exists to avoid.

---

## `M-SaldoAwal` — opening balances (one row)

| Col | Header | Notes |
| --- | --- | --- |
| A | tanggal | the day the app goes live |
| B | kas | cash on hand that day |
| C | rekening | bank balance that day |

`'D-API'!kas_tunai`/`rekening` start from these. Money collected before this date is
already inside them — which is why historical payments go into `Impor`, not the
`L-Tunai` Form.

---

## `M-Impor<tahun>` — historical payments, one tab per year

`M-Impor2023`, `M-Impor2024`, `M-Impor2025`, `M-Impor2026` — one tab per year so the
komite can split the work and check a year at a time. They hold **every payment
made before go-live**, including the go-live year's earlier months (go-live
October 2026 → January–September 2026 go in `M-Impor2026`). Start with the year of
the earliest `M-RumahRiwayat` baseline; tracking only from 2026 means baselines in
January 2026 and just `M-Impor2026`. One row per house per paid month:

| Col | Header | Notes |
| --- | --- | --- |
| A | alamat | *Data validation → Dropdown (from a range)* `'M-Rumah'!A2:A` |
| B | bulan | number 1–12 (*Data validation → Number between 1 and 12*) |
| C | nominal | what was actually paid for that month |
| D | tanggal_bayar | optional — the date the money came in, if known (*Is valid date*) |

No tahun column: the year comes from the tab name. The `D-Pembayaran` formula adds
it per tab (one `HSTACK(...)` line each), so a row can never land in the wrong
year.

These rows count as paid months (`'D-Pembayaran'!F = impor`) and in the `D-Riwayat`
chart, but **never** in `kas_tunai`/`rekening` — that money is already in
`M-SaldoAwal`. Unlike Form ledgers, these tabs are typed by hand, so a mistake is
fixed by editing the row directly. A duplicate row is harmless (marked `dobel`).
Each tab is listed in the `D-Pembayaran` formula — adding a year means copying one
`HSTACK(...)` line there and changing both the tab name and the year number;
removing a year you don't import means deleting its line.

Make sure each house's first `M-RumahRiwayat` row matches the first month you
import for it: every month from that row on counts as owed until paid.

---

# Ledger tabs (Google Forms, append-only, no formulas)

Form setup and entry ids: `docs/form-mapping.md`.

## `L-Tunai` — cash received by the satpam (Form A)

| Col | Header |
| --- | --- |
| A | Timestamp |
| B | alamat |
| C | rincian — `202607=360000,202609=360000` |
| D | total |
| E | petugas — a `M-Petugas` name (peran `satpam`) |

## `L-Transfer` — transfer confirmed by a resident (Form B)

| Col | Header |
| --- | --- |
| A | Timestamp |
| B | alamat |
| C | rincian |
| D | total |
| E | bukti — the file-upload answer (Drive link) |

**Rincian** is one payment's months: `periode=nominal` pairs, comma-separated,
where `periode = tahun*100 + bulan` and `nominal` is that month's full tarif. The
app fills it in; `D-Pembayaran` splits it back into one row per month.

## `L-Keputusan` — the bendahara's verdict on a submission (Form E)

| Col | Header |
| --- | --- |
| A | Timestamp |
| B | alamat |
| C | waktu — the submission's timestamp, `yyyy-mm-dd hh:mm:ss` (as `'D-Pembayaran'!A` shows it) |
| D | keputusan — `sah` or `tolak` |
| E | oleh — a `M-Petugas` name with peran `bendahara` |

One row per decision about one `L-Tunai`/`L-Transfer` submission (all its months at
once). A submission is identified by alamat + its Form timestamp — the one thing a
resident can't edit.

- **Transfer**: `pending` until a `sah` (verified against the bukti) or `tolak`
  (rejected — the resident resubmits).
- **Tunai**: `sah` on arrival; a `tolak` voids a mistaken entry (wrong house or
  month) so the satpam can record it again correctly.
- A rejected/voided submission's months go back to *Belum* and can be paid again.
- The **latest** decision for a submission wins, so a mistaken `tolak` is undone
  with a new `sah`.

The Kas screen writes these (Verifikasi / Tolak / Batalkan); submitting Form E
directly does the same.

## `L-Setoran` — cash moved from kas into the bank (Form C)

| Col | Header |
| --- | --- |
| A | Timestamp |
| B | nominal (prefilled with the current kas balance) — **negative = withdrawal** from the bank into kas |
| C | oleh — the bendahara's name |

That's the whole flow: `'D-API'!kas_tunai` drops by `nominal`, `'D-API'!rekening` rises by
it (a negative nominal moves money the other way).

## `L-Pengeluaran` — money leaving the cluster (Form D)

| Col | Header |
| --- | --- |
| A | Timestamp |
| B | keterangan |
| C | nominal |
| D | sumber — `kas` or `bank` |

---

# Derived tabs (formulas only)

## `D-Pembayaran` — every paid month, one row per house per month

A single formula in **A1** (it writes its own header row) that turns `L-Tunai`,
`L-Transfer` and the `M-Impor<tahun>` tabs into one row per month, and applies
`L-Keputusan`:

```
A1:
=ARRAYFORMULA(LET(
  impor,   VSTACK(
             HSTACK('M-Impor2023'!A2:A, IF(LEN('M-Impor2023'!A2:A), 2023, ), 'M-Impor2023'!B2:D),
             HSTACK('M-Impor2024'!A2:A, IF(LEN('M-Impor2024'!A2:A), 2024, ), 'M-Impor2024'!B2:D),
             HSTACK('M-Impor2025'!A2:A, IF(LEN('M-Impor2025'!A2:A), 2025, ), 'M-Impor2025'!B2:D),
             HSTACK('M-Impor2026'!A2:A, IF(LEN('M-Impor2026'!A2:A), 2026, ), 'M-Impor2026'!B2:D)),
  ia,      CHOOSECOLS(impor, 1),
  L, VSTACK(
       HSTACK('L-Tunai'!A2:E, IF(LEN('L-Tunai'!A2:A), "tunai", ), IF(LEN('L-Tunai'!A2:A), "", )),
       HSTACK('L-Transfer'!A2:D, IF(LEN('L-Transfer'!A2:A), "Warga", ), IF(LEN('L-Transfer'!A2:A), "transfer", ),
              'L-Transfer'!E2:E),
       HSTACK(CHOOSECOLS(impor, 5), ia,
              IF(LEN(ia), CHOOSECOLS(impor, 2) * 100 + CHOOSECOLS(impor, 3) & "=" & CHOOSECOLS(impor, 4), ),
              CHOOSECOLS(impor, 4), IF(LEN(ia), "", ), IF(LEN(ia), "impor", ), IF(LEN(ia), "", ))),
  potong,  IFERROR(SPLIT(CHOOSECOLS(L, 3), ","), ),
  jumlah,  BYROW(potong, LAMBDA(r,
             SUM(ARRAYFORMULA(IFERROR(VALUE(REGEXEXTRACT(r & "", "=(\d+)$")), 0))))),
  baris,   MAKEARRAY(ROWS(potong), COLUMNS(potong), LAMBDA(r, c, r)),
  p,       SPLIT(TOCOL(IF(LEN(potong), baris & "|" & potong, ), 3), "|="),
  i,       CHOOSECOLS(p, 1),
  periode, CHOOSECOLS(p, 2),
  nominal, CHOOSECOLS(p, 3),
  R,       CHOOSEROWS(L, i),
  alamat,  CHOOSECOLS(R, 2),
  metode,  CHOOSECOLS(R, 6),
  waktu,   IF(LEN(CHOOSECOLS(R, 1)), TEXT(CHOOSECOLS(R, 1), "yyyy-mm-dd hh:mm:ss"), ""),
  tahun,   INT(periode / 100),
  bulan,   MOD(periode, 100),
  cocok,   (CHOOSEROWS(jumlah, i) = CHOOSECOLS(R, 4)) * (bulan >= 1) * (bulan <= 12),
  kepKey,  'L-Keputusan'!B2:B & "|" & TEXT('L-Keputusan'!C2:C, "yyyy-mm-dd hh:mm:ss"),
  kep,     MAP(alamat, waktu, LAMBDA(a, w,
             IFNA(XLOOKUP(a & "|" & w, kepKey, 'L-Keputusan'!D2:D, , 0, -1), ""))),
  status,  IF(cocok = 0, "cek", IF(kep = "tolak", "tolak",
             IF(metode = "transfer", IF(kep = "sah", "sah", "pending"), "sah"))),
  kunci,   IF((status = "sah") + (status = "pending"),
              alamat & "|" & periode & "|" & metode, "#" & SEQUENCE(ROWS(i))),
  dobel,   MATCH(kunci, kunci, 0) <> SEQUENCE(ROWS(i)),
  VSTACK(
    {"waktu", "alamat", "bulan", "tahun", "nominal", "metode", "petugas", "bukti_url", "keabsahan"},
    HSTACK(waktu, alamat, bulan, tahun, nominal, metode, CHOOSECOLS(R, 5), CHOOSECOLS(R, 7),
           IF(dobel, "dobel", status)))
))
```

How it works: `L` stacks the three sources into one shape (timestamp, alamat,
rincian, total, petugas, metode, bukti) — an `Impor` row becomes a one-month
rincian, its `tanggal_bayar` standing in for the timestamp and its year taken
from the tab it sits in. Each rincian is split on `,`, tagged with its source row number,
flattened into one list, and split again on `|`/`=` into (row, periode, nominal).
`jumlah` adds up each submission's rincian **before** it is split, so `cocok` can
compare it with the Form's total. (It can't be a `SUMIF` over the split list:
Sheets only lets the `*IF` family — `SUMIF`, `COUNTIF`, `SUMIFS`, … — read real
cell ranges, not arrays built inside a formula; that fails with *"Argument must be
a range"*. Every `*IF` elsewhere in this document reads plain ranges.)
The latest `L-Keputusan` for that submission is looked up by alamat + `waktu`.
When you add an `Impor` tab for another year, add its range to the `impor` line.

Result columns: A waktu (when the money came in) · B alamat · C bulan · D tahun · E nominal ·
F metode (`tunai` / `transfer` / `impor`) · G petugas · H bukti_url ·
I **keabsahan**, which is one of:

| keabsahan | Meaning | Counted? |
| --- | --- | --- |
| `sah` | cash, an import, or a transfer with a `sah` Keputusan | yes |
| `pending` | transfer waiting for the bendahara | no (and not tunggakan either) |
| `tolak` | rejected transfer or voided cash entry (Keputusan `tolak`) | no — the month is owed again |
| `cek` | the submission's rincian doesn't add up to its total (or has a bad month) — the resident must resubmit | no |
| `dobel` | an earlier sah/pending row already covers the same house, month and method — a double-tap or a re-sent confirmation | no |

Every sum anywhere filters on `I = "sah"`, so duplicates, rejections and bad
submissions are handled in exactly one place. Until the first payment is recorded
this tab shows `#N/A` (nothing to split) — that's expected.

The resident's rincian is prefilled by the app but still editable in the Form.
The Kas screen compares every pending month against its tarif and warns before
the bendahara verifies — verifying is what makes a transfer month count, so that
check is the guard on the amount.

---

## `D-Iuran<tahun>` — one tab per dues year (create ten years at once)

`D-Iuran2026`, `D-Iuran2027`, … `D-Iuran2035` — create them all now, each holding
`D-Pembayaran`'s rows for one **dues year** (the month billed, not the day paid —
a 2025 arrear paid in 2026 is in `D-Iuran2025`; its `waktu` says when it came in).
One formula in A1, only the year differs:

```
'D-Iuran2026'!A1:
=IFNA(VSTACK('D-Pembayaran'!A1:I1, IFERROR(FILTER('D-Pembayaran'!A2:I, 'D-Pembayaran'!D2:D = 2026), )), "")
```

Empty until that year's data arrives, then fills itself — nothing to do each
January. For reading and reporting per year; the Kas screen's **Iuran per
tahun** opens exactly one of these per year picked. Years before 2026 are filled
from the matching `M-Impor<tahun>` tab, so create `D-Iuran<tahun>` from the
earliest imported year.

---

## `D-Pending` and `D-KasMasuk` — what the Kas screen loads

Two small tabs so the Kas screen never downloads the whole history:

```
'D-Pending'!A1:
=IFNA(VSTACK('D-Pembayaran'!A1:I1, IFERROR(FILTER('D-Pembayaran'!A2:I,
    ('D-Pembayaran'!I2:I = "pending") + ('D-Pembayaran'!I2:I = "cek")), )), "")

'D-KasMasuk'!A1:
=IFNA(VSTACK('D-Pembayaran'!A1:I1, IFERROR(FILTER('D-Pembayaran'!A2:I, 'D-Pembayaran'!F2:F = "tunai",
    IFERROR(VALUE(LEFT('D-Pembayaran'!A2:A, 4)), 0) >= YEAR(TODAY()) - 1), )), "")
```

The outer `IFNA(…, "")` matters: with no matching rows, `IFERROR(FILTER(…), )`
gives a single empty cell, `VSTACK` pads it to the header's 9 columns with `#N/A`,
and the app would read that padding as a row. (Same wrapper on `D-Iuran<tahun>`.)

`D-Pending` = submissions waiting on the bendahara (any dues year). `D-KasMasuk` =
cash **received** this year and last year (by `waktu`) — the "Riwayat Kas Masuk"
audit list, including voided (`tolak`) and `dobel` rows so they show struck
through.

---

## `D-Riwayat` — collected per month, for the public trend chart

| Col | Header |
| --- | --- |
| A | tahun (formula) |
| B | bulan (formula) |
| C | terkumpul (formula) |

Row 2 is the current month; each row below steps one month back:

```
A2: =YEAR(TODAY())         B2: =MONTH(TODAY())
A3: =IF(B2=1, A2-1, A2)    B3: =IF(B2=1, 12, B2-1)
C2: =SUMIFS('D-Pembayaran'!$E:$E, 'D-Pembayaran'!$C:$C,B2, 'D-Pembayaran'!$D:$D,A2, 'D-Pembayaran'!$I:$I,"sah")
```

Fill A3:B3 and C2 down for as many months as the dashboard should reach (36 = three
years). This tab exists so the public `/sum` page can show a trend **without ever
fetching `D-Pembayaran`** (see below). The target line next to it is computed by the
app per month from `M-RumahRiwayat`/`M-TarifVersi`.

---

## `D-API` — the one tab shaped for the app

Two side-by-side blocks on the same rows. **Keep column positions stable.**

Balances (A/B, key-value) — only what the app can't compute itself. Column B
stays all-numeric (gviz drops cells whose type differs from their column):

```
A1: key          B1: value
A2: kas_tunai    B2: ='M-SaldoAwal'!B2 + SUMIFS('D-Pembayaran'!$E:$E, 'D-Pembayaran'!$F:$F,"tunai", 'D-Pembayaran'!$I:$I,"sah")
                      - SUM('L-Setoran'!$B:$B) - SUMIFS('L-Pengeluaran'!$C:$C, 'L-Pengeluaran'!$D:$D,"kas")
A3: rekening     B3: ='M-SaldoAwal'!C2 + SUMIFS('D-Pembayaran'!$E:$E, 'D-Pembayaran'!$F:$F,"transfer", 'D-Pembayaran'!$I:$I,"sah")
                      + SUM('L-Setoran'!$B:$B) - SUMIFS('L-Pengeluaran'!$C:$C, 'L-Pengeluaran'!$D:$D,"bank")
A4: updated      B4: =VALUE(TEXT(NOW(), "yyyymmddhhmm"))
```

Every cluster number — jumlah rumah, target, tunggakan, aging, lunas bulan ini,
terkumpul bulan ini, dibayar di muka, OPEX — is computed by the app (`useSheet.js`)
from rows it already fetches.

Per-house block (from D), one row per `M-Rumah` row:

| Col | Header | Formula (row 2) |
| --- | --- | --- |
| D | alamat | `='M-Rumah'!A2` |
| E | nama | `='M-Rumah'!E2` |
| F | telp | `=TO_TEXT('M-Rumah'!F2)` |
| G | cluster | `='M-Rumah'!B2` |
| H | blok | `=TO_TEXT('M-Rumah'!C2)` |
| I | rumah | `=TO_TEXT('M-Rumah'!D2)` |
| J | pin | `=TO_TEXT('M-Rumah'!G2)` |
| K | aktif | `='M-Rumah'!K2` |
| L | lunas | periodes with a `sah` row, e.g. `202601,202602` |
| M | pending | periodes with a `pending` row |

```
L2: =IFERROR(TEXTJOIN(",", TRUE, SORT(UNIQUE(FILTER('D-Pembayaran'!$D$2:$D*100 + 'D-Pembayaran'!$C$2:$C,
       'D-Pembayaran'!$B$2:$B=$D2, 'D-Pembayaran'!$I$2:$I="sah")))), "")
M2: same with "pending"
```

`TO_TEXT` keeps mixed-looking columns (blok `Blvd` vs `7`, a PIN typed as a
number) a single type, so gviz never drops a cell. Everything else a screen shows
about a house — the 12-month card for any year, tunggakan across all years since
its first `M-RumahRiwayat` row, oldest unpaid month, months paid in advance — is
derived from L/M by `src/lib/tagihan.js`.

### What gets published and fetched

Publish the whole spreadsheet to the web. `useSheet.js` (every screen, including the
public `/sum`) fetches seven tabs:

```
https://docs.google.com/spreadsheets/d/<SHEET_ID>/gviz/tq?tqx=out:json&headers=1&sheet=<TAB>
TAB = API, Blok, Petugas, Opex, Riwayat, RumahRiwayat, TarifVersi
```

`headers=1` makes row 1 the header explicitly — without it gviz guesses, and can
swallow a data row.

`D-Pending`, `D-KasMasuk` and `D-Iuran<tahun>` are fetched only by
`usePembayaranLedger.js`, used only by the PIN-gated Kas screen; `D-Pembayaran`
itself is never fetched. **`/sum` must never fetch any of them** — they hold
addresses and links to transfer-proof photos. Everything the public page shows is
an aggregate. gviz can't restrict individual tabs, so this is a convention the
code keeps, not something the Sheet enforces (`docs/deploy.md`).

Sheet-side caching is ~1–5 minutes; after a cash payment the Pos screen keeps a
local pending entry until a re-fetch confirms it (`usePendingSync.js`), and the
Kas Verifikasi/Tolak/Batalkan buttons stay disabled until the submission's new
status shows up.

Fetch size stays flat as years pass: public screens read `D-API` (one row per
house), and the Kas screen reads `D-Pending` + `D-KasMasuk` (≈ two years of cash) plus
one `D-Iuran<tahun>` on demand.
