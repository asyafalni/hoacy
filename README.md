# Iuran Digital Cluster N — handoff

Static site (Vue Vapor + shadcn-style primitives) for ISLK & Iuran RT 03/14 dues,
Cluster N (Cypress).
No server: **Google Sheet is the database**, **Google Form is the write endpoint**.

## Contents

    docs/setup.md           Step-by-step setup guide (Bahasa Indonesia) — start here
    docs/sheets-schema.md   Tabs, columns and every formula (build this first)
    docs/form-mapping.md    Form fields, prefill URLs, silent-submit option
    docs/deploy.md          Build + Fly.io deploy, what is public and what isn't
    Dockerfile nginx.conf fly.toml   The Fly.io deployment
    apps-script/Code.gs     The read gateway to paste into Apps Script (generated)
    scripts/                build-apps-script.mjs, test-server.mjs
    src/                    Vue Vapor app
    package.json vite.config.js index.html

## How it works

1. **Read** — the Sheet is private. The site calls one **Apps Script web app**
   (`apps-script/Code.gs`) that checks the PIN and returns each role only its own
   slice: a resident gets their own house, Pos the houses without phone numbers,
   Kas everything, and the public `/sum` page only aggregates. The Sheet splits
   Form answers into one row per paid month and computes the kas/rekening
   balances; the app computes everything else — each house's status and
   tunggakan across all years, aging, a house's tarif for any month
   (`src/lib/tagihan.js`, shared with the server).
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
3. Deploy the Apps Script gateway (`npm run build:script`, then `docs/setup.md`
   Tahap 6) — the Sheet stays private.
4. Copy `.env.example` → `.env`, fill the web-app URL + form ids.
5. `npm install && npm run dev` (with `VITE_API_URL` blank it runs on mock data).

## Address model

An address is three parts — **cluster code + block number + house number** — joined as
the key `N7-09` (cluster `N`, blok `7`, rumah `09`). The parts live in their own
columns on the `M-Rumah` tab so you can group and sort by block; `alamat` is a formula
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
`/pos` or `/kas` from a bookmarked link and unlock it with a PIN (Script Properties
`POS_PIN` / `KAS_PIN`, remembered per device after the first entry). The server
checks the PIN and only then sends the data, so — unlike a client-side gate — it is
real access control (rate-limited against guessing); it also keeps warga from wandering
into a cash-recording screen by accident. See `PinGate.vue`.

Design reference: the `Iuran Warga - Cluster N v2` design component in this project.
Visual language: the Organic design system (`src/assets/tokens.css` is its token sheet).
