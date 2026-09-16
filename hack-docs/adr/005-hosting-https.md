---
title: ADR-005 Хостинг, HTTPS и развёртывание
type: adr
status: accepted
date: 2026-09-17
deciders: [команда]
supersedes: —
tags: [hosting, https, docker, deploy]
---

# ADR-005: Хостинг, HTTPS и развёртывание

## Контекст
- Webhook MAX принимается только по HTTPS на 443 с сертификатом доверенного CA; исходящие запросы к `platform-api2.max.ru` требуют доверенного сертификата Минцифры ([max-platform-capabilities](../research/max-platform-capabilities.md)).
- Мини-приложению нужен стабильный HTTPS-URL: его правка в настройках бота модерируется до 48 рабочих часов ([R1](../research/risks.md)).
- 152-ФЗ: ПДн граждан РФ хранятся в РФ. Формат сдачи: одна команда Docker, сборка ≤ 5 мин ([Case.md](../Case.md)).
- Ответы интервью: VPS + свой домен, российский VPS ([Блок А](../interview/BLOCK-A.md), [Блок В](../interview/BLOCK-V.md)).

## Решение
1. **Российский VPS** (Timeweb Cloud / Selectel / VK Cloud / Yandex Cloud — выбор в Спайке 0 по цене и ресурсам под Ollama: ориентир 8 vCPU, 16 ГБ RAM) + **свой домен**.
2. **Caddy** — обратный прокси с автоматическим TLS (Let's Encrypt).
   - Стабильные адреса, которые не меняются до конца проверки: `https://<домен>/` — статика мини-приложения; `/api/` — API; `/webhook/max` — webhook.
3. **Docker Compose**, профили:
   - базовый (одна команда `docker compose up --build`): `api`, `miniapp` (сборка статики), `postgres`, `minio`, `caddy`;
   - `llm`: `ollama` (модель скачивается при старте);
   - локально без публичного HTTPS: бот в режиме long polling (`BOT_MODE=polling`).
4. **Сертификат Минцифры** (Russian Trusted Root CA) добавляется в доверенные сертификаты образа `api` при сборке.
5. **Секреты** — только через переменные окружения (`.env` на сервере, `.env.example` в репозитории без значений).
6. **Сборка:** многоэтапные Dockerfile, кэш Go-модулей и npm, `.dockerignore`; время сборки замеряется в CI.
7. **Стенд проверки** замораживается после code freeze; healthcheck и `restart: unless-stopped`.

## Рассмотренные альтернативы
| Вариант | Почему не выбран |
|---|---|
| VK Cloud хостинг только мини-приложения | бэкенд с webhook всё равно нужен |
| Туннели (ngrok и т. п.) | нестабильный URL, модерация настроек бота |
| Зарубежный VPS | 152-ФЗ, риск доступности |
| nginx + certbot | больше ручной настройки, чем Caddy |

## Последствия
- **Плюсы:** воспроизводимость одной командой; стабильные URL для модерации; соответствие 152-ФЗ.
- **Минусы:** затраты на VPS и домен; ресурсы под Ollama ([R20](../research/risks.md)).
- **Задачи (Спайк 0, без токена):** VPS, домен, DNS, Caddy, каркас compose, hello-world мини-приложения по HTTPS, проверка доступа к `platform-api2` с сертификатом.

## Связанные требования
DEP-01…DEP-08, SEC-01, SEC-04 ([nfr](../requirements/nfr.md)).
