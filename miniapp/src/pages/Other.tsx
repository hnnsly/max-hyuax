import { Button, CellList, CellSimple, Input } from '@maxhub/max-ui';
import { MagnifyingGlass, NavigationArrow } from '@phosphor-icons/react';
import { useEffect, useState } from 'react';
import { useRouter } from '../app/router';
import { demoRoles, useSession, useUser, type DemoRole } from '../app/session';
import { api, ApiError } from '../shared/api/client';
import type { GeoPlace, House } from '../shared/api/types';
import { isClosed } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { bridge, BOT_NAME } from '../shared/bridge/bridge';
import { calendarDaysBetween } from '../shared/lib/format';
import { groupQueue } from '../shared/lib/model';
import { IssueList } from '../shared/ui/IssueRow';
import { EmptyState, ErrorState, Island, Loading, Screen, Section, useToast } from '../shared/ui/Layout';
import { Segmented } from '../shared/ui/Segmented';
import s from './pages.module.css';
import { StickersView } from './Stickers';
import { UkMetricsView } from './UkMetrics';

/** Выбор дома: поиск по адресу или ближайшие по геопозиции браузера. */
export function HouseSearch() {
  const { setUser } = useSession();
  const { reset, back, canGoBack } = useRouter();
  const [query, setQuery] = useState('');
  const [found, setFound] = useState<House[] | null>(null);
  const [place, setPlace] = useState<GeoPlace | null>(null);
  const [error, setError] = useState('');
  const [toast, showToast] = useToast();

  useEffect(() => {
    const q = query.trim();
    if (q.length < 2) {
      setFound(null);
      return;
    }
    // Ответ на устаревший запрос не должен перезаписать результат для нового текста.
    let stale = false;
    const t = setTimeout(() => {
      api.searchHouses(q).then(
        (list) => {
          if (stale) return;
          setFound(list);
          setError('');
        },
        (err: unknown) => {
          if (!stale) setError(err instanceof ApiError ? err.message : 'Поиск не удался');
        },
      );
    }, 300);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [query]);

  const nearby = () => {
    if (!('geolocation' in navigator)) {
      showToast('Геопозиция недоступна. Найдите дом по адресу');
      return;
    }
    navigator.geolocation.getCurrentPosition(
      ({ coords: { latitude, longitude } }) => {
        api.nearestHouses(latitude, longitude).then(setFound, () => showToast('Не удалось найти дома рядом'));
        // Адрес по карте только подсказывает, какой дом выбрать: без него список домов работает.
        api.reverseGeocode(latitude, longitude).then(setPlace, () => setPlace(null));
      },
      () => showToast('Нет доступа к геопозиции. Найдите дом по адресу'),
      { timeout: 10_000 },
    );
  };

  const choose = async (h: House) => {
    try {
      setUser(await api.setHouse(h.id));
      reset({ name: 'home' });
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не удалось выбрать дом');
    }
  };

  return (
    <Screen title="Мой дом" onBack={canGoBack ? back : undefined}>
      <div className={s.intro}>
        <h2 className={s.introTitle}>Где вы живёте?</h2>
        <p className={s.text}>Найдите свой дом, чтобы видеть его проблемы и сообщать о новых.</p>
      </div>
      <Input
        placeholder="Улица и номер дома"
        aria-label="Адрес"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setPlace(null); // адрес по карте относится к поиску рядом, не к поиску по тексту
        }}
        iconBefore={<MagnifyingGlass size={20} />}
        withClearButton
        autoFocus
      />
      <p className={s.hint}>Например: Ореховый бульвар 17к2</p>
      <Button variant="secondary" size="medium" iconBefore={<NavigationArrow size={18} />} onClick={nearby}>
        Найти дома рядом со мной
      </Button>
      {error && <p className={s.hint} role="alert">{error}</p>}
      {place?.address && (
        <div role="status">
          <p className={s.text}>Вы сейчас здесь: {place.address}</p>
          {/* Подпись требует лицензия данных OpenStreetMap (ODbL). */}
          <p className={s.hint}>Адрес по карте: {place.attribution}</p>
        </div>
      )}
      {found && found.length === 0 && <EmptyState title="Ничего не нашли" text="Проверьте адрес. Пока в сервисе дома одного района Москвы." />}
      {found && found.length > 0 && (
        <CellList mode="island">
          {found.map((h) => (
            <CellSimple key={h.id} title={h.address} subtitle={h.district} showChevron onClick={() => choose(h)} />
          ))}
        </CellList>
      )}
      {toast}
    </Screen>
  );
}

export function MyIssues() {
  const { push, back } = useRouter();
  const res = useResource(() => api.myIssues(), []);
  return (
    <Screen title="Мои заявки" onBack={back}>
      {res.loading && !res.data ? (
        <Loading />
      ) : res.error || !res.data ? (
        <ErrorState title="Не удалось загрузить заявки" message={res.error?.message ?? ''} onRetry={res.reload} />
      ) : res.data.length === 0 ? (
        <EmptyState title="Заявок пока нет" text="Здесь появятся заявки, о которых вы сообщили или к которым присоединились." />
      ) : (
        <IssueList issues={res.data} now={new Date()} showAddress onOpen={(id) => push({ name: 'issue', id })} />
      )}
    </Screen>
  );
}

type UkTab = 'queue' | 'metrics' | 'stickers';
const ukTabs: { id: UkTab; title: string }[] = [
  { id: 'queue', title: 'Заявки' },
  { id: 'metrics', title: 'Метрики' },
  { id: 'stickers', title: 'Наклейки' },
];
// Вкладка переживает переход в карточку заявки и возврат назад.
let lastUkTab: UkTab = 'queue';

