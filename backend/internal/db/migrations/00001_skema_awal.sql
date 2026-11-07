-- +goose Up
-- The Postgres version of docs/sheets-schema.md. Same facts, same rules:
-- master data is edited by admins (every change lands in `audit`), payments
-- and decisions are append-only, and every number is derived from them —
-- `pembayaran` below is D-Pembayaran as a view.
--
-- A periode is yyyymm as an integer (202609), exactly as the app writes it.
-- Money is whole rupiah in bigint. Times are timestamptz; "which month is it"
-- is always asked in Asia/Jakarta.

create type tipe_rumah     as enum ('rumah', 'kavling');
create type komponen_tarif as enum ('islk_rumah', 'islk_kavling', 'iuran_rt');
create type metode_bayar   as enum ('tunai', 'transfer', 'impor');
create type keputusan_kas  as enum ('sah', 'tolak');
create type peran_pengurus as enum ('admin', 'bendahara', 'komite');
create type sumber_dana    as enum ('kas', 'bank');

create function periode_valid(p integer) returns boolean
  language sql immutable as $$ select p between 190001 and 299912 and p % 100 between 1 and 12 $$;

-- ── Master (was M-Blok, M-Rumah, M-RumahRiwayat, M-TarifVersi, M-Petugas) ──

create table blok (
  kode   text primary key check (kode ~ '^(Blvd|[0-9]{1,2})$'),   -- '7', 'Blvd'
  warna  text check (warna ~ '^#[0-9a-fA-F]{6}$')                  -- null = app default
);

create table rumah (
  alamat          text primary key,                  -- 'N7-09'
  cluster         text not null default 'N',
  blok            text not null references blok (kode),
  nomor           text not null check (nomor ~ '^[0-9]{2,3}$'),   -- '09', leading zero kept
  nama            text not null,
  telp            text check (telp ~ '^62[0-9]{8,13}$'),
  pin_hash        text not null,                     -- argon2id; never leaves the server
  pin_diganti_at  timestamptz,                       -- null = still the default (last 6 of telp)
  -- Deactivation (merger / uncollectable): who and when, never why — the
  -- reason is kept outside the system on purpose. Both or neither.
  nonaktif_at     date,
  nonaktif_oleh   text,
  dibuat_at       timestamptz not null default now(),
  check (alamat = cluster || blok || '-' || nomor),
  check ((nonaktif_at is null) = (nonaktif_oleh is null))
);

-- luas/tipe per house over time. The first row is when billing starts.
create table rumah_riwayat (
  alamat   text    not null references rumah (alamat),
  berlaku  integer not null check (periode_valid(berlaku)),
  luas     numeric(8, 2) not null check (luas > 0),
  tipe     tipe_rumah not null,
  primary key (alamat, berlaku)
);

-- One rate card = every row sharing a `berlaku`. islk_rumah is ISLK alone
-- (the price list minus iuran_rt); the app adds iuran_rt on top.
create table tarif_versi (
  berlaku   integer not null check (periode_valid(berlaku)),
  komponen  komponen_tarif not null,
  luas_min  numeric(8, 2) not null default 0 check (luas_min >= 0),  -- islk_rumah tiers only
  nominal   bigint not null check (nominal >= 0),   -- islk_kavling: per m²
  primary key (berlaku, komponen, luas_min),
  check (komponen = 'islk_rumah' or luas_min = 0)
);

-- Pengurus sign in with Clerk; this table says what each may do.
create table pengurus (
  email          text primary key check (email = lower(email)),
  clerk_user_id  text unique,                       -- filled on first sign-in
  nama           text not null,                     -- recorded on keputusan/kiriman/setoran
  peran          peran_pengurus not null,
  aktif          boolean not null default true,
  dibuat_at      timestamptz not null default now()
);

-- ── Ledgers (was L-Tunai, L-Transfer, M-Impor<tahun>, L-Keputusan, …) ─────

-- One payment = one row here + one row per month in kiriman_bulan.
create table kiriman (
  id          bigserial primary key,
  alamat      text not null references rumah (alamat),
  metode      metode_bayar not null,
  waktu       timestamptz,                -- when the money came in; impor may not know
  total       bigint not null check (total >= 0),
  petugas     text not null,              -- pengurus nama, or 'Warga' for a transfer
  bukti_url   text,                       -- transfer: Drive file (link from the old Form, or our upload)
  -- Where the row came from. A Sheet sync sets sumber_ref to the source row's
  -- natural key, so re-syncing updates instead of duplicating.
  sumber_ref  text unique,
  dibuat_at   timestamptz not null default now(),
  check (metode = 'impor' or waktu is not null)
);
create index on kiriman (alamat);

