# Deploy

```bash
npm install
npm run dev        # http://localhost:5173
npm run build      # -> dist/
```

## Fly.io

The site is static: `Dockerfile` builds it with Vite and serves `dist/` with nginx
(`nginx.conf`) on port 8080; `fly.toml` runs it in Singapore (`sin`) and scales to
zero when idle.

**The build reads `.env`.** Every `VITE_*` value is baked into the JavaScript at
build time, so the Docker build stage reads the project's `.env` (it never reaches
the final image — only `dist/` does). `scripts/cek-env.mjs` stops the build if
`VITE_API_URL` or any Form id/entry id is missing — otherwise the site would
silently run on mock data, or record nothing. Leave `VITE_BASE` empty (the site
lives at the domain root). PINs are **not** in `.env`; they're Script Properties.

First deploy:

```bash
fly auth login                       # once per machine (opens the browser)
fly apps create iuran-cypress        # or another free name — then set `app` in fly.toml
fly deploy --ha=false                # one machine is plenty for this site
```

The site is then at `https://iuran-cypress.fly.dev`. Every later deploy is just
`fly deploy`. After changing a Form id or the API URL: edit `.env`, `fly deploy`.

Things to know:

- **Decide the domain before printing QR codes.** Each house's QR (Kas → Cetak QR)
  encodes the site's address at the time. For a custom domain:
  `fly certs add iuran.example.id`, point the DNS records `fly certs show` lists,
  then print.
- **A deploy reaches phones on their next visit.** `index.html` and the service
  worker are served `no-cache`, hashed assets are cached for a year — an open
  Pos/Kas tab picks up the new version after a reload.
- **Cost:** with `auto_stop_machines` the machine only runs while someone is using
  the site; Fly bills per second of running time (a card is required on the
  account).
- `npm run build` without a complete `.env` still works locally (mock mode) — only
  the Docker build insists on the real values.

## What is and isn't public

- **The spreadsheet is private** — shared with the pengurus only, never published.
  Names, phone numbers, PINs, per-house payment status and proof links never leave
  it except through the Apps Script gateway, which checks the PIN first and returns
  each role only its own slice (`docs/sheets-schema.md`, *How the app reads the
  Sheet*). PINs live in Script Properties, not in the bundle.
- **Public by design:** the web-app URL (`VITE_API_URL`) and the Form ids/entry ids
  — they ship in the site's JavaScript. The URL alone returns only aggregates (the
  `/sum` numbers); anything per-house needs the PIN, and guesses are rate-limited.
- **Known limit — Forms still accept anonymous writes.** Anyone who reads the
  bundle can submit a forged cash row or a Keputusan to the Forms, which the Sheet
  would count. The Kas screen's per-submission view and the bendahara's checks
  against the bukti are the guard; closing it fully would mean routing writes through
  the gateway too (a later change, not needed for reading).

## Recovery — the Sheet *is* the database

There's no separate backup job. If a formula gets fat-fingered or a row deleted,
recover it the same way you'd recover any Google Sheet: **File → Version history →
See version history** (or `Ctrl/Cmd+Alt+Shift+H`). It keeps a full cell-level
history — pick an earlier version to preview it, then "Restore this version," or
copy a range out of the old version into the current sheet without a full restore.
That's the whole recovery story for this project; no custom export/snapshot
tooling exists or is planned.
