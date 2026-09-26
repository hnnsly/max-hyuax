// Карта домов кабинета УК и района (ADR-020): тон и размер точки, подписи, порядок, рамка карты.
import type { MapHouse } from '../api/types';
import { plural } from './format';

/** Тон точки: есть просрочка, есть открытые заявки, всё спокойно. */
export type MarkerTone = 'late' | 'open' | 'calm';

export function markerTone(h: Pick<MapHouse, 'open' | 'overdue'>): MarkerTone {
  if (h.overdue > 0) return 'late';
  return h.open > 0 ? 'open' : 'calm';
}

/**
 * Диаметр точки в пикселях. Растёт как корень из числа открытых заявок: площадь круга
 * пропорциональна нагрузке, и дом с 20 заявками не закрывает соседей.
 */
export function markerSize(open: number): number {
  if (open <= 0) return 14;
  return Math.min(56, Math.round(28 + 8 * Math.sqrt(open - 1)));
}

/** Подпись дома для экранного чтеца и строки списка. */
export function markerLabel(h: MapHouse): string {
  if (h.open === 0) return `${h.address}: открытых заявок нет`;
  let label = `${h.address}: открыто ${h.open} ${plural(h.open, 'заявка', 'заявки', 'заявок')}`;
  if (h.overdue > 0) {
    label += `, просрочено ${h.overdue}`;
    if (h.max_overdue_days > 0) {
      label += `, самая давняя на ${h.max_overdue_days} ${plural(h.max_overdue_days, 'день', 'дня', 'дней')}`;
    }
  }
  return label;
}

/** Порядок списка: сначала дома с самой давней просрочкой, затем по числу открытых, затем по адресу. */
export function byUrgency(a: MapHouse, b: MapHouse): number {
  return (
    b.max_overdue_days - a.max_overdue_days ||
    b.overdue - a.overdue ||
    b.open - a.open ||
    a.address.localeCompare(b.address, 'ru')
  );
}

export type Bounds = [[west: number, south: number], [east: number, north: number]];

// Около 300 м: рамка вокруг одного дома, иначе карта уйдёт в предельное приближение.
const SINGLE_PAD = 0.003;

/** Рамка, в которую помещаются все дома; null — домов нет. */
export function housesBounds(houses: Pick<MapHouse, 'lat' | 'lon'>[]): Bounds | null {
  if (houses.length === 0) return null;
  let west = Infinity;
  let south = Infinity;
  let east = -Infinity;
  let north = -Infinity;
  for (const h of houses) {
    west = Math.min(west, h.lon);
    east = Math.max(east, h.lon);
    south = Math.min(south, h.lat);
    north = Math.max(north, h.lat);
  }
  if (east - west < SINGLE_PAD && north - south < SINGLE_PAD) {
    return [
      [west - SINGLE_PAD, south - SINGLE_PAD],
      [east + SINGLE_PAD, north + SINGLE_PAD],
    ];
  }
  return [
    [west, south],
    [east, north],
  ];
}

/** Сводка над картой. */
export function mapSummary(houses: MapHouse[]) {
  let open = 0;
  let overdue = 0;
  let lateHouses = 0;
  for (const h of houses) {
    open += h.open;
    overdue += h.overdue;
    if (h.overdue > 0) lateHouses++;
  }
  return { houses: houses.length, open, overdue, lateHouses };
}

/**
 * Подложка OpenFreeMap: без ключей и лимитов. Обязательную атрибуцию OpenMapTiles и OpenStreetMap
 * тайлы отдают сами (поле attribution в TileJSON), MapLibre показывает её в углу карты.
 */
export function mapStyleUrl(scheme: 'light' | 'dark'): string {
  return `https://tiles.openfreemap.org/styles/${scheme === 'dark' ? 'dark' : 'positron'}`;
}
