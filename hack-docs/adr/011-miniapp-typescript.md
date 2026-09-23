---
title: ADR-011 Мини-приложение на TypeScript
type: adr
status: accepted
date: 2026-09-23
deciders: [команда]
supersedes: часть ADR-006 про JavaScript
tags: [frontend, typescript, miniapp]
---

# ADR-011: Мини-приложение на TypeScript

## Контекст
- В интервью выбирали JavaScript, чтобы фронтенд могли читать все участники ([Блок В](../interview/BLOCK-V.md)).
- 23.09 решено: фронтенд полностью пишет Claude, команда его код не читает. Главным становится число ошибок интеграции, а не порог входа.
- MAX UI написан на TypeScript.

## Решение
- **Стек:** Vite + React + **TypeScript** (strict) + `@maxhub/max-ui` + MAX Bridge.
- **Структура:** `miniapp/src/{app,shared/{api,bridge,ui,theme},pages}`.
- **Типы:** типы API пишутся вручную по `api/openapi.yaml` (генератор не тянем, YAGNI).
- **Библиотеки:** без TanStack Query и Redux; свои хуки загрузки и мутаций с состояниями «загрузка, ошибка, пусто».
- **Дизайн:** по холсту v2 (`hack-docs/design/canvas-v2/`) и [DESIGN-SYSTEM](../design/DESIGN-SYSTEM.md).
- **Демо-режим:** `?demo=resident|uk` для проверки в обычном браузере и для жюри.

## Последствия
- Ошибки контракта ловятся при `npm run typecheck`, а не на экране.
- В [ADR-006](006-api-contract.md) пункт «JS-клиент — тонкий fetch-слой» читается как «TS-клиент».
