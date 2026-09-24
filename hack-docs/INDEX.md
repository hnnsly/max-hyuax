---
title: Индекс базы знаний проекта
type: index
status: accepted
updated: 2026-09-17
tags: [index, hub]
---

# Индекс базы знаний

**Точка входа для людей и агентов.** Если контекст потерян: [HANDOFF](HANDOFF.md) (что делается сейчас и следующий шаг) → этот файл → [MINDMAP](research/MINDMAP.md) → заметка по задаче. Правила проекта — в [`/CLAUDE.md`](../CLAUDE.md).

**DevOps:** развёртывание на VPS — [DEVOPS.md](DEVOPS.md).

> `hack-docs/` — внутренняя рабочая база, **в сдачу не входит**. Документация продукта для жюри живёт в `docs/` и `README.md` в корне ([ADR-001](adr/001-docs-separation.md)).

## Текущее состояние (17.09.2026)
- **Этап (25.09):** разработка MVP. Готовы бэкенд (Postgres, сценарии, API v1, вход, webhook, outbox с живой карточкой, диалог бота) и мини-приложение (дом, заявка, форма в 3 шага с проверкой дубля, очередь УК со сменой статуса). Образы `api` и `web` собираются. Нужны деплой и проверка в MAX. Команды — `task`. Документация продукта — `docs/`. План на 23–30.09 — в [GENERAL_PLAN](GENERAL_PLAN.md), текущий шаг — в [HANDOFF](HANDOFF.md).
- **Фокус (принят):** ядро H1 + H2 + шеринг из H5; пилот Москва; B2G → SaaS ([ADR-002](adr/002-product-scope.md)).
- **Блокеры:** токен бота не выдан ([R2](research/risks.md)); точный дедлайн неизвестен ([R25](research/risks.md)).
- **Ближайшие вехи:** M0 Спайк 0 — 18.09 (без токена); M1 ядро e2e и ворота экосистемы — 22.09 ([ROADMAP](ROADMAP.md)).

## Правила ведения
1. **Одна мысль — одна заметка.** Новая тема — новый файл, а не раздел в чужом.
2. **Решения — только в ADR** ([реестр](adr/README.md)); заметки ссылаются на ADR, а не дублируют их.
3. **Факт или гипотеза:** каждое утверждение о платформе ссылается на `dev-max/`, о нормативке — на источник; непроверенное помечается «сверить» или «Г».
4. **Frontmatter обязателен:** `title`, `type`, `status` (draft / accepted / answered / superseded), `updated`, `tags`.
5. **Создали, переименовали или изменили статус заметки — обновите таблицу ниже** и «Текущее состояние».
6. Ссылки только относительные markdown (работают в GitHub и Obsidian).
7. Ничего из `hack-docs/` не копируется в `docs/` как есть: документация продукта пишется заново для людей.

---

## Каталог

### Исходные материалы (только чтение)
| Файл | О чём |
|---|---|
| [Case.md](Case.md) | кейс «Умный дом»: задача, ограничения, формат сдачи, критерии (не публиковать — п. 11.3) |
| [hack rules.md](hack%20rules.md) | официальные правила хакатона (сроки, п. 9.5, 9.6, 11.3, 12.3) |
| [max rules.md](max%20rules.md) | пользовательское соглашение MAX (ред. 09.09.2026) |
| `Умный город.pdf` | PDF-версия кейса |
| [dev-max/index.md](dev-max/index.md) | локальная копия документации MAX: Bot API, WebApps и Bridge, MAX UI, help, legal (119 файлов) |

