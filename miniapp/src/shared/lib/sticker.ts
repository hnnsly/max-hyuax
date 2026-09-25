// Наклейка с QR-кодом объекта: параметр запуска, матрица QR в SVG-путь, тексты по категории.
import { encode } from 'uqr';

/** «o_<код>» для ?startapp=: MAX принимает до 512 символов [A-Za-z0-9_-]. */
export function objectStartParam(code: string): string | null {
  const param = `o_${code}`;
  return code && param.length <= 512 && /^[A-Za-z0-9_-]+$/.test(param) ? param : null;
}

/**
 * SVG-путь QR-кода: тёмные модули подряд в строке склеены в один прямоугольник.
 * Рамка в 2 модуля входит в размер; рисовать с viewBox="0 0 size size".
 */
export function qrPath(text: string): { size: number; d: string } {
  const { data, size } = encode(text, { ecc: 'M', border: 2 });
  let d = '';
  data.forEach((row, y) => {
    let x = 0;
    while (x < size) {
      if (!row[x]) {
        x++;
        continue;
      }
      const start = x;
      while (x < size && row[x]) x++;
      d += `M${start} ${y}h${x - start}v1h-${x - start}z`;
    }
  });
  return { size, d };
}

/** Строка матрицы QR-кода (строки из 0 и 1 через точку) для векторной отрисовки в PDF на сервере. */
export function qrMatrixString(text: string): string {
  const { data } = encode(text, { ecc: 'M', border: 1 });
  return data.map((row) => row.map((cell) => (cell ? '1' : '0')).join('')).join('.');
}

export interface StickerCopy {
  plate: string; // крупное слово на табличке
  question: string;
  opens: string; // что произойдёт после сканирования
  emergency: string; // когда звонить, а не писать заявку
}

const byCategory: Record<string, { plate: string; question: string; where: string }> = {
  lift: { plate: 'Лифт', question: 'Лифт не работает?', where: 'по этому лифту' },
  lighting: { plate: 'Свет', question: 'Не горит свет?', where: 'по свету в этом подъезде' },
  leak: { plate: 'Кровля', question: 'Протекает крыша?', where: 'по кровле этого дома' },
  garbage: { plate: 'Мусоропровод', question: 'Засор мусоропровода?', where: 'по мусоропроводу этого дома' },
};

/** Тексты наклейки по холсту Sticker; categoryTitle — для категорий без своего текста. */
export function stickerCopy(category: string, categoryTitle: string, inEntrance: boolean): StickerCopy {
  const c = byCategory[category] ?? { plate: categoryTitle, question: 'Что-то сломалось?', where: 'по этому месту' };
  return {
    plate: c.plate,
    question: c.question,
    opens: `В MAX откроется заявка ${c.where}: ${inEntrance ? 'дом и подъезд уже указаны' : 'дом уже указан'}`,
    emergency: category === 'lift' ? 'Застряли в лифте' : 'Авария или затопление',
  };
}
