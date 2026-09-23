// Форматирование для интерфейса: склонения, даты и сроки по московскому времени.
// Сроки в заявках считаются в рабочих днях по Москве, поэтому и показываем их по Москве.

const TZ = 'Europe/Moscow';

/** Выбирает форму слова для числа: plural(4, 'сосед', 'соседа', 'соседей') → «соседа». */
export function plural(n: number, one: string, few: string, many: string): string {
  const mod10 = Math.abs(n) % 10;
  const mod100 = Math.abs(n) % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

const dayMonthFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', timeZone: TZ });
const partsFmt = new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone: TZ,
});

type DateLike = string | Date;
const toDate = (d: DateLike) => (typeof d === 'string' ? new Date(d) : d);

/** «19 сентября» */
export function dayMonth(d: DateLike): string {
  return dayMonthFmt.format(toDate(d));
}

function parts(d: DateLike) {
  const p = Object.fromEntries(partsFmt.formatToParts(toDate(d)).map((x) => [x.type, x.value]));
  return { day: p.day ?? '', month: p.month ?? '', year: p.year ?? '', hour: p.hour ?? '', minute: p.minute ?? '' };
}

/** «17.09 08:10» — компактная дата для хронологии. */
export function dotDateTime(d: DateLike): string {
  const p = parts(d);
  return `${p.day}.${p.month} ${p.hour}:${p.minute}`;
}

/** «12:38» */
export function time(d: DateLike): string {
  const p = parts(d);
  return `${p.hour}:${p.minute}`;
}

function moscowDay(d: DateLike): number {
  const p = parts(d);
  return Date.UTC(Number(p.year), Number(p.month) - 1, Number(p.day)) / 86_400_000;
}

/** Разница в календарных днях по Москве: to − from. */
export function calendarDaysBetween(from: DateLike, to: DateLike): number {
  return moscowDay(to) - moscowDay(from);
}

/** «Ореховый бульвар, 17к2» → улица и номер для таблички дома. */
export function splitAddress(address: string): { street: string; number: string } {
  const i = address.lastIndexOf(', ');
  return i < 0 ? { street: address, number: '' } : { street: address.slice(0, i), number: address.slice(i + 2) };
}

export const capitalize = (s: string) => (s ? s[0]!.toUpperCase() + s.slice(1) : s);

export interface DeadlineLabel {
  text: string;
  overdue: boolean;
}

/** Подпись срока в строке заявки: «до 19 сентября», «срок сегодня», «просрочено на 2 дня». */
export function deadlineLabel(deadline: DateLike, now: Date, overdue: boolean): DeadlineLabel {
  if (overdue) {
    const days = Math.max(1, calendarDaysBetween(deadline, now));
    return { text: `просрочено на ${days} ${plural(days, 'день', 'дня', 'дней')}`, overdue: true };
  }
  if (calendarDaysBetween(now, deadline) <= 0) return { text: 'срок сегодня', overdue: false };
  return { text: `до ${dayMonth(deadline)}`, overdue: false };
}
