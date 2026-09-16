# Chat

Объект содержит общую информацию о групповом чате или канале: его тип, настройки отображения (название, аватар, описание, ссылка), публичную доступность, а также информацию об участниках (владельце, боте и других пользователях), времени их последней активности и событиях

- **`chat_id`** — integer &lt;int64&gt;

  ID чата или канала — в зависимости от ограничений метода и от того, с чем вы работаете. Как получить ID — в [разделе «Получение chat_id»](../index.md#Получение%20chat_id)

- **`type`** — enum ChatType

  Возможные значения в enum: `"chat"` `"channel"` `"dialog"`

  Тип чата:

  - `"chat"` — Групповой чат
  - `"channel"` — Канал
  - `"dialog"` — Диалог

- **`status`** — enum ChatStatus

  Возможные значения в enum: `"active"` `"removed"` `"left"` `"closed"`

  Статус чата:

  - `"active"` — Бот является активным участником чата
  - `"removed"` — Бот был удалён из чата
  - `"left"` — Бот покинул чат
  - `"closed"` — Чат был закрыт

- **`title`** — string · Nullable

  Отображаемое название чата или канала. Может быть `null` для диалогов

- **`icon`** — object Image · Nullable

  Аватар группового чата или канала

  - **`url`** — string

    URL изображения

- **`last_event_time`** — integer &lt;int64&gt;

  Время последнего события в чате или канале в формате Unix timestamp в миллисекундах

- **`participants_count`** — integer &lt;int32&gt;

  Количество участников чата или канала. Для диалогов всегда `2`

- **`owner_id`** — integer &lt;int64&gt; · Nullable · optional

  ID владельца чата или канала

- **`participants`** — object · Nullable · optional

  Список участников в формате ключ-значение, где ключ — идентификатор участника `user_id`, а значение — время его последней активности в чате или канале `last_event_time`. Может быть `null`, если запрашивается список чатов

- **`is_public`** — boolean

  Параметр показывает, доступен ли групповой чат или канал публично. Для диалогов и приватных каналов — всегда `false`

- **`link`** — string · Nullable · optional

  Ссылка на чат

- **`description`** — string · Nullable

  Описание чата или канала

- **`dialog_with_user`** — object [UserWithPhoto](UserWithPhoto.md) · Nullable · optional

  Данные о пользователе в диалоге (только для чатов типа `"dialog"`)

- **`messages_count`** — integer · Nullable · optional

  Количество сообщений в групповом чате или постов канале

- **`pinned_message`** — object [Message](Message.md) · Nullable · optional

  Закреплённое сообщение в чате (возвращается только при запросе конкретного чата или канала)

## Пример объекта

```json
{
 "chat_id": 0,
 "type": "chat",
 "status": "active",
 "title": "string",
  "icon": { ... },
 "last_event_time": 0,
 "participants_count": 0,
 "owner_id": 0,
 "participants": object,
 "is_public": true,
 "link": "string",
 "description": "string",
  "dialog_with_user": { ... },
 "messages_count": 0,
  "pinned_message": { ... }
}
```

---

Источник: <https://dev.max.ru/docs-api/objects/Chat>
