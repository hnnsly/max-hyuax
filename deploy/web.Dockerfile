# Сборка мини-приложения (контекст сборки — корень репозитория).
FROM node:24-alpine AS build
WORKDIR /app
COPY miniapp/package.json miniapp/package-lock.json ./
RUN npm ci
COPY miniapp/ ./
RUN npm run build

# Caddy: HTTPS, раздача мини-приложения, прокси /api и /webhook на сервис api.
FROM caddy:2.10-alpine
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/dist /srv
