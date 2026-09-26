import { describe, expect, it } from 'vitest';
import type { MapHouse } from '../api/types';
import { byUrgency, housesBounds, mapStyleUrl, mapSummary, markerLabel, markerSize, markerTone } from './map';

const house = (over: Partial<MapHouse>): MapHouse => ({
  id: 'h',
  address: 'Ореховый бульвар, 17к2',
  lat: 55.61,
  lon: 37.74,
  open: 0,
  overdue: 0,
  max_overdue_days: 0,
  ...over,
});

describe('точка дома', () => {
  it('тон: просрочка важнее открытых заявок', () => {
    expect(markerTone({ open: 3, overdue: 1 })).toBe('late');
    expect(markerTone({ open: 3, overdue: 0 })).toBe('open');
    expect(markerTone({ open: 0, overdue: 0 })).toBe('calm');
  });

  it('размер растёт как корень и ограничен сверху', () => {
    expect(markerSize(0)).toBe(14);
    expect(markerSize(1)).toBe(28);
    expect(markerSize(5)).toBe(44);
    expect(markerSize(100)).toBe(56);
  });

  it('подпись для экранного чтеца', () => {
    expect(markerLabel(house({}))).toBe('Ореховый бульвар, 17к2: открытых заявок нет');
    expect(markerLabel(house({ open: 2 }))).toBe('Ореховый бульвар, 17к2: открыто 2 заявки');
    expect(markerLabel(house({ open: 5, overdue: 2, max_overdue_days: 1 }))).toBe(
      'Ореховый бульвар, 17к2: открыто 5 заявок, просрочено 2, самая давняя на 1 день',
    );
  });
});

describe('список домов', () => {
  it('сначала самая давняя просрочка, затем нагрузка, затем адрес', () => {
    const list = [
      house({ id: 'calm', address: 'Б' }),
      house({ id: 'busy', open: 7 }),
      house({ id: 'old', open: 1, overdue: 1, max_overdue_days: 9 }),
      house({ id: 'late', open: 4, overdue: 2, max_overdue_days: 3 }),
      house({ id: 'calm2', address: 'А' }),
    ];
    expect(list.sort(byUrgency).map((h) => h.id)).toEqual(['old', 'late', 'busy', 'calm2', 'calm']);
  });

  it('сводка', () => {
    expect(mapSummary([house({ open: 3, overdue: 1 }), house({ open: 2 }), house({})])).toEqual({
      houses: 3,
      open: 5,
      overdue: 1,
      lateHouses: 1,
    });
  });
});

describe('рамка карты', () => {
  it('нет домов — нет рамки', () => {
    expect(housesBounds([])).toBeNull();
  });

  it('охватывает все дома', () => {
    expect(housesBounds([house({ lat: 55.6, lon: 37.7 }), house({ lat: 55.65, lon: 37.8 })])).toEqual([
      [37.7, 55.6],
      [37.8, 55.65],
    ]);
  });

  it('вокруг одного дома — отступ, а не точка', () => {
    const [[w, s], [e, n]] = housesBounds([house({})])!;
    expect(e - w).toBeCloseTo(0.006);
    expect(n - s).toBeCloseTo(0.006);
  });
});

it('подложка под тему', () => {
  expect(mapStyleUrl('light')).toContain('/positron');
  expect(mapStyleUrl('dark')).toContain('/dark');
});
