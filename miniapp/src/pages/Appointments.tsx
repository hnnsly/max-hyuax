import { Button, Input, Radio } from '@maxhub/max-ui';
import { CalendarBlank, Wrench } from '@phosphor-icons/react';
import { useState, type FormEvent } from 'react';
import { useRouter } from '../app/router';
import { useSession, useUser } from '../app/session';
import { api, ApiError } from '../shared/api/client';
import type { House } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { dotDateTime, splitAddress } from '../shared/lib/format';
import { EmptyState, ErrorState, Island, Loading, Screen, Section, useToast } from '../shared/ui/Layout';
import { Sheet } from '../shared/ui/Sheet';
import s from './pages.module.css';
import st from './stickers.module.css';

function buildUpcomingSlots(): { label: string; iso: string }[] {
  const now = new Date();
  const slots: { label: string; iso: string }[] = [];
  for (let dayOffset = 1; dayOffset <= 3; dayOffset++) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + dayOffset);
    if (d.getDay() === 0 || d.getDay() === 6) continue;
    for (const hour of [10, 14, 16]) {
      const slotDate = new Date(d.getFullYear(), d.getMonth(), d.getDate(), hour, 0, 0);
      const dayStr = slotDate.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' });
      slots.push({
        label: `${dayStr} в ${hour}:00`,
        iso: slotDate.toISOString(),
      });
    }
  }
  if (slots.length === 0) {
    const fallback = new Date(now.getTime() + 24 * 3600_000);
    fallback.setHours(14, 0, 0, 0);
    slots.push({ label: 'Ближайший рабочий день, 14:00', iso: fallback.toISOString() });
  }
  return slots;
}

/** Экран жителя: запись на личный приём в управляющую компанию (ПП РФ № 416 п. 28, ADR-024). */
export function Appointments() {
  const user = useUser();
  const { setUser } = useSession();
  const { back } = useRouter();
  const [toast, showToast] = useToast();
  const res = useResource(async () => {
    const [specialists, mine] = await Promise.all([api.specialists(), api.myAppointments()]);
    return { specialists, mine };
  }, []);

  const slots = buildUpcomingSlots();
  const [specCode, setSpecCode] = useState('chief_engineer');
  const [slotIso, setSlotIso] = useState(slots[0]?.iso ?? '');
  const [topic, setTopic] = useState('');
  const [agree, setAgree] = useState(user.has_consent);
  const [busy, setBusy] = useState(false);

  const book = async (e: FormEvent) => {
    e.preventDefault();
    if (topic.trim().length < 3) {
      showToast('Опишите тему обращения (минимум 3 символа)');
      return;
    }
    if (!agree) {
      showToast('Нужно согласие на обработку персональных данных');
      return;
    }
    setBusy(true);
    try {
      if (!user.has_consent) {
        setUser(await api.acceptConsent(user.consent_version));
      }
      await api.bookAppointment({
        specialist: specCode,
        topic: topic.trim(),
        slot_at: slotIso,
      });
      setTopic('');
      showToast('Вы записаны на личный приём в УК');
      res.reload();
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не удалось записаться на приём');
    } finally {
      setBusy(false);
    }
  };

  const cancel = async (id: string) => {
    try {
      await api.cancelAppointment(id);
      showToast('Запись на приём отменена');
      res.reload();
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не удалось отменить запись');
    }
  };

  if (res.loading && !res.data) {
    return (
      <Screen title="Приём в УК" onBack={back}>
        <Loading />
      </Screen>
    );
  }
  if (res.error || !res.data) {
    return (
      <Screen title="Приём в УК" onBack={back}>
        <ErrorState title="Не удалось загрузить расписание приёма" message={res.error?.message ?? ''} onRetry={res.reload} />
      </Screen>
    );
  }

  const { specialists, mine } = res.data;

  return (
    <Screen title="Запись на приём в УК" onBack={back}>
      <p className={s.text}>
        По Правилам управления МКД (ПП РФ № 416) управляющая организация проводит личный приём жителей. Выберите специалиста, время и укажите вопрос.
      </p>

      <Island>
        <form className={s.block} onSubmit={book}>
          <h3 className={s.blockTitle}>Специалист</h3>
          <fieldset className={s.radios} style={{ margin: '8px 0 12px' }}>
            <legend className="visually-hidden">Выбор специалиста</legend>
            {specialists.map((sp) => (
              <label key={sp.code} className={s.radio} style={{ alignItems: 'flex-start' }}>
                <Radio name="specialist" value={sp.code} checked={specCode === sp.code} onChange={() => setSpecCode(sp.code)} />
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <span style={{ fontWeight: 650 }}>{sp.title}</span>
                  <span className={s.hint}>{sp.schedule}</span>
                  <span className={s.hint}>{sp.description}</span>
                </div>
              </label>
            ))}
          </fieldset>

          <div className={s.blockTitle} style={{ marginBottom: 6 }}>
            Дата и время
          </div>
          <div className={st.chips} role="group" aria-label="Слот времени" style={{ marginBottom: 12 }}>
            {slots.map((sl) => (
              <button
                key={sl.iso}
                type="button"
                className={st.chip}
                aria-pressed={slotIso === sl.iso}
                onClick={() => setSlotIso(sl.iso)}
              >
                {sl.label}
              </button>
            ))}
          </div>

          <label className={s.blockTitle} htmlFor="visit-topic" style={{ display: 'block', marginBottom: 6 }}>
            Вопрос для обсуждения
          </label>
          <Input
            id="visit-topic"
            placeholder="Например: перерасчёт за отопление или замена стояка"
            value={topic}
            maxLength={500}
            required
            onChange={(e) => setTopic(e.target.value)}
          />

          {!user.has_consent && (
            <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 'var(--fs-secondary)', marginTop: 10, cursor: 'pointer' }}>
              <input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)} />
              <span>Согласен на обработку персональных данных</span>
            </label>
          )}

          <div style={{ marginTop: 12 }}>
            <Button type="submit" variant="primary" size="large" stretched loading={busy} disabled={!agree || topic.trim().length < 3}>
              Записаться на приём
            </Button>
          </div>
        </form>
      </Island>

      <Section title="Мои записи на приём" />
      {mine.length === 0 ? (
        <EmptyState title="Записей пока нет" text="Вы ещё не записывались на личный приём в управляющую компанию." />
      ) : (
        mine.map((a) => (
          <Island key={a.id}>
            <div className={s.block}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8 }}>
                <h3 className={s.blockTitle}>{a.specialist_title}</h3>
                <span className={s.hint}>{a.status === 'cancelled' ? 'Отменена' : 'Запланирована'}</span>
              </div>
              <p className={s.text} style={{ margin: '4px 0' }}>
                <CalendarBlank size={15} style={{ verticalAlign: '-2px', marginRight: 4 }} />
                {dotDateTime(a.slot_at)}
              </p>
              <p className={s.hint}>{a.topic}</p>
              {a.status === 'booked' && (
                <div style={{ marginTop: 8 }}>
                  <Button variant="secondary" size="small" onClick={() => cancel(a.id)}>
                    Отменить запись
                  </Button>
                </div>
              )}
            </div>
          </Island>
        ))
      )}
      {toast}
    </Screen>
  );
}

