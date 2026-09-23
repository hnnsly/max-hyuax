---
title: Заметка передачи работы
type: handoff
status: active
updated: 2026-09-23
tags: [handoff, status]
---

# Передача работы

Правило команды: кто пишет, тот пишет. Закончив порцию, он обновляет эту заметку — что сделано, что дальше, как запустить, что сломано.

## Сейчас (23.09, вечер)
- **Кто в работе:** Claude — бэкенд, затем мини-приложение.
- **Сделано:**
  - ADR-010 (три слоя + DDD-lite; исключение для `transport/bot` → `storage/maxapi`), ADR-011 (TypeScript);
  - `backend/` на Go 1.27.1, модуль `dommax`;
  - `domain/issue` — агрегат: статусы и переходы, «это и у меня», просрочка, доменные события; тесты;
  - `domain/rules` — справочник категорий (lift, leak, heating, lighting, door, garbage, other): ответственный, основание, срок в рабочих днях, ключевые слова для классификатора; тесты;
  - `storage/maxapi` — тонкий клиент Bot API: `Me`, `Updates`, `Send`, `Edit`, `Answer`, построители кнопок; TLS с сертификатом Минцифры или `MAX_API_INSECURE_TLS`; тесты + live-тест на реальном токене (прошёл);
  - `transport/bot` — обработчик событий (`bot_started`, `/start` и любой текст → приветствие с кнопками «Сообщить о проблеме» и «Открыть приложение»; в групповых чатах бот молчит) и long polling; тесты;
  - `cmd/api` — конфиг из env, проверка токена через `/me`, запуск polling.
  - `domain/user` — роли (житель, оператор УК своей организации), согласие на ПДн по версии документа, удаление с обезличиванием; тесты;
  - `domain/house` — модели чтения без поведения: дом, подъезд, объект с QR, организация;
  - комментарии в коде переведены на русский (новое правило в CLAUDE.md).
- **Не проверено руками:** ответ бота на `/start` в самом MAX. Шелл агента не даёт держать процесс; проверка — команда ниже, затем `/start` в https://max.ru/t105_hakaton_max_bot.

## Следующий шаг
1. **DevOps (люди), СРОЧНО:** письмо в корне `PRIVET_OT_CLAUDE_FOR_DEVOPS.md`:
   - заглушка по HTTPS и URL мини-приложения на модерацию (до 48 рабочих часов);
   - секреты для автодеплоя (`.github/workflows/ci.yml`);
   - по возможности `backend/Dockerfile` и сервис `api` в compose, бот в режиме polling.
2. **Claude, 24.09:**
   - миграции goose + sqlc, репозитории;
   - сценарии Report / Join / ChangeStatus / Get / List / FindSimilar (слой `app`, порты в `app/ports.go`);
   - HTTP на Fiber v3 (документация через context7): `/api/v1/health`, auth (initData HMAC+TTL, `/auth/demo`);
   - webhook `POST /webhook/max` с проверкой `X-Max-Bot-Api-Secret` и дедупликацией `processed_updates`; `BOT_MODE=webhook`;
   - `api/openapi.yaml` v1; первые разделы `docs/`.

**Статус деплоя:** этап А ждёт DevOps; этап Б — после пометки «бэкенд готов к деплою» в этой заметке.

## Как запустить
Команды — в `CLAUDE.md`, раздел «Команды». Бот локально (из `backend/`, Git Bash):
```bash
MAX_BOT_TOKEN=<токен из CLAUDE.md> MAX_API_INSECURE_TLS=true go run ./cmd/api
```

## Известные проблемы
- `platform-api2.max.ru` использует сертификат Минцифры. Локально — `MAX_API_INSECURE_TLS=true` или `MAX_API_CA_FILE=<pem>`; в Docker-образ кладём корневой сертификат (сделать в Dockerfile на 28.09).
- lean-ctx не пропускает запуск собственных бинарников и `curl.exe` (и через `timeout`). Живые проверки API — через `go test -tags live`.
- Документация объекта `Update` в `dev-max/` описывает только `bot_added`; поля `bot_started` (`chat_id`, `user`, `payload`) и `message_callback` (`callback.callback_id`, `callback.payload`) взяты из `POST /answers` и формата API. Первые реальные события стоит сверить в логах.

## Открытые вопросы к пользователю
- Домен: после этапа А DevOps пришлёт его в чат команды, внести в эту заметку.
