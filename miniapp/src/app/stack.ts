// Стек экранов мини-приложения. В MAX нет адресной строки, поэтому навигация — стек,
// а «Назад» — нативная кнопка клиента (BackButton).

export type Route =
  | { name: 'home' }
  | { name: 'houseSearch' }
  | { name: 'mine' }
  | { name: 'issue'; id: string; flash?: string }
  | { name: 'report'; objectCode?: string; category?: string }
  | { name: 'consent' }
  | { name: 'uk' }
  | { name: 'district' }
  | { name: 'districtMap' }
  | { name: 'housePick' }
  | { name: 'rating' }
  | { name: 'appointments' }
  | { name: 'council' };

export type StackAction = { type: 'push'; route: Route } | { type: 'back' } | { type: 'reset'; routes: Route[] };

export function stackReducer(stack: Route[], action: StackAction): Route[] {
  switch (action.type) {
    case 'push':
      return [...stack, action.route];
    case 'back':
      return stack.length > 1 ? stack.slice(0, -1) : stack;
    case 'reset':
      return action.routes.length > 0 ? action.routes : stack;
  }
}
