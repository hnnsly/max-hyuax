import { Button, CellList, CellSimple, Counter, Input, Radio } from '@maxhub/max-ui';
import { WarningCircle } from '@phosphor-icons/react';
import { useId, useState, type FormEvent } from 'react';
import { useRouter } from '../app/router';
import { useUser } from '../app/session';
import { api, ApiError } from '../shared/api/client';
import type { Poll, Proposal } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { canVote, DEFAULT_OPTIONS, pollQuestion, pollResults, proposalStatus } from '../shared/lib/council';
import { dayMonth, plural } from '../shared/lib/format';
import { EmptyState, ErrorState, Island, Loading, Screen, Section, useToast } from '../shared/ui/Layout';
import { Sheet } from '../shared/ui/Sheet';
import c from './council.module.css';
import s from './pages.module.css';

const errText = (err: unknown, fallback: string) => (err instanceof ApiError ? err.message : fallback);

/**
 * Блок «Совет дома» на главном экране жителя (ADR-017): опросы дома, предложение председателю,
 * свои предложения с ответами. Председателю ещё и вход в папку предложений.
 */
export function HouseCouncil({ onToast }: { onToast: (msg: string) => void }) {
  const user = useUser();
  const { push } = useRouter();
  const [proposing, setProposing] = useState(false);
  const chairman = Boolean(user.chairman);
  const res = useResource(async () => {
    const [polls, mine, folder] = await Promise.all([
      api.polls(),
      api.myProposals(),
      chairman ? api.councilFolder() : Promise.resolve<Proposal[]>([]),
    ]);
    return { polls, mine, folder };
  }, [chairman]);

  const proposed = () => {
    setProposing(false);
    res.reload();
    onToast('Предложение отправлено председателю совета');
  };

  let body;
  if (res.loading && !res.data) body = <Loading />;
  else if (res.error || !res.data) body = <ErrorState title="Не удалось загрузить совет дома" message={res.error?.message ?? ''} onRetry={res.reload} />;
  else {
    const { polls, mine, folder } = res.data;
    const fresh = folder.filter((p) => p.status === 'new').length;
    body = (
      <>
        {polls.map((p) => (
          <PollCard key={p.id} poll={p} onVoted={res.reload} onToast={onToast} />
        ))}
        <Island>
          <div className={s.block}>
            <p className={s.text}>Есть идея для дома? Напишите председателю совета. Он увидит текст без вашего имени.</p>
            <Button variant="secondary" size="medium" stretched onClick={() => setProposing(true)}>
              Предложить совету
            </Button>
          </div>
        </Island>
        {mine.length > 0 && (
          <Island>
            <h3 className={s.blockTitle} style={{ padding: '14px 16px 0' }}>
              Мои предложения
            </h3>
            {mine.map((p) => (
              <ProposalItem key={p.id} proposal={p} />
            ))}
          </Island>
        )}
        {chairman && (
          <CellList mode="island">
            <CellSimple
              title="Папка предложений"
              showChevron
              after={fresh > 0 && <Counter value={fresh} rounded />}
              onClick={() => push({ name: 'council' })}
            />
          </CellList>
        )}
      </>
    );
  }

  return (
    <>
      <Section title="Совет дома" />
      {body}
      <ProposeSheet open={proposing} onClose={() => setProposing(false)} onDone={proposed} />
    </>
  );
}

