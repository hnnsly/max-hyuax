# Сборка: статический бинарник без CGO.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Запуск: alpine с системными сертификатами и сертификатами Минцифры.
FROM alpine:3.22
# platform-api2.max.ru подписан сертификатом Минцифры: без него запросы к Bot API падают на TLS.
# Корневой и промежуточный сертификаты скачиваются с Госуслуг (https://www.gosuslugi.ru/crt).
RUN apk add --no-cache ca-certificates \
    && wget -qO /usr/local/share/ca-certificates/russian_trusted_root_ca.crt https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt \
    && wget -qO /usr/local/share/ca-certificates/russian_trusted_sub_ca.crt https://gu-st.ru/content/lending/russian_trusted_sub_ca_pem.crt \
    && grep -q "BEGIN CERTIFICATE" /usr/local/share/ca-certificates/russian_trusted_root_ca.crt \
    && update-ca-certificates \
    && adduser -D -u 10001 app
USER app
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
