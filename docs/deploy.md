# Deploy

```bash
npm install
npm run dev        # http://localhost:5173
npm run build      # -> dist/
```

## GitHub Pages

`vite.config.js` already reads `base` from `VITE_BASE`. For
`https://<user>.github.io/hoacy/`:

```bash
VITE_BASE=/hoacy/ npm run build
```

`.env` is not in git, so the build on GitHub gets it from a secret — without it
the site silently runs on the local mock data:

1. Repo **Settings → Secrets and variables → Actions → New repository secret**:
   name `DOTENV`, value = the whole content of your working `.env` (every
   `VITE_*` line, including `VITE_BASE=/hoacy/`).
2. Repo **Settings → Pages → Source: GitHub Actions**.
3. Add `.github/workflows/pages.yml`, push to `main`:

```yaml
name: pages
on: { push: { branches: [main] } }
permissions: { contents: read, pages: write, id-token: write }
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: 20, cache: npm }
      - run: npm ci
      - run: printf '%s\n' "$DOTENV" > .env
        env: { DOTENV: '${{ secrets.DOTENV }}' }
      - run: npm run build
      - uses: actions/upload-pages-artifact@v3
        with: { path: dist }
  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment: github-pages
    steps: [{ uses: actions/deploy-pages@v4 }]
```

After a change to any id or PIN, update the `DOTENV` secret and re-run the
workflow (Actions → pages → Run workflow, or push again).

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
