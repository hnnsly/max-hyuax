// Подписи для экрана метрик УК: длительности, сравнение недель, шкала графика.
import type { DayMedian } from '../api/types';
import { plural } from './format';

/** «5:40 часов», «2 часа», «40 минут» — время до первого ответа для крупной цифры. */
export function formatResponse(min: number): { value: string; unit: string } {
  if (min < 60) return { value: String(min), unit: plural(min, 'минута', 'минуты', 'минут') };
  const h = Math.floor(min / 60);
  const m = min % 60;
  if (m === 0) return { value: String(h), unit: plural(h, 'час', 'часа', 'часов') };
  return { value: `${h}:${String(m).padStart(2, '0')}`, unit: 'часов' };
}

/** «5:40», «4 ч», «40 мин» — подпись линии медианы на графике. */
export function shortResponse(min: number): string {
  if (min < 60) return `${min} мин`;
  return min % 60 === 0 ? `${min / 60} ч` : formatResponse(min).value;
}

export type Tone = 'good' | 'bad' | 'same';

/** «на 2 часа быстрее» по сравнению с прошлой неделей; разница меньше 10 минут не считается. */
export function compareWeeks(week: number | null, prev: number | null): { text: string; tone: Tone } | null {
  if (week === null || prev === null) return null;
  const diff = prev - week;
  const abs = Math.abs(diff);
  if (abs < 10) return { text: 'как неделю назад', tone: 'same' };
  let amount: string;
  if (abs < 60) {
    amount = `${abs} ${plural(abs, 'минуту', 'минуты', 'минут')}`;
  } else {
    const h = Math.round(abs / 60);
    amount = `${h} ${plural(h, 'час', 'часа', 'часов')}`;
  }
  return diff > 0 ? { text: `на ${amount} быстрее`, tone: 'good' } : { text: `на ${amount} медленнее`, tone: 'bad' };
}

/** Верх шкалы графика в часах: чётное число, не меньше самого высокого столбца и медианы. */
export function chartTopHours(days: DayMedian[], median: number | null): number {
  const max = Math.max(0, median ?? 0, ...days.map((d) => d.median_min ?? 0));
  return Math.max(2, Math.ceil(max / 60 / 2) * 2);
}

const shortDays = ['вс', 'пн', 'вт', 'ср', 'чт', 'пт', 'сб'];
const fullDays = ['воскресенье', 'понедельник', 'вторник', 'среда', 'четверг', 'пятница', 'суббота'];

function weekday(date: string): number {
  const [y, m, d] = date.split('-').map(Number);
  return new Date(Date.UTC(y!, m! - 1, d!)).getUTCDay();
}

/** «пн» по дате ГГГГ-ММ-ДД; дата уже московская, часовой пояс не применяется. */
export const weekdayShort = (date: string) => shortDays[weekday(date)]!;

/** День недели с самым долгим первым ответом или null, если ответов не было. */
export function longestDay(days: DayMedian[]): string | null {
  let best: DayMedian | null = null;
  for (const d of days) {
    if (d.median_min !== null && (best === null || d.median_min > best.median_min!)) best = d;
  }
  return best ? fullDays[weekday(best.date)]! : null;
}

/** Процент закрытых в срок; null, если за период ничего не закрыто. */
export function onTimeShare(m: { closed_total: number; closed_on_time: number }): number | null {
  return m.closed_total === 0 ? null : Math.round((m.closed_on_time / m.closed_total) * 100);
}

/** Подписи к числам проверки ремонта жителями; null, если за период жители ничего не проверяли. */
export function residentCheck(m: { confirmed_by_residents: number; reopened_by_residents: number }): { confirmed: string; reopened: string } | null {
  const { confirmed_by_residents: c, reopened_by_residents: r } = m;
  if (c === 0 && r === 0) return null;
  return {
    confirmed: `${plural(c, 'ремонт', 'ремонта', 'ремонтов')} подтвердили жители`,
    reopened: `${plural(r, 'заявку', 'заявки', 'заявок')} вернули в работу`,
  };
}
