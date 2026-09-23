---
title: Заметка передачи работы
type: handoff
status: active
updated: 2026-09-24
tags: [handoff, status]
---

# Передача работы

Правило команды: кто пишет, тот пишет. Закончив порцию, он обновляет эту заметку — что сделано, что дальше, как запустить, что сломано.

## Сейчас (24.09, ночь)
- **Кто в работе:** Claude — бэкенд готов к деплою, дальше мини-приложение.
- **Бэкенд готов к деплою (этап Б в [DEVOPS.md](DEVOPS.md)).**
- **Сделано 24.09 (спайк «БД и API»):**
  - `Taskfile.yml` в корне — единая точка команд (`task` покажет список); `.env` в корне для локальной разработки;
  - `.claude/settings.json` с `permissions.deny` (node_modules, dist, bin, exe, tmp, .obsidian, .idea, .vscode). `.claudeignore` Claude Code не поддерживает, файла нет;
  - Postgres 18.6 в compose; миграции goose (`00001_init`, `00002_demo_seed` — Ореховый бульвар, УК «Ореховый квартал», демо-пользователи, 3 заявки); sqlc (`go tool sqlc generate`, код в `postgres/sqlcdb`);
  - репозитории `storage/postgres` + интеграционные тесты на временной базе (`pgtest.Fresh`);
  - слой `app`: порты (`ports.go`), `issues` (Report, Join, ChangeStatus, Get, ListByHouse, FindSimilar, Queue), `auth` (initData по документации MAX, сессии HMAC на 12 ч, демо-вход, согласие, дом, удаление аккаунта), `houses`; хранилище в памяти `apptest` для юнит-тестов;
  - HTTP на Fiber v3 (`transport/httpapi`), 18 путей `/api/v1` + `POST /webhook/max`; E2E-тест сценария на настоящей базе;
  - webhook бота: секрет, дедупликация `processed_updates`, фоновая обработка; `maxapi.Subscribe`; `BOT_MODE=off|polling|webhook`;
  - `backend/Dockerfile` (образ 35 МБ, сертификаты Минцифры с Госуслуг; проверено: бот авторизуется в MAX из контейнера без `insecure`), сервис `api` в compose, integration-тесты в CI;
  - `api/openapi.yaml` v1; `docs/architecture.md`, `docs/api.md`, `docs/data.md`.
- **Проверено руками:** `task run` + `curl` по сценарию демо-вход → поиск дома → заявка №104 → «это и у меня» → 403 жителю на статус → статус от УК → очередь УК → карточка с УК и основанием.

## Следующий шаг
1. **DevOps (люди), СРОЧНО:** письмо `PRIVET_OT_CLAUDE_FOR_DEVOPS.md` — заглушка по HTTPS и URL мини-приложения на модерацию (до 48 рабочих часов); затем этап Б (`docker compose up -d --build`).
2. **Claude, 25.09 (по плану):**
   - каркас `miniapp/` (Vite + React + TS strict + MAX UI) по холсту v2: Home, Issue; клиент API по `openapi.yaml`; демо-режим `?demo=`;
   - outbox + живая карточка в боте (`PUT /messages`, лимиты 2 rps на чат);
   - диалог бота: простая заявка текстом, гео → ближайшие дома, кнопка `open_app` с payload;
   - задачи `miniapp:*` в Taskfile, сборка мини-приложения в `web` (Caddy отдаёт `miniapp/dist`).

## Как запустить
Всё через `task` (список: `task`), переменные — из `.env` в корне.
```bash
task db                 # Postgres в Docker
task test               # юнит-тесты
task test:integration   # тесты на Postgres
task run                # api на :8080, миграции при старте, BOT_MODE=off
```
Демо-вход: `POST /api/v1/auth/demo {"role":"resident"|"resident_2"|"uk_operator"}`.

## Известные проблемы
- **Кодировка в консоли Windows:** `curl` из Git Bash отправляет кириллицу в cp1251. API отвечает 422 `invalid_input` (так и должно быть). Для ручных проверок: тело из файла в UTF-8 (`--data-binary @file.json`), в query — `%D0%BA`.
- **После включения webhook на сервере** long polling перестаёт работать у всех: локально держать `BOT_MODE=off`.
- **`timeout N docker compose run …` не останавливает контейнер:** останавливать через `docker stop`.
- **Git предупреждает про LF → CRLF** (`core.autocrlf`). Пока не мешает: Dockerfile и Go к этому нечувствительны.
- **Документация `Update` в `dev-max/`** описывает только `bot_added`. Поля `bot_started` и `message_callback` взяты из `POST /answers`; первые реальные события сверить в логах.
- **Сроки в `rules`** — модельные, праздники не учитываются (записано в `docs/data.md`).

## Открытые вопросы к пользователю
- Домен: после этапа А DevOps пришлёт его в чат команды, внести в эту заметку.
