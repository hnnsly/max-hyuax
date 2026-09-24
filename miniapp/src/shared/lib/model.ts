// Модели отображения: шкала срока, хронология заявки, разбор параметра запуска.
import type { CategoryHint, IssueEvent, Status } from '../api/types';
import { calendarDaysBetween, plural } from './format';

export type RailItem =
  | { kind: 'seg'; tone: 'done' | 'left' | 'late' }
  | { kind: 'today'; late: boolean }
  | { kind: 'deadline' };

/**
 * Шкала срока: n отрезков от подачи до срока, отметка «сегодня» и отметка срока.
 * После срока отрезки плана закрашены, дни просрочки (не больше n) красные, «сегодня» в конце.
 */
export function buildRail(created: string, deadline: string, now: Date, n: number): RailItem[] {
  const seg = (tone: 'done' | 'left' | 'late'): RailItem => ({ kind: 'seg', tone });
  const late = calendarDaysBetween(deadline, now);
  if (late > 0) {
    return [
      ...Array.from({ length: n }, () => seg('done')),
      { kind: 'deadline' },
      ...Array.from({ length: Math.min(late, n) }, () => seg('late')),
      { kind: 'today', late: true },
    ];
  }
  const total = Math.max(1, calendarDaysBetween(created, deadline));
  const elapsed = Math.min(total, Math.max(0, calendarDaysBetween(created, now)));
  const done = Math.round((elapsed / total) * n);
  return [
    ...Array.from({ length: done }, () => seg('done')),
    { kind: 'today', late: false },
    ...Array.from({ length: n - done }, () => seg('left')),
    { kind: 'deadline' },
  ];
}

export interface TimelineItem {
  at: string;
  text: string;
  comment?: string;
  /** Событие просрочки: точка на шкале красная. */
  late?: boolean;
  last: boolean;
}

const statusText: Record<Status, string> = {
  sent: 'Заявка отправлена в УК',
  accepted: 'УК приняла заявку',
  in_progress: 'УК взяла заявку в работу',
  done: 'УК отметила заявку выполненной',
  rejected: 'УК отклонила заявку',
};

/** Хронология для карточки: присоединения подряд склеиваются, имена не показываются. */
export function buildTimeline(events: IssueEvent[]): TimelineItem[] {
  const out: Omit<TimelineItem, 'last'>[] = [];
  let joined = 0;
  const flushJoined = (at: string) => {
    if (joined === 0) return;
    const verb = joined === 1 ? 'Присоединился' : 'Присоединились';
    out.push({ at, text: `${verb} ещё ${joined} ${plural(joined, 'сосед', 'соседа', 'соседей')}` });
    joined = 0;
  };
  let lastJoinAt = '';
  for (const e of events) {
    if (e.kind === 'joined') {
      joined++;
      lastJoinAt = e.at;
      continue;
    }
    flushJoined(lastJoinAt);
    if (e.kind === 'created') out.push({ at: e.at, text: 'Житель сообщил о проблеме' });
    else if (e.kind === 'overdue') out.push({ at: e.at, text: 'Срок ответа истёк', late: true });
    else out.push({ at: e.at, text: statusText[e.status], ...(e.comment ? { comment: e.comment } : {}) });
  }
  flushJoined(lastJoinAt);
  return out.map((item, i) => ({ ...item, last: i === out.length - 1 }));
}

const transitions: Record<Status, Status[]> = {
  sent: ['accepted', 'in_progress', 'rejected'],
  accepted: ['in_progress', 'done', 'rejected'],
  in_progress: ['done', 'rejected'],
  done: [],
  rejected: [],
};

/** Куда можно перевести заявку; повторяет правила агрегата Issue на бэкенде. */
export const nextStatuses = (s: Status): Status[] => transitions[s];

interface QueueItem {
  status: Status;
  overdue: boolean;
  deadline: string;
}

/** Группы очереди УК: сначала то, что горит, закрытые в конце. Пустые группы не показываются. */
export function groupQueue<T extends QueueItem>(items: T[], now: Date): { title: string; late?: boolean; items: T[] }[] {
  const closed = (i: T) => i.status === 'done' || i.status === 'rejected';
  const groups = [
    { title: 'Просрочено', late: true, items: items.filter((i) => !closed(i) && i.overdue) },
    { title: 'Срок сегодня и завтра', items: items.filter((i) => !closed(i) && !i.overdue && calendarDaysBetween(now, i.deadline) <= 1) },
    { title: 'Новые', items: [] as T[] },
    { title: 'В работе', items: [] as T[] },
    { title: 'Закрытые', items: items.filter(closed) },
  ];
  const taken = new Set([...groups[0]!.items, ...groups[1]!.items]);
  for (const i of items) {
    if (closed(i) || taken.has(i)) continue;
    (i.status === 'sent' ? groups[2]! : groups[3]!).items.push(i);
  }
  return groups.filter((g) => g.items.length > 0);
}

/** Текст для чата дома: без имён и квартир, только заявка и число сообщивших. */
export function shareText(it: { number: number; title: string; address?: string; place?: string; participant_count: number }): string {
  const n = it.participant_count;
  const where = [it.address, it.place].filter(Boolean).join(', ');
  return `Заявка № ${it.number}: ${it.title}. ${where}. Уже ${plural(n, 'сообщил', 'сообщили', 'сообщили')} ${n} ${plural(n, 'сосед', 'соседа', 'соседей')}. Если у вас то же самое, присоединяйтесь:`;
}

export type StartTarget =
  | { kind: 'issue'; id: string }
  | { kind: 'object'; code: string }
  | { kind: 'new'; category: string };

/** Разбирает start_param мини-приложения или ссылку из QR-кода с ?startapp=. */
export function parseStartParam(raw: string): StartTarget | null {
  let value = raw.trim();
  if (value.startsWith('https://')) {
    try {
      value = new URL(value).searchParams.get('startapp') ?? '';
    } catch {
      return null;
    }
  }
  const [prefix, rest] = [value.slice(0, 2), value.slice(2)];
  if (!rest) return null;
  switch (prefix) {
    case 'i_':
      return { kind: 'issue', id: rest };
    case 'o_':
      return { kind: 'object', code: rest };
    case 'n_':
      return { kind: 'new', category: rest };
  }
  return null;
}

/** Что предложить жителю по подсказке: категорию, если она отличается от уже выбранной. */
export function hintOffer(hint: CategoryHint | null, current: string): { code: string; title: string } | null {
  if (!hint?.category || hint.category === current) return null;
  return { code: hint.category, title: hint.title ?? hint.category };
}

/** Подсказку спрашиваем, когда в тексте уже есть о чём судить. */
export const worthHint = (text: string) => text.trim().length >= 8;
