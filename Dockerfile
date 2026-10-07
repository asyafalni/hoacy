# Static site: build with Vite, serve the files with nginx. The build reads the
# project's .env (public VITE_* values only — PINs live in Apps Script), and
# scripts/cek-env.mjs refuses to build without the API URL and Form ids.
# docs/deploy.md
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json .npmrc ./
RUN npm ci
COPY . .
RUN node scripts/cek-env.mjs && npm run build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
