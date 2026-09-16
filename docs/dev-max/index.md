# MAX для разработчиков — документация

Офлайн-копия сайта <https://dev.max.ru>: разделы «Документация», «API», «MAX UI», «Помощь» и «История изменений». Ссылки между страницами ведут на локальные md-файлы, якоря — на заголовки внутри файлов. Внешние ресурсы (платформа, боты, GitHub и т. п.) и изображения остаются ссылками на сайт.

## [Документация](docs/index.md)

- [О платформе](docs/index.md)
- **Подключение к платформе**
  - [Создание профиля организации, ИП или самозанятого](docs/maxbusiness/connection.md)
  - [Выбор сервисов](docs/maxbusiness/selectionservices.md)
- **Чат-боты**
  - [Создание на платформе](docs/chatbots/bots-create/create.md)
  - [Управление](docs/chatbots/bots-create/manage.md)
  - [Конструктор сценариев: без кода](docs/chatbots/bots-nocode.md)
  - **Разработка сценариев: с кодом**
    - [API](docs/chatbots/bots-coding/prepare.md)
    - [Библиотека JavaScript](docs/chatbots/bots-coding/js.md)
    - [Библиотека Golang](docs/chatbots/bots-coding/go.md)
    - [Примеры создания ботов](docs/chatbots/bots-coding/examples.md)
- **Мини-приложения**
  - [Общее описание](docs/webapps/introduction.md)
  - [MAX Bridge](docs/webapps/bridge.md)
  - [Валидация данных](docs/webapps/validation.md)
- **Каналы**
  - [Создание на платформе](docs/channels/create.md)
  - [Управление](docs/channels/manage.md)
- [Цифровой ID](docs/digital-id.md)
- [Интеграция с партнёрами](docs/partners-integration.md)
- **Правила**
  - [Правила размещения чат-ботов и мини-приложений на платформе «MAX»](docs/legal/rules.md)
  - [Требования к содержанию и функциональности Приложений Разработчиков](docs/legal/requirements.md)
  - [Типовое пользовательское соглашение](docs/legal/agreement.md)
  - [Типовая политика конфиденциальности](docs/legal/privacy.md)
- [История изменений платформы](docs/changelog-platform.md)
- [Open Client API](docs/open-api.md)

## [API](docs-api/index.md)

- [Общее описание](docs-api/index.md)
- [История изменений API](docs-api/changelog-api.md)
- **Методы API**
  - **bots**
    - `GET` [Получение информации о боте](docs-api/methods/GET/me.md)
    - `PATCH` [Редактирование команд бота](docs-api/methods/PATCH/me/commands.md)
  - **chats**
    - `GET` [Получение списка всех групповых чатов и каналов для бота](docs-api/methods/GET/chats/index.md)
    - `GET` [Получение информации о групповом чате или канале](docs-api/methods/GET/chats/-chatId-/index.md)
    - `PATCH` [Изменение информации о групповом чате или канале](docs-api/methods/PATCH/chats/-chatId-.md)
    - `POST` [Отправка действия бота в групповой чат](docs-api/methods/POST/chats/-chatId-/actions.md)
    - `GET` [Получение закреплённого сообщения в групповом чате или канале](docs-api/methods/GET/chats/-chatId-/pin.md)
    - `PUT` [Закрепление сообщения в групповом чате или канале](docs-api/methods/PUT/chats/-chatId-/pin.md)
    - `DEL` [Открепление сообщения в групповом чате или канале](docs-api/methods/DELETE/chats/-chatId-/pin.md)
    - `GET` [Получение информации о членстве бота в групповом чате или канале](docs-api/methods/GET/chats/-chatId-/members/me.md)
    - `DEL` [Удаление бота из группового чата или канала](docs-api/methods/DELETE/chats/-chatId-/members/me.md)
    - `GET` [Получение списка администраторов группового чата или канала](docs-api/methods/GET/chats/-chatId-/members/admins.md)
    - `POST` [Назначить администратора группового чата или канала](docs-api/methods/POST/chats/-chatId-/members/admins.md)
    - `DEL` [Отменить права администратора в групповом чате или канале](docs-api/methods/DELETE/chats/-chatId-/members/admins/-userId-.md)
    - `GET` [Получение участников группового чата или канала](docs-api/methods/GET/chats/-chatId-/members/index.md)
    - `POST` [Добавление участников в групповой чат](docs-api/methods/POST/chats/-chatId-/members/index.md)
    - `DEL` [Удаление участников из группового чата или канала](docs-api/methods/DELETE/chats/-chatId-/members/index.md)
  - **subscriptions**
    - `GET` [Получение всех подписок через Webhook](docs-api/methods/GET/subscriptions.md)
    - `POST` [Подписка на обновления о новых событиях через Webhook](docs-api/methods/POST/subscriptions.md)
    - `DEL` [Отписка от обновлений о новых событиях через Webhook](docs-api/methods/DELETE/subscriptions.md)
    - `GET` [Получение обновлений о событиях через Long Polling](docs-api/methods/GET/updates.md)
  - **upload**
    - `POST` [Загрузка медиафайлов](docs-api/methods/POST/uploads.md)
  - **messages**
    - `GET` [Получение информации о сообщениях или постах](docs-api/methods/GET/messages/index.md)
    - `POST` [Отправка сообщений](docs-api/methods/POST/messages/index.md)
    - `PUT` [Редактирование сообщений](docs-api/methods/PUT/messages/index.md)
    - `DEL` [Удаление сообщений](docs-api/methods/DELETE/messages/index.md)
    - `GET` [Получение сообщений](docs-api/methods/GET/messages/-messageId-/index.md)
    - `GET` [Получение информации о видео, прикреплённом к сообщению](docs-api/methods/GET/videos/-videoToken-.md)
    - `POST` [Отправка ответа на callback](docs-api/methods/POST/answers.md)
  - **comments**
    - `GET` [Получение всех комментариев к посту](docs-api/methods/GET/messages/-messageId-/comments/index.md)
    - `POST` [Отправка комментария](docs-api/methods/POST/messages/-messageId-/comments.md)
    - `PUT` [Редактирование комментария](docs-api/methods/PUT/messages/-messageId-/comments.md)
    - `DEL` [Удаление комментария](docs-api/methods/DELETE/messages/-messageId-/comments.md)
    - `GET` [Получение комментария по его ID](docs-api/methods/GET/messages/-messageId-/comments/-commentId-.md)
