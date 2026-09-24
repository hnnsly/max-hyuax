import { api } from '../shared/api/client';
import type { UkMetrics } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { plural } from '../shared/lib/format';
import { chartTopHours, compareWeeks, formatResponse, longestDay, onTimeShare, shortResponse, weekdayShort } from '../shared/lib/metrics';
import { EmptyState, ErrorState, Island, Loading } from '../shared/ui/Layout';
import s from './metrics.module.css';

/** Метрики УК по холсту UkMetrics: первый ответ, сколько сообщений на проблему, закрыто в срок. */
export function UkMetricsView() {
  const res = useResource(() => api.ukMetrics(), []);
  if (res.loading && !res.data) return <Loading />;
  if (res.error || !res.data) {
    return <ErrorState title="Не удалось загрузить метрики" message={res.error?.message ?? ''} onRetry={res.reload} />;
  }
  const m = res.data;
  if (m.issues_total === 0 && m.open_total === 0 && m.closed_total === 0) {
    return <EmptyState title="Пока нечего считать" text="Метрики появятся, когда жители сообщат о проблемах и УК начнёт отвечать." />;
  }
  const period = `${m.period_days} ${plural(m.period_days, 'день', 'дня', 'дней')}`;
  return (
    <>
      <FirstResponse m={m} />
      {m.issues_total > 0 && <ReportsPerIssue perIssue={m.reports_per_issue} />}
      <OnTime m={m} period={period} />
      <p className={s.note}>
        {m.sample_data && 'Пример данных. '}
        Считается по событиям заявок за {period}: создание, присоединение, смена статуса.
      </p>
    </>
  );
}

function FirstResponse({ m }: { m: UkMetrics }) {
  const week = m.first_response_median_min;
  const days = m.first_response_by_day;
  const top = chartTopHours(days, week);
  const cmp = compareWeeks(week, m.prev_week_median_min);
  const longest = longestDay(days);
  const pct = (min: number) => `${Math.min(100, (min / 60 / top) * 100)}%`;
  const figure = week === null ? null : formatResponse(week);
  const chartLabel = days
    .map((d) => `${weekdayShort(d.date)}: ${d.median_min === null ? 'нет ответов' : `${formatResponse(d.median_min).value} ${formatResponse(d.median_min).unit}`}`)
    .join(', ');
  return (
    <Island>
      <div className={s.card}>
        <div className={s.head}>
          <div className={s.headMain}>
            <span className={s.label}>Первый ответ жителю</span>
            {figure ? (
              <span className={s.figure}>
                <span className={s.bigNum}>{figure.value}</span>
                <span className={s.bigUnit}>{figure.unit}</span>
              </span>
            ) : (
              <span className={s.empty}>За неделю УК ещё не отвечала на новые заявки</span>
            )}
          </div>
          {cmp && (
            <div className={s.compare}>
              <span className={`${s.compareText} ${s[cmp.tone]}`}>{cmp.text}</span>
              {cmp.tone !== 'same' && <span className={s.hint}>чем неделю назад</span>}
            </div>
          )}
        </div>

        <div className={s.chart} role="img" aria-label={`Медиана первого ответа по дням подачи. ${chartLabel}`}>
          <div className={s.axis} aria-hidden="true">
            <span>{top} ч</span>
            <span>{top / 2} ч</span>
            <span>0</span>
          </div>
          <div className={s.plot} aria-hidden="true">
            <div className={s.bars}>
              <div className={`${s.grid} ${s.gridTop}`} />
              <div className={`${s.grid} ${s.gridMid}`} />
              {days.map((d, i) => (
                <div
                  key={d.date}
                  className={`${s.bar} ${d.median_min === null ? s.barNone : ''} ${i === days.length - 1 ? s.barToday : ''}`}
                  style={d.median_min === null ? undefined : { height: pct(d.median_min) }}
                />
              ))}
              {week !== null && (
                <div className={s.median} style={{ bottom: pct(week) }}>
                  <span>медиана {shortResponse(week)}</span>
                </div>
              )}
            </div>
            <div className={s.days}>
              {days.map((d, i) => (
                <span key={d.date} className={i === days.length - 1 ? s.today : ''}>
                  {weekdayShort(d.date)}
                </span>
              ))}
            </div>
          </div>
        </div>
        <p className={s.hint}>
          Часы до первого ответа по дням подачи заявки.{longest && ` Самый долгий день ${longest}.`}
        </p>
      </div>
    </Island>
  );
}

function ReportsPerIssue({ perIssue }: { perIssue: number }) {
  const n = Math.max(1, Math.round(perIssue));
  return (
    <Island>
      <div className={s.card}>
        <div className={s.line}>
          <span className={s.midNum}>{n}</span>
          <span className={s.text}>
            {plural(n, 'сообщение', 'сообщения', 'сообщений')} соседей приходится на одну проблему
          </span>
        </div>
        <div className={s.compareRows}>
          <span className={s.rowLabel}>Было заявок</span>
          <span className={s.squares}>
            {Array.from({ length: Math.min(n, 12) }, (_, i) => (
              <span key={i} className={`${s.square} ${s.squareLate}`} />
            ))}
          </span>
          <span className={s.rowLabel}>Стало</span>
          <span className={`${s.squares} ${s.single}`}>
            <span className={s.square} />
            <span className={s.rowText}>
              одна заявка с {n} {plural(n, 'подтверждением', 'подтверждениями', 'подтверждениями')}
            </span>
          </span>
        </div>
      </div>
    </Island>
  );
}

function OnTime({ m, period }: { m: UkMetrics; period: string }) {
  const share = onTimeShare(m);
  if (share === null) {
    return (
      <Island>
        <div className={s.card}>
          <span className={s.label}>Закрыто в срок</span>
          <span className={s.empty}>За {period} закрытых заявок нет</span>
        </div>
      </Island>
    );
  }
  const late = m.closed_total - m.closed_on_time;
  // Больше 90 квадратов не помещается: сетка показывает долю, а не штуки.
  const cells = Math.min(m.closed_total, 90);
  const onCells = Math.round((m.closed_on_time / m.closed_total) * cells);
  return (
    <Island>
      <div className={s.card}>
        <div className={s.line}>
          <span className={`${s.midNum} ${s.ink}`}>{share}%</span>
          <span className={s.text}>заявок закрыто в срок за {period}</span>
        </div>
        <div className={s.cells} aria-hidden="true">
          {Array.from({ length: cells }, (_, i) => (
            <span key={i} className={`${s.cell} ${i >= onCells ? s.cellLate : ''}`} />
          ))}
        </div>
        <div className={s.legend}>
          <span>
            <i className={s.dot} />
            {m.closed_on_time} в срок
          </span>
          {late > 0 && (
            <span>
              <i className={`${s.dot} ${s.dotLate}`} />
              {late} {plural(late, 'просрочена', 'просрочены', 'просрочено')}
            </span>
          )}
        </div>
      </div>
    </Island>
  );
}
