# CellSimple

[Source](https://github.com/max-messenger/max-ui/tree/main/src/components/CellSimple) · [Issues](https://github.com/max-messenger/max-ui/issues) · [Storybook](https://max-messenger.github.io/max-ui/?path=/docs/common-cellsimple--docs)

## Playground

```jsx
<CellSimple
  after={<Button key="icon" mode="secondary" size="small">Открыть</Button>}
  before={<Icon24Placeholder />}
  height="normal"
  overline=""
  subtitle="Subtitle"
  title="Title"
/>
```

| Параметр | Значение |
| --- | --- |
| title | `ReactNode` |
| subtitle | `ReactNode` |
| overline | `ReactNode` |
| height | `"compact"` `"normal"` |
| showChevron | `boolean` |
| disabled | `boolean` |
| before | `ReactNode` |
| after | `ReactNode` |
| innerClassNames | `InnerClassNamesProp<CellSimpleInnerElementKey>` |
| asChild | `boolean` |
| as | `ElementType` |
| onClick | `function` |

## Tappable

```jsx
<CellSimple
  after={<Counter key="counter" value={1200}/>}
  before={<Icon24Placeholder />}
  height="normal"
  onClick={() => {}}
  overline=""
  showChevron
  subtitle="Subtitle"
  title="Title"
/>
```

## As link

```jsx
<CellSimple
  asChild
  before={<Icon24Placeholder />}
  height="normal"
  overline=""
  title="Я — ссылка!"
>
  <a
    href="/"
    rel="noreferrer"
    target="_blank"
  />
</CellSimple>
```

## Ellipsized title

```jsx
<CellSimple
  after={<Button key="icon" mode="secondary" size="small">Открыть</Button>}
  before={<Icon24Placeholder />}
  height="normal"
  overline=""
  subtitle="Подпись тоже очень длинная, но в этом примере она будет выводиться полностью"
  title={<EllipsisText>Я — ячейка с очень длинным заголовком, поэтому люди не смогут дочитать меня до конца</EllipsisText>}
/>
```

## Ellipsized subtitle

```jsx
<CellSimple
  after={<Button key="icon" mode="secondary" size="small">Открыть</Button>}
  before={<Icon24Placeholder />}
  height="normal"
  overline=""
  subtitle={<EllipsisText>Чего не скажешь о длинной подписи, в этом примере она будет обрезан</EllipsisText>}
  title="Я — ячейка с очень длинным заголовком, но в этот раз текст не будет обрезан"
/>
```

---

Источник: <https://dev.max.ru/ui/components/CellSimple>
