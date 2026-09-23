import { useEffect, useId, useRef, type ReactNode } from 'react';
import s from './sheet.module.css';

/**
 * Нижний лист на нативном <dialog>: showModal() держит фокус внутри и закрывается по Esc,
 * closedby="any" — нажатием мимо листа. В Safari closedby нет, там то же делает обработчик клика.
 */
export function Sheet({ open, title, onClose, children }: { open: boolean; title: string; onClose: () => void; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  useEffect(() => {
    const d = ref.current;
    if (!d || 'closedBy' in HTMLDialogElement.prototype) return;
    const onClick = (e: MouseEvent) => {
      if (e.target !== d) return;
      const r = d.getBoundingClientRect();
      const inside = r.top <= e.clientY && e.clientY <= r.bottom && r.left <= e.clientX && e.clientX <= r.right;
      if (!inside) d.close();
    };
    d.addEventListener('click', onClick);
    return () => d.removeEventListener('click', onClick);
  }, []);

  return (
    <dialog ref={ref} className={s.sheet} aria-labelledby={titleId} onClose={onClose} {...{ closedby: 'any' }}>
      <div className={s.grabber} aria-hidden="true" />
      <h2 id={titleId} className={s.title}>
        {title}
      </h2>
      {children}
    </dialog>
  );
}
