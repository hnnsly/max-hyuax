import { describe, expect, it } from 'vitest';
import { calendarDaysBetween, capitalize, dayMonth, deadlineLabel, dotDateTime, plural, splitAddress } from './format';

describe('plural', () => {
  it('согласует числительные', () => {
    const f = (n: number) => `${n} ${plural(n, 'сосед', 'соседа', 'соседей')}`;
    expect([1, 2, 4, 5, 11, 12, 14, 21, 22, 25, 101, 111].map(f)).toEqual([
      '1 сосед', '2 соседа', '4 соседа', '5 соседей', '11 соседей', '12 соседей', '14 соседей',
      '21 сосед', '22 соседа', '25 соседей', '101 сосед', '111 соседей',
    ]);
  });
});

describe('даты по Москве', () => {
  it('день и месяц в родительном падеже', () => {
    expect(dayMonth('2026-09-19T20:59:59Z')).toBe('19 сентября');
    // 23:30 UTC — уже следующий день по Москве.
    expect(dayMonth('2026-09-19T21:30:00Z')).toBe('20 сентября');
  });

  it('дата и время для хронологии', () => {
    expect(dotDateTime('2026-09-17T05:10:00Z')).toBe('17.09 08:10');
  });

  it('календарные дни между датами по Москве', () => {
    expect(calendarDaysBetween('2026-09-17T08:00:00+03:00', '2026-09-19T23:59:59+03:00')).toBe(2);
    expect(calendarDaysBetween('2026-09-19T23:00:00+03:00', '2026-09-19T01:00:00+03:00')).toBe(0);
    expect(calendarDaysBetween('2026-09-21T10:00:00+03:00', '2026-09-19T23:59:59+03:00')).toBe(-2);
  });
});

describe('адрес и место', () => {
  it('делит адрес на улицу и номер дома для таблички', () => {
    expect(splitAddress('Ореховый бульвар, 17к2')).toEqual({ street: 'Ореховый бульвар', number: '17к2' });
    expect(splitAddress('Без номера')).toEqual({ street: 'Без номера', number: '' });
  });
  it('делает первую букву заглавной', () => {
    expect(capitalize('подъезд 2, пассажирский лифт')).toBe('Подъезд 2, пассажирский лифт');
    expect(capitalize('')).toBe('');
  });
});

describe('подпись срока', () => {
  const deadline = '2026-09-19T23:59:59+03:00';
  it('до срока', () => {
    expect(deadlineLabel(deadline, new Date('2026-09-17T10:00:00+03:00'), false)).toEqual({ text: 'до 19 сентября', overdue: false });
  });
  it('срок сегодня', () => {
    expect(deadlineLabel(deadline, new Date('2026-09-19T10:00:00+03:00'), false)).toEqual({ text: 'срок сегодня', overdue: false });
  });
  it('просрочено', () => {
    expect(deadlineLabel(deadline, new Date('2026-09-20T10:00:00+03:00'), true)).toEqual({ text: 'просрочено на 1 день', overdue: true });
    expect(deadlineLabel(deadline, new Date('2026-09-24T10:00:00+03:00'), true)).toEqual({ text: 'просрочено на 5 дней', overdue: true });
  });
});
