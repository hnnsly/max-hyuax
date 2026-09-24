import { Button } from '@maxhub/max-ui';
import { Phone } from '@phosphor-icons/react';
import { useState } from 'react';
import { useSession, useUser } from '../app/session';
import { api, ApiError } from '../shared/api/client';
import type { Contact } from '../shared/api/types';
import { bridge } from '../shared/bridge/bridge';
import { phoneView } from '../shared/lib/format';
import { Island } from '../shared/ui/Layout';
import s from './pages.module.css';

/**
 * Телефон для мастера у участника открытой заявки. Номер берётся только из клиента MAX
 * (requestContact с подписью); оставленный номер и есть согласие показывать его УК по заявкам
 * жителя. Соседи номер не видят.
 */
export function PhoneForRepair({ onToast }: { onToast: (msg: string) => void }) {
  const user = useUser();
  const { setUser } = useSession();
  const [busy, setBusy] = useState(false);

  const share = async () => {
    setBusy(true);
    try {
      const contact = await bridge.requestContact();
      if (!contact) return; // житель отказался в окне MAX: это его выбор, не ошибка
      setUser(await api.sharePhone(contact));
      bridge.hapticSuccess();
      onToast('Телефон сохранён. Его увидит только УК по вашим открытым заявкам.');
    } catch (err) {
      onToast(err instanceof ApiError ? err.message : 'Не получилось получить номер. Попробуйте ещё раз');
    } finally {
      setBusy(false);
    }
  };

  const hide = async () => {
    setBusy(true);
    try {
      setUser(await api.hidePhone());
      onToast('Телефон больше не показывается УК.');
    } catch (err) {
      onToast(err instanceof ApiError ? err.message : 'Не получилось. Попробуйте ещё раз');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Island>
      <div className={s.block}>
        <h3 className={s.blockTitle}>Телефон для мастера</h3>
        {user.phone_shared ? (
          <>
            <p className={s.text}>УК видит ваш телефон по вашим открытым заявкам и может позвонить, если нужно попасть в квартиру.</p>
            <Button variant="secondary" size="medium" stretched loading={busy} onClick={hide}>
              Не показывать телефон
            </Button>
          </>
        ) : (
          <>
            <p className={s.text}>Мастер позвонит, если нужно попасть в квартиру или уточнить детали. Соседи номер не увидят.</p>
            {bridge.canRequestContact() ? (
              <Button variant="secondary" size="medium" stretched iconBefore={<Phone size={18} />} loading={busy} onClick={share}>
                Оставить телефон для мастера
              </Button>
            ) : (
              <p className={s.hint}>Оставить телефон можно в приложении MAX: номер подтверждает сам MAX.</p>
            )}
          </>
        )}
      </div>
    </Island>
  );
}

/** Контакты жителей для сотрудника ответственной УК: имя и телефон тех, кто оставил номер. */
export function ResidentContacts({ contacts }: { contacts: Contact[] }) {
  return (
    <Island>
      <div className={s.block}>
        <h3 className={s.blockTitle}>Контакты жителей</h3>
        <ul className={s.contacts}>
          {contacts.map((c) => {
            const phone = phoneView(c.phone);
            return (
              <li key={c.phone} className={s.contact}>
                <span>{c.first_name || 'Житель'}</span>
                <a className={s.contactPhone} href={phone.href}>
                  <Phone size={16} aria-hidden="true" /> {phone.text}
                </a>
              </li>
            );
          })}
        </ul>
        <p className={s.hint}>Жители оставили телефон сами для связи по заявке. Не передавайте номера третьим лицам.</p>
      </div>
    </Island>
  );
}
