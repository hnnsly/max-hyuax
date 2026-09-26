import { Star } from '@phosphor-icons/react';
import s from './ui.module.css';

const STARS = [1, 2, 3, 4, 5];

/**
 * Оценка ремонта от 1 до 5 (ADR-022): пять кнопок, нажатие сразу отправляет оценку.
 * Кнопки, а не радиогруппа: выбор один и окончательный, подтверждать нечего.
 */
export function StarsInput({ busy, onRate }: { busy: boolean; onRate: (stars: number) => void }) {
  return (
    <div className={s.stars} role="group" aria-label="Оценка ремонта от 1 до 5">
      {STARS.map((n) => (
        <button key={n} type="button" className={s.starButton} disabled={busy} aria-label={`${n} из 5`} onClick={() => onRate(n)}>
          <Star size={34} weight="bold" aria-hidden="true" />
        </button>
      ))}
    </div>
  );
}

/** Оценка звёздами для показа. */
export function StarsValue({ value, size = 18 }: { value: number; size?: number }) {
  return (
    <span className={s.starsValue} role="img" aria-label={`${value} из 5`}>
      {STARS.map((n) => (
        <Star key={n} size={size} weight={n <= Math.round(value) ? 'fill' : 'regular'} aria-hidden="true" />
      ))}
    </span>
  );
}
