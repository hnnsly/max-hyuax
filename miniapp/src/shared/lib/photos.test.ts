import { describe, expect, it } from 'vitest';
import { MAX_PHOTO_BYTES, pickPhotos } from './photos';

const file = (name: string, type: string, size = 1000) => new File([new Uint8Array(size)], name, { type });

describe('выбор фото для заявки', () => {
  it('берёт JPEG и PNG до лимита', () => {
    const r = pickPhotos([], [file('a.jpg', 'image/jpeg'), file('b.png', 'image/png')], 3);
    expect(r.files.map((f) => f.name)).toEqual(['a.jpg', 'b.png']);
    expect(r.error).toBe('');
  });

  it('отклоняет другие форматы и слишком большие файлы с понятной причиной', () => {
    const r = pickPhotos([], [file('c.heic', 'image/heic'), file('d.jpg', 'image/jpeg', MAX_PHOTO_BYTES + 1), file('e.jpg', 'image/jpeg')], 3);
    expect(r.files.map((f) => f.name)).toEqual(['e.jpg']);
    expect(r.error).toBe('Часть файлов не добавлена: нужны фото JPEG или PNG до 5 МБ');
  });

  it('не больше трёх за раз', () => {
    const have = [file('1.jpg', 'image/jpeg'), file('2.jpg', 'image/jpeg')];
    const r = pickPhotos(have, [file('3.jpg', 'image/jpeg'), file('4.jpg', 'image/jpeg')], 3);
    expect(r.files).toHaveLength(3);
    expect(r.error).toBe('Можно добавить не больше 3 фото');
  });
});