create table kiriman_bulan (
  kiriman_id  bigint  not null references kiriman (id) on delete cascade,
  periode     integer not null,           -- unchecked on purpose: a bad Sheet row becomes `cek`
  nominal     bigint  not null check (nominal >= 0),
  primary key (kiriman_id, periode)
);
create index on kiriman_bulan (periode);

-- The bendahara's verdict on a whole kiriman. Latest wins.
create table keputusan (
  id          bigserial primary key,
  kiriman_id  bigint not null references kiriman (id),
  keputusan   keputusan_kas not null,
  oleh        text not null,
  waktu       timestamptz not null default now(),
  sumber_ref  text unique
);
create index on keputusan (kiriman_id, waktu desc);

-- Cash moved into the bank (negative = withdrawn into kas).
create table setoran (
  id          bigserial primary key,
  nominal     bigint not null check (nominal <> 0),
  oleh        text not null,
  waktu       timestamptz not null default now(),
  sumber_ref  text unique
);

create table pengeluaran (
  id          bigserial primary key,
  keterangan  text not null,
  nominal     bigint not null check (nominal > 0),
  sumber      sumber_dana not null,
  oleh        text not null,
  nota_url    text,
  waktu       timestamptz not null default now(),
  sumber_ref  text unique
);

-- ── D-Pembayaran: every paid month, with its keabsahan ──────────────────
-- sah     tunai/impor, or a transfer with a `sah` keputusan   → counted
-- pending transfer waiting for the bendahara                  → not counted, not owed
-- tolak   latest keputusan is `tolak`                         → owed again
-- cek     months don't add up to the total, or a bad month    → resident resubmits
-- dobel   an earlier sah/pending row already covers this house + month,
--         by ANY method (the Sheet only compared within a method)
create view pembayaran as
with k as (
  select k.*,
         (select sum(b.nominal) from kiriman_bulan b where b.kiriman_id = k.id) as jumlah,
         (select d.keputusan from keputusan d where d.kiriman_id = k.id
           order by d.waktu desc, d.id desc limit 1) as kep
    from kiriman k
), r as (
  select b.kiriman_id, k.alamat, b.periode, b.nominal, k.metode, k.waktu, k.petugas, k.bukti_url,
         case when k.jumlah <> k.total or not periode_valid(b.periode) then 'cek'
              when k.kep = 'tolak' then 'tolak'
              when k.metode = 'transfer' and k.kep is distinct from 'sah' then 'pending'
              else 'sah' end as status
    from kiriman_bulan b join k on k.id = b.kiriman_id
)
select kiriman_id, alamat, periode, periode / 100 as tahun, periode % 100 as bulan,
       nominal, metode, waktu, petugas, bukti_url,
       case when status in ('sah', 'pending')
             and row_number() over (partition by alamat, periode, status in ('sah', 'pending')
                                    order by waktu nulls first, kiriman_id) > 1
            then 'dobel' else status end as keabsahan
  from r;

-- ── Sheet snapshots (M-Opex, M-SaldoAwal stay edited in the Sheet) ──────
-- Loaded at start-up and daily; the last good copy survives a Sheet outage.
create table sheet_snapshot (
  nama       text primary key,             -- 'M-Opex', 'M-SaldoAwal'
  data       jsonb not null,
  dimuat_at  timestamptz not null default now()
);

-- ── Security and audit ───────────────────────────────────────────────────

-- Every warga PIN attempt. Lock-outs are computed from this table (failures
-- since the last success) and the admin "Keamanan" page reads it.
create table login_warga (
  id          bigserial primary key,
  alamat      text not null,               -- as typed: may not be a real house
  berhasil    boolean not null,
  ip          inet,
  user_agent  text,
  waktu       timestamptz not null default now()
);
create index on login_warga (alamat, waktu desc);
create index on login_warga (ip, waktu desc);

-- One row per applied Sheet sync: which tab, who, what changed.
create table sinkron_log (
  id         bigserial primary key,
  tab        text not null,
  oleh       text not null,
  ringkasan  jsonb not null,               -- {"tambah": n, "ubah": n, "hapus": n, …}
  waktu      timestamptz not null default now()
);

-- Every change to master data (rumah, rumah_riwayat, tarif_versi, blok, pengurus).
create table audit (
  id       bigserial primary key,
  tabel    text not null,
  kunci    text not null,
  sebelum  jsonb,
  sesudah  jsonb,
  oleh     text not null,
  waktu    timestamptz not null default now()
);

-- +goose Down
drop table audit, sinkron_log, login_warga, sheet_snapshot;
drop view pembayaran;
drop table pengeluaran, setoran, keputusan, kiriman_bulan, kiriman,
           pengurus, tarif_versi, rumah_riwayat, rumah, blok;
drop function periode_valid(integer);
drop type sumber_dana, peran_pengurus, keputusan_kas, metode_bayar, komponen_tarif, tipe_rumah;
