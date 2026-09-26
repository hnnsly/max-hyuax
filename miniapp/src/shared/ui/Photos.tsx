import { Button } from '@maxhub/max-ui';
import { Plus, X } from '@phosphor-icons/react';
import { useEffect, useRef, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { Photo } from '../api/types';
import { useResource } from '../api/useResource';
import { fitSize, PHOTO_SIDE, pickPhotos } from '../lib/photos';
import { Sheet } from './Sheet';
import s from './ui.module.css';

const ACCEPT = 'image/jpeg,image/png';
const ISSUE_MAX = 6; // столько фото сервер хранит у одной заявки

/**
 * Ужимает снимок на телефоне до PHOTO_SIDE и перекодирует в JPEG: снимки камер бывают больше 5 МБ,
 * а метаданные с геопозицией не уходят с устройства. Если браузер не смог декодировать файл,
 * возвращаем как есть: pickPhotos объяснит, что не так.
 */
async function shrinkPhoto(file: File): Promise<File> {
  if (typeof createImageBitmap !== 'function') return file;
  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
  } catch {
    return file;
  }
  const { width, height } = fitSize(bitmap.width, bitmap.height, PHOTO_SIDE);
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  canvas.getContext('2d')?.drawImage(bitmap, 0, 0, width, height);
  bitmap.close();
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.85));
  if (!blob) return file;
  return new File([blob], `${file.name.replace(/\.[^.]*$/, '') || 'photo'}.jpg`, { type: 'image/jpeg' });
}

/** Кнопка «Добавить»: скрытый input с выбором файлов (в MAX UI загрузки файлов нет). */
function AddTile({ onFiles, disabled }: { onFiles: (files: File[]) => void; disabled?: boolean }) {
  const input = useRef<HTMLInputElement>(null);
  const [preparing, setPreparing] = useState(false);
  return (
    <>
      <button
        type="button"
        className={s.photoAdd}
        disabled={disabled || preparing}
        aria-label="Добавить фото"
        onClick={() => input.current?.click()}
      >
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
          const picked = [...(e.target.files ?? [])];
          e.target.value = ''; // тот же файл можно выбрать снова
          if (picked.length === 0) return;
          setPreparing(true);
          Promise.all(picked.map(shrinkPhoto))
            .then(onFiles)
            .finally(() => setPreparing(false));
        }}
      />
    </>
  );
}

/** Превью выбранного, но ещё не загруженного файла. */
function FilePreview({ file, n, onRemove }: { file: File; n: number; onRemove: () => void }) {
  const [url, setUrl] = useState('');
  useEffect(() => {
    const u = URL.createObjectURL(file);
    setUrl(u);
    return () => URL.revokeObjectURL(u);
  }, [file]);
  return (
    <div className={s.photoTile}>
      {url && <img src={url} alt="" />}
      <button type="button" className={s.photoRemove} aria-label={`Убрать фото ${n}`} onClick={onRemove}>
        <X size={14} weight="bold" aria-hidden="true" />
      </button>
    </div>
  );
}

/** Фото в форме заявки (холст Report): до трёх снимков, загружаются после отправки. */
export function PhotoSlots({ files, onChange, onError }: { files: File[]; onChange: (files: File[]) => void; onError: (msg: string) => void }) {
  const max = 3;
  return (
    <div>
      <div className={s.photoRow}>
        {files.map((f, i) => (
          <FilePreview key={`${f.name}-${f.size}-${i}`} file={f} n={i + 1} onRemove={() => onChange(files.filter((_, j) => j !== i))} />
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
      <p className={s.hint} style={{ marginTop: 6 }}>
        Можно прикрепить до 3 фото (JPEG или PNG до 5 МБ)
      </p>
    </div>
  );
}

/** Загруженное фото: файл отдаётся только с токеном, поэтому берём его через fetch. */
function Thumb({ photo, label, onOpen }: { photo: Photo; label: string; onOpen: (url: string, photo: Photo) => void }) {
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
    <button type="button" className={s.photoTile} aria-label={label} disabled={!url} onClick={() => onOpen(url, photo)}>
      {url && <img src={url} alt="" />}
    </button>
  );
}

/** Фото заявки в карточке: видят участники и УК; добавить можно, пока заявка открыта (УК — и после «выполнено»). */
export function IssuePhotos({
  issueId,
  reloadKey,
  canAdd,
  onToast,
}: {
  issueId: string;
  reloadKey?: string;
  canAdd: boolean;
  onToast: (msg: string) => void;
}) {
  const res = useResource(() => api.photos(issueId), [issueId, reloadKey]);
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState<{ url: string; photo: Photo } | null>(null);
  const [removing, setRemoving] = useState(false);
  const list = res.data ?? [];
  if (res.error || (!res.data && res.loading) || (list.length === 0 && !canAdd)) return null;

  // За раз не больше трёх и не больше, чем осталось мест у заявки.
  const left = ISSUE_MAX - list.length;
  const upload = async (added: File[]) => {
    const r = pickPhotos([], added, Math.min(3, left), left < 3 ? `У заявки уже ${list.length} фото из ${ISSUE_MAX}` : undefined);
    if (r.error) onToast(r.error);
    if (r.files.length === 0) return;
    setBusy(true);
    try {
      // Сервер сохраняет пачку целиком или ничего: после ошибки список не меняется.
      await api.uploadPhotos(issueId, r.files);
      res.reload();
      onToast(r.files.length > 1 ? 'Фотографии добавлены к заявке' : 'Фото добавлено к заявке');
    } catch (err) {
      onToast(err instanceof ApiError ? err.message : 'Не получилось загрузить фото');
    } finally {
      setBusy(false);
    }
  };

  // Убрать можно только своё фото: сервер проверяет это же.
  const remove = async () => {
    if (!open) return;
    setRemoving(true);
    try {
      await api.removePhoto(open.photo.id);
      setOpen(null);
      onToast('Фото убрано');
      res.reload();
    } catch (err) {
      onToast(err instanceof ApiError ? err.message : 'Не получилось убрать фото');
    } finally {
      setRemoving(false);
    }
  };

  return (
    <section className={s.photoSection} aria-label="Фото">
      <h3 className={s.photoTitle}>Фото</h3>
      <div className={s.photoRow}>
        {list.map((p, i) => (
          <Thumb key={p.id} photo={p} label={`Открыть фото ${i + 1} из ${list.length}`} onOpen={(url, photo) => setOpen({ url, photo })} />
        ))}
        {canAdd && left > 0 && <AddTile onFiles={upload} disabled={busy} />}
      </div>
      <Sheet open={open !== null} title="Просмотр фото" onClose={() => setOpen(null)} locked={removing}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {open && <img className={s.photoFull} src={open.url} alt="Фото к заявке" />}
          {open?.photo.mine && (
            <Button variant="secondary" size="medium" stretched loading={removing} onClick={remove}>
              Убрать фото
            </Button>
          )}
        </div>
      </Sheet>
    </section>
  );
}
