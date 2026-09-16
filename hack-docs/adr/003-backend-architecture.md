---
title: ADR-003 Архитектура бэкенда
type: adr
status: accepted
date: 2026-09-17
deciders: [команда]
supersedes: —
tags: [architecture, go, fiber, postgres, minio]
---

# ADR-003: Архитектура бэкенда — модульный монолит на Go

## Контекст
- Две недели, 4 человека в фулстек-парах и агенты → нужны простые и понятные границы модулей, чтобы параллельная работа не ломала целостность ([R17](../research/risks.md)).
- Критерий «архитектура» (20% техники) оценивает логичность структуры, разделение ключевой логики и соответствие масштабу MVP.
- Выбор стека сделан в интервью [Блок В](../interview/BLOCK-V.md).
- Ограничения платформы MAX: webhook с ответом ≤ 30 с, дубли событий, 2 rps на чат, сертификат Минцифры ([max-platform-capabilities](../research/max-platform-capabilities.md)).

## Решение

### Стек
| Слой | Выбор |
|---|---|
| Язык | Go (актуальная стабильная версия, фиксируется в `go.mod` в Спайке 0; перед правками — `modern-go-guidelines list`) |
| HTTP | **Fiber v3** — и для API мини-приложения, и для webhook бота |
| БД | **PostgreSQL** + **pgx** + **sqlc** |
| Миграции | **goose** (SQL-файлы) |
| Файлы | **MinIO** (S3 API) — фото и PDF |
| Bot API | официальный `github.com/max-messenger/max-bot-api-client-go` за интерфейсом `MaxBotGateway`; недостающие методы (новые API) — тонкие HTTP-вызовы в том же адаптере |
| LLM | Ollama за интерфейсом `LLMAssistant` ([ADR-008](008-llm-self-hosted.md)) |
| PDF | Go-библиотека с поддержкой кириллицы (выбрать в Спайке 0 по context7) |

Версии библиотек сверяются через context7 при создании `go.mod`; стабильность Fiber v3 проверяется там же.

### Структура (плановая)
```
backend/
  cmd/server/            # точка входа: HTTP (API + webhook) и фоновые воркеры
  internal/
    domain/              # сущности и правила: Issue, Participation, DeadlinePolicy… без зависимостей
    app/                 # сценарии: CreateIssue, JoinIssue, ChangeStatus, ResolveResponsibility…
    ports/               # интерфейсы: MaxBotGateway, AddressDirectory, HouseRegistry,
                         # ResponsibilityRules, LLMAssistant, FileStorage, Clock
    adapters/
      http/              # Fiber: API мини-приложения, валидация по OpenAPI, auth-middleware
      maxbot/            # webhook-хендлер, диалоги бота, отправка и редактирование карточек
      postgres/          # sqlc-репозитории, миграции goose
      minio/             # FileStorage
      llm/               # Ollama + откат на правила
      gar/, gisgkh/      # данные Москвы и мок ГИС ЖКХ
    worker/              # очередь исходящих сообщений, таймеры сроков
  db/queries/, db/migrations/
```

### Ключевые механизмы
1. **Webhook:** проверить `X-Max-Bot-Api-Secret` → записать событие в `processed_updates` (уникальный ключ — дедупликация) → ответить 200 сразу → обработать асинхронно.
2. **Исходящие сообщения:** outbox-таблица + воркер с лимитами 30 rps глобально и 2 rps на чат, ретраи с backoff, обработка `attachment.not.ready`.
3. **Живая карточка:** хранить `mid` карточки у каждого участника (`bot_message_refs`); при смене статуса поставить `PUT /messages` в outbox.
4. **Сроки:** справочник `DeadlineRule` + производственный календарь; периодический воркер отмечает просрочку и ставит уведомления участникам.
5. **Порты и адаптеры:** доменная логика не знает о Fiber, MAX, Ollama и PostgreSQL → юнит-тесты домена без инфраструктуры (TDD).
6. **Совместимость Fiber и net/http:** если зависимость требует `http.Handler`, подключаем её через адаптер Fiber.

## Рассмотренные альтернативы
| Вариант | Почему не выбран |
|---|---|
| net/http (стандартный роутинг) | команда выбрала Fiber |
| Микросервисы (бот, API, воркер отдельно) | избыточно для MVP, дольше сборка и отладка |
| GORM | скрывает SQL, хуже контроль запросов; выбран sqlc |
| Файлы на диске | команда выбрала MinIO (готовность к S3 при масштабировании) |

## Последствия
- **Плюсы:** один бинарник и один образ; понятные слои для критерия «архитектура»; замена адаптеров (ГИС ЖКХ, S3, LLM) без правки домена — аргумент масштабирования.
- **Минусы:** Fiber (fasthttp) требует адаптера для net/http-зависимостей ([R23](../research/risks.md)); в compose больше контейнеров (postgres, minio, ollama).
- **Задачи:** каркас в Спайке 0; `sqlc.yaml`; миграции; e2e-тест webhook → карточка.

## Связанные требования
[nfr](../requirements/nfr.md): NFR-01, NFR-02, NFR-06, SEC-04, SEC-06, DEP-01…DEP-04.
