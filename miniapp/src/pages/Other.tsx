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
import { calendarDaysBetween, splitAddress } from '../shared/lib/format';
import { groupQueue } from '../shared/lib/model';
import { IssueList } from '../shared/ui/IssueRow';
import { EmptyState, ErrorState, Island, Loading, Screen, Section, useToast } from '../shared/ui/Layout';
import { Segmented } from '../shared/ui/Segmented';
import s from './pages.module.css';
import { StickersView } from './Stickers';
import st from './stickers.module.css';
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
  const startHouse = useUser().house_id;

  // Житель мог выбрать дом в чате с ботом (геопозицией): при возврате в приложение подхватываем его.
  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState !== 'visible') return;
      api.me().then(
        (u) => {
          if (u.house_id && u.house_id !== startHouse) {
            setUser(u);
            reset({ name: 'home' });
          }
        },
        () => {},
      );
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => document.removeEventListener('visibilitychange', onVisible);
  }, [startHouse, setUser, reset]);

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

  // В WebView MAX геолокации обычно нет, а в Bridge её нет совсем (dev-max/docs/webapps/bridge.md):
  // тогда дома рядом выбираются кнопкой геопозиции в чате с ботом, дом подхватится при возврате.
  const viaBot = () => {
    bridge.openMaxLink(`https://max.ru/${BOT_NAME}?start=geo`);
    showToast('Отправьте геопозицию в чате с ботом и выберите дом, затем вернитесь сюда');
  };

  const nearby = () => {
    if (!('geolocation' in navigator)) {
      if (bridge.inMax()) viaBot();
      else showToast('Геопозиция не поддерживается вашим браузером. Введите адрес');
      return;
    }
    navigator.geolocation.getCurrentPosition(
      ({ coords: { latitude, longitude } }) => {
        api.nearestHouses(latitude, longitude).then(
          (list) => {
            setFound(list);
            if (list.length === 0) {
              showToast('Рядом нет подключенных домов. Введите адрес вручную');
            }
          },
          () => showToast('Не удалось найти дома рядом'),
        );
        // Адрес по карте только подсказывает, какой дом выбрать: без него список домов работает.
        api.reverseGeocode(latitude, longitude).then(setPlace, () => setPlace(null));
      },
      (err) => {
        if (bridge.inMax()) {
          viaBot();
          return;
        }
        if (err.code === 1) {
          showToast('Доступ к геопозиции заблокирован. Разрешите его в настройках браузера или введите адрес');
        } else {
          showToast('Не удалось определить координаты. Введите адрес вручную');
        }
      },
      // В MAX долго не ждём: если геопозиции нет, сразу переходим в чат с ботом.
      { timeout: bridge.inMax() ? 6_000 : 15_000, enableHighAccuracy: true },
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
      {found && found.length === 0 && (
        <EmptyState
          title="Ничего не нашли"
          text="Укажите улицу и номер дома в Москве, например: Тверская 12 или Ореховый бульвар 17к2"
        />
      )}
      {found && found.length > 0 && (
        <CellList mode="island">
          {found.map((h) => (
            <CellSimple key={h.id} title={h.address} subtitle={h.district} showChevron onClick={() => choose(h)} />
          ))}
        </CellList>
      )}
      {/* Поиск дополняется домами из OpenStreetMap: подпись требует лицензия ODbL (ADR-016). */}
      <p className={s.hint}>Адреса: © участники OpenStreetMap</p>
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
      <RoleSwitcher />
    </Screen>
  );
}

/** Очередь УК: сводка, фильтры по дому и категории, заявки по срочности; статус меняется в карточке заявки. */
function QueueView() {
  const { push } = useRouter();
  const res = useResource(() => api.ukQueue(), []);
  const [houseFilter, setHouseFilter] = useState('');
  const [catFilter, setCatFilter] = useState('');
  const now = new Date();
  return (
    <>
      {res.loading && !res.data ? (
        <Loading />
      ) : res.error || !res.data ? (
        <ErrorState title="Не удалось загрузить очередь" message={res.error?.message ?? ''} onRetry={res.reload} />
      ) : (
        (() => {
          const housesMap = new Map<string, string>();
          const catsMap = new Map<string, string>();
          for (const item of res.data) {
            if (item.house_id && item.address && !housesMap.has(item.house_id)) {
              housesMap.set(item.house_id, splitAddress(item.address).number || item.address);
            }
            if (item.category && !catsMap.has(item.category)) {
              catsMap.set(item.category, item.category_title || item.category);
            }
          }
          const filtered = res.data.filter(
            (i) => (!houseFilter || i.house_id === houseFilter) && (!catFilter || i.category === catFilter),
          );
          const open = filtered.filter((i) => !isClosed(i.status));
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
              {housesMap.size > 1 && (
                <div className={st.chips} role="group" aria-label="Фильтр по дому">
                  <button type="button" className={st.chip} aria-pressed={houseFilter === ''} onClick={() => setHouseFilter('')}>
                    Все дома
                  </button>
                  {[...housesMap.entries()].map(([id, label]) => (
                    <button key={id} type="button" className={st.chip} aria-pressed={houseFilter === id} onClick={() => setHouseFilter(id)}>
                      {label}
                    </button>
                  ))}
                </div>
              )}
              {catsMap.size > 1 && (
                <div className={st.chips} role="group" aria-label="Фильтр по категории">
                  <button type="button" className={st.chip} aria-pressed={catFilter === ''} onClick={() => setCatFilter('')}>
                    Все категории
                  </button>
                  {[...catsMap.entries()].map(([code, title]) => (
                    <button key={code} type="button" className={st.chip} aria-pressed={catFilter === code} onClick={() => setCatFilter(code)}>
                      {title}
                    </button>
                  ))}
                </div>
              )}
              {filtered.length === 0 ? (
                <EmptyState title="Заявок нет" text="По выбранному фильтру заявок не найдено." />
              ) : (
                groupQueue(filtered, now).map((g) => (
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

/** Быстрое переключение роли внутри мини-приложения (для демонстрации и проверки всех кабинетов). */
export function RoleSwitcher() {
  const user = useUser();
  const { setUser, loginDemo } = useSession();
  const { reset } = useRouter();
  const [busy, setBusy] = useState(false);

  const active: DemoRole =
    user.role === 'uk_operator'
      ? 'uk_operator'
      : user.role === 'district'
        ? 'district'
        : user.chairman
          ? 'chairman'
          : 'resident';

  const roles: { id: DemoRole; label: string }[] = [
    { id: 'resident', label: 'Житель' },
    { id: 'chairman', label: 'Председатель' },
    { id: 'uk_operator', label: 'Сотрудник УК' },
    { id: 'district', label: 'Управа района' },
  ];

  const select = async (role: DemoRole) => {
    if (role === active || busy) return;
    if (!bridge.inMax()) {
      loginDemo(role);
      return;
    }
    setBusy(true);
    try {
      const updated = await api.switchRole(role);
      setUser(updated);
      if (updated.role === 'uk_operator') reset({ name: 'uk' });
      else if (updated.role === 'district') reset({ name: 'district' });
      else if (updated.house_id) reset({ name: 'home' });
      else reset({ name: 'houseSearch' });
    } catch {
      /* игнорируем при сбое сети */
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <Section title="Роль для проверки" />
      <div className={st.chips} role="group" aria-label="Роль пользователя">
        {roles.map((r) => (
          <button
            key={r.id}
            type="button"
            className={st.chip}
            disabled={busy}
            aria-pressed={active === r.id}
            onClick={() => select(r.id)}
          >
            {r.label}
          </button>
        ))}
      </div>
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
