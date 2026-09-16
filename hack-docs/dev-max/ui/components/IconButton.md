# IconButton

[Source](https://github.com/max-messenger/max-ui/tree/main/src/components/IconButton) · [Issues](https://github.com/max-messenger/max-ui/issues) · [Storybook](https://max-messenger.github.io/max-ui/?path=/docs/common-iconbutton--docs)

## Playground

```jsx
<IconButton
  appearance="themed"
  aria-label="Название кнопки"
  mode="primary"
  size="medium"
>
  <Icon24Placeholder />
</IconButton>
```

| Параметр | Значение |
| --- | --- |
| mode | `"link"` `"primary"` `"secondary"` `"tertiary"` |
| appearance | `"themed"` `"negative"` `"neutral"` `"neutral-themed"` `"contrast-static"` |
| size | `"small"` `"medium"` `"large"` |
| disabled | `boolean` |
| loading | `boolean` |
| innerClassNames | `InnerClassNamesProp<IconButtonInnerElementKey>` |
| asChild | `boolean` |

## As link

```jsx
<IconButton
  appearance="themed"
  aria-label="Название кнопки"
  asChild
  mode="primary"
  size="medium"
>
  <a
    href="/"
    rel="noreferrer"
    target="_blank"
  >
    <Icon24Placeholder />
  </a>
</IconButton>
```

---

Источник: <https://dev.max.ru/ui/components/IconButton>
