// Отбор фото перед загрузкой: те же ограничения, что проверяет сервер, но с понятным текстом сразу.

export const MAX_PHOTO_BYTES = 5 * 1024 * 1024;
const TYPES = new Set(['image/jpeg', 'image/png']);

/** Добавляет выбранные файлы к уже выбранным: только JPEG и PNG до 5 МБ, не больше max. */
export function pickPhotos(current: File[], added: File[], max: number): { files: File[]; error: string } {
  const ok = added.filter((f) => TYPES.has(f.type) && f.size <= MAX_PHOTO_BYTES);
  const files = [...current, ...ok].slice(0, max);
  let error = '';
  if (ok.length < added.length) error = 'Часть файлов не добавлена: нужны фото JPEG или PNG до 5 МБ';
  else if (current.length + ok.length > max) error = `Можно добавить не больше ${max} фото`;
  return { files, error };
}
