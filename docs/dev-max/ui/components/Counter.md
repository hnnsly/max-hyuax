# Counter

[Source](https://github.com/max-messenger/max-ui/tree/main/src/components/Counter) · [Issues](https://github.com/max-messenger/max-ui/issues) · [Storybook](https://max-messenger.github.io/max-ui/?path=/docs/common-counter--docs)

## Playground

```jsx
<Counter
  appearance="themed"
  mode="filled"
  value={1200}
/>
```

| Параметр | Значение |
| --- | --- |
| appearance | `"themed"` `"negative"` `"neutral"` `"neutral-themed"` `"neutral-static"` |
| value\* | `number` |
| rounded | `boolean` |
| disabled | `boolean` |
| muted | `boolean` |
| mode | `"filled"` `"inverse"` |

## Counter in Button

```jsx
<Button indicator={<Counter appearance="themed" mode="filled" value={32}/>}>
  Messages
</Button>
```

---

Источник: <https://dev.max.ru/ui/components/Counter>
