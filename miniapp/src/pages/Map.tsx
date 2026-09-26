import { Button, CellList, CellSimple } from '@maxhub/max-ui';
import { ListBullets, MapTrifold } from '@phosphor-icons/react';
import type { Map as MapLibreMap } from 'maplibre-gl';
import { useEffect, useRef, useState } from 'react';
import { useRouter } from '../app/router';
import { useSession } from '../app/session';
import { api, ApiError } from '../shared/api/client';
import { isClosed, type House, type MapHouse } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { plural } from '../shared/lib/format';
import { byUrgency, housesBounds, mapStyleUrl, mapSummary, markerLabel, markerSize, markerTone } from '../shared/lib/map';
import { IssueList } from '../shared/ui/IssueRow';
import { EmptyState, ErrorState, Island, Loading, Screen, useToast } from '../shared/ui/Layout';
import { Sheet } from '../shared/ui/Sheet';
import s from './map.module.css';
import ps from './pages.module.css';

type View = 'map' | 'list';

// Выбранный вид переживает переход в карточку заявки и возврат назад.
let lastView: View = 'map';

// Подпись кнопок MapLibre для экранного чтеца: по умолчанию они английские.
const locale = {
  'Map.Title': 'Карта домов',
  'Marker.Title': 'Дом',
  'NavigationControl.ZoomIn': 'Приблизить',
  'NavigationControl.ZoomOut': 'Отдалить',
  'NavigationControl.ResetBearing': 'Повернуть на север',
  'AttributionControl.ToggleAttribution': 'Источники карты',
};

// Тайлы не пришли за это время: показываем точки на однотонном фоне с пояснением.
const BASE_TIMEOUT_MS = 5000;

/** WebGL есть не везде (старые WebView, выключенное ускорение): без него сразу список. */
function hasWebGL(): boolean {
  try {
    const c = document.createElement('canvas');
    return Boolean(c.getContext('webgl2') ?? c.getContext('webgl'));
  } catch {
    return false;
  }
}

type MapLib = typeof import('maplibre-gl');
type BasePhase = 'loading' | 'ready' | 'nobase';

/**
 * Карта в контейнере el: MapLibre грузится отдельным чанком только здесь, подложка под тему
 * приложения, поворот и наклон выключены (на телефоне их включают случайно). onBase сообщает,
 * загрузилась ли подложка: без тайлов карта остаётся рабочей на однотонном фоне.
 */
async function createMap(
  el: HTMLElement,
  view: { bounds?: [[number, number], [number, number]]; center?: [number, number]; zoom?: number },
  onBase: (phase: BasePhase) => void,
): Promise<{ ml: MapLib; map: MapLibreMap; stop: () => void }> {
  const [ml, worker] = await Promise.all([
    import('maplibre-gl'),
    // Воркер MapLibre 6 лежит отдельным файлом рядом с библиотекой, и после сборки Vite он
    // по относительному пути не находится. Vite собирает его сам и отдаёт адрес.
    import('maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'),
    import('maplibre-gl/dist/maplibre-gl.css'),
  ]);
  ml.setWorkerUrl(worker.default);
  const scheme = el.closest('[data-scheme]')?.getAttribute('data-scheme') === 'dark' ? 'dark' : 'light';
  const map = new ml.Map({
    container: el,
    style: mapStyleUrl(scheme),
    ...view,
    fitBoundsOptions: { padding: 48, maxZoom: 16 },
    attributionControl: { compact: true },
    dragRotate: false,
    pitchWithRotate: false,
    touchPitch: false,
    locale,
  });
  map.touchZoomRotate.disableRotation();
  map.addControl(new ml.NavigationControl({ showCompass: false }), 'top-right');
  const timer = window.setTimeout(() => {
    if (!map.isStyleLoaded()) onBase('nobase');
  }, BASE_TIMEOUT_MS);
  map.on('load', () => {
    window.clearTimeout(timer);
    onBase('ready');
  });
  map.on('error', () => {
    if (!map.isStyleLoaded()) onBase('nobase');
  });
  return { ml, map, stop: () => window.clearTimeout(timer) };
}

