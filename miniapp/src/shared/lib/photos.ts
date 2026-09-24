// Отбор фото перед загрузкой: те же ограничения, что проверяет сервер, но с понятным текстом сразу.

export const MAX_PHOTO_BYTES = 5 * 1024 * 1024;
/** Длинная сторона снимка после ужатия на телефоне: сервер всё равно хранит не больше. */
export const PHOTO_SIDE = 1600;
const TYPES = new Set(['image/jpeg', 'image/png']);

/**
 * Добавляет выбранные файлы к уже выбранным: только JPEG и PNG до 5 МБ, не больше max.
 * tooMany — свой текст, когда лимит считается не от формы, а от остатка у заявки.
 */
export function pickPhotos(current: File[], added: File[], max: number, tooMany = `Можно добавить не больше ${max} фото`): { files: File[]; error: string } {
  const ok = added.filter((f) => TYPES.has(f.type) && f.size <= MAX_PHOTO_BYTES);
  const files = [...current, ...ok].slice(0, max);
  let error = '';
  if (ok.length < added.length) error = 'Часть файлов не добавлена: нужны фото JPEG или PNG до 5 МБ';
  else if (current.length + ok.length > max) error = tooMany;
  return { files, error };
}

/** Размер после ужатия: длинная сторона не больше max, пропорции сохраняются, маленькие не растут. */
export function fitSize(width: number, height: number, max: number): { width: number; height: number } {
  const k = Math.min(1, max / Math.max(width, height));
  return { width: Math.round(width * k), height: Math.round(height * k) };
}
