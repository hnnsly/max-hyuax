---
title: Возможности платформы MAX для продукта
type: research
status: draft
updated: 2026-09-16
tags: [max, bot-api, bridge, max-ui, deeplinks]
sources: hack-docs/dev-max/**
---

# Возможности платформы MAX

Выжимка из локальной документации [`../dev-max/`](../dev-max/index.md): каждое утверждение ссылается на файл. «Не указано» — документация молчит, проверяем в Спайке 0 ([GENERAL_PLAN](../GENERAL_PLAN.md)).

Связанные заметки: [MINDMAP](MINDMAP.md) · [platform-bonus](platform-bonus.md) · [risks](risks.md)

---

## 1. Архитектурная рамка платформы

- Мини-приложение живёт только внутри чат-бота: без бота оно не существует. Кнопка запуска появляется в чате с ботом ([webapps/introduction.md](../dev-max/docs/webapps/introduction.md), [help/miniapps.md](../dev-max/help/miniapps.md)).
- Мини-приложение — это обычный веб (HTML/JS/CSS) по HTTPS. URL задаётся в настройках бота на business.max.ru: не длиннее 1024 символов, только `https://` ([webapps/introduction.md](../dev-max/docs/webapps/introduction.md)).
- **Любая правка настроек бота уходит на модерацию (до 48 рабочих часов)**, пока идёт модерация, работает старая версия. Ник бота не меняется ([chatbots/bots-create/manage.md](../dev-max/docs/chatbots/bots-create/manage.md), [help/chatbots.md](../dev-max/help/chatbots.md)).
- Добавление бота в групповые чаты **по умолчанию выключено** (настройка «Приватность») ([chatbots/bots-create/manage.md](../dev-max/docs/chatbots/bots-create/manage.md)).
- При статичном URL новый деплой мини-приложения доступен пользователям сразу, модерация не нужна ([help/miniapps.md](../dev-max/help/miniapps.md)).
- Одновременно открыто только одно мини-приложение ([help/miniapps.md](../dev-max/help/miniapps.md)).

---

## 2. Bot API

### 2.1 Транспорт
| Параметр | Значение | Источник |
|---|---|---|
| Базовый URL | `https://platform-api2.max.ru` (с 19.07.2026), в доверенных должен быть сертификат Минцифры | [docs-api/index.md](../dev-max/docs-api/index.md), [changelog-api.md](../dev-max/docs-api/changelog-api.md) |
| Авторизация | заголовок `Authorization: <token>` (query-параметр больше не работает) | [docs-api/index.md](../dev-max/docs-api/index.md) |
| Лимиты | 30 rps глобально; 2 rps на один чат для `POST/PUT/DELETE /messages` и `POST /answers` | [docs-api/index.md](../dev-max/docs-api/index.md), [POST/messages](../dev-max/docs-api/methods/POST/messages/index.md) |
| Webhook | `POST /subscriptions`: только HTTPS на 443, доверенный CA (самоподписанные запрещены с 25.05), ответ 200 не позже 30 с; ретраи 60/150/375 с; через 8 ч сбоев автоотписка; секрет приходит в `X-Max-Bot-Api-Secret` | [POST/subscriptions.md](../dev-max/docs-api/methods/POST/subscriptions.md), [help/events.md](../dev-max/help/events.md) |
| Long polling | `GET /updates` — только для разработки; не работает при активном webhook; для групп бот должен быть админом | [GET/updates.md](../dev-max/docs-api/methods/GET/updates.md) |
| Дубли | одно событие может прийти повторно → нужна идемпотентная обработка | [GET/chats/index.md](../dev-max/docs-api/methods/GET/chats/index.md) |

### 2.2 События (Update)
`bot_started` (с `payload` из диплинка), `bot_stopped`, `bot_added`, `bot_removed`, `message_created`, `message_edited`, `message_removed`, `message_callback`, `user_added`, `user_removed`, `chat_title_changed`, `dialog_cleared/muted/unmuted/removed`, `comment_created/edited/removed` ([objects/Update.md](../dev-max/docs-api/objects/Update.md)). Подробные схемы задокументированы только для `bot_added` и `bot_started`.

### 2.3 Сообщения
- Текст не длиннее 4000 символов; `format: markdown | html` (жирный, курсив, зачёркнутый, подчёркнутый, моноширинный, ссылка, упоминание, выделение `^^…^^`, заголовок, цитата); `notify`; `link` (reply/forward); `disable_link_preview` ([objects/NewMessageBody.md](../dev-max/docs-api/objects/NewMessageBody.md), [docs-api/index.md](../dev-max/docs-api/index.md)).
- Вложения: `image` (до 50 МБ), `video` (до 250 МБ), `audio` (до 256 МБ и 60 мин), `file` (до 4 ГБ, только вместе с клавиатурой), `sticker`, `contact`, `location`, `share`, `inline_keyboard`; всего до 12 штук ([POST/uploads.md](../dev-max/docs-api/methods/POST/uploads.md), [docs-api/index.md](../dev-max/docs-api/index.md)).
- Загрузка: `POST /uploads?type=` → загрузка по выданному URL → `token` во вложении; «attachment.not.ready» — повторить с паузой ([POST/uploads.md](../dev-max/docs-api/methods/POST/uploads.md)).
- **Редактирование** `PUT /messages`: в диалоге сообщение с `inline_keyboard` редактируется без ограничения по сроку, остальные — в течение 7 дней; в группах и каналах — всегда ([PUT/messages](../dev-max/docs-api/methods/PUT/messages/index.md)). → «Живая карточка» заявки реализуема.
- В полученных аудиосообщениях есть поле `transcription` ([objects/Message.md](../dev-max/docs-api/objects/Message.md)).

### 2.4 Inline-клавиатура
| Тип кнопки | Что делает | Лимиты |
|---|---|---|
| `callback` | событие `message_callback`, ответ через `POST /answers` (можно заменить сообщение) | payload ≤ 1024 |
| `link` | открывает URL | url ≤ 2048 |
| `request_contact` | пользователь отправляет контакт; `hash` проверяется HMAC-SHA256(token, vcf_info) | — |
| `request_geo_location` | отправка геопозиции, `quick=true` без подтверждения | — |
| `open_app` | открывает мини-приложение, `payload` попадает в initData | — |
| `message` | отправляет текст кнопки от имени пользователя | — |
| `clipboard` | копирует payload в буфер | payload ≤ 1024 |

Общие ограничения: до 210 кнопок, 30 рядов, 7 кнопок в ряду (3 — если в ряду есть link/open_app/request_*). Текст кнопки 1–128 символов. **Кнопки пропадают при пересылке сообщения.** Reply-клавиатуры нет ([docs-api/index.md](../dev-max/docs-api/index.md), [objects/NewMessageBody.md](../dev-max/docs-api/objects/NewMessageBody.md)).

### 2.5 Группы, каналы, комментарии
- В группе бот может отправлять и бессрочно редактировать свои сообщения, закреплять (`PUT /chats/{id}/pin`), менять название, описание и иконку (`PATCH /chats/{id}`), показывать «печатает» (`/actions`), читать историю и участников — **только будучи админом** с нужными правами (`read_all_messages`, `write`, `pin_message`, `change_chat_info`…) ([POST/chats/-chatId-/members/admins.md](../dev-max/docs-api/methods/POST/chats/-chatId-/members/admins.md)).
- `GET /chats` удалён (06.2026): chat_id собираем сами из событий. `POST /chats/{id}/members` ограничен с 09.09 и **удаляется 30.09.2026** ([changelog-api.md](../dev-max/docs-api/changelog-api.md)).
- **Комментарии к постам каналов** (новое, 08.2026): CRUD `/messages/{id}/comments` и события `comment_*`; текст ≤ 4000, без ссылок, упоминаний и вложений. Бот-модератор: админ канала с `read_all_messages`, `write`, `delete` ([POST/messages/-messageId-/comments.md](../dev-max/docs-api/methods/POST/messages/-messageId-/comments.md), [channels/manage.md](../dev-max/docs/channels/manage.md)).
- В API нет: опросов, реакций, платежей, отложенных сообщений, создания чатов, инвайт-ссылок.

### 2.6 Команды и диплинки
- `PATCH /me/commands` — до 32 команд (07.2026) ([PATCH/me/commands.md](../dev-max/docs-api/methods/PATCH/me/commands.md)).
- Бот: `https://max.ru/<botName>?start=<payload>` — payload **≤ 128 символов**, иначе не передаётся; приходит в `bot_started.payload` ([chatbots/bots-coding/prepare.md](../dev-max/docs/chatbots/bots-coding/prepare.md)).
- Мини-приложение: `https://max.ru/<botName>?startapp=<payload>` — **≤ 512 символов**, только `[A-Za-z0-9_-]`; попадает в `initDataUnsafe.start_param` ([webapps/introduction.md](../dev-max/docs/webapps/introduction.md), [help/deeplinks.md](../dev-max/help/deeplinks.md)).
- Шеринг: `https://max.ru/:share?text=<urlencoded>` — iOS, Android, web; на desktop в разработке.
- Конфиденциальные данные в payload не передавать: только непрозрачные идентификаторы или одноразовые токены.

### 2.7 SDK
- Go: `github.com/max-messenger/max-bot-api-client-go` (версия в доке не указана), фреймворк `github.com/max-messenger/maxbot` (08.2026), пример `github.com/max-messenger/demo-bot-go` (09.2026) ([chatbots/bots-coding/go.md](../dev-max/docs/chatbots/bots-coding/go.md), [changelog-platform.md](../dev-max/docs/changelog-platform.md)).
- JS/TS: `@maxhub/max-bot-api` ([chatbots/bots-coding/js.md](../dev-max/docs/chatbots/bots-coding/js.md)).

---

## 3. MAX Bridge (`window.WebApp`)

Источник: [webapps/bridge.md](../dev-max/docs/webapps/bridge.md), валидация — [webapps/validation.md](../dev-max/docs/webapps/validation.md). Подключение: `<script src="https://st.max.ru/js/max-web-app.js">`.

| Возможность | iOS | Android | Desktop | Web | Применение у нас |
|---|---|---|---|---|---|
| `initData` / `initDataUnsafe` (user, chat, start_param, auth_date, hash) | ✓ | ✓ | ✓ | ✓ | аутентификация: HMAC на бэке, `auth_date` не старше 1 ч |
| `platform`, `version`, `deviceName` | ✓ | ✓ | ✓ | ✓ | деградация функций по платформе |
| `getLaunchContext()` (tabbar/default) | ≥ 26.20.0 | ≥ 26.19.2 | не указано | не указано | — |
| `requestContact()` + HMAC-проверка телефона | не указано | не указано | не указано | не указано | проверенный телефон для обратной связи |
| `enable/disableClosingConfirmation()` | не указано | не указано | не указано | не указано | защита длинной формы |
| `openLink()` / `openMaxLink()` | ✓ | ✓ | ✓ | ✓ | переход в бот/чат/ГИС ЖКХ; `openLink` срабатывает только после клика пользователя |
| `downloadFile(url, name)` | ✓ | ✓ | ✓ | ✓ | PDF обращения; только внутри клиента MAX (не в обычном браузере), только после клика |
| `shareContent({text, link})` | ✓ | ✓ | ✗ | ✗ | внешний шеринг |
| `shareMaxContent({text/link} \| {mid, chatType})` | не указано | не указано | не указано | не указано | шеринг карточки в домовой чат; только после клика |
| `openCodeReader(fileSelect)` | не указано | не указано | не указано | не указано | сканирование QR объекта |
| `BackButton` | не указано | не указано | не указано | не указано | навигация (ограничений в доке нет) |
| `DeviceStorage` / `SecureStorage` (10 ключей) | ✓ | ✓ | не указано | ✗ | черновики (с запасным вариантом) |
| `BiometricManager` | ✓ | ✓ | ✗ | ✗ | не нужно |
| `HapticFeedback` | ✓ | ✓ | ✗ | ✗ | отклик на успех/ошибку |
| `NfcManager` | ✗ | ✓ | ✗ | ✗ | не нужно |
| `requestScreenMaxBrightness`, `ScreenCapture`, `getViewportSize` | не указано | не указано | не указано | не указано | — |

Чего в Bridge **нет**: MainButton, параметров темы, геолокации, `close()`/`ready()`/`expand()`. Геолокация — только кнопкой бота `request_geo_location`. Фото из мини-приложения — стандартный `<input type="file" accept="image/*">` (проверить в Спайке 0 на всех платформах).

---

## 4. MAX UI (`@maxhub/max-ui`)

Источник: [ui/index.md](../dev-max/ui/index.md), [ui/components/](../dev-max/ui/components/Button.md), [ui/compositions/Profile.md](../dev-max/ui/compositions/Profile.md).

- React 18+, TypeScript; обёртка `<MaxUI>` и `@maxhub/max-ui/dist/styles.css`; платформа (`ios`/`android`) и тема (`light`/`dark`) определяются автоматически. Поддержка `asChild`, кастомизация через CSS-переменные. Figma-кит `MAXUI-Figma.fig` лежит в репозитории max-ui.
- **Есть**: Button, IconButton, ToolButton, CellList, CellHeader, CellSimple, CellAction, CellInput, Panel, Container, Flex, Grid, Input, Textarea, SearchInput, Switch, Counter, Dot, Spinner, Avatar.*, Typography.* (Display/Headline/Title/Body/Label/Action), EllipsisText, Ripple, композиция Profile.
- **Нет, делаем сами**: модалка / bottom sheet, табы, навбар, select, выбор даты, toast, checkbox/radio, stepper, skeleton, прогресс, загрузка файлов и фото, галерея, чипы, состояния ошибок у Input, набор иконок.

---

## 5. Сервисы, которые нам недоступны или не подходят
- **Цифровой ID** — подключают только ЮЛ и ИП; подтверждает статусы (18+, студент, пенсионер…), но не собственность и не адрес ([docs/digital-id.md](../dev-max/docs/digital-id.md), [help/digital-id.md](../dev-max/help/digital-id.md)).
- **Каналы** — приватный канал может создать любой пользователь с российским номером; публичный требует верифицированного профиля ([help/channels.md](../dev-max/help/channels.md)).
- **Партнёрские сервисы** (HelpDeskEddy и т. п.) подключаются по токену бота, но это закрытые продукты → в MVP не используем ([docs/partners-integration.md](../dev-max/docs/partners-integration.md)).

---

## 6. Выводы для архитектуры
1. Бэкенд обязан быть публичным по HTTPS:443 с доверенным сертификатом (webhook), а в образе должен быть корневой сертификат Минцифры (исходящие запросы к `platform-api2`).
2. Обработка webhook: быстрый ответ 200 и асинхронная обработка, дедупликация событий, очередь отправки с ограничением 2 rps на чат.
3. Аутентификация мини-приложения — только по `initData` с проверкой HMAC на сервере; `initDataUnsafe` не доверяем.
4. Всё, что не работает в web-клиенте (Storage, Haptic, shareContent), — только как улучшение с запасным вариантом.
5. Настройки бота (URL мини-приложения, разрешение групп) выставляем в первые 1–2 дня из-за модерации.