/**
 * Карта домов кабинета УК и района (ADR-020): точка на дом, размер по открытым заявкам,
 * цвет по просрочке. Нажатие на дом открывает лист с его заявками. Тот же список доступен
 * без карты: кнопкой «Списком», без WebGL и для экранного чтеца.
 */
export function HouseMapView() {
  const { push } = useRouter();
  const res = useResource(() => api.mapHouses(), []);
  const [webgl] = useState(hasWebGL);
  const [view, setView] = useState<View>(() => (webgl ? lastView : 'list'));
  const [selected, setSelected] = useState<MapHouse | null>(null);
  const choose = (v: View) => {
    lastView = v;
    setView(v);
  };

  if (res.loading && !res.data) return <Loading />;
  if (res.error || !res.data) {
    return <ErrorState title="Не удалось загрузить карту" message={res.error?.message ?? ''} onRetry={res.reload} />;
  }
  const houses = res.data;
  if (houses.length === 0) {
    return (
      <EmptyState
        title="Домов на карте нет"
        text="У домов пока нет координат. Они появятся после импорта реестра или первой заявки по адресу."
      />
    );
  }

  const sum = mapSummary(houses);
  return (
    <>
      <Island>
        <div className={ps.kpi}>
          <div className={ps.kpiItem}>
            <span className={`${ps.kpiValue} ${sum.lateHouses > 0 ? ps.figureLate : ''}`}>{sum.lateHouses}</span>
            <span className={ps.kpiLabel}>домов с просрочкой</span>
          </div>
          <div className={ps.kpiItem}>
            <span className={ps.kpiValue}>{sum.open}</span>
            <span className={ps.kpiLabel}>открыто заявок</span>
          </div>
          <div className={ps.kpiItem}>
            <span className={ps.kpiValue}>{sum.houses}</span>
            <span className={ps.kpiLabel}>домов</span>
          </div>
        </div>
      </Island>

      <div className={s.toolbar}>
        <p className={s.legend}>
          <span className={s.legendItem}>
            <span className={`${s.legendDot} ${s.late}`} aria-hidden="true" />
            просрочено
          </span>
          <span className={s.legendItem}>
            <span className={`${s.legendDot} ${s.open}`} aria-hidden="true" />
            открыто
          </span>
          <span className={s.legendItem}>
            <span className={`${s.legendDot} ${s.calm}`} aria-hidden="true" />
            нет заявок
          </span>
        </p>
        {webgl && (
          <Button
            variant="secondary"
            size="small"
            iconBefore={view === 'map' ? <ListBullets size={18} /> : <MapTrifold size={18} />}
            onClick={() => choose(view === 'map' ? 'list' : 'map')}
          >
            {view === 'map' ? 'Списком' : 'На карте'}
          </Button>
        )}
      </div>

      {view === 'map' ? (
        <MapCanvas houses={houses} onSelect={setSelected} onFail={() => choose('list')} />
      ) : (
        <HouseLoadList houses={houses} onSelect={setSelected} />
      )}
      <p className={ps.hint}>Размер точки растёт с числом открытых заявок. Цифра на точке: сколько заявок открыто.</p>

      <HouseSheet
        house={selected}
        onClose={() => setSelected(null)}
        onOpenIssue={(id) => {
          setSelected(null);
          push({ name: 'issue', id });
        }}
      />
    </>
  );
}

/**
 * Подложка MapLibre и точки-кнопки. Библиотека грузится отдельным чанком только здесь:
 * остальные экраны её не скачивают.
 */