### Исследование
| Файл | О чём | Статус | Обновлён |
|---|---|---|---|
| [research/MINDMAP.md](research/MINDMAP.md) | **хаб**: карта домена, As-Is → To-Be, матрица «бот или мини-приложение», ядро и переменная часть, цепочка бонуса, итоги интервью | accepted | 2026-09-17 |
| [research/max-platform-capabilities.md](research/max-platform-capabilities.md) | Bot API, Bridge по платформам, MAX UI (чего нет), диплинки, ограничения | draft (матрица Bridge уточняется в Спайке 0) | 2026-09-16 |
| [research/legal-framework.md](research/legal-framework.md) | 156-ФЗ, Приказ 856/пр, ПП 416 и ПП 40, письмо Минстроя, 463-ФЗ, 59-ФЗ, ГАР, ГИС ЖКХ, ПДн, правила MAX | draft (пункты «сверить») | 2026-09-16 |
| [research/competitors.md](research/competitors.md) | Госуслуги.Дом, домовые чаты, MAX Основа, московский контекст | accepted | 2026-09-17 |
| [research/problems.md](research/problems.md) | проблемы P1–P17, ключевая формулировка, подтверждение без полевых интервью | accepted | 2026-09-17 |
| [research/product-hypotheses.md](research/product-hypotheses.md) | H1–H5, оценка, решение | accepted | 2026-09-17 |
| [research/platform-bonus.md](research/platform-bonus.md) | условия +0.15, сквозная цепочка, деградация на web | draft | 2026-09-16 |
| [research/judging-criteria.md](research/judging-criteria.md) | критерии → артефакты; чек-лист сдачи | accepted | 2026-09-16 |
| [research/risks.md](research/risks.md) | реестр рисков R1–R25 | accepted | 2026-09-17 |

### Требования
| Файл | О чём | Статус | Обновлён |
|---|---|---|---|
| [SRS.md](SRS.md) | хаб требований: область, роли, FR (ONB, BOT, LLM, RESP, ISSUE, COLL, NOTIF, SLA, ORG, ESC, INFO, PRIV, POLL, ECO), интерфейсы, трассировка | accepted v1.0 | 2026-09-17 |
| [requirements/user-stories.md](requirements/user-stories.md) | истории US-01…US-40 с критериями приёмки | accepted | 2026-09-17 |
| [requirements/ux-flows.md](requirements/ux-flows.md) | потоки бота и мини-приложения, карта экранов, живая карточка, команды, тон | accepted | 2026-09-17 |
| [requirements/nfr.md](requirements/nfr.md) | NFR-01…11, SEC-01…14, DEP-01…08 | accepted | 2026-09-17 |
| [requirements/data-model.md](requirements/data-model.md) | сущности (ER), источники данных Москвы, маркировка, seed и демо-роли | accepted | 2026-09-17 |

### Дизайн
| Файл | О чём | Статус | Обновлён |
|---|---|---|---|
| [design/DESIGN-SYSTEM.md](design/DESIGN-SYSTEM.md) | визуальная система «Адресная табличка»: токены, шрифты, форма, движение, компоненты, шаблон бота, тексты, привязка к MAX UI, запреты | accepted | 2026-09-17 |
| [design/tokens.css](design/tokens.css) | CSS-переменные light/dark + переопределение переменных MAX UI | accepted | 2026-09-17 |
| [design/prototype/index.html](design/prototype/index.html) | кликабельный прототип (житель, УК, бот, QR-наклейка); опубликован приватным артефактом | accepted | 2026-09-17 |

### План
| Файл | О чём | Статус | Обновлён |
|---|---|---|---|
| [GENERAL_PLAN.md](GENERAL_PLAN.md) | этапы 17–30.09, Спайк 0 без токена и с токеном, пары, ворота качества | accepted v1 | 2026-09-17 |
| [ROADMAP.md](ROADMAP.md) | вехи M0–M5, эпики с FR и историями, пост-MVP и финал | accepted | 2026-09-17 |

