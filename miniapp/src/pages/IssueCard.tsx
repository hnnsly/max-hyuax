import { Button } from '@maxhub/max-ui';
import { CheckCircle, ShareNetwork } from '@phosphor-icons/react';
import { useEffect, useState } from 'react';
import { useRouter } from '../app/router';
import { useUser } from '../app/session';
import { api, ApiError } from '../shared/api/client';
import { isClosed, type Issue, type Status } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { appLink, bridge } from '../shared/bridge/bridge';
import { calendarDaysBetween, capitalize, dayMonth, dotDateTime, plural } from '../shared/lib/format';
import { buildTimeline, nextStatuses, shareText } from '../shared/lib/model';
import { ErrorState, Island, Loading, Screen, useToast } from '../shared/ui/Layout';
import { IssuePlate, Stamp, statusLabel } from '../shared/ui/Plate';
import { Sheet } from '../shared/ui/Sheet';
import { Rail } from '../shared/ui/Rail';
import s from './pages.module.css';

export function IssueCard({ id, flash }: { id: string; flash?: string }) {
  const user = useUser();
  const { push, back, canGoBack } = useRouter();
  const [toast, showToast] = useToast();
  const [busy, setBusy] = useState(false);
  const [sheet, setSheet] = useState(false);
  const [landed, setLanded] = useState(false);
  const res = useResource(async () => {
    const [issue, events] = await Promise.all([api.issue(id), api.timeline(id)]);
    return { issue, events };
  }, [id]);

  // Сообщение с предыдущего экрана (заявка отправлена, присоединились).
  useEffect(() => {
    if (flash) showToast(flash);
  }, [flash, showToast]);

  const onBack = canGoBack ? back : undefined;
  if (res.loading && !res.data) {
    return (
      <Screen title="Заявка" onBack={onBack}>
        <Loading />
      </Screen>
    );
  }
  if (res.error || !res.data) {
    const notFound = res.error?.code === 'not_found';
    return (
      <Screen title="Заявка" onBack={onBack}>
        <ErrorState
          title={notFound ? 'Заявка не найдена' : 'Не удалось загрузить заявку'}
          message={notFound ? 'Возможно, ссылка устарела.' : (res.error?.message ?? '')}
          onRetry={res.reload}
        />
      </Screen>
    );
  }

  const { issue, events } = res.data;
  const now = new Date();
  const closed = isClosed(issue.status);
  const n = issue.participant_count;
  const daysLeft = calendarDaysBetween(now, issue.deadline);
  const lateDays = Math.max(1, -daysLeft);
  const timeline = buildTimeline(events);
  const where = [issue.address, issue.place].filter(Boolean).join(', ');

  const join = async () => {
    setBusy(true);
    try {
      await api.join(issue.id);
      bridge.hapticSuccess();
      showToast('Вы присоединились. Карточка со статусом придёт в чат с ботом.');
      res.reload();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'consent_required') push({ name: 'consent' });
      else showToast(err instanceof ApiError ? err.message : 'Не получилось. Попробуйте ещё раз');
    } finally {
      setBusy(false);
    }
  };

  const share = () => {
    bridge.share(shareText(issue), appLink(`i_${issue.id}`)).catch(() => showToast('Не получилось поделиться'));
  };

  const canJoin = !closed && !issue.joined && user.role === 'resident';
  const canManage = !closed && user.role === 'uk_operator' && user.organization_id === issue.responsible?.id;
  const saved = (changed: Issue) => {
    setSheet(false);
    setLanded(true);
    bridge.hapticSuccess();
    const people = changed.participant_count;
    showToast(`Статус сохранён. ${people} ${plural(people, 'житель увидит', 'жителя увидят', 'жителей увидят')} его в карточке.`);
    res.reload();
  };

  const actions = canManage ? (
    <>
      <Button variant="primary" size="large" stretched onClick={() => setSheet(true)}>
        Сменить статус
      </Button>
      <Button variant="secondary" size="medium" stretched iconBefore={<ShareNetwork size={18} />} onClick={share}>
        Отправить в чат дома
      </Button>
    </>
  ) : (
    <>
      {canJoin ? (
        <Button variant="primary" size="large" stretched loading={busy} onClick={join}>
          Это и у меня
        </Button>
      ) : (
        <Button variant="primary" size="large" stretched iconBefore={<ShareNetwork size={20} />} onClick={share}>
          Отправить в чат дома
        </Button>
      )}
      {issue.joined && !canJoin && (
        <div className={s.joinedNote}>
          <CheckCircle size={20} weight="fill" aria-hidden="true" /> Вы среди сообщивших
        </div>
      )}
      {canJoin && (
        <Button variant="secondary" size="medium" stretched iconBefore={<ShareNetwork size={18} />} onClick={share}>
          Отправить в чат дома
        </Button>
      )}
    </>
  );

  return (
    <Screen title="Заявка" onBack={onBack} actions={actions}>
      <div className={s.cardTop}>
        <IssuePlate number={issue.number} />
        <span className={s.cardStamp}>
          <Stamp key={issue.status} status={issue.status} land={landed} />
        </span>
      </div>
      <div>
        <h2 className={s.cardTitle}>{issue.title}</h2>
        {where && <p className={s.cardPlace}>{capitalize(where)}</p>}
        {issue.description && <p className={s.cardDescription}>{issue.description}</p>}
      </div>

      <Island>
        <div className={s.figures}>
          <div className={s.figureRow}>
            <div className={s.figure}>
              <span className={`${s.figureValue} ${s.figureAccent}`}>{n}</span>
              <span className={s.figureLabel}>
                {plural(n, 'сосед сообщил', 'соседа сообщили', 'соседей сообщили')}
              </span>
            </div>
            {!closed && <span className={s.figureDivider} />}
            {!closed &&
              (issue.overdue ? (
                <div className={s.figure}>
                  <span className={`${s.figureValue} ${s.figureLate}`}>{lateDays}</span>
                  <span className={s.figureLabel}>{plural(lateDays, 'день', 'дня', 'дней')} просрочки</span>
                </div>
              ) : (
                <div className={s.figure}>
                  <span className={s.figureValue}>{Math.max(0, daysLeft)}</span>
                  <span className={s.figureLabel}>
                    {daysLeft <= 0 ? 'срок ответа сегодня' : `${plural(daysLeft, 'день', 'дня', 'дней')} до срока ответа`}
                  </span>
                </div>
              ))}
          </div>
          {!closed && <Rail created={issue.created_at} deadline={issue.deadline} now={now} full />}
        </div>
      </Island>

      {issue.overdue && (
        <Island>
          <div className={s.escalate}>
            <h3 className={s.escalateTitle}>УК не уложилась в срок</h3>
            <p className={s.text}>Если ответа не будет, соседи могут обратиться в Мосжилинспекцию. Даты, комментарии УК и число сообщивших уже собраны в заявке.</p>
          </div>
        </Island>
      )}

      <Island>
        <div className={s.block}>
          <dl className={s.keyValue}>
            <dt>Отвечает</dt>
            <dd>{issue.responsible?.name ?? 'Управляющая компания'}</dd>
            <dt>Срок ответа</dt>
            <dd>до {dayMonth(issue.deadline)}</dd>
          </dl>
          {issue.basis && <p className={s.basis}>{issue.basis}</p>}
        </div>
      </Island>

      {timeline.length > 0 && (
        <Island>
          <div className={s.block}>
            <h3 className={s.blockTitle}>Что происходило</h3>
            <ol className={s.timeline}>
              {timeline.map((t) => (
                <li key={t.at + t.text} className={s.tlItem}>
                  <span className={s.tlWhen}>{dotDateTime(t.at).replace(' ', '\n')}</span>
                  <span className={`${s.tlDot} ${t.last ? s.tlDotLast : ''} ${t.late ? s.tlDotLate : ''}`} aria-hidden="true" />
                  <span className={s.tlText}>
                    {t.text}
                    {t.comment && <span className={s.quote}>«{t.comment}»</span>}
                  </span>
                </li>
              ))}
            </ol>
          </div>
        </Island>
      )}
      {canManage && <StatusSheet open={sheet} issue={issue} onClose={() => setSheet(false)} onSaved={saved} />}
      {toast}
    </Screen>
  );
}

