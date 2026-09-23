# Iuran Digital Cluster N — handoff

Static site (Vue Vapor + shadcn-style primitives) for ISLK & Iuran RT 03/14 dues,
Cluster N (Cypress).
No server: **Google Sheet is the database**, **Google Form is the write endpoint**.

## Contents

    docs/setup.md           Step-by-step setup guide (Bahasa Indonesia) — start here
    docs/sheets-schema.md   Tabs, columns and every formula (build this first)
    docs/form-mapping.md    Form fields, prefill URLs, silent-submit option
    docs/deploy.md          Build + GitHub Pages
    src/                    Vue Vapor app
    package.json vite.config.js index.html

## How it works

1. **Read** — the site fetches a handful of published tabs over Google's gviz
   endpoint (`/gviz/tq?tqx=out:json&headers=1&sheet=<TAB>`). The Sheet splits
   Form answers into one row per paid month and computes the kas/rekening
   balances; the app computes everything else — each house's status and
   tunggakan across all years, aging, cluster totals, a house's tarif for any
   month (`src/lib/tagihan.js`).
2. **Write** — every action submits a **prefilled Google Form**:
   satpam records cash, warga confirms a transfer, bendahara verifies/rejects or deposits.
   Form responses are an append-only ledger — nothing is ever edited; a change
   (a verification, a bigger house, a new rate) is always a new row. A month is
   always paid in full — there are no partial payments in the system.
3. **Money location** — `tunai` counts as **Kas Tunai** and `transfer` as
   **Rekening** (once verified); a "Setor ke Bank" row moves its amount from kas to
   rekening.

## Setup order

1. Create the spreadsheet exactly as in `docs/sheets-schema.md`.
2. Create the five Forms in `docs/form-mapping.md`, point each to the right tab.
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
| `/` | Warga | enter blok + house no (no login), see the 12-month card for any year, confirm one transfer for any months with proof |
| `/pos` | Satpam | search house, pick owed months (any year), record cash |
| `/kas` | Bendahara | Kas Tunai vs Rekening, Setor ke Bank, verify/reject transfers, void mistaken cash entries, print QR (`/kas/qr`), browse every house's card (`/kas/rumah`) |
| `/sum` | Public — anyone | cluster-wide aggregates only: collected vs target this month, surplus/deficit vs OPEX. No PIN, no per-house or per-block numbers — see the PDP note in `RingkasanPublik.vue` |

`/pos` and `/kas` are cash/money-handling screens, so they are **not** in the public
nav — only `/` is. Petugas (satpam, bendahara, admin, the relevant komite) open
`/pos` or `/kas` from a bookmarked link and unlock it with a PIN
(`VITE_PIN_POS` / `VITE_PIN_KAS`, remembered per device after the first entry).
This is a deterrent, not real security — the PIN ships in the public JS bundle
like everything else in this no-server app — but it stops warga from wandering
into a cash-recording screen by accident. See `PinGate.vue`.

Design reference: the `Iuran Warga - Cluster N v2` design component in this project.
Visual language: the Organic design system (`src/assets/tokens.css` is its token sheet).
