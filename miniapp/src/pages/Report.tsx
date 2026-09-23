import { Button } from '@maxhub/max-ui';
import { Door, Drop, Elevator, Lightbulb, Thermometer, Trash } from '@phosphor-icons/react';
import { useState, type ComponentType } from 'react';
import { useRouter } from '../app/router';
import { useSession, useUser } from '../app/session';
import { api, ApiError } from '../shared/api/client';
import type { AssetObject, Category, HouseDetails, Issue } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { capitalize, dayMonth, plural, time } from '../shared/lib/format';
import { EmptyState, ErrorState, Island, Loading, Screen, useToast } from '../shared/ui/Layout';
import { Stamp } from '../shared/ui/Plate';
import s from './report.module.css';
import p from './pages.module.css';

type IconType = ComponentType<{ size?: number; weight?: 'regular' | 'bold' }>;

/** Плитки категорий по холсту Report: короткие подписи, иконки phosphor. */
const tiles: { code: string; label: string; icon: IconType }[] = [
  { code: 'lift', label: 'Лифт', icon: Elevator },
  { code: 'lighting', label: 'Свет', icon: Lightbulb },
  { code: 'leak', label: 'Протечка', icon: Drop },
  { code: 'heating', label: 'Отопление', icon: Thermometer },
  { code: 'door', label: 'Дверь', icon: Door },
  { code: 'garbage', label: 'Мусор', icon: Trash },
];

type Step = 'describe' | 'check' | 'send';
const stepNames: Record<Step, string> = { describe: 'Описание', check: 'Проверка', send: 'Отправка' };
const order: Step[] = ['describe', 'check', 'send'];

function Steps({ current }: { current: Step }) {
  const at = order.indexOf(current);
  return (
    <ol className={s.steps} aria-label={`Шаг ${at + 1} из 3`}>
      {order.map((st, i) => (
        <li key={st} className={`${s.step} ${i < at ? s.stepDone : ''} ${i === at ? s.stepActive : ''}`} aria-current={i === at ? 'step' : undefined}>
          {stepNames[st]}
        </li>
      ))}
    </ol>
  );
}

interface Context {
  house: HouseDetails;
  categories: Category[];
  fixedObject?: AssetObject;
}

/** Новая заявка: описание, проверка на дубль, отправка. Вход по QR сразу знает дом и объект. */
export function Report({ objectCode, category }: { objectCode?: string; category?: string }) {
  const user = useUser();
  const { back } = useRouter();
  const res = useResource<Context>(async () => {
    const categories = await api.categories();
    if (objectCode) {
      const { object, house } = await api.objectByCode(objectCode);
      return { house: await api.house(house.id), categories, fixedObject: object };
    }
    return { house: await api.house(user.house_id ?? ''), categories };
  }, [objectCode, user.house_id]);

  if (res.loading && !res.data) {
    return (
      <Screen title="Новая заявка" onBack={back}>
        <Loading />
      </Screen>
    );
  }
  if (res.error || !res.data) {
    return (
      <Screen title="Новая заявка" onBack={back}>
        {res.error?.code === 'not_found' ? (
          <EmptyState title="Код не найден" text="Возможно, наклейка устарела. Выберите дом и опишите проблему вручную." />
        ) : (
          <ErrorState title="Не удалось открыть форму" message={res.error?.message ?? ''} onRetry={res.reload} />
        )}
      </Screen>
    );
  }
  return <ReportFlow ctx={res.data} initialCategory={res.data.fixedObject?.category ?? category} />;
}

