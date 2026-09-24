import { Plus, X } from '@phosphor-icons/react';
import { useEffect, useRef, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { Photo } from '../api/types';
import { useResource } from '../api/useResource';
import { pickPhotos } from '../lib/photos';
import { Sheet } from './Sheet';
import s from './ui.module.css';

const ACCEPT = 'image/jpeg,image/png';

/** Кнопка «Добавить»: скрытый input с выбором файлов (в MAX UI загрузки файлов нет). */
function AddTile({ onFiles, disabled }: { onFiles: (files: File[]) => void; disabled?: boolean }) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <>
      <button type="button" className={s.photoAdd} disabled={disabled} onClick={() => input.current?.click()}>
        <Plus size={20} aria-hidden="true" />
        Добавить
      </button>
      <input
        ref={input}
        type="file"
        accept={ACCEPT}
        multiple
        hidden
        onChange={(e) => {
          onFiles([...(e.target.files ?? [])]);
          e.target.value = ''; // тот же файл можно выбрать снова
        }}
      />
    </>
  );
}

/** Превью выбранного, но ещё не загруженного файла. */
function FilePreview({ file, onRemove }: { file: File; onRemove: () => void }) {
  const [url, setUrl] = useState('');
  useEffect(() => {
    const u = URL.createObjectURL(file);
    setUrl(u);
    return () => URL.revokeObjectURL(u);
  }, [file]);
  return (
    <div className={s.photoTile}>
      {url && <img src={url} alt="" />}
      <button type="button" className={s.photoRemove} aria-label="Убрать фото" onClick={onRemove}>
        <X size={14} weight="bold" aria-hidden="true" />
      </button>
    </div>
  );
}

/** Фото в форме заявки (холст Report): до трёх снимков, загружаются после отправки. */
export function PhotoSlots({ files, onChange, onError }: { files: File[]; onChange: (files: File[]) => void; onError: (msg: string) => void }) {
  const max = 3;
  return (
    <div className={s.photoRow}>
      {files.map((f, i) => (
        <FilePreview key={`${f.name}-${f.size}-${i}`} file={f} onRemove={() => onChange(files.filter((_, j) => j !== i))} />
      ))}
      {files.length < max && (
        <AddTile
          onFiles={(added) => {
            const r = pickPhotos(files, added, max);
            onChange(r.files);
            if (r.error) onError(r.error);
          }}
        />
      )}
    </div>
  );
}

/** Загруженное фото: файл отдаётся только с токеном, поэтому берём его через fetch. */
function Thumb({ photo, onOpen }: { photo: Photo; onOpen: (url: string) => void }) {
  const [url, setUrl] = useState('');
  useEffect(() => {
    let alive = true;
    let u = '';
    api.photoBlob(photo.id).then(
      (b) => {
        if (!alive) return;
        u = URL.createObjectURL(b);
        setUrl(u);
      },
      () => {},
    );
    return () => {
      alive = false;
      if (u) URL.revokeObjectURL(u);
    };
  }, [photo.id]);
  return (
    <button type="button" className={s.photoTile} aria-label="Открыть фото" disabled={!url} onClick={() => onOpen(url)}>
      {url && <img src={url} alt="" />}
    </button>
  );
}

/** Фото заявки в карточке: видят участники и УК; добавить можно, пока заявка открыта. */
export function IssuePhotos({ issueId, canAdd, onToast }: { issueId: string; canAdd: boolean; onToast: (msg: string) => void }) {
  const res = useResource(() => api.photos(issueId), [issueId]);
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState('');
  const list = res.data ?? [];
  if (res.error || (!res.data && res.loading) || (list.length === 0 && !canAdd)) return null;

  const upload = async (added: File[]) => {
    const r = pickPhotos([], added, 3);
    if (r.error) onToast(r.error);
    if (r.files.length === 0) return;
    setBusy(true);
    try {
      await api.uploadPhotos(issueId, r.files);
      res.reload();
    } catch (err) {
      onToast(err instanceof ApiError ? err.message : 'Не получилось загрузить фото');
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className={s.photoSection} aria-label="Фото">
      <h3 className={s.photoTitle}>Фото</h3>
      <div className={s.photoRow}>
        {list.map((p) => (
          <Thumb key={p.id} photo={p} onOpen={setOpen} />
        ))}
        {canAdd && list.length < 6 && <AddTile onFiles={upload} disabled={busy} />}
      </div>
      <Sheet open={open !== ''} title="Фото" onClose={() => setOpen('')}>
        {open && <img className={s.photoFull} src={open} alt="Фото к заявке" />}
      </Sheet>
    </section>
  );
}
