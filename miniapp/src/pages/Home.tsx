import { Button, CellList, CellSimple, Counter } from '@maxhub/max-ui';
import { Phone, Plus, QrCode } from '@phosphor-icons/react';
import { useState } from 'react';
import { useRouter } from '../app/router';
import { useSession, useUser } from '../app/session';
import { api, ApiError } from '../shared/api/client';
import { useResource } from '../shared/api/useResource';
import { bridge } from '../shared/bridge/bridge';
import { capitalize, plural } from '../shared/lib/format';
import { houseOpenIssues, parseStartParam } from '../shared/lib/model';
import { IssueList } from '../shared/ui/IssueRow';
import { DemoMark, EmptyState, ErrorState, Facts, Island, Loading, Screen, Section, useToast } from '../shared/ui/Layout';
import { HousePlate } from '../shared/ui/Plate';
import { Sheet } from '../shared/ui/Sheet';
import { HouseCouncil } from './Council';
import s from './pages.module.css';

/** Главный экран жителя: табличка дома, УК, открытые проблемы дома, мои заявки. */
export function Home() {
  const user = useUser();
  const { push } = useRouter();
  const [deleting, setDeleting] = useState(false); // открыт лист удаления аккаунта
  const [toast, showToast] = useToast();
  const houseId = user.house_id ?? '';
  const res = useResource(async () => {
    const [house, issues, mine] = await Promise.all([api.house(houseId), api.houseIssues(houseId), api.myIssues()]);
    return { house, issues, mine };
  }, [houseId]);

  const scan = async () => {
    try {
      const target = parseStartParam(await bridge.scan());
      if (target?.kind === 'issue') push({ name: 'issue', id: target.id });
      else if (target?.kind === 'object') push({ name: 'report', objectCode: target.code });
    } catch {
      /* пользователь закрыл сканер */
    }
  };

  const actions = (
    <>
      <Button variant="primary" size="large" stretched iconBefore={<Plus size={20} weight="bold" />} onClick={() => push({ name: 'report' })}>
        Сообщить о проблеме
      </Button>
      {bridge.canScan() && (
        <Button variant="ghost" size="medium" stretched iconBefore={<QrCode size={20} />} onClick={scan}>
          Сканировать код в лифте
        </Button>
      )}
    </>
  );

  if (res.loading && !res.data) {
    return (
      <Screen title="Мой дом">
        <Loading />
      </Screen>
    );
  }
  if (res.error || !res.data) {
    return (
      <Screen title="Мой дом">
        <ErrorState title="Не удалось загрузить дом" message={res.error?.message ?? ''} onRetry={res.reload} />
      </Screen>
    );
  }

  const { house, issues, mine } = res.data;
  const open = houseOpenIssues(issues);
  const org = house.organization;
  const dispatcher = org.phone_dispatcher ?? org.phone_office;
  const schedule = capitalize(org.schedule?.split('; ').pop() ?? '');
  const now = new Date();
  // У дома из OpenStreetMap год и этажность неизвестны, а число подъездов условное: его не выдаём за факт.
  const osm = house.source === 'osm';
  const facts = [
    ...(house.year_built ? [{ value: house.year_built, label: 'построен' }] : []),
    ...(house.floors ? [{ value: house.floors, label: plural(house.floors, 'этаж', 'этажа', 'этажей') }] : []),
    ...(osm ? [] : [{ value: house.entrances_count, label: plural(house.entrances_count, 'подъезд', 'подъезда', 'подъездов') }]),
  ];

  return (
    <Screen title="Мой дом" actions={actions}>
      <HousePlate address={house.address} />
      {facts.length > 0 && <Facts items={facts} />}
      {house.source === 'model' && <DemoMark />}

      <Island>
        <div className={s.org}>
          <div style={{ minWidth: 0 }}>
            <div className={s.orgName}>{org.name}</div>
            {schedule && <div className={s.orgNote}>{schedule}</div>}
          </div>
          {dispatcher && (
            <a className={s.callButton} href={`tel:${dispatcher.replace(/[^+\d]/g, '')}`} aria-label="Позвонить в диспетчерскую">
              <Phone size={20} />
            </a>
          )}
        </div>
      </Island>

      <Section title="Сейчас в доме" aside={open.length > 0 && `${open.length} ${plural(open.length, 'открытая', 'открытые', 'открытых')}`} />
      {open.length > 0 ? (
        <IssueList issues={open} now={now} onOpen={(id) => push({ name: 'issue', id })} />
      ) : (
        <EmptyState title="В доме нет открытых проблем" text="Если что-то сломалось, сообщите. Соседи увидят заявку и смогут присоединиться." />
      )}

      <HouseCouncil onToast={showToast} />

      <CellList mode="island">
        <CellSimple title="Мои заявки" showChevron after={mine.length > 0 && <Counter value={mine.length} rounded />} onClick={() => push({ name: 'mine' })} />
        <CellSimple title="Другой дом" showChevron onClick={() => push({ name: 'houseSearch' })} />
      </CellList>
      <button type="button" className={s.dangerLink} onClick={() => setDeleting(true)}>
        Удалить аккаунт
      </button>
      <DeleteAccountSheet open={deleting} onClose={() => setDeleting(false)} />
      {toast}
    </Screen>
  );
}

/** Подтверждение удаления: что сотрётся, что останется. */
function DeleteAccountSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { accountDeleted } = useSession();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const remove = async () => {
    setBusy(true);
    setError('');
    try {
      await api.deleteAccount();
      accountDeleted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось удалить аккаунт. Попробуйте позже');
      setBusy(false);
    }
  };

  // Ошибка прошлой попытки не должна встречать при следующем открытии листа.
  const close = () => {
    setError('');
    onClose();
  };

  return (
    <Sheet open={open} title="Удалить аккаунт?" onClose={close} locked={busy}>
      <div className={s.sheetBody}>
        <ul className={s.plainList}>
          <li>Имя, телефон и согласие на обработку данных сотрутся.</li>
          <li>Ваши заявки останутся в доме без ваших данных: соседи и УК продолжат по ним работать.</li>
          <li>Фото, которые вы приложили к заявкам, удалятся.</li>
          <li>Уведомления о заявках в чат с ботом больше не придут.</li>
        </ul>
        {error && (
          <p className={s.hint} role="alert">
            {error}
          </p>
        )}
        <Button variant="destructive" size="large" stretched loading={busy} onClick={remove}>
          Удалить аккаунт
        </Button>
        <Button variant="secondary" size="large" stretched disabled={busy} onClick={close}>
          Отмена
        </Button>
      </div>
    </Sheet>
  );
}
