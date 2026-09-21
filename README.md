# Iuran Digital Blok N — handoff

Static site (Vue Vapor + shadcn-style primitives) for ISLK & Iuran RT 03/14 dues.
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
2. Create the three Forms in `docs/form-mapping.md`, point each to the right tab.
3. Publish the sheet (File → Share → Publish to web) so gviz is readable.
4. Copy `.env.example` → `.env`, fill the sheet + form ids.
5. `npm install && npm run dev`.

## Roles

| Route | Who | Does |
| --- | --- | --- |
| `/` | Warga | enter blok + house no (no login), see the 12-month card, pay by transfer |
| `/pos` | Satpam | search house, pick months, full or partial, record cash |
| `/kas` | Bendahara | Kas Tunai vs Rekening, Setor ke Bank, verify transfers |

Design reference: the `Iuran Warga - Blok N v2` design component in this project.
Visual language: the Organic design system (`src/assets/tokens.css` is its token sheet).
