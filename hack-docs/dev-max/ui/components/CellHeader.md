# CellHeader

[Source](https://github.com/max-messenger/max-ui/tree/main/src/components/CellHeader) · [Issues](https://github.com/max-messenger/max-ui/issues) · [Storybook](https://max-messenger.github.io/max-ui/?path=/docs/common-cellheader--docs)

## Playground

```jsx
<CellList
  header={<CellHeader titleStyle="caps">Пользователь</CellHeader>}
  mode="island"
>
  <CellSimple
    before={<Avatar.Container size={40}><Avatar.Image src="https://sun9-67.userapi.com/s/v1/ig2/CY_xDesKnMtl0OiJynK0oc7QnxQVJUgeciJSi_MpZUiE3EHSCNltr76jugXaygGd2Xh0M8-61v7Jwfl1kO87YWVe.jpg?quality=95&crop=0,0,1440,1440&as=32x32,48x48,72x72,108x108,160x160,240x240,360x360,480x480,540x540,640x640,720x720,1080x1080,1280x1280,1440x1440&ava=1&u=SpmuDKJYdLKKRYYDgjLVQdEn6QnBonR3kSYxCSkCnm4&cs=200x200" /></Avatar.Container>}
    onClick={() => {}}
    showChevron
    title="Vadim Tregubenko"
  />
</CellList>
```

| Параметр | Значение |
| --- | --- |
| children | `string` |
| titleStyle | `"normal"` `"caps"` |
| fullWidth | `boolean` |
| innerClassNames | `InnerClassNamesProp<CellHeaderInnerElementKey>` |
| after | `ReactNode` |

---

Источник: <https://dev.max.ru/ui/components/CellHeader>
