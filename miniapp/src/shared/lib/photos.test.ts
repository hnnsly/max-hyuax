import { describe, expect, it } from 'vitest';
import { fitSize, MAX_PHOTO_BYTES, pickPhotos } from './photos';

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

  it('текст о лимите можно задать: в карточке считается остаток до 6 фото', () => {
    const r = pickPhotos([], [file('1.jpg', 'image/jpeg'), file('2.jpg', 'image/jpeg'), file('3.jpg', 'image/jpeg')], 2, 'У заявки уже 4 фото из 6');
    expect(r.files).toHaveLength(2);
    expect(r.error).toBe('У заявки уже 4 фото из 6');
  });
});

describe('размер фото перед загрузкой', () => {
  it('длинная сторона ужимается до предела с сохранением пропорций', () => {
    expect(fitSize(4000, 3000, 1600)).toEqual({ width: 1600, height: 1200 });
    expect(fitSize(3000, 4000, 1600)).toEqual({ width: 1200, height: 1600 });
    expect(fitSize(4033, 3025, 1600)).toEqual({ width: 1600, height: 1200 });
  });

  it('маленькие снимки не растягиваются', () => {
    expect(fitSize(1000, 800, 1600)).toEqual({ width: 1000, height: 800 });
  });
});
