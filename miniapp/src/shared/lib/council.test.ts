import { describe, expect, it } from 'vitest';
import type { Poll } from '../api/types';
import { canVote, pollQuestion, pollResults, proposalStatus } from './council';

const poll = (over: Partial<Poll> = {}): Poll => ({
  id: 'q1',
  question: 'Ставим шлагбаум?',
  options: [
    { text: 'За', votes: 4 },
    { text: 'Против', votes: 2 },
    { text: 'Воздержусь', votes: 1 },
  ],
  total: 7,
  my_vote: null,
  open: true,
  created_at: '2026-09-22T10:00:00Z',
  closes_at: '2026-09-29T10:00:00Z',
  ...over,
});

describe('pollResults', () => {
  it('считает доли в процентах, сумма 100, отмечает голос и лидера', () => {
    const r = pollResults(poll({ my_vote: 1 }));
    expect(r.map((o) => o.share)).toEqual([57, 29, 14]);
    expect(r.map((o) => o.mine)).toEqual([false, true, false]);
    expect(r.map((o) => o.leader)).toEqual([true, false, false]);
  });
  it('без голосов доли нулевые и лидера нет', () => {
    const r = pollResults(poll({ options: [{ text: 'За', votes: 0 }, { text: 'Против', votes: 0 }], total: 0 }));
    expect(r.map((o) => o.share)).toEqual([0, 0]);
    expect(r.some((o) => o.leader)).toBe(false);
  });
  it('при ничьей лидеров нет', () => {
    const r = pollResults(poll({ options: [{ text: 'За', votes: 2 }, { text: 'Против', votes: 2 }], total: 4 }));
    expect(r.some((o) => o.leader)).toBe(false);
  });
});

describe('canVote', () => {
  it('голосовать можно в открытом опросе и только один раз', () => {
    expect(canVote(poll())).toBe(true);
    expect(canVote(poll({ my_vote: 0 }))).toBe(false);
    expect(canVote(poll({ open: false }))).toBe(false);
  });
});

describe('pollQuestion', () => {
  it('делает вопрос из текста предложения', () => {
    expect(pollQuestion('Поставить шлагбаум на въезде во двор.')).toBe('Поставить шлагбаум на въезде во двор?');
    expect(pollQuestion('  Покрасить лавочки!! ')).toBe('Покрасить лавочки?');
    expect(pollQuestion('Нужна ли велопарковка?')).toBe('Нужна ли велопарковка?');
    expect(pollQuestion('а'.repeat(400))).toHaveLength(300);
  });
});

describe('proposalStatus', () => {
  it('подписи статусов для жителя и председателя', () => {
    expect(proposalStatus('new')).toBe('Ждёт ответа председателя');
    expect(proposalStatus('accepted')).toBe('Председатель взял в работу');
    expect(proposalStatus('declined')).toBe('Председатель отклонил');
  });
});
