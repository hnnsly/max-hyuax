---
title: Интервью — Блок В. Интеграции, данные, стек
type: interview
status: answered
updated: 2026-09-17
tags: [interview, integrations, data, stack, llm]
---

# Интервью. Блок В: интеграции, данные и технологический стек

Проведено 17.09.2026 в plan mode. Решения — в [ADR-003](../adr/003-backend-architecture.md), [ADR-004](../adr/004-auth-and-roles.md), [ADR-005](../adr/005-hosting-https.md), [ADR-006](../adr/006-api-contract.md), [ADR-008](../adr/008-llm-self-hosted.md); данные — [requirements/data-model](../requirements/data-model.md).
Предыдущий блок: [Б](BLOCK-B.md) · следующий: [Г](BLOCK-G.md)

| № | Вопрос | Варианты | Ответ |
|---|---|---|---|
| В1 | Объявлять ли бэкенд «собственным API» | да · нет | **Да, объявляем** (HTTPS, openapi.yaml, DATA-API.yaml, тестовые учётки) |
| В2 | Данные о домах и адресах Москвы | открытые данные одного района · синтетика · DaData | **Открытые данные одного района** + синтетические заявки с пометкой |
| В3 | LLM-провайдер | GigaChat · YandexGPT · Gemini через OpenRouter · решим позже | **Self-hosted модель в Ollama** («или подобная китайская открытая модель») |
| В4 | Где VPS | российский · зарубежный · уже есть | **Российский VPS** |
| В5 | Стек мини-приложения | Vite + React + TS + MAX UI · Next.js · свой UI | **Vite + React + MAX UI** |
| В5b | TypeScript или JavaScript | TS · JS | **JavaScript** |
| В6 | Стек бэкенда | net/http + pgx + sqlc + PostgreSQL · chi/echo + GORM · SQLite | Ответ «net/http + fiber + pgx + sqlc + PostgreSQL» → уточнение ниже |
| В6b | HTTP-слой (Fiber несовместим с net/http) | Fiber v3 везде · net/http · смешанно | **Fiber v3 везде** |
| В7 | Работа с Bot API из Go | официальный клиент · фреймворк maxbot · свой клиент | **Официальный `max-bot-api-client-go`** |
| В8 | Хранение фото и PDF | volume на диске · MinIO · загрузка в MAX | **MinIO (S3) в Docker** |

## Итоги блока
- **Стек:** Go + Fiber v3 + pgx + sqlc + PostgreSQL + MinIO + Ollama; фронт Vite + React (JS) + MAX UI; бот через официальный Go-клиент за собственным интерфейсом.
- **Следствия выбора:**
  - **Fiber** — вебхук и API на одном фреймворке; зависимости с интерфейсом net/http подключаются через адаптер ([R23](../research/risks.md)).
  - **JS без типов:** контракт проверяется на бэкенде валидацией по OpenAPI и e2e; на фронте JSDoc для данных API ([R24](../research/risks.md)).
  - **Ollama:** модель скачивается при старте, а не при сборке (сборка ≤ 5 мин); нужны ресурсы VPS; при недоступности — откат на правила ([R20](../research/risks.md)).
  - **Российский VPS** закрывает требование локализации ПДн (152-ФЗ).
- **Открыто для Спайка 0:** конкретная модель и её лицензия, замер задержки на VPS, конкретные наборы data.mos.ru и выбор района.
