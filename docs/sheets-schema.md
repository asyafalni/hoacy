# The Google Sheet — sync source

Since the Go backend, **Postgres is the source of truth**. The spreadsheet
(**Iuran_ClusterN**) stays for two jobs:

1. **Sync on demand** (Kas → Admin → Sinkron, or `backend/cmd/sinkron`):
   historical payments the komite types (`M-Impor<tahun>`), the old Form ledgers
   (`L-…`), and — once, at cutover — the master tabs. Always **Sheet → database**.
2. **Daily snapshot**: `M-Opex` and `M-SaldoAwal` are edited in the Sheet and
   copied by the server at start-up and every day (last good copy kept).

The server reads the Sheet as the RT's Google account with **read-only** access;
nothing in the app can change it. The derived `D-…` tabs and their formulas
are no longer read — everything they computed is now the `pembayaran` view and
`backend/internal/tagihan` (formulas: see tag `v0.1.0`).

## Sync rules

- Each tab is read **by column position** (row 1 is the header, ignored), as
  below. Numbers are read unformatted; dates as the Sheet shows them —
  `yyyy-mm-dd` (komite-typed) or US-locale Form timestamps (`9/16/2026 7:55:00`),
  in Asia/Jakarta.
- Every row has a **natural key** (below). Syncing again updates rows whose key
  already exists and adds new ones — never a duplicate. A row typed twice in the
  Sheet stays two rows (`…#2`), and the second is `dobel`.
- **Pratinjau** shows *tambah / ubah / sama / hilang / galat* first; nothing
  changes until **Terapkan**.
- A row that is in the database but **gone from the Sheet** is only listed
  ("hilang"); it is deleted only if ticked.
- A row that can't be read (unknown alamat, month 13, bad number) is a *galat*:
  skipped and listed with its Sheet row number.
- Every apply is logged (`sinkron_log`); master-tab changes also go to `audit`.
- First import order: `M-Blok` → `M-Rumah` → `M-RumahRiwayat` → `M-TarifVersi` →
  `L-Tunai` → `L-Transfer` → `L-Keputusan` → `L-Setoran` → `L-Pengeluaran` →
  `M-Impor<tahun>` (a payment needs its house; a verdict needs its payment).

After cutover the master data (houses, phone numbers, PINs, luas history, rate
cards, pengurus) is edited in the app (Admin). Clear the **telp** and **pin**
columns of `M-Rumah` in the Sheet once it's imported.

## Tabs

### `M-Blok` → `blok` · key: blok
A blok (`7`, `Blvd` — plain text) · B warna (`#rrggbb`, optional)

### `M-Rumah` → `rumah` · key: alamat
A alamat (must equal B+C+"-"+D) · B cluster (`N`) · C blok · D rumah (`09`) ·
E nama · F telp (`62…`) · G pin (**ignored**) · H konfirmasi_nonaktif ·
I dinonaktifkan_oleh · J tanggal_nonaktif · K aktif (`FALSE` = inactive; then I
and J are required).

A new house's PIN is the last 6 digits of its phone (random if none — reset it
in Admin to see it). A re-sync never changes an existing house's PIN.

### `M-RumahRiwayat` → `rumah_riwayat` · key: alamat + month
A alamat · B tahun_berlaku · C bulan_berlaku · D luas (m²) · E tipe (`rumah` /
`kavling`). The first row of a house is the month billing starts.

### `M-TarifVersi` → `tarif_versi` · key: month + komponen + luas_min
A tahun_berlaku · B bulan_berlaku · C komponen (`islk_rumah` / `islk_kavling` /
`iuran_rt`) · D luas_min (`islk_rumah` tiers only) · E nominal (`islk_kavling`:
per m²). All rows sharing a month are one rate card. `islk_rumah` is the price
list **minus** `iuran_rt`.

### `M-Impor<tahun>` → `kiriman` (metode `impor`) · key: tab + alamat + bulan
A alamat · B bulan (1–12) · C nominal · D tanggal_bayar (optional). The year is
the tab's. Every payment before go-live, one row per house per paid month.
Counted as paid, never in the cash/bank balances (that money is in
`M-SaldoAwal`).

### `L-Tunai` → `kiriman` (metode `tunai`) · key: alamat + Timestamp
A Timestamp · B alamat · C rincian (`202607=360000,202609=360000`) · D total ·
E petugas. A rincian that doesn't add up to the total (or has a bad month)
becomes `cek`.

### `L-Transfer` → `kiriman` (metode `transfer`) · key: alamat + Timestamp
A Timestamp · B alamat · C rincian · D total · E bukti (the Form's Drive link —
share the upload folder with the RT account so Kas can show it). `pending`
until a verdict.

### `L-Keputusan` → `keputusan` · key: alamat + Timestamp
A Timestamp · B alamat · C waktu (the submission's timestamp,
`yyyy-mm-dd hh:mm:ss`) · D keputusan (`sah` / `tolak`) · E oleh. Matched to the
synced `L-Tunai`/`L-Transfer` row by alamat + waktu; the latest verdict wins.

### `L-Setoran` → `setoran` · key: Timestamp
A Timestamp · B nominal (negative = withdrawn from the bank) · C oleh

### `L-Pengeluaran` → `pengeluaran` · key: Timestamp
A Timestamp · B keterangan · C nominal · D sumber (`kas` / `bank`)

### `M-Opex` → snapshot (daily)
A kategori · B ikon (emoji) · C nominal · D diperbarui (`yyyy-mm-dd`). Category
totals only — never one person's wage (it's on the public page).

### `M-SaldoAwal` → snapshot (daily)
Row 2: A tanggal · B kas · C rekening — the balances on go-live day.

## Statuses (the `pembayaran` view)

| keabsahan | Meaning | Counted? |
| --- | --- | --- |
| `sah` | cash, an import, or a transfer with a `sah` verdict | yes |
| `pending` | transfer waiting for the bendahara | no (and not owed either) |
| `tolak` | latest verdict `tolak` (rejected transfer / voided cash) | no — owed again |
| `cek` | months don't add up to the total, or a bad month | no |
| `dobel` | an earlier sah/pending row already covers this house + month (any method) | no |