function MapCanvas({ houses, onSelect, onFail }: { houses: MapHouse[]; onSelect: (h: MapHouse) => void; onFail: () => void }) {
  const box = useRef<HTMLDivElement>(null);
  const handlers = useRef({ onSelect, onFail });
  const [phase, setPhase] = useState<BasePhase>('loading');

  useEffect(() => {
    handlers.current = { onSelect, onFail };
  });

  useEffect(() => {
    const el = box.current;
    if (!el) return;
    let map: MapLibreMap | undefined;
    let stop = () => {};
    let cancelled = false;

    createMap(el, { bounds: housesBounds(houses) ?? undefined }, (p) => !cancelled && setPhase(p))
      .then((made) => {
        if (cancelled) {
          made.stop();
          made.map.remove();
          return;
        }
        ({ map, stop } = made);
        const { ml, map: m } = made;

        // Спокойные дома ложатся вниз, просроченные сверху: их точки не закрывает сосед.
        const order = [...houses].sort(byUrgency).reverse();
        for (const h of order) {
          const button = document.createElement('button');
          button.type = 'button';
          button.className = `${s.marker} ${s[markerTone(h)]}`;
          button.style.setProperty('--size', `${markerSize(h.open)}px`);
          button.setAttribute('aria-label', markerLabel(h));
          if (h.open > 0) button.textContent = String(h.open);
          button.addEventListener('click', (e) => {
            e.stopPropagation();
            handlers.current.onSelect(h);
          });
          new ml.Marker({ element: button }).setLngLat([h.lon, h.lat]).addTo(m);
        }
      })
      .catch(() => {
        // Нет WebGL или чанк не загрузился: тот же список без карты.
        if (!cancelled) handlers.current.onFail();
      });

    return () => {
      cancelled = true;
      stop();
      map?.remove();
    };
  }, [houses]);

  return (
    <div className={s.frame} role="region" aria-label="Карта домов">
      <div ref={box} className={s.canvas} />
      {phase === 'loading' && <p className={s.overlay}>Загружаем карту…</p>}
      {phase === 'nobase' && <p className={s.overlay}>Подложка карты недоступна, дома показаны на пустом фоне.</p>}
    </div>
  );
}

