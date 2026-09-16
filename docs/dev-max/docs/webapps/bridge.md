# MAX Bridge

Библиотека MAX Bridge позволяет мини-приложениям корректно взаимодействовать с API MAX и API операционной системы на устройстве пользователя

## Подключение библиотеки

Через CDN добавьте библиотеку max-web-app.js

```html
<script src="https://st.max.ru/js/max-web-app.js"></script>
```

После подключения библиотеки мини-приложение получит доступ к объекту `WebApp` через глобальный объект `window`

```javascript
window.WebApp
```

`window.WebApp` — это глобальный объект, который связывает мини-приложение с клиентом и позволяет взаимодействовать с МAX, управлять интерфейсом приложения и получать информацию о пользователях. Объект создаётся с каждым запуском сервиса, предзагружает данные и не требует отдельной инициализации: его методы и параметры доступны напрямую

## Функциональность библиотеки

### Работа с данными инициализации

Чтобы получить инициализационные данные, в объекте `WebApp` предусмотрены следующие методы:

- [`initData`](#window.WebApp.initData)
- [`initDataUnsafe`](#window.WebApp.initDataUnsafe)
- [`platform`](#window.WebApp.platform)
- [`version`](#window.WebApp.version)
- [`deviceName`](#window.WebApp.deviceName)

#### window.WebApp.initData

Строка со стартовыми параметрами в URL-кодировке.
Содержит данные о пользователе и другие инициализационные данные в виде закодированной в UTF-8 строки для [валидации](validation.md) на стороне сервера

**Тип возвращаемых данных**

```javascript
string
```

#### window.WebApp.initDataUnsafe

Объект, который содержит данные из `initData` в виде JSON-объекта

> Обратите внимание, что объект **нельзя использовать** для [валидации](validation.md) данных

**Пример**

```javascript
interface InitData {
    query_id: string;
    ip?: string;
    auth_date: number;
    hash: string;
    user: {
        id: number;
        first_name: string;
        last_name: string;
        username: string;
        language_code: string;
        photo_url: string;
    };
    chat: {
        id: number;
        type: 'DIALOG' | 'CHAT' | 'CHANNEL';
    };
    start_param: string;
}
```

**Описание свойств объекта**

| Поле | Тип данных | Описание |
| --- | --- | --- |
| `query_id` | `string` | Уникальный идентификатор текущей сессии |
| `ip?` | `string` | IP-адрес пользователя |
| `auth_date` | `number` | Время выдачи данных. Позволяет определить момент инвалидации данных. Рекомендуемый интервал составляет 1 час |
| `hash` | `string` | Хеш переданных параметров, который можно использовать для проверки их достоверности |
| `user` | `object` | Объект содержит данные о пользователе, который открывает мини-приложение |
| `user.id` | `number` | Идентификатор пользователя |
| `user.first_name` | `string` | Имя пользователя |
| `user.last_name` | `string` | Фамилия пользователя |
| `user.username` | `string` | Никнейм пользователя |
| `user.language_code` | `string` | Язык [интерфейса приложения MAX](https://datatracker.ietf.org/doc/html/rfc5646) |
| `user.photo_url` | `string` | Ссылка на фото профиля пользователя |
| `chat` | `object` | Объект содержит данные о чате, в котором открыто мини-приложение |
| `chat.id` | `number` | Идентификатор чата |
| `chat.type` | `string` | Тип чата (`DIALOG` / `CHAT` / `CHANNEL`) |
| `start_param` | `string` | Значение, переданное в мини-приложение через query-параметр <br><br> Пример:<br><br>`https://max.ru/<your_awesome_bot>?startapp=someData`, где поле `start_param` будет содержать значение `someData` |

#### window.WebApp.platform

Платформа, с которой запущено мини-приложение.
Возможные значения:

- `ios`
- `android`
- `desktop`
- `web`

**Тип возвращаемых данных**

```javascript
string
```

**Пример**

```javascript
const platform = window.WebApp.platform;
console.log('Платформа:', platform);
```

#### window.WebApp.version

Версия приложения MAX, с которого запущено мини-приложение

Имеет формат `<year>.<build_number — возрастающий счётчик>.<patch_version — для патчей>`, например `25.9.16`

Этот параметр не участвует в формировании хеша для валидации — в хеше учитываются только данные из `WebAppData`

**Тип возвращаемых данных**

```javascript
string
```

**Пример**

```javascript
const version = window.WebApp.version;
console.log('Версия приложения MAX:', version);
```

#### window.WebApp.deviceName

Возвращает устройство, с которого запущено мини-приложение. Например, может возвращать:

- `network name, macOS Tahoe (26.6)` — десктоп-приложение на macOS
- `network name, Windows 11 Version 25H2` — десктоп-приложение на Windows
- `Google Pixel 6, Android 17` — мобильное приложение Android
- `iPhone 16, iOS 26.5` — мобильное приложение iOS
- `Chrome, macOS` — веб-приложение на macOS
- `Chrome, Windows` — веб-приложение на Windows

**Тип возвращаемых данных**

```javascript
string
```

**Пример**

```javascript
const deviceName = window.WebApp.deviceName;
console.log('Название устройства:', deviceName);
```

### Контекст запущенного приложения

#### window.WebApp.getLaunchContext()

Позволяет мини-приложению адаптировать поведение и интерфейс в зависимости от источника запуска — это может быть таббар (нижняя панель вкладок приложения MAX) или список чатов, экран чата, экран настроек

> Метод доступен для версий:
>
> - Android — 26.19.2 и выше
> - iOS — 26.20.0 и выше

**Тип возвращаемых данных**

```javascript
Promise<{
  entryPoint: 'tabbar' | 'default'
}>
```

**Пример**

```javascript
window.WebApp.getLaunchContext().then(({entryPoint}) => {
    console.log(`Приложение было запущено через ${entryPoint}`)
});
```

- `entryPoint = tabbar` — мини приложение запущено из таббара
- `entryPoint = default` — мини приложение запущено из списка чатов / экрана чата / экрана настроек

### Работа с экраном

#### window.WebApp.requestScreenMaxBrightness()

Устанавливает яркость экрана пользователя на максимум

Приложение поддержит максимальную яркость 30 секунд, затем восстановит исходное значение

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
Promise<{maxBrightness: boolean}>
```

**Пример**

```javascript
window.WebApp.requestScreenMaxBrightness().then(({maxBrightness}) => {
    console.log('Яркость установлена на максимум')
});
```

#### window.WebApp.restoreScreenBrightness()

Восстанавливает яркость экрана пользователя до исходного значения

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
Promise<{maxBrightness: boolean}>
```

**Пример**

```javascript
window.WebApp.restoreScreenBrightness().then(({maxBrightness}) => {
    console.log('Яркость восстановлена до исходного значения')
});
```

#### window.WebApp.ScreenCapture.enableScreenCapture()

Включает возможность делать скриншоты или записывать экран

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
Promise<{isScreenCaptureEnabled: boolean}>
```

**Пример**

```javascript
window.WebApp.ScreenCapture.enableScreenCapture().then(({isScreenCaptureEnabled}) => {
    console.log('Включена возможность захвата экрана')
});
```

#### window.WebApp.ScreenCapture.disableScreenCapture()

Отключает возможность делать скриншоты или записывать экран

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
Promise<{isScreenCaptureEnabled: boolean}>
```

**Пример**

```javascript
window.WebApp.ScreenCapture.disableScreenCapture().then(({isScreenCaptureEnabled}) => {
    console.log('Отключена возможность захвата экрана')
});
```

#### window.WebApp.getViewportSize()

Возвращает текущий размер доступной области просмотра мини-приложения (viewport). Эти данные необходимо учитывать для корректного отображения мини-приложения

**Тип возвращаемых данных**

```javascript
Promise<{
    height: string,  
    width: string  
    }>
```

**Пример**

```javascript
window.WebApp.getViewportSize().then(({width, height}) => {
    console.log(`Размер viewport ${width}x${height}`)
});
```

### Запрос номера телефона

> Обратите внимание: отправка номера телефона в чат-бот описана на [странице API](../../docs-api/index.md#Кнопка%20request_contact)

#### window.WebApp.requestContact()

Запрашивает номер телефона пользователя в модальном окне нативного клиента MAX

> Данные пользователя (включая номер телефона), полученные с помощью метода `requestContact()`, могут использоваться только для взаимодействия с текущим мини-приложением. Например, их можно применять для регистрации в программе лояльности, проверки статуса заказа, идентификации пользователя

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
Promise<{
    phone: string;
    authDate: string; // timestamp создания hash
    hash: string;
}>
```

**Пример**

**Пример**

```javascript
window.WebApp.requestContact().then(({phone}) => {
    console.log(`Номер телефона пользователя ${phone}`)
});
```

**Проверка номера телефона**

Для проверки, что полученный на запрос номер телефона совпадает с номером, привязанным к аккаунту пользователя в MAX, сравните:

- Значение поля `hash`, полученное от клиента
- Значение функции `HMAC_SHA256(authDate + phone + userId, botToken)`, где:
  - `HMAC_SHA256` — стандартная для большинства языков программирования криптографическая функция
  - `authDate + phone + userId` — параметры в алфавитном порядке, используемые для вычисления хеша: сформируйте строку, объединив пары `key=value` с разделителем `\n`
  - `botToken` — токен бота, чьё мини-приложение запрашивает номер телефона пользователя

Если значения совпадают, это подтверждает, что пользователь поделился номером телефона, привязанным к его аккаунту в MAX

> При вычислении хеша значение `phone` не должно содержать `+`: вместо `+7**********` используется `7**********`

**Возможные ошибки**

Если пользователь отказывается поделиться номером телефона или запрос завершился ошибкой, возвращает:

```json
{
    "error": {
        "code": "client.request_phone.<reason>"
    }
}
```

| Номер ошибки | Возможное значение `reason` | Описание ошибки |
| --- | --- | --- |
| 01 | `user_refused_provide_phone_number` | Пользователь отказался предоставить номер телефона |
| 02 | `request_error` | Ошибка при выполнении запроса (нет сети / не ответил backend) |

### Подтверждение закрытия мини-приложения

> Обратите внимание, что возможности из этой категории отправляют запрос приложению MAX в **одностороннем порядке**

#### window.WebApp.enableClosingConfirmation()

Включает предупреждение о риске потерять заполненные данные, если закрыть мини-приложение

**Пример**

```javascript
window.WebApp.enableClosingConfirmation()
```

#### window.WebApp.disableClosingConfirmation()

Выключает предупреждение о риске потерять заполненные данные, если закрыть мини-приложение

**Пример**

```javascript
window.WebApp.disableClosingConfirmation()
```

### Открытие ссылок

Библиотека поддерживает два формата открытия ссылок:

- во внешнем браузере
- в виде диплинка, связанного с max.ru

#### window.WebApp.openLink(url)

Открывает ссылку во внешнем браузере

> Чтобы обезопасить процесс, перед вызовом метода MAX Bridge проверяет клик пользователя в мини-приложении. Если клика не было, перехода по ссылке не будет

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
// URL веб-страницы, которую нужно открыть
url: string
```

**Пример**

```javascript
// После вызова метода откроется ссылка во внешнем браузере
window.WebApp.openLink('https://max.ru/');
```

#### window.WebApp.openMaxLink(url)

Открывает диплинк вида `https://max.ru/<some-url>` из мини-приложения внутри MAX. Если передать ссылку другого вида, метод откроет её во внешнем браузере

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
// Диплинк для клиента MAX
url: string
```

**Пример**

```javascript
// Будет открыт нужный чат, контакт или мини-приложение в MAX
window.WebApp.openMaxLink('https://max.ru/<your-url>');
```

### Скачивание файла

Условие для скачивания файла:

- Наличие защищённого `https`-соединения, `http`-ссылки не работают
- Перед вызовом метода MAX Bridge проверяет клик пользователя в мини-приложении. Если клика не было, файл не будет скачан
- Скачивание должно происходить в мини-приложении, открытом в мессенджере MAX. В браузере метод не работает

> Скачивание файла через href не поддерживается — используйте только метод `window.WebApp.downloadFile(url, file_name)`

#### window.WebApp.downloadFile(url, file_name)

Скачивает файл по переданной `https`-ссылке под нужным названием

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
url: string // URL для скачивания файла — любой прямой хост: свой сервер, S3, CDN
file_name: string // Название файла при сохранении
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'downloading' | 'cancelled';
}>
```

**Пример**

```javascript
const fileName = 'document.pdf';
const fileUrl = 'https://some-url.com/document.pdf';

window.WebApp.downloadFile(fileUrl, fileName)
  .then(({ status }) => {
    if (status === 'downloading') {
      console.log(`${fileName} загружается`);
    }
    if (status === 'cancelled') {
      console.log('Скачивание отменено пользователем');
    }
  })
  .catch(({ error }) => {
    switch (error.code) {
      case 'client.download_file.invalid_params':
        console.error('Ошибка: невалидный URL или параметры');
        break;
      case 'client.download_file.request_timeout':
        console.error('Ошибка: превышен тайм-аут — нативный клиент не ответил за 60 сек');
        break;
      default:
        console.error(`Ошибка: ${error.code}`);
    }
  });
```

#### Коды ошибок

| Код ошибки | Описание |
| --- | --- |
| `client.download_file.invalid_params` | Невалидный URL или параметры |
| `client.download_file.request_timeout` | Превышен тайм-аут — нативный клиент не ответил за 60 сек |

### Шеринг контента

В библиотеке есть два способа для шеринга контента:

- во внешние приложения
- внутри MAX

#### window.WebApp.shareContent(params)

Вызывает нативный экран шеринга из мини-приложения на iOS, Android.
Передаются параметры `text` и/или `link`: один из параметров всегда должен быть передан. Разделение является условным и сделано для удобства восприятия: если передать и текст, и ссылку в одном поле `text`, то результат не изменится

> Этот метод **не поддерживается веб-приложением**

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
params: {
    text?: string;
    link?: string
}
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'shared' | 'cancelled';
}>
```

**Пример**

```javascript
const text = 'Look at this'
const url = 'https://epic-video-url'

// Вызывается нативное окно шеринга во внешние приложения
window.WebApp.shareContent({text, link}).then(({status}) => {
    if(status === 'shared') {
        console.log('Сообщение было отправлено')
    }
});
```

#### window.WebApp.shareMaxContent(params)

Открывает экран шеринга внутри MAX

> Чтобы обезопасить процесс, перед вызовом метода MAX Bridge проверяет клик пользователя в мини-приложении. Если клика не было, экран шеринга не откроется

Метод предоставляет возможность шеринга контента из мини-приложения в диалоги или групповые чаты MAX. Метод работает в двух режимах:

- шеринг текста, который аналогичен `WebApp.shareContent(params)`
- шеринг текста с контентом: файл, медиа

Для шеринга файла или медиа бот, на котором работает мини-приложение, предварительно отправляет контент пользователю через [POST/messages](../../docs-api/methods/POST/messages/index.md). Шеринг медиа работает как пересылка сообщения, поэтому поддерживается любой тип контента:

1. Бот отправляет контент пользователю, например медиафайл или открытку
2. Мини-приложение получает идентификатор этого сообщения `mid`. Его возвращает MAX Bot API, когда сообщение отправляется пользователю
3. В мини-приложении вызывается `shareMaxContent({ mid, chatType })`, где `mid` — идентификатор сообщения от бота, а `chatType` — тип чата, сообщением из которого нужно поделиться:
   - `DIALOG` — для диалога, личного чата между двумя пользователями
   - `CHAT` — для группового чата. Пользователь должен быть участником чата
4. Пользователь выбирает, куда отправить контент — сообщение пересылается в выбранный чат

В метод передаются либо `text` и/или `link`, либо `mid` и `chatType`. Если при шеринге медиа или файла передать `text` или `link`, они будут проигнорированы

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
params: {
    text?: string;
    link?: string
} | {
    mid: string;
    chatType: 'DIALOG' | 'CHAT'
}
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'shared' | 'cancelled';
}>
```

**Пример**

```javascript
const text = 'Look at this'
const url = 'https://epic-video-url'

// Отправит текст и ссылку в выбранный пользователем чат
window.WebApp.shareMaxContent({text, link}).then(({status}) => {
    if(status === 'shared') {
        console.log('Сообщение успешно отправлено')
    }

    if(status === 'cancelled') {
      console.log('Пользователь закрыл шторку без шеринга')
    }
})

const mid = `mid.<hash>`
const chatType = 'CHAT'

// Сообщение с файлом будет отправлено в выбранный пользователем чат
window.WebApp.shareMaxContent({mid, chatType}).then(({status}) => {
    if(status === 'shared') {
        console.log('Сообщение c файлом успешно отправлено')
    }

    if(status === 'cancelled') {
      console.log('Пользователь закрыл шторку без шеринга')
    }
})
```

### Сканирование QR-кодов

Библиотекой предусмотрено два режима работы:

- сканирование QR-кода камерой
- выбор файла для сканирования из файловой системы

По умолчанию установлен режим выбора файла из системы

#### window.WebApp.openCodeReader(fileSelect = true)

Открывает камеру для считывания QR-кода

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
// Использовать файл из системы или сканировать камерой
fileSelect: boolean
```

**Тип возвращаемых данных**

```javascript
Promise<{
    value: string;
}>
```

**Пример**

```javascript
window.WebApp.openCodeReader().then(({value}) => {
    console.log('Данные с QR-кода', value)
})
```

Вернётся результат в виде строки, если QR-код был найден и распознан

- `fileSelect = true` — доступен также выбор из галереи
- `fileSelect = false` — доступно сканирование только через камеру

Если `fileSelect` не передан, то по умолчанию считается `fileSelect = true`

### Управление кнопкой «Назад» в шапке приложения

Управление кнопкой **Назад** происходит через объект `BackButton`

#### window.WebApp.BackButton.show()

Делает кнопку **Назад** активной и видимой

**Пример**

```javascript
window.WebApp.BackButton.show()
```

#### window.WebApp.BackButton.hide()

Скрывает кнопку **Назад**

**Пример**

```javascript
window.WebApp.BackButton.hide()
```

#### window.WebApp.BackButton.isVisible

Управляет отображением кнопки **Назад** в заголовке мини-приложения в интерфейсе MAX

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
boolean
```

**Пример**

```javascript
window.WebApp.BackButton.isVisible

// isVisible = true — кнопка отображается
// isVisible = false — кнопка не отображается
```

> Значение false задано по умолчанию

#### window.WebApp.BackButton.onClick(callback)

Устанавливает обработчик событий нажатия на кнопку **Назад**

> Чтобы оставить возможность отписки от события нажатия на кнопку, сохраните ссылку на функцию, которая будет передана в качестве callbcak

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
callback: () => void
```

**Пример**

```javascript
const onBackButtonPress = () => {
    console.log('Кнопка назад была нажата')
}

window.WebApp.BackButton.onClick(onBackButtonPress)
```

#### window.WebApp.BackButton.offClick(callback)

Отключает обработчик событий нажатия кнопки **Назад**

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
callback: () => void
```

**Пример**

```javascript
const onBackButtonPress = () => {
    console.log('Кнопка назад была нажата')
}

// Подписаться на событие нажатия
window.WebApp.BackButton.onClick(onBackButtonPress)

// Отписаться от события нажатия
window.WebApp.BackButton.offClick(onBackButtonPress)
```

### Хранилище устройства

С помощью `DeviceStorage` можно сохранять данные на устройстве пользователя. Объект предоставляет мини-приложению доступ к хранилищу данных, ассоциированному с конкретным пользователем MАХ

> Методы этого объекта **не поддерживаются веб-приложением**

#### window.WebApp.DeviceStorage.setItem(key, value)

Сохраняет переданную пару «ключ-значение» в локальном хранилище устройства для этого мини-приложения

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
key: string
value: string
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'updated' | 'removed'
}>
```

**Пример**

```javascript
const key = 'key'
const value = 'value'

window.WebApp.DeviceStorage.setItem(key, value).then(({status}) => {
    if(status === 'updated') {
        console.log(`Данные [${key}]: ${value} успешно сохранены`)
    }
});
```

#### window.WebApp.DeviceStorage.getItem(key)

Получает значение из локального хранилища устройства по указанному ключу

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
key: string
```

**Тип возвращаемых данных**

```javascript
Promise<{
    key: string;
    value: string;
}>
```

**Пример**

```javascript
const key = 'storageEntryKey'

window.WebApp.DeviceStorage.getItem(getKey).then((result) => {
    console.log(result) // {key: 'storageEntryKey', value: 'some value'}
});
```

#### window.WebApp.DeviceStorage.removeItem(key)

Удаляет значение из локального хранилища устройства по указанному ключу

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
key: string
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'updated' | 'removed'
}>
```

**Пример**

```javascript
const key = 'key'

window.WebApp.DeviceStorage.removeItem(key).then(({status}) => {
    if(status === 'removed') {
        console.log(`Данные по ключу ${key} успешно удалены`)
    }
});
```

#### window.WebApp.DeviceStorage.clear()

Очищает все ключи, ранее сохранённые ботом в локальном хранилище устройства

**Пример**

```javascript
// Хранилище очищено
window.WebApp.DeviceStorage.clear();
```

### Защищённое хранилище устройства

С помощью объекта `SecureStorage` можно получить доступ к безопасному хранилищу конфиденциальных данных на устройстве пользователя.
Это гарантирует, что все сохраненные значения зашифрованы и недоступны для неавторизованных приложений

Защищённое хранилище подходит для хранения токенов, секретов, состояния аутентификации и другой конфиденциальной пользовательской информации.
Каждый бот может хранить до 10 ключей на пользователя

> Методы этого объекта **не поддерживаются веб-приложением**

#### window.WebApp.SecureStorage.setItem(key, value)

Сохраняет переданную пару «ключ-значение» в защищённом хранилище устройства

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
key: string
value: string
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'updated' | 'removed'
}>
```

**Пример**

```javascript
const key = 'key'
const value = 'value'

window.WebApp.SecureStorage.setItem(key, value).then(({status}) => {
    if(status === 'updated') {
        console.log(`Данные [${key}]: ${value} успешно сохранены`)
    }
});
```

#### window.WebApp.SecureStorage.getItem(key)

Получает значение из защищённого хранилища устройства по указанному ключу

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
key: string
```

**Тип возвращаемых данных**

```javascript
Promise<{
    key: string;
    value: string;
}>
```

**Пример**

```javascript
const key = 'secureStorageEntryKey'

window.WebApp.SecureStorage.getItem(getKey).then((result) => {
    console.log(result) // {key: 'secureStorageEntryKey', value: 'some value'}
});
```

#### window.WebApp.SecureStorage.removeItem(key)

Удаляет значение из защищённого хранилища устройства по указанному ключу

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
key: string
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'updated' | 'removed'
}>
```

**Пример**

```javascript
const key = 'key'

window.WebApp.SecureStorage.removeItem(key).then(({status}) => {
    if(status === 'removed') {
        console.log(`Данные по ключу ${key} успешно удалены`)
    }
});
```

#### window.WebApp.SecureStorage.clear()

Очищает все ключи, ранее сохранённые в защищённом хранилище устройства

**Пример**

```javascript
// Хранилище очищено
window.WebApp.SecureStorage.clear();
```

### Использование биометрии

Работа с биометрией доступна через объект `BiometricManager`. Он нужен для аутентификации, когда доступ к данным в `keychain` получается через биометрические идентификаторы

> Методы этого объекта **не поддерживаются десктоп- и веб-клиентом**

#### window.WebApp.BiometricManager.init()

Перед использованием методов объекта `BiometricManager` нужно однократно вызвать метод первичной инициализации биометрии — `init`:

- Проверяет наличие функции биометрии на устройстве
- Проверяет, предоставлен ли доступ к биометрии на устройстве

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
type BiometryType = 'finger' | 'face' | 'unknown';

interface BiometryInfo {
    available: boolean;
    type: BiometryType[];
    accessRequested: boolean;
    accessGranted: boolean;
    tokenSaved: boolean;
    deviceId: string | null;
}

Promise<BiometryInfo>
```

**Пример**

```javascript
window.WebApp.BiometricManager.init().then((biometricManagerData) => {
    console.log('Данные менеджера биометрии', biometricManagerData)
});
```

| Поле | Тип данных | Описание |
| --- | --- | --- |
| `available` | `boolean` | Проверка доступности биометрии на устройстве пользователя, который запустил мини-приложение |
| `type` | `array` | Типы биометрии: `fingerprint`, `faceid`, `unknown` <br> <br> Если пользователь отказался предоставить доступ к биометрии, то `biometricType= array<unknown>`. Для Android всегда `unknown` |
| `accessRequested` | `boolean` | Проверка отправки запроса на предоставление доступа к биометрии устройства <br> <br> Если пользователь отказался предоставить доступ к биометрии, то `accessRequested = false` |
| `accessGranted` | `boolean` | Проверка предоставления доступа к биометрии |
| `tokenSaved` | `boolean` | Проверка наличия токена авторизации через биометрию в безопасном хранилище устройства |
| `deviceId` | `string` | Идентификатор устройства — можно использовать для сопоставления токена с устройством |

#### window.WebApp.BiometricManager.isInited

Получает состояние инициализации `BiometricManager` — была ли ранее первичная инициализация

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
boolean
```

**Пример**

```javascript
// true или false
window.WebApp.BiometricManager.isInited
```

#### window.WebApp.BiometricManager.isBiometricAvailable

Проверяет доступность биометрии на устройстве пользователя, который запустил мини-приложение

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
boolean
```

**Пример**

```javascript
// true или false
window.WebApp.BiometricManager.isBiometricAvailable
```

Если пользователь отказался предоставить доступ к биометрии, значение будет `false`

#### window.WebApp.BiometricManager.isAccessRequested

Проверяет, был ли ранее отправлен запрос на предоставление доступа к биометрии устройства

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
boolean
```

**Пример**

```javascript
// true или false
window.WebApp.BiometricManager.isAccessRequested
```

Если пользователь отказался предоставить доступ к биометрии, значение будет `false`

#### window.WebApp.BiometricManager.isAccessGranted

Проверяет, предоставлен ли доступ к биометрии

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
boolean
```

**Пример**

```javascript
// true или false
window.WebApp.BiometricManager.isAccessGranted
```

Если пользователь отказался предоставить доступ к биометрии, значение будет `false`

#### window.WebApp.BiometricManager.isBiometricTokenSaved

Проверяет наличие токена в безопасном хранилище устройства

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
boolean
```

**Пример**

```javascript
// true или false
window.WebApp.BiometricManager.isBiometricTokenSaved
```

#### window.WebApp.BiometricManager.biometricType

Позволяет посмотреть доступные типы биометрии:

- `fingerprint`
- `faceid`
- `unknown`

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
Array<'finger' | 'face' | 'unknown'>
```

**Пример**

```javascript
window.WebApp.BiometricManager.biometricType
```

Если пользователь отказался предоставить доступ к биометрии, то `biometricType=["unknown"]`

Для Android всегда `["unknown"]`

#### window.WebApp.BiometricManager.deviceId

Возвращает идентификатор устройства — можно использовать для сопоставления токена с устройством

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
string | null
```

**Пример**

```javascript
window.WebApp.BiometricManager.deviceId
```

Возвращает `null`, если пользователь отказался предоставить доступ к биометрии

#### window.WebApp.BiometricManager.requestAccess(reason)

Отправляет запрос на доступ к использованию биометрии на устройстве

Возвращает тип данных `BiometryInfo`, подробнее — в подразделе про [использование биометрии](#Использование%20биометрии)

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
// Причина запроса мини-приложения на использование доступа
// Размер: 1-128 символов, остальное будет отрезаться
// Необязательное поле
reason?: string
```

**Тип возвращаемых данных**

```javascript
Promise<BiometryInfo>
```

**Пример**

```javascript
const reason = 'some reason'

window.WebApp.BiometricManager.requestAccess(reason).then((response) => {
    console.log('Данные биометрии', response)
})
```

#### window.WebApp.BiometricManager.authenticate(reason)

Запускает процесс аутентификации при помощи биометрических данных

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
// Причина запроса мини-приложения на использование доступа
// Размер: 1-128 символов, остальное будет отрезаться
// Необязательное поле
reason?: string
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'authorized';
    token: string;
}>
```

**Пример**

```javascript
const reason = 'some reason'

window.WebApp.BiometricManager.authenticate(reason).then(({token}) => {
    console.log('Авторизационный токен', token)
})
```

#### window.WebApp.BiometricManager.updateBiometricToken(token, reason)

Обновляет биометрический токен в безопасном хранилище устройства

> Для удаления токена вызовите метод без передачи параметра токена

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
token?: string
// Причина запроса мини-приложения на использование доступа
// Размер: 1-128 символов, остальное будет отрезаться
// Необязательное поле
reason?: string
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'updated' | 'removed';
}>
```

**Пример**

```javascript
const reason = 'some reason'

const {token} = await window.WebApp.BiometricManager.authenticate(reason)

// Обновление/сохранение токена
window.WebApp.BiometricManager.updateBiometricToken(token).then((response) => {
    console.log('Биометрический токен успешно обновлен')
})

// Удаление токена
window.WebApp.BiometricManager.updateBiometricToken().then(({status}) => {
    if(status === 'removed') {
        console.log('Биометрический токен успешно удален')
    }
})
```

#### window.WebApp.BiometricManager.openSettings()

Отображает нативное диалоговое окно с предложением перейти в настройки MAХ на экран приватности, чтобы дать доступ к биометрии устройства для мини-приложения

Вызывает закрытие мини-приложения

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'opened'
}>
```

**Пример**

```javascript
// Откроется диалоговое окно
window.WebApp.BiometricManager.openSettings()
```

### Тактильные отклики

Чтобы активировать и настроить тактильную обратную связь при взаимодействии пользователя с веб-приложением, используйте объект `HapticFeedback`

> Методы этого объекта **не поддерживаются десктоп- и веб-клиентом**

#### window.WebApp.HapticFeedback.impactOccurred(impactStyle, disableVibrationFallback)

С помощью этого метода приложение MAX может воспроизвести соответствующие тактильные эффекты на основе переданного значения стиля

Подходит для тактильного отклика на интерактивные элементы, например при нажатии на кнопку

Стиль может иметь одно из следующих значений:

- `soft` — мягкая вибрация
- `light` — лёгкая вибрация
- `medium` — средняя вибрация
- `heavy` — сильная вибрация
- `rigid` — жёсткая вибрация

`disableVibrationFallback` — разрешение использовать вибрацию с постоянной амплитудой на устройствах, которые не поддерживают вибрацию с переменной амплитудой. Значение по умолчанию: `false`

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
impactStyle: 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'
disableVibrationFallback?: boolean // По умолчанию false
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'impactOccured'
}>
```

**Пример**

```javascript
window.WebApp.HapticFeedback.impactOccurred('light').then(() => {
    console.log('Произошло тактильное воздействие')
});
```

#### window.WebApp.HapticFeedback.notificationOccurred(notificationType, disableVibrationFallback)

Возвращает статус событий или действий: выполнены успешно, не удалось выполнить или выдано предупреждение

Приложение MAХ может воспроизводить соответствующие тактильные сигналы на основе переданного значения типа. Тип может быть одним из следующих значений:

- `error` — не удалось выполнить
- `success` — выполнены успешно
- `warning` — выдано предупреждение

`disableVibrationFallback` — разрешение использовать вибрацию с постоянной амплитудой на устройствах, которые не поддерживают вибрацию с переменной амплитудой. Значение по умолчанию: `false`

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
impactStyle: 'error' | 'success' | 'warning'
disableVibrationFallback?: boolean // По умолчанию false
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'notificationOccured'
}>
```

**Пример**

```javascript
window.WebApp.HapticFeedback.notificationOccurred('error').then(() => {
    console.log('Появилось тактильное уведомление об ошибке')
});
```

#### window.WebApp.HapticFeedback.selectionChanged(disableVibrationFallback)

Сообщает, что пользователь изменил выбор

Приложение MAX может воспроизвести соответствующие тактильные сигналы

> Не используйте эту обратную связь, когда пользователь делает или подтверждает выбор. Используйте её только при изменении выбора

`disableVibrationFallback` — разрешение использовать вибрацию с постоянной амплитудой на устройствах, которые не поддерживают вибрацию с переменной амплитудой. Значение по умолчанию: `false`

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
// По умолчанию false
disableVibrationFallback?: boolean
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'selectionChanged'
}>
```

**Пример**

```javascript
window.WebApp.HapticFeedback.selectionChanged().then(() => {
    console.log('Сработал тактильный отклик на действие пользователя')
});
```

### NFC-модуль

Работа с NFC-модулем доступна через объект `NfcManager`

> Методы этого объекта **поддерживаются только для Android**

Чтобы начать использовать методы объекта `NfcManager`, необходимо сначала вызвать его метод инициализации `init`

#### window.WebApp.NfcManager.init()

Инициализирует `NfcManager`

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
interface NfcInfo {
  available: boolean;
  enabled: boolean;
  accessRevoked?: boolean;
}
```

**Пример**

```javascript
window.WebApp.NfcManager.init().then((nfcManagerData) => {
    console.log('Данные менеджера NFC', nfcManagerData)
});
```

**Описание свойств объекта**

| Поле | Тип данных | Описание |
| --- | --- | --- |
| `available` | `boolean` | Проверка наличия NFC-модуля на устройстве пользователя |
| `enabled` | `boolean` | Проверка включения NFC-модуля в настройках системы |
| `accessRevoked?` | `boolean` | Отозвал ли пользователь разрешение использовать NFC-модуль для текущего мини-приложения в настройках приватности MAX |

#### window.WebApp.NfcManager.isInited

Возвращает состояние инициализации `NfcManager`

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
boolean
```

**Пример**

```javascript
window.WebApp.NfcManager.isInited // true или false
```

#### window.WebApp.NfcManager.openSystemSettings()

Открывает страницу системных настроек доступа к NFC-модулю и вызывает закрытие мини-приложения

> Если пользователь не отключал NFC-модуль, то переход не будет выполнен

**Типы данных и пример**

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'opened'
}>
```

**Пример**

```javascript
// Откроются системные настройки
window.WebApp.NfcManager.openSystemSettings()
```

#### window.WebApp.NfcManager.emulateNfcTag(nfctag)

Запускает через NFC-модуль передачу данных, полученных из мини-приложения

> Если не передать данные NFC-метки, то вещание будет остановлено

**Типы данных и пример**

**Тип передаваемых данных**

```javascript
nfctag?: 'string'
```

**Тип возвращаемых данных**

```javascript
Promise<{
    status: 'scanned' | 'stopped'
}>
```

**Пример**

```javascript
const nfcTagData = 'Some data'

window.WebApp.NfcManager.emulateNfcTag(nfcTagData).then(({status}) => {
  console.log('NFC метка отсканирована')
});

window.WebApp.NfcManager.emulateNfcTag().then(({status}) => {
    if(status === 'stopped') {
        console.log('Вещание остановлено')
    }
});
```

## Ошибки и обработка исключений

Большинство методов возвращают Promise-объекты, и в случае ошибки вызывается `reject`

**Типы данных и пример**

**Структура объекта ошибки**

```javascript
{
    error: {
        code: string
    }
}
```

**Пример**

```javascript
window.WebApp.SecureStorage.setItem('key', 'value')
    .then((result) => {
        console.log('Успешно сохранено');
    })
    .catch(({error}) => {
        console.error('Произошла ошибка:', error.code);
    });
```

---

Источник: <https://dev.max.ru/docs/webapps/bridge>