/** Раздел кабинета УК: управление плановыми работами в домах и очередь жителей на личный приём. */
export function UkOfficeView() {
  const [toast, showToast] = useToast();
  const [creating, setCreating] = useState(false);
  const res = useResource(async () => {
    const [maintenance, appointments, houses] = await Promise.all([
      api.ukMaintenance(),
      api.ukAppointments(),
      api.ukHouses(),
    ]);
    return { maintenance, appointments, houses };
  }, []);

  if (res.loading && !res.data) return <Loading />;
  if (res.error || !res.data) {
    return <ErrorState title="Не удалось загрузить данные" message={res.error?.message ?? ''} onRetry={res.reload} />;
  }

  const { maintenance, appointments, houses } = res.data;

  const stopMaintenance = async (id: string) => {
    try {
      await api.deleteMaintenance(id);
      showToast('Плановые работы завершены');
      res.reload();
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не удалось удалить запись');
    }
  };

  const cancelAppt = async (id: string) => {
    try {
      await api.cancelAppointment(id);
      showToast('Приём отменён');
      res.reload();
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не удалось отменить приём');
    }
  };

  return (
    <>
      <Section title="Плановые работы в домах" />
      <Button variant="primary" size="medium" stretched iconBefore={<Wrench size={18} />} onClick={() => setCreating(true)}>
        Объявить о плановых работах
      </Button>
      {maintenance.length === 0 ? (
        <EmptyState title="Нет плановых работ" text="Объявите о работах, чтобы жители видели баннер в приложении и не дублировали заявки." />
      ) : (
        maintenance.map((m) => (
          <Island key={m.id}>
            <div className={s.block}>
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 }}>
                <h3 className={s.blockTitle}>{m.title}</h3>
                <span className={s.hint}>{m.category_title}</span>
              </div>
              {m.address && <p className={s.text}>{m.address}</p>}
              {m.description && <p className={s.hint}>{m.description}</p>}
              <p className={s.hint}>Окончание: {dotDateTime(m.ends_at)}</p>
              <div style={{ marginTop: 8 }}>
                <Button variant="secondary" size="small" onClick={() => stopMaintenance(m.id)}>
                  Завершить работы
                </Button>
              </div>
            </div>
          </Island>
        ))
      )}

      <Section title="Записи жителей на приём" />
      {appointments.length === 0 ? (
        <EmptyState title="Записей на приём нет" text="Когда жители запишутся к специалистам УК, они появятся здесь." />
      ) : (
        appointments.map((a) => (
          <Island key={a.id}>
            <div className={s.block}>
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 }}>
                <h3 className={s.blockTitle}>{a.specialist_title}</h3>
                <span className={s.hint}>{a.status === 'cancelled' ? 'Отменена' : dotDateTime(a.slot_at)}</span>
              </div>
              {a.address && (
                <p className={s.text}>
                  {a.address}
                  {a.user_name ? ` • ${a.user_name}` : ''}
                </p>
              )}
              <p className={s.hint}>{a.topic}</p>
              {a.status === 'booked' && (
                <div style={{ marginTop: 8 }}>
                  <Button variant="secondary" size="small" onClick={() => cancelAppt(a.id)}>
                    Отменить приём
                  </Button>
                </div>
              )}
            </div>
          </Island>
        ))
      )}

      <CreateMaintenanceSheet
        open={creating}
        houses={houses}
        onClose={() => setCreating(false)}
        onCreated={() => {
          setCreating(false);
          showToast('Плановые работы опубликованы для жителей дома');
          res.reload();
        }}
      />
      {toast}
    </>
  );
}

