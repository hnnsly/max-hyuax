import { dayMonth } from '../lib/format';
import { buildRail } from '../lib/model';
import s from './ui.module.css';

interface Props {
  created: string;
  deadline: string;
  now: Date;
  full?: boolean;
}

/** Шкала срока: это данные, а не украшение. Отрезки — дни от подачи до срока. */
export function Rail({ created, deadline, now, full }: Props) {
  const items = buildRail(created, deadline, now, full ? 7 : 5);
  const late = items.some((i) => i.kind === 'today' && i.late);
  const rail = (
    <div className={`${s.rail} ${full ? s.railFull : ''}`} aria-hidden="true">
      {items.map((it, i) =>
        it.kind === 'seg' ? (
          <span key={i} className={`${s.seg} ${it.tone !== 'left' ? s[`seg_${it.tone}`] : ''}`} />
        ) : it.kind === 'today' ? (
          <span key={i} className={`${s.tick} ${it.late ? s.tickLate : ''}`} />
        ) : full ? (
          <span key={i} className={s.tickDeadline} />
        ) : null,
      )}
    </div>
  );
  if (!full) return rail;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      {rail}
      <div className={s.railLabels}>
        <span>{dayMonth(created)}, подали</span>
        {late ? (
          <>
            <span className={s.railDeadline}>{dayMonth(deadline)}, срок истёк</span>
            <span className={s.railToday}>сегодня</span>
          </>
        ) : (
          <>
            <span className={s.railToday}>сегодня</span>
            <span className={s.railDeadline}>{dayMonth(deadline)}, срок</span>
          </>
        )}
      </div>
    </div>
  );
}
