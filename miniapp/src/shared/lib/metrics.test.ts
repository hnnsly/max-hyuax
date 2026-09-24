import { describe, expect, it } from 'vitest';
import { chartTopHours, compareWeeks, formatResponse, longestDay, onTimeShare, residentCheck, shortResponse, weekdayShort } from './metrics';

describe('время первого ответа', () => {
  it('меньше часа показывается в минутах', () => {
    expect(formatResponse(40)).toEqual({ value: '40', unit: 'минут' });
    expect(formatResponse(1)).toEqual({ value: '1', unit: 'минута' });
  });

  it('от часа: часы и минуты, как на холсте', () => {
    expect(formatResponse(340)).toEqual({ value: '5:40', unit: 'часов' });
    expect(formatResponse(65)).toEqual({ value: '1:05', unit: 'часов' });
  });

  it('ровные часы склоняются', () => {
    expect(formatResponse(120)).toEqual({ value: '2', unit: 'часа' });
    expect(formatResponse(60)).toEqual({ value: '1', unit: 'час' });
  });

  it('короткая подпись для линии медианы', () => {
    expect([340, 240, 40].map(shortResponse)).toEqual(['5:40', '4 ч', '40 мин']);
  });
});

describe('сравнение с прошлой неделей', () => {
  it('быстрее и медленнее в часах', () => {
    expect(compareWeeks(340, 460)).toEqual({ text: 'на 2 часа быстрее', tone: 'good' });
    expect(compareWeeks(460, 340)).toEqual({ text: 'на 2 часа медленнее', tone: 'bad' });
  });

  it('меньше часа разницы в минутах, мелкая разница не считается', () => {
    expect(compareWeeks(300, 340)).toEqual({ text: 'на 40 минут быстрее', tone: 'good' });
    expect(compareWeeks(300, 305)).toEqual({ text: 'как неделю назад', tone: 'same' });
  });

  it('без данных за одну из недель сравнения нет', () => {
    expect(compareWeeks(null, 300)).toBeNull();
    expect(compareWeeks(300, null)).toBeNull();
  });
});

describe('график по дням', () => {
  const days = [
    { date: '2026-09-11', median_min: 300 },
    { date: '2026-09-12', median_min: 610 },
    { date: '2026-09-13', median_min: null },
    { date: '2026-09-14', median_min: 120 },
    { date: '2026-09-15', median_min: null },
    { date: '2026-09-16', median_min: 200 },
    { date: '2026-09-17', median_min: 90 },
  ];

  it('верх шкалы: чётное число часов не меньше максимума', () => {
    expect(chartTopHours(days, 340)).toBe(12);
    expect(chartTopHours([{ date: '2026-09-17', median_min: 30 }], null)).toBe(2);
    expect(chartTopHours([], null)).toBe(2);
  });

  it('день недели по дате без сдвига часового пояса', () => {
    expect(days.map((d) => weekdayShort(d.date))).toEqual(['пт', 'сб', 'вс', 'пн', 'вт', 'ср', 'чт']);
  });

  it('самый долгий день', () => {
    expect(longestDay(days)).toBe('суббота');
    expect(longestDay([{ date: '2026-09-17', median_min: null }])).toBeNull();
  });
});

describe('закрыто в срок', () => {
  it('процент и пустой случай', () => {
    expect(onTimeShare({ closed_total: 16, closed_on_time: 13 })).toBe(81);
    expect(onTimeShare({ closed_total: 0, closed_on_time: 0 })).toBeNull();
  });
});

describe('проверка ремонта жителями', () => {
  it('подписи с согласованием чисел', () => {
    expect(residentCheck({ confirmed_by_residents: 14, reopened_by_residents: 1 })).toEqual({
      confirmed: 'ремонтов подтвердили жители',
      reopened: 'заявку вернули в работу',
    });
    expect(residentCheck({ confirmed_by_residents: 1, reopened_by_residents: 3 })).toEqual({
      confirmed: 'ремонт подтвердили жители',
      reopened: 'заявки вернули в работу',
    });
  });

  it('пусто, пока жители ничего не проверяли', () => {
    expect(residentCheck({ confirmed_by_residents: 0, reopened_by_residents: 0 })).toBeNull();
  });
});
