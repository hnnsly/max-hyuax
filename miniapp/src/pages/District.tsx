import { Button } from '@maxhub/max-ui';
import { MapTrifold } from '@phosphor-icons/react';
import { A11yToggle } from '../app/a11y';
import { useRouter } from '../app/router';
import { useUser } from '../app/session';
import { api } from '../shared/api/client';
import { useResource } from '../shared/api/useResource';
import { plural } from '../shared/lib/format';
import { rankDistrict } from '../shared/lib/metrics';
import { IssueList } from '../shared/ui/IssueRow';
import { EmptyState, ErrorState, Island, Loading, Screen, Section } from '../shared/ui/Layout';
import { RoleSwitcher } from './Other';
import s from './pages.module.css';

/**
 * Кабинет района (управа или жилинспекция): сравнение УК района и просроченные заявки.
 * Только чтение: из списка открывается карточка заявки без кнопок действий.
 */
export function District() {
  const user = useUser();
  const { push } = useRouter();
  const res = useResource(async () => {
    const [metrics, overdue] = await Promise.all([api.districtMetrics(), api.districtOverdue()]);
    return { metrics, overdue };
  }, []);
  const title = user.district ? `Район ${user.district}` : 'Район';

  if (res.loading && !res.data) {
    return (
      <Screen title={title}>
        <Loading />
      </Screen>
    );
  }
  if (res.error || !res.data) {
    return (
      <Screen title={title}>
        <ErrorState title="Не удалось загрузить район" message={res.error?.message ?? ''} onRetry={res.reload} />
      </Screen>
    );
  }

  const { metrics, overdue } = res.data;
  const rows = rankDistrict(metrics.organizations);
  const period = `${metrics.period_days} ${plural(metrics.period_days, 'день', 'дня', 'дней')}`;
  const now = new Date();

  return (
    <Screen title={title}>
      <Button
        variant="secondary"
        size="medium"
        stretched
        iconBefore={<MapTrifold size={20} />}
        onClick={() => push({ name: 'districtMap' })}
      >
        Карта района
      </Button>
      <Section title="Управляющие компании" aside={`${rows.length} УК`} />
      {rows.length === 0 ? (
        <EmptyState title="В районе пока нет УК" text="Когда дома района подключат к сервису, здесь появится сравнение управляющих компаний." />
      ) : (
        rows.map((o) => (
          <Island key={o.id}>
            <div className={s.orgCard}>
              <h3 className={s.orgCardName}>{o.name}</h3>
              <dl className={s.orgFigures}>
                <div>
                  <dt>открыто</dt>
                  <dd>{o.open_total}</dd>
                </div>
                <div>
                  <dt>просрочено</dt>
                  <dd className={o.overdue_open > 0 ? s.figureLate : undefined}>{o.overdue_open}</dd>
                </div>
                <div>
                  <dt>в срок</dt>
                  <dd>{o.onTime}</dd>
                </div>
                <div>
                  <dt>первый ответ</dt>
                  <dd>{o.response}</dd>
                </div>
              </dl>
              {(o.confirmed_by_residents > 0 || o.reopened_by_residents > 0) && (
                <p className={s.hint}>
                  Жители подтвердили {o.confirmed_by_residents} {plural(o.confirmed_by_residents, 'ремонт', 'ремонта', 'ремонтов')}
                  {o.reopened_by_residents > 0 &&
                    `, вернули в работу ${o.reopened_by_residents} ${plural(o.reopened_by_residents, 'заявку', 'заявки', 'заявок')}`}
                </p>
              )}
            </div>
          </Island>
        ))
      )}

      <Section title="Просрочено в районе" aside={overdue.length > 0 && `${overdue.length} ${plural(overdue.length, 'заявка', 'заявки', 'заявок')}`} />
      {overdue.length === 0 ? (
        <EmptyState title="Просроченных заявок нет" text="Все УК района отвечают жителям в срок." />
      ) : (
        <IssueList issues={overdue} now={now} showAddress onOpen={(id) => push({ name: 'issue', id })} />
      )}

      <p className={s.hint}>
        {metrics.sample_data && 'Пример данных. '}
        Счётчики за {period}, первый ответ считается как медиана за 7 дней. УК считается целиком, со всеми своими домами.
      </p>
      <A11yToggle />
      <RoleSwitcher />
    </Screen>
  );
}