function CreateMaintenanceSheet({
  open,
  houses,
  onClose,
  onCreated,
}: {
  open: boolean;
  houses: House[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [houseId, setHouseId] = useState(houses[0]?.id ?? 'h-17k2');
  const [category, setCategory] = useState('heating');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [hours, setHours] = useState(4);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const categories = [
    { code: 'heating', label: 'Отопление и ГВС' },
    { code: 'water', label: 'Холодная вода' },
    { code: 'electricity', label: 'Электричество' },
    { code: 'lift', label: 'Лифт' },
    { code: 'lighting', label: 'Свет в подъезде' },
    { code: 'other', label: 'Другое' },
  ];

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!title.trim()) {
      setError('Укажите название работ');
      return;
    }
    setBusy(true);
    setError('');
    try {
      await api.createMaintenance({
        house_id: houseId || houses[0]?.id || 'h-17k2',
        category,
        title: title.trim(),
        description: description.trim(),
        hours,
      });
      setTitle('');
      setDescription('');
      onCreated();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось опубликовать работы');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet open={open} title="Объявить о работах" onClose={onClose} locked={busy}>
      <form className={s.sheetBody} onSubmit={submit}>
        {houses.length > 0 && (
          <>
            <div className={s.blockTitle}>Дом</div>
            <div className={st.chips} role="group" aria-label="Дом">
              {houses.map((h) => (
                <button
                  key={h.id}
                  type="button"
                  className={st.chip}
                  aria-pressed={(houseId || houses[0]?.id) === h.id}
                  onClick={() => setHouseId(h.id)}
                >
                  {splitAddress(h.address).number || h.address}
                </button>
              ))}
            </div>
          </>
        )}

        <div className={s.blockTitle}>Категория</div>
        <div className={st.chips} role="group" aria-label="Категория работ">
          {categories.map((c) => (
            <button
              key={c.code}
              type="button"
              className={st.chip}
              aria-pressed={category === c.code}
              onClick={() => setCategory(c.code)}
            >
              {c.label}
            </button>
          ))}
        </div>

        <label className={s.blockTitle} htmlFor="maint-title">
          Что проводится
        </label>
        <Input
          id="maint-title"
          placeholder="Например: Опрессовка стояков отопления"
          value={title}
          maxLength={200}
          required
          onChange={(e) => setTitle(e.target.value)}
        />

        <label className={s.blockTitle} htmlFor="maint-desc">
          Подробности для жителей (по желанию)
        </label>
        <textarea
          id="maint-desc"
          className={s.sheetText}
          value={description}
          maxLength={1000}
          onChange={(e) => setDescription(e.target.value)}
        />

        <div className={s.blockTitle}>Длительность работ</div>
        <div className={st.chips} role="group" aria-label="Длительность">
          {[2, 4, 8, 24].map((h) => (
            <button key={h} type="button" className={st.chip} aria-pressed={hours === h} onClick={() => setHours(h)}>
              {h} ч
            </button>
          ))}
        </div>

        {error && (
          <p className={s.hint} role="alert">
            {error}
          </p>
        )}
        <Button type="submit" variant="primary" size="large" stretched loading={busy}>
          Опубликовать
        </Button>
      </form>
    </Sheet>
  );
}
