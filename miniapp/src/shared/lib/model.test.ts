import { describe, expect, it } from 'vitest';
import { buildRail, buildTimeline, parseStartParam, type RailItem } from './model';

const sym = { done: 'd', left: '-', late: 'x' } as const;
const tones = (items: RailItem[]) =>
  items.map((i) => (i.kind === 'seg' ? sym[i.tone] : i.kind === 'today' ? (i.late ? 'T!' : 'T') : 'D')).join('');

describe('шкала срока', () => {
  it('до срока: прошедшие дни, сегодня, оставшиеся, отметка срока', () => {
    const r = buildRail('2026-09-17T08:00:00+03:00', '2026-09-23T23:59:59+03:00', new Date('2026-09-20T10:00:00+03:00'), 6);
    expect(tones(r)).toBe('dddT---D');
  });

  it('в первый день всё впереди', () => {
    const r = buildRail('2026-09-17T08:00:00+03:00', '2026-09-19T23:59:59+03:00', new Date('2026-09-17T09:00:00+03:00'), 6);
    expect(tones(r)).toBe('T------D');
  });

  it('просрочка: план закрашен, дни просрочки красные, сегодня в конце', () => {
    const r = buildRail('2026-09-09T08:00:00+03:00', '2026-09-16T23:59:59+03:00', new Date('2026-09-18T10:00:00+03:00'), 4);
    expect(tones(r)).toBe('ddddDxxT!');
  });
});

describe('хронология', () => {
  it('склеивает присоединения подряд и не раскрывает, кто действовал', () => {
    const items = buildTimeline([
      { kind: 'created', status: 'sent', at: '2026-09-17T05:10:00Z' },
      { kind: 'joined', status: 'sent', at: '2026-09-17T06:00:00Z' },
      { kind: 'joined', status: 'sent', at: '2026-09-17T06:30:00Z' },
      { kind: 'status_changed', status: 'accepted', comment: 'Мастер приедет завтра', at: '2026-09-17T08:05:00Z' },
      { kind: 'joined', status: 'accepted', at: '2026-09-17T09:00:00Z' },
    ]);
    expect(items.map((i) => i.text)).toEqual([
      'Житель сообщил о проблеме',
      'Присоединились ещё 2 соседа',
      'УК приняла заявку',
      'Присоединился ещё 1 сосед',
    ]);
    expect(items[2]?.comment).toBe('Мастер приедет завтра');
    expect(items.map((i) => i.last)).toEqual([false, false, false, true]);
  });
});

describe('параметр запуска', () => {
  it('разбирает заявку, объект и новую заявку', () => {
    expect(parseStartParam('i_0190a000-0000-7000-8000-000000000001')).toEqual({ kind: 'issue', id: '0190a000-0000-7000-8000-000000000001' });
    expect(parseStartParam('o_h-17k2-e2-lift')).toEqual({ kind: 'object', code: 'h-17k2-e2-lift' });
    expect(parseStartParam('n_lift')).toEqual({ kind: 'new', category: 'lift' });
    expect(parseStartParam('')).toBeNull();
    expect(parseStartParam('x_1')).toBeNull();
  });

  it('понимает ссылку из QR-кода', () => {
    expect(parseStartParam('https://max.ru/t105_hakaton_max_bot?startapp=o_h-17k2-e2-lift')).toEqual({ kind: 'object', code: 'h-17k2-e2-lift' });
  });
});
