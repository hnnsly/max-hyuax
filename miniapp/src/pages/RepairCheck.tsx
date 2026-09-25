import { Button } from '@maxhub/max-ui';
import { CheckCircle, WarningCircle } from '@phosphor-icons/react';
import { useState, type FormEvent } from 'react';
import { api, ApiError } from '../shared/api/client';
import type { Issue } from '../shared/api/types';
import { bridge } from '../shared/bridge/bridge';
import { dayMonth } from '../shared/lib/format';
import { repairCheck } from '../shared/lib/model';
import { Island } from '../shared/ui/Layout';
import { PhotoSlots } from '../shared/ui/Photos';
import { Sheet } from '../shared/ui/Sheet';
import s from './pages.module.css';

const EMPTY_COMMENT = 'Напишите, что осталось не так';

/**
 * Проверка ремонта жителем в карточке выполненной заявки: «Починили» или «Не починили»
 * с комментарием. Ответить можно один раз и только 7 дней после отметки «выполнено».
 */
export function RepairCheck({ issue, onChanged, onToast }: { issue: Issue; onChanged: (i: Issue) => void; onToast: (msg: string) => void }) {
  const [busy, setBusy] = useState(false);
  const [reopening, setReopening] = useState(false);
  const state = repairCheck(issue, new Date());
  if (!state) return null;

  if (state === 'thanks') {
    return (
      <div className={s.joinedNote}>
        <CheckCircle size={20} weight="fill" aria-hidden="true" /> Вы подтвердили, что починили
      </div>
    );
  }

  const confirm = async () => {
    setBusy(true);
    try {
      onChanged(await api.confirmRepair(issue.id));
      bridge.hapticSuccess();
      onToast('Спасибо. Соседи и УК увидят, что ремонт подтверждён.');
    } catch (err) {
      onToast(err instanceof ApiError ? err.message : 'Не получилось сохранить ответ');
    } finally {
      setBusy(false);
    }
  };

  const reopened = (changed: Issue) => {
    setReopening(false);
    onChanged(changed);
    onToast(`Заявка снова в работе. Новый срок ответа до ${dayMonth(changed.deadline)}.`);
  };

  return (
    <Island>
      <div className={s.block}>
        <h3 className={s.blockTitle}>Проверьте ремонт</h3>
        <p className={s.text}>
          УК отметила заявку выполненной. Если не починили, заявка вернётся в работу. Ответить можно до {dayMonth(issue.answer_until ?? issue.status_at)}.
        </p>
        <div className={s.pair}>
          <Button variant="primary" size="medium" stretched loading={busy} onClick={confirm}>
            Починили
          </Button>
          <Button variant="secondary" size="medium" stretched disabled={busy} onClick={() => setReopening(true)}>
            Не починили
          </Button>
        </div>
      </div>
      <ReopenSheet open={reopening} issueId={issue.id} onClose={() => setReopening(false)} onDone={reopened} />
    </Island>
  );
}

/**
 * Лист «Не починили»: комментарий обязателен. Ошибка поля видна только после попытки ввода
 * или отправки (:user-invalid), aria-invalid синхронизируется для экранных чтецов.
 */
function ReopenSheet({ open, issueId, onClose, onDone }: { open: boolean; issueId: string; onClose: () => void; onDone: (i: Issue) => void }) {
  const [comment, setComment] = useState('');
  const [files, setFiles] = useState<File[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const changed = await api.reopenRepair(issueId, comment.trim());
      if (files.length > 0) {
        await api.uploadPhotos(issueId, files);
      }
      setComment('');
      setFiles([]);
      onDone(changed);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не получилось вернуть заявку в работу');
    } finally {
      setBusy(false);
    }
  };

  const close = () => {
    setError('');
    onClose();
  };

  return (
    <Sheet open={open} title="Не починили" onClose={close} locked={busy}>
      <form className={s.sheetBody} onSubmit={submit}>
        <label className={s.blockTitle} htmlFor="reopen-comment">
          Что осталось не так
        </label>
        <textarea
          id="reopen-comment"
          className={s.sheetText}
          value={comment}
          maxLength={1000}
          required
          aria-describedby="reopen-hint"
          aria-errormessage="reopen-error"
          onChange={(e) => {
            // Одни пробелы не комментарий: поле считается пустым. Читаем validity, а не checkValidity():
            // тот шлёт событие invalid, и обработчик ниже вернул бы фокус в поле.
            e.currentTarget.setCustomValidity(e.currentTarget.value.trim() ? '' : EMPTY_COMMENT);
            if (e.currentTarget.validity.valid) e.currentTarget.removeAttribute('aria-invalid');
            setComment(e.target.value);
          }}
          onBlur={(e) => {
            // Ошибку объявляем после ввода и ухода с поля, как её показывает :user-invalid.
            // Значение нужно явное: пустой aria-invalid экранные чтецы считают за false.
            const field = e.currentTarget;
            if (field.value === '') return;
            if (field.validity.valid) field.removeAttribute('aria-invalid');
            else field.setAttribute('aria-invalid', 'true');
          }}
          onInvalid={(e) => {
            // Приходит только при отправке формы. Своя подпись под полем уже есть: всплывающая
            // подсказка браузера её бы закрыла, поэтому гасим её и сами ставим фокус.
            e.preventDefault();
            e.currentTarget.setAttribute('aria-invalid', 'true');
            e.currentTarget.focus();
          }}
        />
        <p id="reopen-error" className={s.fieldError}>
          <WarningCircle size={16} weight="bold" aria-hidden="true" /> {EMPTY_COMMENT}
        </p>
        <div>
          <div className={s.blockTitle} style={{ marginBottom: 8 }}>
            Фото неисправности (по желанию)
          </div>
          <PhotoSlots files={files} onChange={setFiles} onError={setError} />
        </div>
        <p id="reopen-hint" className={s.hint}>
          Заявка вернётся в работу с новым сроком. Комментарий появится в хронологии заявки без вашего имени.
        </p>
        {error && (
          <p className={s.hint} role="alert">
            {error}
          </p>
        )}
        <Button type="submit" variant="primary" size="large" stretched loading={busy}>
          Вернуть в работу
        </Button>
      </form>
    </Sheet>
  );
}
