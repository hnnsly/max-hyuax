# Сборка: статический бинарник без CGO.
# Отдельного go mod download нет: он скачал бы и зависимости инструмента sqlc из go.mod,
# а go build берёт только модули, нужные бинарнику. Кэши BuildKit ускоряют повторные сборки.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Запуск: alpine с системными сертификатами и сертификатами Минцифры.
FROM alpine:3.22
# platform-api2.max.ru подписан сертификатом Минцифры: без него запросы к Bot API падают на TLS.
# Корневой и промежуточный сертификаты скачиваются с Госуслуг (https://www.gosuslugi.ru/crt).
RUN apk add --no-cache ca-certificates \
    && wget -qO /usr/local/share/ca-certificates/russian_trusted_root_ca.crt https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt \
    && wget -qO /usr/local/share/ca-certificates/russian_trusted_sub_ca.crt https://gu-st.ru/content/lending/russian_trusted_sub_ca_pem.crt \
    && grep -q "BEGIN CERTIFICATE" /usr/local/share/ca-certificates/russian_trusted_root_ca.crt \
    && update-ca-certificates \
    && adduser -D -u 10001 app \
    && mkdir -p /data/photos && chown app:app /data/photos
USER app
# Фото к заявкам: каталог принадлежит app, том compose при первом подключении наследует права.
ENV PHOTOS_DIR=/data/photos
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
