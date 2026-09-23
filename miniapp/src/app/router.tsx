import { createContext, useContext, useEffect, useMemo, useReducer, type ReactNode } from 'react';
import { bridge } from '../shared/bridge/bridge';
import { stackReducer, type Route } from './stack';

interface RouterValue {
  route: Route;
  canGoBack: boolean;
  push(route: Route): void;
  back(): void;
  reset(...routes: Route[]): void;
}

const RouterContext = createContext<RouterValue | null>(null);

export function RouterProvider({ initial, children }: { initial: Route[]; children: ReactNode }) {
  const [stack, dispatch] = useReducer(stackReducer, initial);
  const value = useMemo<RouterValue>(
    () => ({
      route: stack[stack.length - 1]!,
      canGoBack: stack.length > 1,
      push: (route) => dispatch({ type: 'push', route }),
      back: () => dispatch({ type: 'back' }),
      reset: (...routes) => dispatch({ type: 'reset', routes }),
    }),
    [stack],
  );

  // Нативная кнопка «Назад» MAX видна, только когда есть куда возвращаться.
  useEffect(() => {
    if (!value.canGoBack) {
      bridge.backButton.hide();
      return;
    }
    const onBack = () => dispatch({ type: 'back' });
    bridge.backButton.show();
    bridge.backButton.onClick(onBack);
    return () => bridge.backButton.offClick(onBack);
  }, [value.canGoBack]);

  useEffect(() => {
    window.scrollTo(0, 0);
  }, [value.route]);

  return <RouterContext.Provider value={value}>{children}</RouterContext.Provider>;
}

export function useRouter(): RouterValue {
  const v = useContext(RouterContext);
  if (!v) throw new Error('useRouter outside RouterProvider');
  return v;
}
