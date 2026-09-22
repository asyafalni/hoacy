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

`.github/workflows/pages.yml`:

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
      - run: VITE_BASE=/hoacy/ npm run build
      - uses: actions/upload-pages-artifact@v3
        with: { path: dist }
  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment: github-pages
    steps: [{ uses: actions/deploy-pages@v4 }]
```

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
