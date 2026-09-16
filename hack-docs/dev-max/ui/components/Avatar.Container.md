# Avatar.Container

[Source](https://github.com/max-messenger/max-ui/tree/main/src/components/Avatar/parts/AvatarContainer) · [Issues](https://github.com/max-messenger/max-ui/issues) · [Storybook](https://max-messenger.github.io/max-ui/?path=/docs/common-avatar-avatar-container--docs)

## Playground

```jsx
<>
  <Avatar.Container
    form="circle"
    size={64}
  >
    <Avatar.Icon>
      <Icon24Placeholder />
    </Avatar.Icon>
  </Avatar.Container>
  <Avatar.Container
    form="circle"
    size={64}
  >
    <Avatar.Image
      fallback="VT"
      fallbackGradient="green"
      src="https://sun9-21.userapi.com/1N-rJz6-7hoTDW7MhpWe19e_R_TdGV6Wu5ZC0A/67o6-apnAks.jpg"
    />
  </Avatar.Container>
  <Avatar.Container
    form="circle"
    size={64}
  >
    <Avatar.Text gradient="red">
      VT
    </Avatar.Text>
  </Avatar.Container>
</>
```

| Параметр | Значение |
| --- | --- |
| form | `"circle"` `"squircle"` |
| size | `number` |
| overlay | `ReactNode` |
| innerClassNames | `InnerClassNamesProp<AvatarContainerElementKey>` |
| asChild | `boolean` |
| rightTopCorner | `ReactNode` |
| rightBottomCorner | `ReactNode` |

---

Источник: <https://dev.max.ru/ui/components/Avatar.Container>
