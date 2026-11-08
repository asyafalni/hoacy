# One image, one process: the Go server (backend/) serving both /api and the
# built Vue app. docs/deploy.md.
#
# The frontend's public settings (VITE_*) are build arguments — DigitalOcean
# passes its BUILD_TIME env vars as such; scripts/cek-env.mjs refuses a build
# without them. Secrets are never build arguments: they are runtime env vars.

FROM node:22-alpine AS web
WORKDIR /app
COPY package.json package-lock.json .npmrc ./
RUN npm ci
COPY index.html vite.config.js ./
COPY public ./public
COPY src ./src
COPY scripts ./scripts
ARG VITE_CLERK_PUBLISHABLE_KEY
ARG VITE_WA_OPERASIONAL
ARG VITE_BASE=
RUN node scripts/cek-env.mjs && npm run build

FROM golang:1.27-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Test deploy on Fly (fly.beta.toml): the same server, plus the CSV "Sheet" for
# sync without Google, running as root so it can write proofs to a Fly volume.
# Never the production image — that is the last stage below.
FROM gcr.io/distroless/static-debian12 AS staging
WORKDIR /app
COPY --from=api /out/server /app/server
COPY --from=web /app/dist /app/web
COPY --from=api /src/dev/sheet /app/contoh-sheet
ENV APP_ENV=staging WEB_DIR=/app/web PORT=8080 SHEET_CSV_DIR=/app/contoh-sheet BERKAS_DIR=/data/berkas
EXPOSE 8080
ENTRYPOINT ["/app/server"]

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=api /out/server /app/server
COPY --from=web /app/dist /app/web
ENV APP_ENV=production WEB_DIR=/app/web PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/server"]
