import type { Status } from '../api/types';
import { splitAddress } from '../lib/format';
import s from './ui.module.css';

const bolts = [
  { left: 13, top: 13 },
  { right: 13, top: 13 },
  { left: 13, bottom: 13 },
  { right: 13, bottom: 13 },
];

/** Эмалевая табличка дома: улица, крупный номер, подъезд. */
export function HousePlate({ address, entrance }: { address: string; entrance?: string }) {
  const { street, number } = splitAddress(address);
  return (
    <div className={s.plate} role="img" aria-label={entrance ? `${address}, ${entrance}` : address}>
      {bolts.map((b, i) => (
        <span key={i} className={s.bolt} style={b} aria-hidden="true" />
      ))}
      <div className={s.houseInner} aria-hidden="true">
        <span className={s.street}>{street}</span>
        <div className={s.houseLine}>
          <span className={s.houseNumber}>{number || street}</span>
          {entrance && <span className={s.chip}>{entrance}</span>}
        </div>
      </div>
    </div>
  );
}

/** Табличка с номером заявки. */
export function IssuePlate({ number }: { number: number }) {
  return (
    <div className={`${s.plate} ${s.issuePlate}`} role="img" aria-label={`Заявка номер ${number}`}>
      <div className={s.issuePlateInner} aria-hidden="true">
        <span className={s.issuePlateLabel}>Заявка</span>
        <span className={s.issuePlateNumber}>{String(number).padStart(4, '0')}</span>
      </div>
    </div>
  );
}

export const statusLabel: Record<Status, string> = {
  sent: 'Отправлено',
  accepted: 'Принято',
  in_progress: 'В работе',
  done: 'Выполнено',
  rejected: 'Отклонено',
};

/** Штамп статуса. land — анимация «приземления» при смене статуса. */
export function Stamp({ status, small, land }: { status: Status; small?: boolean; land?: boolean }) {
  const cls = [s.stamp, s[`st_${status}`], small && s.stampSmall, land && s.stampLand].filter(Boolean).join(' ');
  return (
    <span className={cls}>
      <span className="visually-hidden">Статус: </span>
      {statusLabel[status]}
    </span>
  );
}