/** Те же дома списком: самые проблемные сверху. */
function HouseLoadList({ houses, onSelect }: { houses: MapHouse[]; onSelect: (h: MapHouse) => void }) {
  const sorted = [...houses].sort(byUrgency);
  return (
    <ul className={s.list} aria-label="Дома">
      {sorted.map((h) => (
        <li key={h.id}>
          <button type="button" className={s.row} onClick={() => onSelect(h)} aria-label={markerLabel(h)}>
            <span className={`${s.rowDot} ${s[markerTone(h)]}`} aria-hidden="true" />
            <span className={s.rowText}>
              {h.address}
              <span className={`${s.rowMeta} ${h.overdue > 0 ? s.rowMetaLate : ''}`}>{houseMeta(h)}</span>
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}

function houseMeta(h: MapHouse): string {
  if (h.open === 0) return 'открытых заявок нет';
  if (h.overdue === 0) return `открыто ${h.open}`;
  const late = `открыто ${h.open}, просрочено ${h.overdue}`;
  // Меньше суток просрочки: число дней не пишем, «0 дней» читается как ошибка.
  const days = h.max_overdue_days;
  return days > 0 ? `${late}, самая давняя ${days} ${plural(days, 'день', 'дня', 'дней')}` : late;
}

/** Лист дома: открытые заявки, из них переход в карточку. */
function HouseSheet({ house, onClose, onOpenIssue }: { house: MapHouse | null; onClose: () => void; onOpenIssue: (id: string) => void }) {
  const res = useResource(async () => (house ? api.houseIssues(house.id) : []), [house?.id]);
  const open = (res.data ?? []).filter((i) => !isClosed(i.status));
  return (
    <Sheet open={house !== null} title={house?.address ?? ''} onClose={onClose}>
      {house && <p className={ps.hint}>{houseMeta(house)}</p>}
      {res.loading ? (
        <Loading />
      ) : res.error ? (
        <ErrorState title="Не удалось загрузить заявки дома" message={res.error.message} onRetry={res.reload} />
      ) : open.length === 0 ? (
        <EmptyState title="Открытых заявок нет" text="Жители этого дома сейчас ничего не ждут." />
      ) : (
        <IssueList issues={open} now={new Date()} onOpen={onOpenIssue} />
      )}
    </Sheet>
  );
}

/** Отдельный экран карты для кабинета района. */
export function DistrictMap() {
  const { back } = useRouter();
  return (
    <Screen title="Карта района" onBack={back}>
      <HouseMapView />
    </Screen>
  );
}

// Москва целиком: с этого вида житель приближает карту к своему дому.
const MOSCOW = { center: [37.6176, 55.7558] as [number, number], zoom: 10 };
// С этого приближения нажатие выбирает здание, а не просто приближает карту к точке.
const PICK_ZOOM = 15;

/**
 * Выбор дома на карте для жителя. Работает и в WebView MAX, где нет геолокации: нажатие на здание
 * ищет дома рядом с точкой, а дом из OpenStreetMap создаётся автоматически, как при поиске рядом.
 */
export function HousePick() {
  const { setUser } = useSession();
  const { reset, back } = useRouter();
  const box = useRef<HTMLDivElement>(null);
  const [webgl] = useState(hasWebGL);
  const [phase, setPhase] = useState<BasePhase | 'failed'>(webgl ? 'loading' : 'failed');
  const [point, setPoint] = useState<{ lat: number; lon: number } | null>(null);
  const [toast, showToast] = useToast();
  const nearby = useResource(async () => (point ? api.nearestHouses(point.lat, point.lon) : []), [point?.lat, point?.lon]);

  useEffect(() => {
    const el = box.current;
    if (!el || !webgl) return;
    let map: MapLibreMap | undefined;
    let stop = () => {};
    let cancelled = false;

    createMap(el, MOSCOW, (p) => !cancelled && setPhase(p))
      .then((made) => {
        if (cancelled) {
          made.stop();
          made.map.remove();
          return;
        }
        ({ map, stop } = made);
        const { ml, map: m } = made;
        const pin = document.createElement('span');
        pin.className = `${s.pin}`;
        pin.setAttribute('aria-hidden', 'true');
        const head = document.createElement('span');
        head.className = `${s.pinHead}`;
        pin.append(head);
        const marker = new ml.Marker({ element: pin, anchor: 'bottom' });
        m.on('click', (e) => {
          if (m.getZoom() < PICK_ZOOM) {
            m.easeTo({ center: e.lngLat, zoom: 17 });
            return;
          }
          marker.setLngLat(e.lngLat).addTo(m);
          setPoint({ lat: e.lngLat.lat, lon: e.lngLat.lng });
        });
      })
      .catch(() => !cancelled && setPhase('failed'));

    return () => {
      cancelled = true;
      stop();
      map?.remove();
    };
  }, [webgl]);

  const choose = async (h: House) => {
    try {
      setUser(await api.setHouse(h.id));
      reset({ name: 'home' });
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не удалось выбрать дом');
    }
  };

  const found = (nearby.data ?? []).slice(0, 3);
  return (
    <Screen title="Дом на карте" onBack={back}>
      {phase === 'failed' ? (
        <EmptyState
          title="Карта на этом устройстве не открывается"
          text="Найдите дом по адресу или отправьте геопозицию в чате с ботом."
          action={
            <Button variant="secondary" size="medium" onClick={back}>
              К поиску по адресу
            </Button>
          }
        />
      ) : (
        <>
          <p className={ps.hint}>Приблизьте карту к своему дому и нажмите на здание.</p>
          <div className={s.frame} role="region" aria-label="Карта для выбора дома">
            <div ref={box} className={s.canvas} />
            {phase === 'loading' && <p className={s.overlay}>Загружаем карту…</p>}
            {phase === 'nobase' && <p className={s.overlay}>Подложка карты недоступна. Найдите дом по адресу.</p>}
          </div>
        </>
      )}
      <Sheet open={point !== null} title="Какой дом ваш?" onClose={() => setPoint(null)}>
        {nearby.loading ? (
          <Loading />
        ) : nearby.error ? (
          <ErrorState title="Не удалось найти дома" message={nearby.error.message} onRetry={nearby.reload} />
        ) : found.length === 0 ? (
          <EmptyState title="Здесь дом не нашёлся" text="Нажмите точнее на здание или найдите дом по адресу. Сервис работает в Москве." />
        ) : (
          <CellList mode="island">
            {found.map((h) => (
              <CellSimple key={h.id} title={h.address} subtitle={h.district} showChevron onClick={() => choose(h)} />
            ))}
          </CellList>
        )}
        {/* Адреса найдены по данным OpenStreetMap: подпись требует лицензия ODbL (ADR-016). */}
        <p className={ps.hint}>Адреса: © участники OpenStreetMap</p>
      </Sheet>
      {toast}
    </Screen>
  );
}
