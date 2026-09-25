// Совет дома: итоги опросов и подписи предложений (ADR-017).
import type { Poll, ProposalStatus } from '../api/types';

export const MAX_QUESTION = 300;
export const DEFAULT_OPTIONS = ['За', 'Против', 'Воздержусь'];

export interface OptionResult {
  text: string;
  votes: number;
  share: number; // целые проценты, в сумме 100
  mine: boolean;
  leader: boolean; // единственный вариант с наибольшим числом голосов
}

/**
 * Итоги опроса. Проценты округляются методом наибольшего остатка, чтобы сумма была ровно 100:
 * иначе 57 + 28 + 14 = 99 смущает жителей.
 */
export function pollResults(poll: Poll): OptionResult[] {
  const exact = poll.options.map((o) => (poll.total > 0 ? (o.votes * 100) / poll.total : 0));
  const share = exact.map(Math.floor);
  let rest = poll.total > 0 ? 100 - share.reduce((a, b) => a + b, 0) : 0;
  const order = exact.map((v, i) => ({ i, frac: v - Math.floor(v) })).sort((a, b) => b.frac - a.frac);
  for (const { i } of order) {
    if (rest <= 0) break;
    share[i] = (share[i] ?? 0) + 1;
    rest--;
  }
  const top = Math.max(...poll.options.map((o) => o.votes));
  const leaders = poll.options.filter((o) => o.votes === top).length;
  return poll.options.map((o, i) => ({
    text: o.text,
    votes: o.votes,
    share: share[i] ?? 0,
    mine: poll.my_vote === i,
    leader: top > 0 && leaders === 1 && o.votes === top,
  }));
}

/** Голосовать можно в открытом опросе и один раз. */
export function canVote(poll: Poll): boolean {
  return poll.open && poll.my_vote === null;
}

/** Вопрос опроса из текста предложения: без точки или восклицания в конце, со знаком вопроса. */
export function pollQuestion(text: string): string {
  const base = text.trim().replace(/[.!?…]+$/u, '');
  return base.slice(0, MAX_QUESTION - 1) + '?';
}

const statusLabels: Record<ProposalStatus, string> = {
  new: 'Ждёт ответа председателя',
  accepted: 'Председатель взял в работу',
  declined: 'Председатель отклонил',
};

export function proposalStatus(s: ProposalStatus): string {
  return statusLabels[s];
}
