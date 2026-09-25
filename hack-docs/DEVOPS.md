---
title: Инструкция для DevOps — развёртывание на VPS
type: runbook
status: active
updated: 2026-09-23
tags: [devops, deploy, runbook]
---

# Развёртывание на VPS

Коротко: на сервере крутится Docker Compose из папки `deploy/`. Caddy сам получает HTTPS-сертификат и раздаёт мини-приложение, а когда будет готов бэкенд — проксирует на него `/api` и `/webhook`.

Выкатываем в два этапа:
- **этап А — сегодня:** только веб-сервер с мини-приложением по HTTPS (без бэкенда откроется экран входа);
- **этап Б — когда в [HANDOFF](HANDOFF.md) появится пометка «бэкенд готов к деплою»:** всё остальное.

---

## Что нужно заранее
- VPS в России, Docker 24+ с плагином `docker compose`.
- Домен: A-запись указывает на IP сервера (проверка: `dig +short <домен>` возвращает этот IP).
- Открыты входящие порты **80** и **443**. На 80 приходит проверка Let's Encrypt, на 443 — webhook MAX (другие порты MAX не принимает).
- Доступ к приватному репозиторию проекта.

## Этап А. Мини-приложение по HTTPS (сделать сегодня)
Зачем: адрес мини-приложения нужно сразу прописать в настройках бота, а каждая такая правка проходит модерацию до 48 рабочих часов. Поэтому адрес ставим сейчас, а код за ним будем менять сколько угодно.

1. Скачать репозиторий и заполнить `.env`:
   ```bash
   git clone <репозиторий> dom-max && cd dom-max/deploy
   cp .env.example .env
   nano .env            # DOMAIN=ваш.домен
   ```
2. Собрать и запустить веб-сервер (образ собирает мини-приложение, около 1,5 минуты):
   ```bash
   docker compose up -d --build web
   docker compose logs -f web     # ждать строку про успешно выданный сертификат
   ```
3. Проверить: `https://<домен>/` открывается в браузере без предупреждений о сертификате и показывает экран «Заявки по дому в MAX». Демо-вход заработает после этапа Б.
4. **Прописать адрес в боте**, сразу после проверки:
   - открыть https://business.max.ru/self → «Чат-боты» → бот `t105_hakaton_max_bot` → «⋮» → «Настройки»;
   - в поле ссылки на мини-приложение вписать `https://<домен>/`, вид кнопки — «Открыть», нажать «Сохранить»;
   - написать в чат команды, когда отправили на модерацию и когда её прошли.
5. Сообщить домен в чат команды: он нужен для `.env` и проверки на этапе Б.

## Этап Б. Полный стек (бэкенд готов с 24.09)
1. Обновить код и дописать переменные в `deploy/.env` по `deploy/.env.example`:
   ```bash
   cd dom-max && git pull
   cd deploy && nano .env
   ```
   Что заполнить:
   - `POSTGRES_PASSWORD`;
   - `SESSION_SECRET` (`openssl rand -hex 32`);
   - `DEMO_AUTH_ENABLED=true` на демо-стенде;
   - `BOT_MODE=webhook`;
   - `MAX_BOT_TOKEN` (токен в `CLAUDE.md`);
   - `MAX_WEBHOOK_SECRET` (5–256 символов `A-Z a-z 0-9 _ -`).

   Сертификат Минцифры уже встроен в образ `api`, ставить его на сервер не нужно.

   **Важно:** после включения webhook long polling у разработчиков перестаёт работать (так устроен MAX). Локально держим `BOT_MODE=off`.
2. Собрать и запустить всё:
   ```bash
   docker compose up -d --build
   docker compose ps          # все сервисы в состоянии running или healthy
   ```
3. Webhook бота регистрируется сам при старте бэкенда (`BOT_MODE=webhook`). Проверка: `docker compose logs api | grep webhook`.
4. Проверить: `https://<домен>/api/v1/health` отвечает `{"status":"ok"}`, а в MAX бот отвечает на `/start`.

## Автоматический деплой через GitHub Actions + Ansible

Пайплайн `.github/workflows/ci.yml` состоит из трёх стадий:
1. **validate** (`validate-backend` и `validate-miniapp`): проверка форматирования, линтеры, unit/integration-тесты, порог покрытия ≥ 80%, сборка.
2. **build**: сборка и публикация образов `api` и `web` в GitHub Container Registry (`ghcr.io/<repo>/api:<sha>` и `ghcr.io/<repo>/web:<sha>`) с кэшированием слоёв `--cache-from`.
3. **deploy**: запуск `deploy/ansible/playbook.yml` — подключение к серверу по SSH, логин в `ghcr.io`, рендеринг `docker-compose.yml` из `deploy/ansible/templates/docker-compose.yml.j2`, `docker compose pull && docker compose up -d --remove-orphans`.

### Настройка перед первым автодеплоем
1. Заполните параметры сервера в `deploy/ansible/vault.yml` (по образцу `deploy/ansible/vault.yml.example`) и зашифруйте его:
   ```bash
   ansible-vault encrypt deploy/ansible/vault.yml
   ```
2. На целевом VPS создайте каталог деплоя (например, `/opt/dom-max`) и положите туда `.env` с настройками `DATABASE_URL`, `SESSION_SECRET`, `MAX_BOT_TOKEN`, `MAX_WEBHOOK_SECRET`, `DOMAIN`.
3. Настройте внешний веб-сервер (Nginx/Caddy) на проксирование трафика домена на `127.0.0.1:10380` (или порт, указанный в `host_port` внутри `vault.yml`).
4. Добавьте в GitHub Secrets (`Settings → Secrets and variables → Actions`):
   - `SSH_PRIVATE_KEY` — приватный SSH-ключ пользователя деплоя (в формате PEM или base64);
   - `ANSIBLE_VAULT_PASSWORD` — пароль от `deploy/ansible/vault.yml`;
   - `DEPLOY_USER` и `DEPLOY_TOKEN` (опционально, если для скачивания приватных пакетов с `ghcr.io` нужен отдельный PAT с правом `read:packages`).

## Обычные операции
| Задача | Команда (из `deploy/`) |
|---|---|
| Обновить после `git pull` | `docker compose up -d --build` |
| Логи | `docker compose logs -f --tail=200 <web\|api\|db>` |
| Остановить | `docker compose down` (данные в volume сохраняются) |
| Перезапустить сервис | `docker compose restart <сервис>` |
| Полный сброс данных | `docker compose down -v`, **только по согласованию**: удаляет базу |

## Если что-то не так
- **Нет сертификата:** A-запись ещё не обновилась или закрыт порт 80. Проверить `dig` и фаервол, затем `docker compose restart web`.
- **Бот молчит на этапе Б:** проверить в логах `api` регистрацию webhook. Порт 443 должен быть открыт снаружи, домен — совпадать с `DOMAIN`.
- **Мини-приложение в MAX не открывается:** настройки бота ещё на модерации, либо адрес в настройках не совпадает с доменом.
