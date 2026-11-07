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

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=api /out/server /app/server
COPY --from=web /app/dist /app/web
ENV APP_ENV=production WEB_DIR=/app/web PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/server"]
