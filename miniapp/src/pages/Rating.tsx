import { useRouter } from '../app/router';
import { api } from '../shared/api/client';
import type { RatingOrg } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { plural } from '../shared/lib/format';
import { EmptyState, ErrorState, Loading, Screen, Section } from '../shared/ui/Layout';
import { StarsValue } from '../shared/ui/Stars';
import s from './pages.module.css';
import r from './rating.module.css';

const ratingText = (v: number) => v.toFixed(1).replace('.', ',');

/**
 * Рейтинг УК района для жителей (ADR-022): 50% закрытые в срок, 30% оценки ремонта жителями,
 * 20% ремонты, подтверждённые жителями. УК дома жителя отмечена.
 */
export function Rating() {
  const { back } = useRouter();
  const res = useResource(() => api.districtRating(), []);
  const title = 'Рейтинг УК района';

  if (res.loading && !res.data) {
    return (
      <Screen title={title} onBack={back}>
        <Loading />
      </Screen>
    );
  }
  if (res.error || !res.data) {
    return (
      <Screen title={title} onBack={back}>
        <ErrorState title="Не удалось загрузить рейтинг" message={res.error?.message ?? ''} onRetry={res.reload} />
      </Screen>
    );
  }

  const { district, period_days, my_org_id, sample_data, organizations } = res.data;
  const rated = organizations.filter((o) => o.score !== null);
  const few = organizations.filter((o) => o.score === null);
  return (
    <Screen title={title} onBack={back}>
      <p className={s.text}>
        Район {district}, за {period_days} {plural(period_days, 'день', 'дня', 'дней')}.
      </p>
      {organizations.length === 0 ? (
        <EmptyState title="В районе пока нет управляющих компаний" text="Рейтинг появится, когда по домам района будут заявки." />
      ) : (
        <ol className={r.list}>
          {rated.map((o, i) => (
            <RatingRow key={o.id} org={o} place={i + 1} mine={o.id === my_org_id} />
          ))}
        </ol>
      )}
      {few.length > 0 && (
        <>
          <Section title="Мало данных" />
          <ul className={r.list}>
            {few.map((o) => (
              <li key={o.id} className={`${r.item} ${o.id === my_org_id ? r.mine : ''}`}>
                <span />
                <div>
                  <h3 className={r.name}>
                    {o.name}
                    {o.id === my_org_id && <span className={r.mineTag}>ваша УК</span>}
                  </h3>
                  <p className={r.meta}>
                    Закрыто заявок: {o.closed_total}. Для оценки нужно не меньше трёх за период.
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </>
      )}
      <p className={s.hint}>
        {sample_data && 'Пример данных. '}
        Как считается: 50% заявки, закрытые в срок, 30% оценки ремонта жителями, 20% ремонты, которые жители подтвердили.
      </p>
    </Screen>
  );
}

function RatingRow({ org, place, mine }: { org: RatingOrg; place: number; mine: boolean }) {
  const onTime = org.closed_total > 0 ? Math.round((org.closed_on_time * 100) / org.closed_total) : 0;
  return (
    <li className={`${r.item} ${mine ? r.mine : ''}`}>
      <span className={r.place} aria-label={`${place} место`}>
        {place}
      </span>
      <div>
        <h3 className={r.name}>
          {org.name}
          {mine && <span className={r.mineTag}>ваша УК</span>}
        </h3>
        <p className={r.meta}>
          {org.rating_avg !== null ? (
            <>
              <StarsValue value={org.rating_avg} /> {ratingText(org.rating_avg)}, {org.ratings}{' '}
              {plural(org.ratings, 'оценка', 'оценки', 'оценок')} жителей
            </>
          ) : (
            'Оценок жителей пока нет'
          )}
        </p>
        <p className={r.meta}>
          В срок {onTime}%, подтверждено жителями {org.confirmed_by_residents}
          {org.overdue_open > 0 && `, просрочено сейчас ${org.overdue_open}`}
        </p>
      </div>
      <span className={r.score}>
        {org.score}
        <span className={r.scoreUnit}>из 100</span>
      </span>
    </li>
  );
}