/** Опрос: до голоса — выбор варианта, после голоса или окончания — итоги. Без юридической силы. */
function PollCard({ poll, onVoted, onToast }: { poll: Poll; onVoted: () => void; onToast: (msg: string) => void }) {
  const [choice, setChoice] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const qid = useId();

  const vote = async (e: FormEvent) => {
    e.preventDefault();
    if (choice === null) return;
    setBusy(true);
    try {
      await api.vote(poll.id, choice);
      onToast('Голос учтён');
      onVoted();
    } catch (err) {
      onToast(errText(err, 'Не получилось проголосовать'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Island>
      <div className={c.poll}>
        <h3 className={c.question} id={qid}>
          {poll.question}
        </h3>
        <p className={c.meta}>
          {poll.open ? `Опрос до ${dayMonth(poll.closes_at)}.` : 'Опрос закончился.'} Без юридической силы: это не общее собрание собственников.
        </p>
        {canVote(poll) ? (
          <form className={c.field} onSubmit={vote}>
            <fieldset className={c.options} aria-labelledby={qid}>
              {poll.options.map((o, i) => (
                <label key={o.text} className={c.option}>
                  <Radio name={`poll-${poll.id}`} value={i} checked={choice === i} onChange={() => setChoice(i)} />
                  {o.text}
                </label>
              ))}
            </fieldset>
            <Button type="submit" variant="primary" size="medium" stretched loading={busy} disabled={choice === null}>
              Проголосовать
            </Button>
          </form>
        ) : (
          <>
            <ul className={c.results} aria-labelledby={qid}>
              {pollResults(poll).map((r) => (
                <li key={r.text} className={`${c.result} ${r.leader ? c.leader : ''}`}>
                  <span className={r.mine ? c.mine : undefined}>
                    {r.text}
                    {r.mine && ', ваш голос'}
                  </span>
                  <span className={c.resultShare}>{r.share}%</span>
                  <span className={c.track} aria-hidden="true">
                    <span className={c.fill} style={{ width: `${r.share}%` }} />
                  </span>
                </li>
              ))}
            </ul>
            <p className={c.meta}>
              {poll.total} {plural(poll.total, 'голос', 'голоса', 'голосов')}
            </p>
          </>
        )}
      </div>
    </Island>
  );
}

function ProposalItem({ proposal, children }: { proposal: Proposal; children?: React.ReactNode }) {
  return (
    <div className={c.proposal}>
      <p className={c.proposalText}>{proposal.text}</p>
      <p className={`${c.status} ${proposal.status === 'new' ? c.statusNew : ''}`}>
        {proposalStatus(proposal.status)}, {dayMonth(proposal.answered_at ?? proposal.created_at)}
      </p>
      {proposal.answer && <p className={c.answer}>{proposal.answer}</p>}
      {children}
    </div>
  );
}

/** Общий лист с текстовым полем: предложение жителя и ответ председателя. */
function TextSheet(props: {
  open: boolean;
  title: string;
  label: string;
  hint: string;
  submit: string;
  required: boolean;
  minLength?: number;
  emptyError: string;
  onClose: () => void;
  onSubmit: (text: string) => Promise<void>;
}) {
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const fid = useId();

  const send = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      await props.onSubmit(text.trim());
      setText('');
    } catch (err) {
      setError(errText(err, 'Не получилось отправить'));
    } finally {
      setBusy(false);
    }
  };
  const close = () => {
    setError('');
    props.onClose();
  };

  return (
    <Sheet open={props.open} title={props.title} onClose={close} locked={busy}>
      <form className={s.sheetBody} onSubmit={send}>
        <label className={s.blockTitle} htmlFor={fid}>
          {props.label}
        </label>
        <textarea
          id={fid}
          className={s.sheetText}
          value={text}
          required={props.required}
          minLength={props.minLength}
          maxLength={1000}
          aria-describedby={`${fid}-hint`}
          aria-errormessage={`${fid}-error`}
          onChange={(e) => {
            if (e.currentTarget.validity.valid) e.currentTarget.removeAttribute('aria-invalid');
            setText(e.target.value);
          }}
          onInvalid={(e) => {
            // Своя подпись под полем уже есть: подсказку браузера гасим и сами ставим фокус.
            e.preventDefault();
            e.currentTarget.setAttribute('aria-invalid', 'true');
            e.currentTarget.focus();
          }}
        />
        <p id={`${fid}-error`} className={s.fieldError}>
          <WarningCircle size={16} weight="bold" aria-hidden="true" /> {props.emptyError}
        </p>
        <p id={`${fid}-hint`} className={s.hint}>
          {props.hint}
        </p>
        {error && (
          <p className={s.hint} role="alert">
            {error}
          </p>
        )}
        <Button type="submit" variant="primary" size="large" stretched loading={busy}>
          {props.submit}
        </Button>
      </form>
    </Sheet>
  );
}

function ProposeSheet({ open, onClose, onDone }: { open: boolean; onClose: () => void; onDone: () => void }) {
  return (
    <TextSheet
      open={open}
      title="Предложение совету"
      label="Что предлагаете"
      hint="Председатель увидит текст без вашего имени и ответит здесь же, в «Моих предложениях»."
      submit="Отправить председателю"
      required
      minLength={10}
      emptyError="Напишите предложение, хотя бы 10 символов"
      onClose={onClose}
      onSubmit={async (text) => {
        await api.propose(text);
        onDone();
      }}
    />
  );
}

type Reply = { proposal: Proposal; status: 'accepted' | 'declined' };

/** Папка председателя: предложения соседей без имён, ответ и вынос на опрос. */
export function CouncilFolder() {
  const { back } = useRouter();
  const res = useResource(async () => {
    const [proposals, polls] = await Promise.all([api.councilFolder(), api.polls()]);
    // По предложению опрос открывается один раз: повторный запутал бы жителей.
    return { proposals, polled: new Set(polls.map((p) => p.proposal_id)) };
  }, []);
  const [toast, showToast] = useToast();
  const [reply, setReply] = useState<Reply | null>(null);
  const [polling, setPolling] = useState<Proposal | null>(null);

  const done = (msg: string) => {
    setReply(null);
    setPolling(null);
    res.reload();
    showToast(msg);
  };

  return (
    <Screen title="Папка предложений" onBack={back}>
      <p className={s.text}>Предложения соседей по дому. Имён авторов здесь нет: отвечайте по существу, ответ увидит автор.</p>
      {res.loading && !res.data ? (
        <Loading />
      ) : res.error || !res.data ? (
        <ErrorState title="Не удалось загрузить предложения" message={res.error?.message ?? ''} onRetry={res.reload} />
      ) : res.data.proposals.length === 0 ? (
        <EmptyState title="Предложений пока нет" text="Когда соседи напишут совету, предложения появятся здесь." />
      ) : (
        <Island>
          {res.data.proposals.map((p) => (
            <ProposalItem key={p.id} proposal={p}>
              <div className={c.actions}>
                {p.status === 'new' && (
                  <>
                    <Button variant="primary" size="small" onClick={() => setReply({ proposal: p, status: 'accepted' })}>
                      Взять в работу
                    </Button>
                    <Button variant="secondary" size="small" onClick={() => setReply({ proposal: p, status: 'declined' })}>
                      Отклонить
                    </Button>
                  </>
                )}
                {p.status !== 'declined' && !res.data?.polled.has(p.id) && (
                  <Button variant="ghost" size="small" onClick={() => setPolling(p)}>
                    Вынести на опрос
                  </Button>
                )}
              </div>
            </ProposalItem>
          ))}
        </Island>
      )}
      <TextSheet
        open={reply !== null}
        title={reply?.status === 'declined' ? 'Отклонить предложение' : 'Взять в работу'}
        label="Ответ автору"
        hint={reply?.status === 'declined' ? 'Объясните, почему не получится: автор увидит ответ.' : 'Можно не писать. Например, когда и что сделаете.'}
        submit={reply?.status === 'declined' ? 'Отклонить' : 'Взять в работу'}
        required={reply?.status === 'declined'}
        emptyError="Напишите, почему отклоняете"
        onClose={() => setReply(null)}
        onSubmit={async (answer) => {
          if (!reply) return;
          await api.replyProposal(reply.proposal.id, reply.status, answer);
          done(reply.status === 'declined' ? 'Предложение отклонено, автор увидит ответ' : 'Предложение в работе, автор увидит ответ');
        }}
      />
      {polling && <PollSheet proposal={polling} onClose={() => setPolling(null)} onDone={() => done('Опрос открыт для жителей дома на 7 дней')} />}
      {toast}
    </Screen>
  );
}

/** Опрос из предложения: вопрос и варианты можно поправить, опрос идёт неделю. */
function PollSheet({ proposal, onClose, onDone }: { proposal: Proposal; onClose: () => void; onDone: () => void }) {
  const [question, setQuestion] = useState(() => pollQuestion(proposal.text));
  const [options, setOptions] = useState(DEFAULT_OPTIONS);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const create = async (e: FormEvent) => {
    e.preventDefault();
    const filled = options.map((o) => o.trim()).filter(Boolean);
    if (filled.length < 2) {
      setError('Нужно хотя бы два варианта ответа');
      return;
    }
    setBusy(true);
    setError('');
    try {
      await api.createPoll({ proposal_id: proposal.id, question: question.trim(), options: filled });
      onDone();
    } catch (err) {
      setError(errText(err, 'Не получилось открыть опрос'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet open title="Опрос жителей" onClose={onClose} locked={busy}>
      <form className={s.sheetBody} onSubmit={create}>
        <div className={c.field}>
          <label className={s.blockTitle} htmlFor="poll-question">
            Вопрос
          </label>
          <Input id="poll-question" value={question} maxLength={300} required onChange={(e) => setQuestion(e.target.value)} />
        </div>
        <fieldset className={c.options}>
          <legend className={s.blockTitle}>Варианты ответа</legend>
          {options.map((o, i) => (
            <Input
              key={i}
              aria-label={`Вариант ${i + 1}`}
              value={o}
              maxLength={100}
              onChange={(e) => setOptions(options.map((v, j) => (j === i ? e.target.value : v)))}
            />
          ))}
        </fieldset>
        <p className={s.hint}>Опрос идёт 7 дней, каждый житель дома голосует один раз. Результат не имеет юридической силы.</p>
        {error && (
          <p className={s.hint} role="alert">
            {error}
          </p>
        )}
        <Button type="submit" variant="primary" size="large" stretched loading={busy}>
          Открыть опрос
        </Button>
      </form>
    </Sheet>
  );
}
