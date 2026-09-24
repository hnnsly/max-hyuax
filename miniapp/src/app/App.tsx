import { MaxUI, Spinner } from '@maxhub/max-ui';
import { useEffect, useState } from 'react';
import type { User } from '../shared/api/types';
import { parseStartParam } from '../shared/lib/model';
import { EmptyState, Screen } from '../shared/ui/Layout';
import { Home } from '../pages/Home';
import { IssueCard } from '../pages/IssueCard';
import { AccountDeleted, Consent, DemoGate, HouseSearch, MyIssues, UkQueue } from '../pages/Other';
import { Report } from '../pages/Report';
import { ErrorBoundary } from './ErrorBoundary';
import { RouterProvider, useRouter } from './router';
import { SessionProvider, useSession } from './session';
import type { Route } from './stack';

type Scheme = 'light' | 'dark';

/** Тема следует системной; ?theme=dark|light — ручная проверка в браузере. */
function useColorScheme(): Scheme {
  const forced = new URLSearchParams(window.location.search).get('theme');
  const query = window.matchMedia('(prefers-color-scheme: dark)');
  const [dark, setDark] = useState(query.matches);
  useEffect(() => {
    const onChange = (e: MediaQueryListEvent) => setDark(e.matches);
    query.addEventListener('change', onChange);
    return () => query.removeEventListener('change', onChange);
  }, [query]);
  if (forced === 'dark' || forced === 'light') return forced;
  return dark ? 'dark' : 'light';
}

/** Стартовый стек: оператор УК — очередь; житель без дома — выбор дома; диплинк — сразу нужный экран. */
function initialStack(user: User, startParam: string): Route[] {
  const target = parseStartParam(startParam);
  const root: Route =
    user.role === 'uk_operator' ? { name: 'uk' } : user.house_id ? { name: 'home' } : { name: 'houseSearch' };
  switch (target?.kind) {
    case 'issue':
      return [root, { name: 'issue', id: target.id }];
    case 'object':
      return user.role === 'resident' ? [root, { name: 'report', objectCode: target.code }] : [root];
    case 'new':
      return user.role === 'resident' ? [root, { name: 'report', category: target.category }] : [root];
  }
  return [root];
}

function Pages() {
  const { route } = useRouter();
  switch (route.name) {
    case 'home':
      return <Home />;
    case 'houseSearch':
      return <HouseSearch />;
    case 'mine':
      return <MyIssues />;
    case 'issue':
      return <IssueCard key={route.id} id={route.id} flash={route.flash} />;
    case 'report':
      return <Report objectCode={route.objectCode} category={route.category} />;
    case 'consent':
      return <Consent />;
    case 'uk':
      return <UkQueue />;
  }
}

function Gate() {
  const { state } = useSession();
  switch (state.phase) {
    case 'loading':
      return (
        <div style={{ minHeight: '100dvh', display: 'grid', placeItems: 'center' }} aria-busy="true" aria-label="Загрузка">
          <Spinner size={32} appearance="themed" />
        </div>
      );
    case 'deleted':
      return <AccountDeleted />;
    case 'anon':
      return <DemoGate />;
    case 'error':
      return (
        <Screen title="Мой дом">
          <EmptyState title="Не удалось войти" text={state.message} />
        </Screen>
      );
    case 'ready':
      return (
        <RouterProvider key={state.user.id} initial={initialStack(state.user, state.startParam)}>
          <Pages />
        </RouterProvider>
      );
  }
}

export function App() {
  const scheme = useColorScheme();
  return (
    <MaxUI colorScheme={scheme}>
      <div className="app-theme" data-scheme={scheme}>
        <SessionProvider>
          <ErrorBoundary>
            <Gate />
          </ErrorBoundary>
        </SessionProvider>
      </div>
    </MaxUI>
  );
}
