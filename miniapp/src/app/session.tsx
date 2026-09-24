import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { api, ApiError, setToken, setUnauthorizedHandler } from '../shared/api/client';
import type { Session, User } from '../shared/api/types';
import { bridge } from '../shared/bridge/bridge';

type State =
  | { phase: 'loading' }
  | { phase: 'anon' }
  | { phase: 'error'; message: string }
  | { phase: 'deleted' }
  | { phase: 'ready'; user: User; startParam: string };

export const demoRoles = {
  resident: 'Житель',
  resident_2: 'Сосед',
  uk_operator: 'Сотрудник УК',
} as const;
export type DemoRole = keyof typeof demoRoles;

const ROLE_KEY = 'dommax.demoRole';

// sessionStorage может быть недоступен (приватный режим, запреты webview): работаем и без него.
const storage = {
  get: () => {
    try {
      return sessionStorage.getItem(ROLE_KEY);
    } catch {
      return null;
    }
  },
  set: (v: string | null) => {
    try {
      if (v) sessionStorage.setItem(ROLE_KEY, v);
      else sessionStorage.removeItem(ROLE_KEY);
    } catch {
      /* без запоминания роли */
    }
  },
};

function demoRoleFromUrl(): DemoRole | null {
  const v = new URLSearchParams(window.location.search).get('demo');
  if (v === 'uk') return 'uk_operator';
  return v && v in demoRoles ? (v as DemoRole) : null;
}

interface SessionValue {
  state: State;
  loginDemo(role: DemoRole): void;
  setUser(u: User): void;
  logout(): void;
  /** Аккаунт удалён на сервере: сессию забываем и показываем экран прощания. */
  accountDeleted(): void;
}

const SessionContext = createContext<SessionValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ phase: 'loading' });

  const accept = useCallback((s: Session, startParam: string) => {
    setToken(s.token);
    setState({ phase: 'ready', user: s.user, startParam });
  }, []);

  const fail = useCallback((err: unknown) => {
    setState({ phase: 'error', message: err instanceof ApiError ? err.message : 'Не удалось войти. Попробуйте позже' });
  }, []);

  const loginDemo = useCallback(
    (role: DemoRole) => {
      setState({ phase: 'loading' });
      storage.set(role);
      api.loginDemo(role).then((s) => accept(s, bridge.startParam()), fail);
    },
    [accept, fail],
  );

  useEffect(() => {
    setUnauthorizedHandler(() => {
      setToken('');
      setState(bridge.inMax() ? { phase: 'error', message: 'Сессия истекла. Закройте и откройте приложение заново.' } : { phase: 'anon' });
    });
    if (bridge.inMax()) {
      api.loginMax(bridge.initData()).then((s) => accept(s, s.start_param || bridge.startParam()), fail);
      return;
    }
    const role = demoRoleFromUrl() ?? (storage.get() as DemoRole | null);
    if (role && role in demoRoles) loginDemo(role);
    else setState({ phase: 'anon' });
  }, [accept, fail, loginDemo]);

  const value: SessionValue = {
    state,
    loginDemo,
    setUser: (user) => setState((s) => (s.phase === 'ready' ? { ...s, user } : s)),
    logout: () => {
      storage.set(null);
      setToken('');
      setState({ phase: 'anon' });
    },
    accountDeleted: () => {
      storage.set(null);
      setToken('');
      setState({ phase: 'deleted' });
    },
  };
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const v = useContext(SessionContext);
  if (!v) throw new Error('useSession outside SessionProvider');
  return v;
}

/** Текущий пользователь; вызывать только на экранах после входа. */
export function useUser(): User {
  const { state } = useSession();
  if (state.phase !== 'ready') throw new Error('useUser before login');
  return state.user;
}
