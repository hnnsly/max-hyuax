import { CaretRight } from '@phosphor-icons/react';
import { isClosed, type Issue } from '../api/types';
import { capitalize, dayMonth, deadlineLabel, plural } from '../lib/format';
import { Stamp, statusLabel } from './Plate';
import { Rail } from './Rail';
import s from './ui.module.css';

interface Props {
  issue: Issue;
  now: Date;
  showAddress?: boolean;
  onOpen: () => void;
}

/** Строка заявки: сколько соседей ждут, что сломалось, где и сколько осталось до срока. */
export function IssueRow({ issue, now, showAddress, onOpen }: Props) {
  const n = issue.participant_count;
  const closed = isClosed(issue.status);
  const dl = deadlineLabel(issue.deadline, now, issue.overdue);
  const place = capitalize([showAddress ? issue.address : '', issue.place].filter(Boolean).join(', '));
  const neighbours = `${n} ${plural(n, 'сосед', 'соседа', 'соседей')}`;
  // Чтец читает строку по смыслу: что, где, сколько ждут, срок. Визуально число стоит первым.
  const label = [issue.title, place, `Сообщили ${neighbours}`, closed ? `${statusLabel[issue.status]} ${dayMonth(issue.status_at)}` : dl.text]
    .filter(Boolean)
    .join('. ');
  return (
    <button type="button" className={s.row} onClick={onOpen} aria-label={label}>
      <span className={s.rowCount}>
        <span className={s.rowCountNumber}>{n}</span>
        <span className={s.rowCountNoun}>{plural(n, 'сосед', 'соседа', 'соседей')}</span>
      </span>
      <span className={s.rowBody}>
        <span className={s.rowTitle}>{issue.title}</span>
        {place && <span className={s.rowPlace}>{place}</span>}
        <span className={s.rowMeta}>
          {closed ? (
            <>
              <Stamp status={issue.status} small />
              <span className={s.rowDeadline}>{dayMonth(issue.status_at)}</span>
            </>
          ) : (
            <>
              <Rail created={issue.created_at} deadline={issue.deadline} now={now} />
              <span className={`${s.rowDeadline} ${dl.overdue ? s.rowDeadlineLate : ''}`}>{dl.text}</span>
            </>
          )}
        </span>
      </span>
      <CaretRight className={s.chevron} size={14} weight="bold" aria-hidden="true" />
    </button>
  );
}

export function IssueList({ issues, now, showAddress, onOpen }: { issues: Issue[]; now: Date; showAddress?: boolean; onOpen: (id: string) => void }) {
  return (
    <div className={s.island}>
      {issues.map((it) => (
        <IssueRow key={it.id} issue={it} now={now} showAddress={showAddress} onOpen={() => onOpen(it.id)} />
      ))}
    </div>
  );
}