/** Смена статуса оператором УК (холст UkStatus): отказ требует причину для жителей. */
function StatusSheet({ open, issue, onClose, onSaved }: { open: boolean; issue: Issue; onClose: () => void; onSaved: (i: Issue) => void }) {
  const options = nextStatuses(issue.status);
  const [to, setTo] = useState<Status>(options[0] ?? 'accepted');
  const [comment, setComment] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const n = issue.participant_count;
  const rejecting = to === 'rejected';
  const valid = !rejecting || comment.trim() !== '';

  const save = async () => {
    setBusy(true);
    setError('');
    try {
      onSaved(await api.changeStatus(issue.id, to, comment.trim()));
      setComment('');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сохранить статус');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet open={open} title="Сменить статус" onClose={onClose}>
      <div className={s.sheetBody}>
        <fieldset className={s.radios}>
          <legend className="visually-hidden">Новый статус</legend>
          {options.map((st) => (
            <label key={st} className={s.radio}>
              <input type="radio" name="status" value={st} checked={to === st} onChange={() => setTo(st)} />
              <span>{statusLabel[st]}</span>
              {to === st && <Stamp status={st} small />}
            </label>
          ))}
        </fieldset>
        <label className={s.blockTitle} htmlFor="status-comment">
          {rejecting ? 'Причина отказа' : 'Комментарий для жителей'}
        </label>
        <textarea
          id="status-comment"
          className={s.sheetText}
          value={comment}
          maxLength={1000}
          required={rejecting}
          onChange={(e) => setComment(e.target.value)}
        />
        <p className={s.hint}>
          {rejecting
            ? 'Обязательно. Жители увидят причину в карточке.'
            : `Его ${plural(n, 'увидит', 'увидят', 'увидят')} ${n} ${plural(n, 'человек, который сообщил', 'человека, которые сообщили', 'человек, которые сообщили')} о проблеме.`}
        </p>
        {error && (
          <p className={s.hint} role="alert">
            {error}
          </p>
        )}
        <Button variant="primary" size="large" stretched loading={busy} disabled={!valid} onClick={save}>
          Сохранить статус
        </Button>
      </div>
    </Sheet>
  );
}
