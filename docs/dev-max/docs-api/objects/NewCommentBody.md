# NewCommentBody

Объект используется при отправке нового комментария к посту в канале [`POST messages/-messageId-/comments`](../methods/POST/messages/-messageId-/comments.md) или редактировании старого [`PUT messages/-messageId-/comments`](../methods/PUT/messages/-messageId-/comments.md). В отличие от обычных сообщений в чатах и постов в каналах (объект [`NewMessageBody`](NewMessageBody.md)), в комментариях не поддерживаются вложения `attachments` и пересылка сообщения (тип `forward`)

- **`text`** — string · Nullable

  до `4000` символов

  Текст комментария

- **`link`** — object NewMessageLink · Nullable

  Ссылка на комментарий

  - **`type`** — enum MessageLinkType

    Возможные значения в enum: `"forward"` `"reply"`

    Тип связанного сообщения:
    - `"reply"` — ответ на сообщение или комментарий в чате или канале
    - `"forward"` — пересланное сообщение в чате или канале

    **Для комментариев поддерживается только тип `reply`**

  - **`mid`** — string

    ID исходного сообщения

- **`format`** — enum TextFormat · Nullable · optional

  Возможные значения в enum: `"markdown"` `"html"`

  Разметка текста комментария. Для комментариев не поддерживается упоминание других пользователей и гиперссылки. Подробнее — в разделе [Форматирование](../index.md#Форматирование%20текста%20в%20сообщениях)

## Пример объекта

```json
{
 "text": "string",
  "link": { ... },
 "format": "markdown"
}
```

---

Источник: <https://dev.max.ru/docs-api/objects/NewCommentBody>
