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

/**
 * Хронология для карточки: присоединения и подтверждения ремонта подряд склеиваются,
 * имена не показываются.
 */
export function buildTimeline(events: IssueEvent[]): TimelineItem[] {
  const out: Omit<TimelineItem, 'last'>[] = [];
  // Серия одинаковых событий подряд: сколько их и когда было последнее.
  // Приведение, а не аннотация: иначе TypeScript сузит тип до null, не видя присваиваний в flush.
  let run = null as { kind: 'joined' | 'confirmed'; count: number; at: string } | null;
  const flush = () => {
    if (!run) return;
    const n = run.count;
    const text =
      run.kind === 'joined'
        ? `${n === 1 ? 'Присоединился' : 'Присоединились'} ещё ${n} ${plural(n, 'сосед', 'соседа', 'соседей')}`
        : n === 1
          ? 'Сосед подтвердил, что починили'
          : `${n} ${plural(n, 'сосед подтвердил', 'соседа подтвердили', 'соседей подтвердили')}, что починили`;
    out.push({ at: run.at, text });
    run = null;
  };
  for (const e of events) {
    if (e.kind === 'joined' || e.kind === 'confirmed') {
      if (run?.kind !== e.kind) flush();
      run = { kind: e.kind, count: (run?.count ?? 0) + 1, at: e.at };
      continue;
    }
    flush();
    const comment = e.comment ? { comment: e.comment } : {};
    if (e.kind === 'created') out.push({ at: e.at, text: 'Житель сообщил о проблеме' });
    else if (e.kind === 'overdue') out.push({ at: e.at, text: 'Срок ответа истёк', late: true });
    else if (e.kind === 'reopened') out.push({ at: e.at, text: 'Житель вернул заявку в работу: не починили', ...comment });
    else out.push({ at: e.at, text: statusText[e.status], ...comment });
  }
  flush();
  return out.map((item, i) => ({ ...item, last: i === out.length - 1 }));
}

/**
 * Что показать жителю в карточке по проверке ремонта: спросить, починили ли
 * (участник, заявка выполнена, окно открыто, ответа ещё нет), поблагодарить за «починили» или ничего.
 */
export function repairCheck(
  it: { status: Status; joined: boolean; my_answer: 'fixed' | null; answer_until: string | null },
  now: Date,
): 'ask' | 'thanks' | null {
  if (it.my_answer === 'fixed') return 'thanks';
  if (!it.joined || it.status !== 'done' || !it.answer_until) return null;
  return now < new Date(it.answer_until) ? 'ask' : null;
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
  reopened_at?: string;
}

/** Открытые заявки дома: просроченные сверху, остальные в порядке сервера (новые первыми). */
export function houseOpenIssues<T extends { status: Status; overdue: boolean }>(items: T[]): T[] {
  return items.filter((i) => i.status !== 'done' && i.status !== 'rejected').sort((a, b) => Number(b.overdue) - Number(a.overdue));
}

/**
 * Группы очереди УК: сначала то, что горит, закрытые в конце. Заявки, которые жители вернули
 * в работу после «выполнено», идут сразу после просроченных. Пустые группы не показываются.
 */
export function groupQueue<T extends QueueItem>(items: T[], now: Date): { title: string; late?: boolean; items: T[] }[] {
  const closed = (i: T) => i.status === 'done' || i.status === 'rejected';
  const late = items.filter((i) => !closed(i) && i.overdue);
  const back = items.filter((i) => !closed(i) && !i.overdue && i.reopened_at);
  const soon = items.filter((i) => !closed(i) && !i.overdue && !i.reopened_at && calendarDaysBetween(now, i.deadline) <= 1);
  const fresh: T[] = [];
  const working: T[] = [];
  const taken = new Set([...late, ...back, ...soon]);
  for (const i of items) {
    if (closed(i) || taken.has(i)) continue;
    (i.status === 'sent' ? fresh : working).push(i);
  }
  const groups = [
    { title: 'Просрочено', late: true, items: late },
    { title: 'Вернули жители', items: back },
    { title: 'Срок сегодня и завтра', items: soon },
    { title: 'Новые', items: fresh },
    { title: 'В работе', items: working },
    { title: 'Закрытые', items: items.filter(closed) },
  ];
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
