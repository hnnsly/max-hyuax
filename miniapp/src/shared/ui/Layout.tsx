import { Button } from '@maxhub/max-ui';
import { useCallback, useEffect, useRef, useState, type CSSProperties, type ReactNode, type RefObject } from 'react';
import { bridge } from '../bridge/bridge';
import s from './ui.module.css';

/**
 * Экран: шапка только вне MAX (в MAX шапку и кнопку «Назад» рисует клиент). Заголовок h1 есть
 * всегда, в MAX он скрыт визуально: по нему экранный чтец понимает, где житель. При смене экрана
 * фокус переходит на заголовок, иначе чтец остаётся на месте нажатой кнопки прошлого экрана.
 */
export function Screen({ title, onBack, children, actions }: { title: string; onBack?: () => void; children: ReactNode; actions?: ReactNode }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    document.title = title;
    // Поле с autoFocus уже забрало фокус: не отнимаем.
    const active = document.activeElement;
    if (!active || active === document.body) heading.current?.focus({ preventScroll: true });
  }, [title]);
  return (
    <div className={s.screen}>
      {bridge.inMax() ? (
        <h1 ref={heading} tabIndex={-1} className={`visually-hidden ${s.screenHeading}`}>
          {title}
        </h1>
      ) : (
        <WebHeader title={title} onBack={onBack} heading={heading} />
      )}
      <main className={s.content}>{children}</main>
      {actions && <div className={s.actionBar}>{actions}</div>}
    </div>
  );
}

function WebHeader({ title, onBack, heading }: { title: string; onBack?: () => void; heading: RefObject<HTMLHeadingElement | null> }) {
  return (
    <header className={s.header}>
      {onBack ? (
        <button type="button" className={s.headerBack} onClick={onBack}>
          Назад
        </button>
      ) : (
        <span />
      )}
      <h1 ref={heading} tabIndex={-1} className={`${s.headerTitle} ${s.screenHeading}`} style={{ margin: 0 }}>
        {title}
      </h1>
      <span />
    </header>
  );
}

export function Section({ title, aside }: { title: string; aside?: ReactNode }) {
  return (
    <div className={s.section}>
      <h2 className={s.sectionTitle}>{title}</h2>
      {aside && <span className={s.sectionAside}>{aside}</span>}
    </div>
  );
}

export function Island({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return (
    <div className={s.island} style={style}>
      {children}
    </div>
  );
}

export function Facts({ items }: { items: { value: ReactNode; label: string }[] }) {
  return (
    <div className={`${s.island} ${s.facts}`}>
      {items.map((f) => (
        <div key={f.label} className={s.fact}>
          <span className={s.factValue}>{f.value}</span>
          <span className={s.factLabel}>{f.label}</span>
        </div>
      ))}
    </div>
  );
}

export function DemoMark() {
  return <p className={s.demoMark}>Пример данных</p>;
}

export function Skeleton({ height }: { height: number }) {
  return <div className={s.skeleton} style={{ height }} aria-hidden="true" />;
}

export function Loading() {
  return (
    <div aria-busy="true" aria-label="Загрузка" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <Skeleton height={140} />
      <Skeleton height={64} />
      <Skeleton height={220} />
    </div>
  );
}

export function EmptyState({ title, text, action }: { title: string; text: string; action?: ReactNode }) {
  return (
    <div className={`${s.island} ${s.state}`}>
      <h3 className={s.stateTitle}>{title}</h3>
      <p className={s.stateText}>{text}</p>
      {action}
    </div>
  );
}

export function ErrorState({ title, message, onRetry }: { title: string; message: string; onRetry: () => void }) {
  return (
    <div className={`${s.island} ${s.state}`} role="alert">
      <h3 className={s.stateTitle}>{title}</h3>
      <p className={s.stateText}>{message}</p>
      <Button variant="secondary" size="medium" onClick={onRetry}>
        Повторить
      </Button>
    </div>
  );
}

/**
 * Тост: popover="manual" не закрывается кликом мимо и держится в верхнем слое.
 * Без поддержки Popover API остаётся обычным фиксированным блоком.
 */
export function Toast({ message, onDone }: { message: string; onDone: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (el && 'showPopover' in el) el.showPopover();
    const t = setTimeout(onDone, 3400);
    return () => clearTimeout(t);
  }, [message, onDone]);
  // Текст для экранного чтеца объявляет постоянный живой регион useToast, здесь только картинка.
  return (
    <div ref={ref} popover="manual" className={s.toast} aria-hidden="true">
      {message}
    </div>
  );
}

/**
 * Хук для показа тоста: const [toast, show] = useToast(). Живой регион есть на экране всегда:
 * если он появляется вместе с текстом, чтецы на телефонах сообщение пропускают.
 */
export function useToast(): [ReactNode, (msg: string) => void] {
  const [msg, setMsg] = useState('');
  const clear = useCallback(() => setMsg(''), []);
  const node = (
    <>
      <div className="visually-hidden" role="status" aria-live="polite">
        {msg}
      </div>
      {msg && <Toast key={msg} message={msg} onDone={clear} />}
    </>
  );
  return [node, setMsg];
}