function ReportFlow({ ctx, initialCategory }: { ctx: Context; initialCategory?: string }) {
  const user = useUser();
  const { setUser } = useSession();
  const { back, reset } = useRouter();
  const [toast, showToast] = useToast();
  const [step, setStep] = useState<Step>('describe');
  const [category, setCategory] = useState(initialCategory ?? '');
  const [objectId, setObjectId] = useState(ctx.fixedObject?.id ?? '');
  const [text, setText] = useState('');
  const [similar, setSimilar] = useState<Issue[]>([]);
  const [agree, setAgree] = useState(user.has_consent);
  const [busy, setBusy] = useState(false);

  const { house } = ctx;
  const rule = ctx.categories.find((c) => c.code === category);
  const objects = house.objects.filter((o) => o.category === category);
  const object = house.objects.find((o) => o.id === objectId);
  const where = [house.address, object?.label].filter(Boolean).join(', ');

  const toHome = (issueId: string, flash: string) => reset({ name: 'home' }, { name: 'issue', id: issueId, flash });

  const check = async () => {
    setBusy(true);
    try {
      const found = await api.similar(house.id, category, objectId || undefined);
      setSimilar(found);
      setStep(found.length > 0 ? 'check' : 'send');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не получилось проверить. Попробуйте ещё раз');
    } finally {
      setBusy(false);
    }
  };

  const ensureConsent = async () => {
    if (user.has_consent) return;
    setUser(await api.acceptConsent(user.consent_version));
  };

  const join = async (issue: Issue) => {
    setBusy(true);
    try {
      await ensureConsent();
      await api.join(issue.id);
      toHome(issue.id, 'Вы присоединились. Карточка со статусом придёт в чат с ботом.');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не получилось. Попробуйте ещё раз');
      setBusy(false);
    }
  };

  const submit = async () => {
    setBusy(true);
    try {
      await ensureConsent();
      const created = await api.report({ house_id: house.id, object_id: objectId || undefined, category, description: text.trim() });
      toHome(created.id, `Заявка № ${created.number} ушла в ${house.organization.name}. Карточка со статусом придёт в чат с ботом.`);
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Не получилось отправить. Попробуйте ещё раз');
      setBusy(false);
    }
  };

  if (step === 'check' && similar[0]) {
    const it = similar[0];
    const n = it.participant_count;
    return (
      <Screen
        title="Проверка"
        onBack={() => setStep('describe')}
        actions={
          <>
            <Button variant="primary" size="large" stretched loading={busy} onClick={() => join(it)}>
              Это и у меня
            </Button>
            <Button variant="secondary" size="medium" stretched onClick={() => setStep('send')}>
              У меня другая проблема
            </Button>
          </>
        }
      >
        <Steps current="check" />
        <h2 className={s.h1}>Об этом уже сообщили</h2>
        <Island>
          <div className={s.dupFigure}>
            <span className={s.dupCount}>{n}</span>
            <p className={s.dupNote}>
              {plural(n, 'сосед уже сообщил', 'соседа уже сообщили', 'соседей уже сообщили')}. Первым сообщил житель {dayMonth(it.created_at)} в {time(it.created_at)}.
            </p>
          </div>
        </Island>
        <Island>
          <div className={s.mini}>
            <div className={s.miniHead}>
              <span className={s.miniNumber}>№ {String(it.number).padStart(4, '0')}</span>
              <Stamp status={it.status} small />
            </div>
            <h3 className={s.miniTitle}>{it.title}</h3>
            {it.place && <p className={s.help}>{capitalize(it.place)}</p>}
            <dl className={p.keyValue}>
              <dt>Отвечает</dt>
              <dd>{house.organization.name}</dd>
              <dt>Срок ответа</dt>
              <dd>до {dayMonth(it.deadline)}</dd>
            </dl>
          </div>
        </Island>
        <p className={s.tip}>Отдельная заявка про то же самое только замедлит ответ УК. Присоединитесь, и УК увидит, сколько человек ждёт ремонта.</p>
        {toast}
      </Screen>
    );
  }

  if (step === 'send') {
    return (
      <Screen
        title="Новая заявка"
        onBack={() => setStep(similar.length > 0 ? 'check' : 'describe')}
        actions={
          <Button variant="primary" size="large" stretched loading={busy} disabled={!agree} onClick={submit}>
            Отправить заявку
          </Button>
        }
      >
        <Steps current="send" />
        <h2 className={s.h1}>Проверьте заявку</h2>
        <Island>
          <div className={p.block}>
            <dl className={p.keyValue}>
              <dt>Что</dt>
              <dd>{rule?.title ?? category}</dd>
              <dt>Где</dt>
              <dd>{capitalize(where)}</dd>
              {text.trim() && (
                <>
                  <dt>Описание</dt>
                  <dd>{text.trim()}</dd>
                </>
              )}
              <dt>Отвечает</dt>
              <dd>{house.organization.name}</dd>
              {rule && (
                <>
                  <dt>Срок ответа</dt>
                  <dd>
                    {rule.business_days} {plural(rule.business_days, 'рабочий день', 'рабочих дня', 'рабочих дней')}
                  </dd>
                </>
              )}
            </dl>
            {rule && <p className={p.basis}>{rule.basis}</p>}
          </div>
        </Island>
        {!user.has_consent && (
          <Island>
            <label className={s.consent}>
              <input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)} />
              <span>
                <span className={s.label}>Согласие на обработку данных для этой заявки</span>
                <br />
                <span className={s.help}>Соседи увидят только число сообщивших. Имя получит только управляющая компания.</span>
              </span>
            </label>
          </Island>
        )}
        {toast}
      </Screen>
    );
  }

  return (
    <Screen
      title="Новая заявка"
      onBack={back}
      actions={
        <Button variant="primary" size="large" stretched loading={busy} disabled={!category} onClick={check}>
          Продолжить
        </Button>
      }
    >
      <Steps current="describe" />
      <div>
        <h2 className={s.h1}>Что случилось?</h2>
        <p className={s.sub}>{capitalize(where)}</p>
      </div>
      <div className={s.tiles} role="group" aria-label="Категория">
        {tiles.map(({ code, label, icon: Icon }) => (
          <button
            key={code}
            type="button"
            className={s.tile}
            aria-pressed={category === code}
            disabled={Boolean(ctx.fixedObject) && ctx.fixedObject?.category !== code}
            onClick={() => {
              setCategory(code);
              if (!ctx.fixedObject) setObjectId('');
            }}
          >
            <Icon size={24} />
            {label}
          </button>
        ))}
      </div>
      {!ctx.fixedObject && objects.length > 0 && (
        <>
          <p className={s.label}>Где именно</p>
          <div className={s.chips} role="group" aria-label="Место">
            {objects.map((o) => (
              <button key={o.id} type="button" className={s.chip} aria-pressed={objectId === o.id} onClick={() => setObjectId(o.id)}>
                {capitalize(o.label)}
              </button>
            ))}
            <button type="button" className={s.chip} aria-pressed={objectId === ''} onClick={() => setObjectId('')}>
              Не знаю
            </button>
          </div>
        </>
      )}
      <label className={s.label} htmlFor="report-text">
        Опишите своими словами
      </label>
      <textarea
        id="report-text"
        className={s.textarea}
        value={text}
        maxLength={2000}
        onChange={(e) => setText(e.target.value)}
        placeholder="Например: кабина не приходит на вызов, горит индикатор"
      />
      <p className={s.help}>Коротко и как есть. Дальше проверим, не сообщали ли уже соседи.</p>
      {toast}
    </Screen>
  );
}
