import { encode } from 'uqr';
import { describe, expect, it } from 'vitest';
import { objectStartParam, qrPath, stickerCopy } from './sticker';

describe('параметр запуска для QR', () => {
  it('код объекта с префиксом o_', () => {
    expect(objectStartParam('h-17k2-e2-lift')).toBe('o_h-17k2-e2-lift');
  });

  it('недопустимые символы и слишком длинный код отклоняются', () => {
    expect(objectStartParam('дом 1')).toBeNull();
    expect(objectStartParam('a/b')).toBeNull();
    expect(objectStartParam('')).toBeNull();
    expect(objectStartParam('x'.repeat(510))).toBe('o_' + 'x'.repeat(510)); // o_ + 510 = 512: предел MAX
    expect(objectStartParam('x'.repeat(511))).toBeNull();
  });
});

describe('QR-код в SVG', () => {
  const text = 'https://max.ru/t105_hakaton_max_bot?startapp=o_h-17k2-e2-lift';

  it('путь закрашивает ровно тёмные модули матрицы', () => {
    const { size, d } = qrPath(text);
    const matrix = encode(text, { ecc: 'M', border: 2 });
    expect(size).toBe(matrix.size);
    const dark = matrix.data.flat().filter(Boolean).length;
    const painted = [...d.matchAll(/h(\d+)v1/g)].reduce((sum, m) => sum + Number(m[1]), 0);
    expect(painted).toBe(dark);
  });

  it('все отрезки внутри матрицы', () => {
    const { size, d } = qrPath(text);
    for (const [, x, y, len] of d.matchAll(/M(\d+) (\d+)h(\d+)/g)) {
      expect(Number(x) + Number(len)).toBeLessThanOrEqual(size);
      expect(Number(y)).toBeLessThan(size);
    }
  });
});

describe('тексты наклейки', () => {
  it('лифт в подъезде', () => {
    const c = stickerCopy('lift', 'Лифт', true);
    expect(c.plate).toBe('Лифт');
    expect(c.question).toBe('Лифт не работает?');
    expect(c.opens).toBe('В MAX откроется заявка по этому лифту: дом и подъезд уже указаны');
    expect(c.emergency).toBe('Застряли в лифте');
  });

  it('объект дома без подъезда и неизвестная категория', () => {
    expect(stickerCopy('leak', 'Протечка', false).opens).toBe('В MAX откроется заявка по кровле этого дома: дом уже указан');
    const other = stickerCopy('door', 'Дверь подъезда и домофон', true);
    expect(other.plate).toBe('Дверь подъезда и домофон');
    expect(other.question).toBe('Что-то сломалось?');
  });

  it('без тире в текстах', () => {
    for (const cat of ['lift', 'lighting', 'leak', 'garbage', 'other']) {
      const c = stickerCopy(cat, 'Другое', true);
      expect(Object.values(c).join(' ')).not.toMatch(/[—–]/);
    }
  });
});