/** Кабинет УК: очередь заявок, метрики и наклейки с QR-кодами. */
export function UkQueue() {
  const [tab, setTab] = useState<UkTab>(lastUkTab);
  const choose = (t: UkTab) => {
    lastUkTab = t;
    setTab(t);
  };
  return (
    <Screen title="Кабинет УК">
      <Segmented label="Раздел кабинета УК" items={ukTabs} value={tab} onChange={choose} />
      {tab === 'queue' ? <QueueView /> : tab === 'metrics' ? <UkMetricsView /> : <StickersView />}
    </Screen>
  );
}

/** Очередь УК: сводка и заявки по срочности; статус меняется в карточке заявки. */
function QueueView() {
  const { push } = useRouter();
  const res = useResource(() => api.ukQueue(), []);
  const now = new Date();
  return (
    <>
      {res.loading && !res.data ? (
        <Loading />
      ) : res.error || !res.data ? (
        <ErrorState title="Не удалось загрузить очередь" message={res.error?.message ?? ''} onRetry={res.reload} />
      ) : (
        (() => {
          const open = res.data.filter((i) => !isClosed(i.status));
          const overdue = open.filter((i) => i.overdue).length;
          const today = open.filter((i) => !i.overdue && calendarDaysBetween(now, i.deadline) <= 0).length;
          return (
            <>
              <Island>
                <div className={s.kpi}>
                  <div className={s.kpiItem}>
                    <span className={`${s.kpiValue} ${s.figureLate}`}>{overdue}</span>
                    <span className={s.kpiLabel}>просрочено</span>
                  </div>
                  <div className={s.kpiItem}>
                    <span className={s.kpiValue}>{today}</span>
                    <span className={s.kpiLabel}>срок сегодня</span>
                  </div>
                  <div className={s.kpiItem}>
                    <span className={s.kpiValue}>{open.length}</span>
                    <span className={s.kpiLabel}>всего открыто</span>
                  </div>
                </div>
              </Island>
              {res.data.length === 0 ? (
                <EmptyState title="Заявок нет" text="Когда жители сообщат о проблеме, она появится здесь." />
              ) : (
                groupQueue(res.data, now).map((g) => (
                  <section key={g.title} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    <h2 className={`${s.groupTitle} ${g.late ? s.groupLate : ''}`}>{g.title}</h2>
                    <IssueList issues={g.items} now={now} showAddress onOpen={(id) => push({ name: 'issue', id })} />
                  </section>
                ))
              )}
            </>
          );
        })()
      )}
    </>
  );
}

export function Consent() {
  const user = useUser();
  const { setUser } = useSession();
  const { back } = useRouter();
  const [busy, setBusy] = useState(false);
  const [toast, showToast] = useToast();
  const accept = async () => {
    setBusy(true);
    try {
      setUser(await api.acceptConsent(user.consent_version));
      back();
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не получилось. Попробуйте ещё раз');
      setBusy(false);
    }
  };
  return (
    <Screen
      title="Согласие"
      onBack={back}
      actions={
        <Button variant="primary" size="large" stretched loading={busy} onClick={accept}>
          Согласен
        </Button>
      }
    >
      <div className={s.intro}>
        <h2 className={s.introTitle}>Согласие на обработку данных</h2>
        <p className={s.text}>Чтобы сообщить о проблеме или присоединиться к заявке, нужно ваше согласие на обработку персональных данных.</p>
        <p className={s.text}>Соседи увидят только число сообщивших. Имя получит только управляющая компания.</p>
        <p className={s.hint}>Согласие можно отозвать, удалив аккаунт. Данные хранятся в России.</p>
      </div>
      {toast}
    </Screen>
  );
}


/** Вход вне MAX: ссылка на бота и демо-роли для проверки. */
export function DemoGate() {
  const { loginDemo } = useSession();
  return (
    <Screen title="Мой дом">
      <div className={s.intro}>
        <h2 className={s.introTitle}>Заявки по дому в MAX</h2>
        <p className={s.text}>Одна заявка на весь дом вместо десятка сообщений в чате: с ответственным, сроком и живым статусом.</p>
      </div>
      <Button variant="primary" size="large" stretched asChild>
        <a href={`https://max.ru/${BOT_NAME}`}>Открыть в MAX</a>
      </Button>
      <Section title="Демо для проверки" />
      <p className={s.hint}>Вход без MAX на модельных данных: дом на Ореховом бульваре, два соседа, сотрудник УК и управа района Зябликово.</p>
      <div className={s.stack}>
        {(Object.keys(demoRoles) as DemoRole[]).map((role) => (
          <Button key={role} variant="secondary" size="medium" stretched onClick={() => loginDemo(role)}>
            {demoRoles[role]}
          </Button>
        ))}
      </div>
    </Screen>
  );
}

/** После удаления аккаунта: что стёрто и как вернуться (в MAX — перезапуск, в браузере — демо-вход). */
export function AccountDeleted() {
  const { logout } = useSession();
  return (
    <Screen title="Аккаунт удалён">
      <div className={s.intro}>
        <h2 className={s.introTitle}>Аккаунт удалён</h2>
        <p className={s.text}>
          Имя, телефон и согласие на обработку данных стёрты. Заявки остались в доме без ваших данных: соседи и УК продолжат по ним работать.
        </p>
      </div>
      {bridge.inMax() ? (
        <p className={s.hint}>Чтобы снова сообщать о проблемах, закройте и откройте приложение. Мы попросим согласие заново.</p>
      ) : (
        <Button variant="secondary" size="medium" stretched onClick={logout}>
          Войти снова
        </Button>
      )}
    </Screen>
  );
}
