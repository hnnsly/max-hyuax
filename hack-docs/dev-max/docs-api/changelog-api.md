# История изменений API

> **Критически важные изменения**
>
> - С **19 июля 2026** для корректной работы чат-ботов и мини-приложений необходимо направлять запросы на домен `platform-api2.max.ru` вместо `platform-api.max.ru`, а также добавить сертификат Минцифры в список доверенных
> - Начиная **с июня 2026 года**, метод `GET /chats` больше не поддерживается. Вместо него для получения списка всех групповых чатов и каналов, в которые добавлен бот, используйте [POST /subscriptions](methods/POST/subscriptions.md). Подробнее – на [странице «Получение списка всех групповых чатов и каналов»](methods/GET/chats/index.md)

## Bots

### Июль 2026

#### Добавлено

- Добавлен новый метод для редактирования, добавления и удаления команд бота [PATCH/me/commands](methods/PATCH/me/commands.md)

## Chats

### Сентябрь 2026

#### Изменено

- С 9 сентября 2026 работа метода [`POST /chats/{chatId}/members`](methods/POST/chats/-chatId-/members/index.md) будет ограничена

##### Август 2026

#### Добавлено

- В метод [`PATCH /chats/{chatId}`](methods/PATCH/chats/-chatId-.md) добавлен параметр `description` для изменения описания чата или канала

##### Июнь 2026

#### Удалено

- Метод [`GET /chats`](methods/GET/chats/index.md) больше не поддерживается, и API не предоставляет готовой возможности для получения списка групповых чатов и каналов, в которые добавлен бот. Если вам требуется получить для бота такой список, используйте `POST /subscriptions` — подробнее в [статье](methods/GET/chats/index.md)

## Сomments

### Август 2026

#### Добавлено

- Добавили методы отправки, редактирования, получения и удаления комментариев к постам в каналах: [`POST /messages/{messageId}/comments`](methods/POST/messages/-messageId-/comments.md), [`GET /messages/{messageId}/comments`](methods/GET/messages/-messageId-/comments/index.md), [`GET /messages/{messageId}/comments/{commentId}`](methods/GET/messages/-messageId-/comments/-commentId-.md), [`PUT /messages/{messageId}/comments`](methods/PUT/messages/-messageId-/comments.md), [`DELETE /messages/{messageId}/comments`](methods/DELETE/messages/-messageId-/comments.md), а также события для работы с комментариями: `comment_created`, `comment_removed`,`comment_edited`

## Uploads

### Июнь 2026

#### Изменено

- В методе `POST /uploads` добавлены [ограничения для видео (video), аудио (audio), изображений (image) и файлов (file)](index.md#Отправка%20медиафайлов), отправляемых во вложении к сообщению

## Messages

### Август 2026

#### Добавлено

- В метод [`POST /answers`](methods/POST/answers.md#Параметры) добавлен параметр `disable_link_preview`, который позволяет управлять отображением превью ссылок в сообщениях и постах

- [Посмотреть историю изменений платформы MAX для партнёров](../docs/changelog-platform.md)

---

Источник: <https://dev.max.ru/docs-api/changelog-api>
