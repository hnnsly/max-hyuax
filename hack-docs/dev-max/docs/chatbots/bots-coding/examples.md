# Примеры создания ботов

Вы можете изучить демонстрационных ботов, которые размещены в наших репозиториях

- [Бот](https://github.com/max-messenger/demo-bot-go), демонстрирующий основные возможности Bot API через интерактивные сценарии
- [To-do list](https://github.com/max-messenger/max-bot-example-todolist) — бот в формате простого таск-менеджера

## Примеры создания Hello-бота с помощью библиотек JavaScript и Golang

В этом разделе разберём пример реализации простого бота с использованием библиотеки MAX Bot API — напишем код для Hello Bot, чтобы научить его здороваться с пользователями

Больше примеров смотрите в наших репозиториях на GitHub:

- [JavaScript](https://github.com/max-messenger/max-bot-api-client-ts)
- [Golang](https://github.com/max-messenger/max-bot-api-client-go)

1. Создайте новый проект в терминале и установите библиотеку:

**JavaScript**

```bash
# Создайте папку и перейдите в неё
mkdir my-first-bot
cd my-first-bot

# Установите MAX Bot API

# Для npm
npm install --save @maxhub/max-bot-api
# Для yarn
yarn add @maxhub/max-bot-api
# Для pnpm
pnpm add @maxhub/max-bot-api
# Для deno
deno add npm:@maxhub/max-bot-api

# Установите и настройте TypeScript (опционально)
yarn add -D typescript
npx tsc --init
```

**Golang**

```bash
# Создайте новую папку для исходного кода вашего модуля Go и перейдите в неё
mkdir my-first-bot
cd my-first-bot
# Запустите свой модуль с помощью команды go mod init (команда создает файл `go.mod` для отслеживания зависимостей вашего кода. Пока что файл включает только имя вашего модуля и версию Go, которую поддерживает ваш код)
go mod init first-max-bot

# Установите библиотеку для работы с MAX API на golang
go get github.com/max-messenger/max-bot-api-client-go
```

2. Создайте файл:

- для JavaScript — `bot.js`, для TypeScript — `bot.ts`
- для Golang — `bot.go`

3. Обеспечьте доступ к методам и утилитам

**JavaScript**

```JavaScript
# Создайте объект класса Bot — он обеспечит доступ к методам и утилитам

filename="bot.js"
import { Bot } from '@maxhub/max-bot-api';
const bot = new Bot(process.env.BOT_TOKEN); // Токен, полученный при регистрации бота в MAX
bot.start(); // Запускает получение обновлений
```

**Golang**

```GO
# Создайте функцию main в пакете main — он обеспечит доступ к методам и утилитам

filename="bot.go"
package main
import (
    "fmt"
    maxbot "github.com/max-messenger/max-bot-api-client-go"
)

func main() {
    api := maxbot.New(os.Getenv("TOKEN"))
    // Some methods demo:
    info, err := api.Bots.GetBot()
    fmt.Printf("Get me: %#v %#v", info, err)
}
```

4. Определите функциональность приветствия — бот будет отвечать на команду `/hello`

**JavaScript**

```JavaScript
filename="bot.js"
import { Bot } from '@maxhub/max-bot-api';
const bot = new Bot(process.env.BOT_TOKEN);

// Устанавливает список команд, который пользователь будет видеть в чате с ботом
bot.api.setMyCommands([
  {
    name: 'hello',
    description: 'Поприветствовать бота',
  },
]);  

// Обработчик команды '/hello'
bot.command('hello', (ctx) => {
  return ctx.reply('Привет! ✨');
});
bot.start();
```

**Golang**

```GO
filename="bot.go"
package main
import (
    "fmt"
    maxbot "github.com/max-messenger/max-bot-api-client-go"
)

func main() {
    api := maxbot.New(os.Getenv("TOKEN"))
    // Some methods demo:
    info, err := api.Bots.GetBot()
    fmt.Printf("Get me: %#v %#v", info, err)

    ctx, cancel := context.WithCancel(context.Background()) // создам
    go func() {
        exit := make(chan os.Signal)
        signal.Notify(exit, os.Kill, os.Interrupt)
        <-exit
        cancel()
    }()

    for upd := range api.GetUpdates(ctx) { // Чтение из канала с обновлениями
        switch upd := upd.(type) { // Определение типа пришедшего обновления
        case *schemes.MessageCreatedUpdate:
            // Отправка сообщения
            _, err := api.Messages.Send(maxbot.NewMessage().SetChat(upd.Message.Recipient.ChatId).SetText("Привет! ✨"))
        }
    }
}
```

5. Протестируйте бота — отправьте команду `/hello`

![](https://dev.max.ru/assets/hello_dark.png)
*Чат с Hello Bot*

ℹ️ Если у вас возникли вопросы, [посмотрите раздел с ответами](../../../help/index.md)

Готово! Вы написали простого и дружелюбного Hello Bot. Воспользуйтесь возможностями и инструментами платформы MAX, чтобы запустить на платформе собственные проекты

---

Источник: <https://dev.max.ru/docs/chatbots/bots-coding/examples>
