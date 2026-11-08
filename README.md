# Iuran Digital Cluster N — handoff

ISLK & Iuran RT 03/14 dues for Cluster N (The Cypress): a Vue Vapor app on a Go
backend (fib) with Postgres. Residents pay by transfer and upload the proof in
the app; pengurus verify it, record the odd cash payment, and see the cluster's
books. The earlier Google Sheet + Apps Script + Google Forms version is tagged
`v0.1.0`.

## Contents

    docs/setup.md           Step-by-step setup (Bahasa Indonesia) — start here
    docs/deploy.md          DigitalOcean deploy, secrets, what is public and what isn't
    docs/sheets-schema.md   The Google Sheet as a sync source: tabs, columns, how each maps to Postgres
    backend/                Go server: API + serves the built app
      cmd/server            the app
      cmd/migrate           schema migrations (also run on every start)
      cmd/sinkron           Sheet → Postgres sync from the command line
      cmd/google-login      one-time: the RT Google account's refresh token
      internal/tagihan      billing rules (tarif per month, tunggakan, /sum)
      internal/db/migrations  the schema
      dev/sheet             CSV "Sheet" for local development
    src/                    Vue Vapor app
    Dockerfile .do/app.yaml One image on DigitalOcean App Platform (blr)

## How it works

- **Postgres is the source of truth** (Aiven, Bangalore). Payments and verdicts
  are append-only; a month is paid in full or not at all; a house's luas/tipe
  and the rate card are versioned by the month they take effect, so a past
  month is always billed at the rate in force then. The `pembayaran` view
  decides each paid month's status: `sah`, `pending`, `tolak`, `cek`, `dobel`.
- **The server computes, the app displays.** Status, tunggakan for every year,
  a month's tarif, the public aggregates — all `backend/internal/tagihan`
  (proven equal to the old JavaScript by `testdata/golden.json`).
- **Every write goes through the server**, which checks it: a resident can only
  pay their own house's owed months at their tarif, with a real image/PDF as
  proof; cash, verdicts, setoran and pengeluaran need a signed-in pengurus with
  the right role.
- **The Google Sheet stays for what's convenient there**: an admin syncs
  historical payments (`M-Impor<tahun>`, `L-…`) on demand with a preview first,
  and `M-Opex` / `M-SaldoAwal` are copied daily. Proof photos live in the RT's
  own Google Drive, served only through the server.

## Screens

| Route | Who | Does |
| --- | --- | --- |
| `/` | Warga | blok + house no + 6-digit PIN → the card for any year, pay by transfer with proof, change PIN |
| `/kas` | Pengurus (Clerk) | balances, verify/reject transfers, Setor ke Bank, Pengeluaran, Iuran per tahun |
| `/kas/tunai` | Bendahara | record cash a resident handed over |
| `/kas/rumah`, `/kas/qr` | Pengurus | every house's card (WhatsApp link), printable QR per house |
| `/kas/admin` | Admin | houses + luas history, rate cards, block colors, pengurus + invitations, Sheet sync, security log, audit |
| `/sum` | Anyone | cluster-wide aggregates only — no house rows (PDP) |

**Roles** (`pengurus` table, by email; sign-in with Google via Clerk, no self sign-up):
admin — everything; bendahara — the money actions; komite — read only.
Sign-in is Google only; the superadmin registers each pengurus (Clerk user + this table).

**Warga PIN**: default the last 6 digits of the registered phone, changeable from
the card. Every attempt is logged; a house locks for 15 min after 5 misses, 1 h
after 10 and 24 h after 20 in a day, and an IP for 1 h after 30 misses anywhere.

## Develop

```sh
docker run -d --name hoacy-dev-db -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=hoacy \
  -p 127.0.0.1:55432:5432 postgres:17-alpine
cp backend/.env.example backend/.env           # development defaults
cd backend && go run ./cmd/sinkron -csv dev/sheet -terapkan   # seed from the CSV "Sheet"
go run ./cmd/server                            # :8080
cd .. && npm install && npm run dev            # :5173, /api proxied to :8080
go test ./... -C backend                       # needs the dev Postgres
```

Development needs no Clerk and no Google: pengurus sign in with any email in the
`pengurus` table (`BOOTSTRAP_ADMIN_EMAIL` creates the first), proofs go to a local
folder. Requires Go ≥ 1.27.3 (fib) and Node 22.

## Address model

`N7-09` = cluster code `N` + block `7` + house `09` (two digits). Blocks:
`Blvd, 1, 2, 3, 5, 6, 7, 8, 9, 10`.
