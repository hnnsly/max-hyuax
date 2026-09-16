# Grid

[Source](https://github.com/max-messenger/max-ui/tree/main/src/components/Grid) · [Issues](https://github.com/max-messenger/max-ui/issues) · [Storybook](https://max-messenger.github.io/max-ui/?path=/docs/layout-grid--docs)

## Playground

```jsx
<Grid
  align="start"
  cols={3}
  display="inline-grid"
  gapX={30}
  gapY={10}
  justify="between"
>
  <div
    style={{
      backgroundColor: 'var(--background-surface-secondary)',
      height: 75,
      width: 75
    }}
   />
  <div
    style={{
      backgroundColor: 'var(--background-surface-secondary)',
      height: 75,
      width: 75
    }}
   />
  <div
    style={{
      backgroundColor: 'var(--background-surface-secondary)',
      height: 75,
      width: 75
    }}
   />
  <div
    style={{
      backgroundColor: 'var(--background-surface-secondary)',
      height: 75,
      width: 75
    }}
   />
  <div
    style={{
      backgroundColor: 'var(--background-surface-secondary)',
      height: 75,
      width: 75
    }}
   />
</Grid>
```

| Параметр | Значение |
| --- | --- |
| gapX | `string` `number` |
| gapY | `string` `number` |
| cols | `number` |
| rows | `number` |
| display | `"grid"` `"inline-grid"` |
| align | `"center"` `"baseline"` `"stretch"` `"start"` `"end"` |
| justify | `"center"` `"start"` `"end"` `"between"` |
| gap | `string` `number` |
| asChild | `boolean` |

---

Источник: <https://dev.max.ru/ui/components/Grid>
