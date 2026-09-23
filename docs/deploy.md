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

The sheet id and form ids are **public by design** (the sheet is published, the form
accepts anonymous responses). Do not put anything private in that spreadsheet.

## Recovery — the Sheet *is* the database

There's no separate backup job. If a formula gets fat-fingered or a row deleted,
recover it the same way you'd recover any Google Sheet: **File → Version history →
See version history** (or `Ctrl/Cmd+Alt+Shift+H`). It keeps a full cell-level
history — pick an earlier version to preview it, then "Restore this version," or
copy a range out of the old version into the current sheet without a full restore.
That's the whole recovery story for this project; no custom export/snapshot
tooling exists or is planned.
