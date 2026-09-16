# Button

[Source](https://github.com/max-messenger/max-ui/tree/main/src/components/Button) · [Issues](https://github.com/max-messenger/max-ui/issues) · [Storybook](https://max-messenger.github.io/max-ui/?path=/docs/common-button--docs)

## Playground

```jsx
<Button
  appearance="themed"
  mode="primary"
  onClick={() => {}}
  size="medium"
>
  Button
</Button>
```

| Параметр | Значение |
| --- | --- |
| mode | `"link"` `"primary"` `"secondary"` `"tertiary"` |
| appearance | `"themed"` `"negative"` `"neutral"` `"neutral-themed"` `"contrast-static"` |
| size | `"small"` `"medium"` `"large"` |
| iconAfter | `ReactNode` |
| iconBefore | `ReactNode` |
| indicator | `ReactNode` |
| children | `string` |
| disabled | `boolean` |
| stretched | `boolean` |
| loading | `boolean` |
| innerClassNames | `InnerClassNamesProp<ButtonInnerElementKey>` |
| asChild | `boolean` |

## Default Button

Состояние компонента по умолчанию представляет собой поведение кнопки при её первой отрисовке на экране. Компонент рендерит кнопку с начальным текстом.

```jsx
<Button
  appearance="themed"
  mode="primary"
  onClick={() => {}}
  size="medium"
>
  Button
</Button>
```

## Link as Button

Компонент кнопки с переходом по заданной ссылке при нажатии. Внешний ресурс по умолчанию будет открыт в новой вкладке браузера.

```jsx
<Button
  appearance="themed"
  asChild
  mode="primary"
  size="medium"
>
  <a
    href="/"
    rel="noreferrer"
    target="_blank"
  >
    Я — ссылка!
  </a>
</Button>
```

---

Источник: <https://dev.max.ru/ui/components/Button>
