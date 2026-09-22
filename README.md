# Iuran Digital Cluster N — handoff

Static site (Vue Vapor + shadcn-style primitives) for ISLK & Iuran RT 03/14 dues,
Cluster N (Cypress).
No server: **Google Sheet is the database**, **Google Form is the write endpoint**.

## Contents

    docs/sheets-schema.md   Tabs, columns and every formula (build this first)
    docs/form-mapping.md    Form fields, prefill URLs, silent-submit option
    docs/deploy.md          Build + GitHub Pages
    src/                    Vue Vapor app
    package.json vite.config.js index.html

## How it works

1. **Read** — the site fetches the `API` tab over Google's gviz endpoint
   (`/gviz/tq?tqx=out:json&sheet=API`). Every derived number — tariff per house,
   monthly status, Kas Tunai vs Rekening balance, arrears — is a **Sheet formula**,
   not app logic. The app renders; the Sheet computes.
2. **Write** — every action opens a **prefilled Google Form**:
   satpam records cash, warga submits a transfer, bendahara deposits to bank.
   The form response row is the immutable ledger; formulas re-derive everything.
3. **Money location** — `metode = tunai` lands in **Kas Tunai** until the treasurer
   fills that row's `disetor_batch`; then it counts as **Rekening**.
   `metode = transfer` is **pending** until `terverifikasi` is checked.

## Setup order

1. Create the spreadsheet exactly as in `docs/sheets-schema.md`.
2. Create the four Forms in `docs/form-mapping.md`, point each to the right tab.
3. Publish the sheet (File → Share → Publish to web) so gviz is readable.
4. Copy `.env.example` → `.env`, fill the sheet + form ids.
5. `npm install && npm run dev`.

## Address model

An address is three parts — **cluster code + block number + house number** — joined as
the key `N7-09` (cluster `N`, blok `7`, rumah `09`). The parts live in their own
columns on the `Rumah` tab so you can group and sort by block; `alamat` is a formula
and is what every other tab, the Form prefill, and the per-house QR link reference.

## Roles

| Route | Who | Does |
| --- | --- | --- |
| `/` | Warga | enter blok + house no (no login), see the 12-month card, confirm a transfer with proof |
| `/pos` | Satpam | search house, pick months, full or partial, record cash |
| `/kas` | Bendahara | Kas Tunai vs Rekening, Setor ke Bank, verify transfers, print QR (`/kas/qr`), browse every house's card (`/kas/rumah`) |
| `/ringkasan` | Public — anyone | cluster-wide aggregates only: collected vs target this month, surplus/deficit vs OPEX. No PIN, no per-house or per-block numbers — see the PDP note in `RingkasanPublik.vue` |

`/pos` and `/kas` are cash/money-handling screens, so they are **not** in the public
nav — only `/` is. Petugas (satpam, bendahara, admin, the relevant komite) open
`/pos` or `/kas` from a bookmarked link and unlock it with a PIN
(`VITE_PIN_POS` / `VITE_PIN_KAS`, remembered per device after the first entry).
This is a deterrent, not real security — the PIN ships in the public JS bundle
like everything else in this no-server app — but it stops warga from wandering
into a cash-recording screen by accident. See `PinGate.vue`.

Design reference: the `Iuran Warga - Cluster N v2` design component in this project.
Visual language: the Organic design system (`src/assets/tokens.css` is its token sheet).