- **Объекты**
  - [User](docs-api/objects/User.md)
  - [UserWithPhoto](docs-api/objects/UserWithPhoto.md)
  - [BotInfo](docs-api/objects/BotInfo.md)
  - [ChatMember](docs-api/objects/ChatMember.md)
  - [Chat](docs-api/objects/Chat.md)
  - [Message](docs-api/objects/Message.md)
  - [CommentMessage](docs-api/objects/CommentMessage.md)
  - [NewMessageBody](docs-api/objects/NewMessageBody.md)
  - [NewCommentBody](docs-api/objects/NewCommentBody.md)
  - [Update](docs-api/objects/Update.md)

## [MAX UI](ui/index.md)

- [Общее описание](ui/index.md)
- **Components**
  - **Avatar**
    - [Avatar.CloseButton](ui/components/Avatar.CloseButton.md)
    - [Avatar.Container](ui/components/Avatar.Container.md)
    - [Avatar.Icon](ui/components/Avatar.Icon.md)
    - [Avatar.Image](ui/components/Avatar.Image.md)
    - [Avatar.OnlineDot](ui/components/Avatar.OnlineDot.md)
    - [Avatar.Overlay](ui/components/Avatar.Overlay.md)
    - [Avatar.Text](ui/components/Avatar.Text.md)
  - [Button](ui/components/Button.md)
  - [CellAction](ui/components/CellAction.md)
  - [CellHeader](ui/components/CellHeader.md)
  - [CellInput](ui/components/CellInput.md)
  - [CellList](ui/components/CellList.md)
  - [CellSimple](ui/components/CellSimple.md)
  - [Counter](ui/components/Counter.md)
  - [Dot](ui/components/Dot.md)
  - [IconButton](ui/components/IconButton.md)
  - [Panel](ui/components/Panel.md)
  - [SearchInput](ui/components/SearchInput.md)
  - [Spinner](ui/components/Spinner.md)
  - [ToolButton](ui/components/ToolButton.md)
  - **Typography**
    - [Typography.Action](ui/components/Typography.Action.md)
    - [Typography.Body](ui/components/Typography.Body.md)
    - [Typography.Display](ui/components/Typography.Display.md)
    - [Typography.Headline](ui/components/Typography.Headline.md)
    - [Typography.Label](ui/components/Typography.Label.md)
    - [Typography.Title](ui/components/Typography.Title.md)
- **Layout**
  - [Container](ui/components/Container.md)
  - [Flex](ui/components/Flex.md)
  - [Grid](ui/components/Grid.md)
- **Helpers**
  - [EllipsisText](ui/components/EllipsisText.md)
  - [Ripple](ui/components/Ripple.md)
- **Forms**
  - [Input](ui/components/Input.md)
  - [Switch](ui/components/Switch.md)
  - [Textarea](ui/components/Textarea.md)
- **Compositions**
  - [Profile](ui/compositions/Profile.md)

## [Помощь](help/index.md)

- [Помощь](help/index.md)
- [Регистрация на платформе](help/platform_connection.md)
- [Создание профиля](help/organization.md)
- [Чат-боты](help/chatbots.md)
- [Каналы](help/channels.md)
- [Мини-приложения](help/miniapps.md)
- [Цифровой ID](help/digital-id.md)
- [Интеграция с партнёрами](help/integration.md)
- [События](help/events.md)
- [Диплинки](help/deeplinks.md)
- [Cлужба поддержки](help/support.md)
- [Мини-приложение и бот MAX для бизнеса](help/miniapp-main.md)

## [История изменений](changelog-docs.md)

- [Платформа MAX для партнёров](docs/changelog-platform.md)
- [API MAX](docs-api/changelog-api.md)
