# Обзор

[MAX UI](https://github.com/max-messenger/max-ui) — библиотека React-компонентов для создания мини-приложений в MAX, сторонних суперприложений, а также standalone-приложений. Готовые компоненты библиотеки умеют подстраиваться под разные платформы и устройства

Ознакомьтесь с основными принципами устройства интерфейса, навигации и типографики, а также примерами мини-приложений в нашем гайдлайне — его можно скачать в формате `.FIG` по [ссылке](https://github.com/max-messenger/max-ui/blob/main/MAXUI-Figma.fig)

## Особенности MAX UI

- Дизайн-система

  Библиотека компонентов разработана на основе дизайн-системы MAX, что позволяет мини-приложениям выглядеть гармонично в интерфейсе цифровой платформы
- Единообразие на разных платформах

  Компоненты библиотеки органично встраиваются в мобильные платформы iOS и Android, а также в экраны устройств разного размера
- Современный UI Kit

  Typescript, React 18+, полиморфные компоненты и подробная документация с примерами

> Знаете, как улучшить MAX UI? Мы открыты к предложениям:
>
> - Чтобы сообщить об ошибках в библиотеке, [создайте **New issue**](https://github.com/max-messenger/max-ui/issues) в [репозитории MAX UI](https://github.com/max-messenger/max-ui)
> - Чтобы предложить идею, создайте **Fork** библиотеки и откройте **Pull request**

## Подключаем библиотеку MAX UI

Установите библиотеку одной из команд

```shell
npm i @maxhub/max-ui
yarn add @maxhub/max-ui
pnpm add @maxhub/max-ui
```

Оберните код вашего приложения в провайдер MAX UI и подключите стили

```jsx
import { createRoot } from 'react-dom/client';
import { MaxUI } from '@maxhub/max-ui';
import '@maxhub/max-ui/dist/styles.css';
import App from './App.jsx';
const Root = () => (
<MaxUI>
<App />
</MaxUI>
)
createRoot(document.getElementById('root')).render(<Root />);
```

Используйте компоненты библиотеки

```jsx
import { Panel, Grid, Container, Flex, Avatar, Typography } from '@maxhub/max-ui';
const App = () => (
<Panel mode="secondary" className="panel">
<Grid gap={12} cols={1}>
<Container className="me">
<Flex direction="column" align="center">
<Avatar.Container size={72} form="squircle" className="me__avatar">
<Avatar.Image src="https://sun9-21.userapi.com/1N-rJz6-7hoTDW7MhpWe19e_R_TdGV6Wu5ZC0A/67o6-apnAks.jpg" />
</Avatar.Container>
<Typography.Title>Иван Иванов</Typography.Title>
</Flex>
</Container>
</Grid>
</Panel>
)
export default App;
```

## Компоненты

Компоненты библиотеки MAX UI мимикрируют под нативные компоненты iOS и Android и умеют поддерживать светлую и тёмную темы оформления. Тема и платформа определяются автоматически в провайдере MAX UI, но могут быть переопределены через свойства `platform` (`'ios'` | `'android'`) и `colorScheme` (`'light'` | `'dark'`)

```jsx
import { createRoot } from 'react-dom/client';
import { MaxUI } from '@maxhub/max-ui';
import '@maxhub/max-ui/dist/styles.css';
import App from './App.jsx';
const Root = () => (
    <MaxUI platform="android" colorScheme="dark">
        <App />
    </MaxUI>
)
createRoot(document.getElementById('root')).render(<Root />);
```

### Полиморфные компоненты

Полиморфность компонентов реализована через паттерн `asChild prop`: это позволяет предотвратить ошибки типизации и не увеличивать время typescript-процессинга

В DOM полиморфные компоненты могут быть представлены в виде тегов. Например, компонент `Button` — как `button`, `a`, `span` и так далее

| React-компонент | DOM\* |
| --- | --- |
| &lt;Button&gt;<br>Я — кнопка<br>&lt;/Button&gt; | &lt;button class="btn-classes"&gt;<br>Я — кнопка<br>&lt;/button&gt; |
| &lt;Button asChild&gt;<br>&lt;a href="#"&gt;Я — ссылка!&lt;/a&gt;<br>&lt;/Button&gt; | &lt;a class="btn-classes" href="#"&gt;<br>Я — ссылка!<br>&lt;/a&gt; |
| import { Link } from "react-router-dom";<br><br>&lt;Button asChild&gt;<br>&lt;Link to="/home"&gt;Я — ссылка RRD!&lt;/a&gt;<br>&lt;/Button&gt; | &lt;a class="btn-classes" href="/home"&gt;<br>Я — ссылка RRD!<br>&lt;/a&gt; |
|  | \*упрощённое представление компонента |

### Корнер-кейс с asChild

Паттерн `asChild prop` может привести к конфликту свойств, если у одинаковых свойств родительского и дочернего компонентов разные значения. В этом случае свойства `className`, `style` и обработчики событий `on*` (`onClick`, `onChange` и другие) объединяются. В остальных случаях приоритет остаётся у свойств родительского компонента

| React-компонент | DOM\* |
| --- | --- |
| &lt;Button disabled={true} asChild&gt;<br>&lt;button disabled={false}&gt;<br>Кнопка<br>&lt;/button&gt; | &lt;/Button&gt;<br>&lt;button class="btn-classes" disabled&gt;<br>Я — кнопка<br>&lt;/button&gt; |
| &lt;Button style={{ color: 'red' }} asChild&gt;<br>&lt;button style={{ background: 'green' }}&gt;<br>Кнопка<br>&lt;/button&gt; | &lt;/Button&gt;<br>&lt;button class="btn-classes" style={{ color: 'red', background: 'green' }}&gt;<br>Я — кнопка<br>&lt;/button&gt; |
|  | \*упрощённое представление компонента |

### Кастомизация компонентов

ℹ️ Библиотека предоставляет API для кастомизации, но не гарантирует отсутствие изменений в следующих мажорных версиях. Любая кастомизация компонентов — ответственность разработчика мини-приложения

В MAX UI есть два способа кастомизации компонентов: переопределение CSS-переменных и свойство `innerClassNames`

- Переопределение CSS-переменных

  Все токены дизайн-системы MAX заданы в CSS-переменных. Вы можете переопределить переменные как для конкретного компонента, так и для всей темы в целом
- Свойство `innerClassNames`

  Многосоставные компоненты, например `Button`, имеют свойство `innerClassNames`. Он позволяет указать `className` для внутренних элементов

| React-компонент | DOM\* |
| --- | --- |
| &lt;Button <br>   disabled={true}<br>   iconBefore={&lt;svg /&gt;}<br>&gt;<br>   Кнопка с иконкой<br>&lt;/Button&gt; | &lt;button class="btn-classes"&gt;<br>   &lt;span class="icon-before-classes"&gt;<br>      &lt;svg /&gt;<br>   &lt;/span&gt;<br>   &lt;span&gt;Я — кнопка&lt;/span&gt;<br>&lt;/button&gt; |
| &lt;Button <br>   disabled={true}<br>   iconBefore={&lt;svg /&gt;}<br>   innerClassNames={{<br>      iconBefore: 'my-custom-icon-class'<br>   }}<br>&gt;<br>   Кнопка с иконкой<br>&lt;/Button&gt; | &lt;button class="btn-classes"&gt;<br>   &lt;span class="icon-before-classes my-custom-icon-class"&gt;<br>      &lt;svg /&gt;<br>   &lt;/span&gt;<br>   &lt;span&gt;Я — кнопка&lt;/span&gt;<br>&lt;/button&gt; |
|  | \*упрощённое представление компонента |

ℹ️ Если у вас возникли вопросы, [посмотрите раздел с ответами](../help/index.md)

---

Источник: <https://dev.max.ru/ui>