### Решения (ADR)
| Файл | О чём | Статус | Обновлён |
|---|---|---|---|
| [adr/README.md](adr/README.md) | реестр и шаблон | accepted | 2026-09-17 |
| [adr/001-docs-separation.md](adr/001-docs-separation.md) | `hack-docs/` и `docs/`, правило «без ИИ-меток» | accepted | 2026-09-16 |
| [adr/002-product-scope.md](adr/002-product-scope.md) | фокус, MoSCoW, ворота экосистемы, сегмент, Москва, модель, тон | accepted | 2026-09-17 |
| [adr/003-backend-architecture.md](adr/003-backend-architecture.md) | Go, Fiber v3, pgx + sqlc, PostgreSQL, MinIO, порты и адаптеры, outbox | accepted | 2026-09-17 |
| [adr/004-auth-and-roles.md](adr/004-auth-and-roles.md) | initData → сессия, роли, демо-вход | accepted | 2026-09-17 |
| [adr/005-hosting-https.md](adr/005-hosting-https.md) | российский VPS, Caddy, compose, сертификат Минцифры | accepted | 2026-09-17 |
| [adr/006-api-contract.md](adr/006-api-contract.md) | OpenAPI 3.1 contract-first, объявленное API, DATA-API.yaml | accepted | 2026-09-17 |
| [adr/007-submission-packaging.md](adr/007-submission-packaging.md) | чистый репозиторий для сдачи, экспорт и проверки | accepted | 2026-09-17 |
| [adr/008-llm-self-hosted.md](adr/008-llm-self-hosted.md) | Ollama только как подсказка, откат на правила | accepted | 2026-09-17 |
| [adr/009-visual-identity.md](adr/009-visual-identity.md) | «Адресная табличка»: нативная база + фирменный слой | accepted | 2026-09-17 |
| [adr/010-backend-layers.md](adr/010-backend-layers.md) | бэкенд: три слоя (транспорт, приложение, хранилище) + DDD-lite, список YAGNI | accepted | 2026-09-23 |
| [adr/011-miniapp-typescript.md](adr/011-miniapp-typescript.md) | мини-приложение на TypeScript вместо JS | accepted | 2026-09-23 |
| [adr/012-taskfile.md](adr/012-taskfile.md) | Taskfile.yml — единая точка команд, `.env` в корне для локального запуска | accepted | 2026-09-24 |
| [adr/013-submission-run-and-data-api.md](adr/013-submission-run-and-data-api.md) | запуск для жюри одной командой из `deploy/.env.example`, формат `DATA-API.yaml` 1.0 и его проверка в CI | accepted | 2026-09-24 |
| [adr/014-llm-model-and-prompt.md](adr/014-llm-model-and-prompt.md) | подсказка категории через Ollama: модель qwen3:4b, русские названия в enum, откат на ключевые слова, прогрев | accepted | 2026-09-24 |
| [adr/015-photo-storage.md](adr/015-photo-storage.md) | фото к заявке: том и порт FileStore вместо MinIO, перекодирование без EXIF, доступ участникам и УК | accepted | 2026-09-24 |
| [HANDOFF.md](HANDOFF.md) | заметка передачи: кто в работе, что сделано, следующий шаг | active | 2026-09-23 |
| [design/canvas-v2/project/](design/canvas-v2/project/canvas.json) | утверждённый дизайн v2: 12 артбордов (житель, УК, бот, наклейка, тёмная тема, разбор системы) | accepted | 2026-09-23 |

### Интервью
| Файл | О чём | Статус | Обновлён |
|---|---|---|---|
| [interview/BLOCK-A.md](interview/BLOCK-A.md) | блокеры, продуктовый фокус и ЦА | answered | 2026-09-17 |
| [interview/BLOCK-B.md](interview/BLOCK-B.md) | UX: бот и мини-приложение | answered | 2026-09-17 |
| [interview/BLOCK-V.md](interview/BLOCK-V.md) | интеграции, данные, стек, LLM | answered | 2026-09-17 |
| [interview/BLOCK-G.md](interview/BLOCK-G.md) | команда, процесс, сдача | answered | 2026-09-17 |

### Вне `hack-docs/`
| Файл | О чём |
|---|---|
| [`/CLAUDE.md`](../CLAUDE.md) | правила проекта для Claude Code и агентов (в сдачу не входит) |
| [`/AGENTS.md`](../AGENTS.md) | указатель на CLAUDE.md и INDEX + блок lean-ctx |

### Будет создано
| Файл | Когда |
|---|---|
| `adr/010…` — структура мини-приложения; справочник ответственности и сроков; модель LLM и PDF-библиотека | этап 2 / Спайк 0 |
| название продукта (плейсхолдер `‹Название›` в дизайне) | до вёрстки наклейки |
| `research/bridge-support-matrix` (или обновление max-platform-capabilities) | Спайк 0 с токеном |
| `docs/*`, `README.md` (продукт) | этапы 3–4, пишут люди |
