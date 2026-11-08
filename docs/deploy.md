# Deploy — DigitalOcean App Platform

One Docker image (`Dockerfile`): the Vue app is built, the Go server is built,
and the server serves both `/api` and the app on port 8080. It runs in
DigitalOcean's `blr` region, the same data center as the Aiven database, so a
query is a ~1 ms hop. `.do/app.yaml` is the app spec.

## First deploy

1. DigitalOcean → **Apps → Create App → GitHub** → repo `asyafalni/hoacy`,
   branch `main`. Or with `doctl`: `doctl apps create --spec .do/app.yaml`.
2. In the app's **Settings → Environment variables**, fill in — and tick
   **Encrypt** for every one marked secret:

   | Variable | Scope | Value |
   | --- | --- | --- |
   | `VITE_CLERK_PUBLISHABLE_KEY` | build | `pk_test_…` / `pk_live_…` (public) |
   | `VITE_WA_OPERASIONAL` | build | `62…` (public) |
   | `APP_ENV` | run | `production` |
   | `IP_HEADER` | run | `do-connecting-ip` |
   | `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_NAMA` | run | the first admin |
   | `SHEET_ID`, `DRIVE_FOLDER_ID` | run | from `docs/setup.md` Tahap 3 |
   | `DATABASE_URL` | run, **secret** | Aiven Service URI |
   | `SESSION_SECRET` | run, **secret** | `openssl rand -base64 48` |
   | `CLERK_SECRET_KEY` | run, **secret** | `sk_test_…` / `sk_live_…` |
   | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REFRESH_TOKEN` | run, **secret** | Tahap 3 |

3. Deploy. The build refuses to start without the two `VITE_*` values (and refuses
   a `sk_…` key or anything secret-looking under a `VITE_` name). The server
   refuses to start in production without `SESSION_SECRET`, `CLERK_SECRET_KEY` and
   the Google credentials, and runs the database migrations on every start.
4. Check `https://<app>.ondigitalocean.app/api/sehat` → `{"ok":true}`.

Every push to `main` redeploys. Check the instance size slug in `.do/app.yaml`
against DigitalOcean's current list if the create step rejects it.

## Domain

**Settings → Domains → Add domain**, add the CNAME DigitalOcean shows at your
DNS provider; HTTPS is automatic. Then create Clerk's production instance for
that domain (`docs/setup.md` Tahap 2.4), switch both Clerk keys, and redeploy.
Print the QR codes only after this — they carry the site's address.

## Aiven access

Aiven accepts connections from anywhere by default, over TLS, with the password.
DigitalOcean App Platform has no fixed outgoing IP on its basic plans, so an IP
allowlist isn't practical; keep the password strong and rotate it if it leaks.

## What is public and what isn't

| Who | Sees |
| --- | --- |
| Anyone | `/sum`: cluster-wide aggregates only (no house, name, phone or proof) |
| A resident with their house's PIN | that house's card and payments; never another house, never phone numbers |
| Pengurus (Clerk + `pengurus` table) | every house incl. phone numbers, balances, proofs; writes by role |
| Nobody through the app | PIN hashes, Clerk/Google/DB secrets |

- Warga PINs are argon2id hashes; a session token (not the PIN) stays on the phone,
  and changing the PIN signs every other phone out.
- Every PIN attempt is logged (Admin → Keamanan); lock-outs per house and per IP.
- Proof photos are private files in the RT's Drive, streamed through the server to
  signed-in pengurus only.
- `VITE_*` values are in the downloaded JavaScript — only the Clerk publishable key
  and the operational WhatsApp number are there, both public by design.
- Master-data changes, verdicts and syncs are attributed to a named pengurus
  (`audit`, `keputusan.oleh`, `sinkron_log`).

## Test deploy on Fly (before DigitalOcean)

`fly.beta.toml` runs this version at **iuran-cypress-beta.fly.dev**, next to
v0.1.0 (`iuran-cypress`), in region `sin` (Fly has no India region). It builds
the Dockerfile's `staging` target: `APP_ENV=staging` needs Clerk and
`SESSION_SECRET` like production, but works without Google — proofs go to the
Fly volume `berkas` (`/data`) and Admin → Sinkron reads the bundled CSV "Sheet"
(`backend/dev/sheet`). Database: `cypress_staging` on the same Aiven service,
seeded with `go run ./cmd/sinkron -csv dev/sheet -terapkan -snapshot`.

```sh
fly secrets set -c fly.beta.toml DATABASE_URL=… SESSION_SECRET=… CLERK_SECRET_KEY=sk_test_… BOOTSTRAP_ADMIN_EMAIL=…
fly deploy -c fly.beta.toml --build-arg VITE_CLERK_PUBLISHABLE_KEY=pk_test_… --build-arg VITE_WA_OPERASIONAL=62…
```
