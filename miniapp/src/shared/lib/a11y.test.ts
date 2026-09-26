import { describe, expect, it } from 'vitest';
import { loadA11y, saveA11y } from './a11y';

function memoryStore() {
  const data = new Map<string, string>();
  return {
    data,
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
    removeItem: (k: string) => void data.delete(k),
  };
}

const broken = {
  getItem: () => {
    throw new Error('denied');
  },
  setItem: () => {
    throw new Error('denied');
  },
  removeItem: () => {
    throw new Error('denied');
  },
};

describe('режим «Крупно и контрастно»', () => {
  it('по умолчанию обычный', () => {
    expect(loadA11y('', memoryStore())).toBe('normal');
    expect(loadA11y('', undefined)).toBe('normal');
  });

  it('сохраняется и читается; обычный режим запись удаляет', () => {
    const store = memoryStore();
    saveA11y('large', store);
    expect(loadA11y('', store)).toBe('large');
    saveA11y('normal', store);
    expect(store.data.size).toBe(0);
    expect(loadA11y('', store)).toBe('normal');
  });

  it('ссылка важнее сохранённого', () => {
    const store = memoryStore();
    saveA11y('large', store);
    expect(loadA11y('?demo=resident&a11y=normal', store)).toBe('normal');
    expect(loadA11y('?a11y=large', memoryStore())).toBe('large');
    expect(loadA11y('?a11y=huge', memoryStore())).toBe('normal');
  });

  it('недоступное хранилище не ломает приложение', () => {
    expect(loadA11y('', broken)).toBe('normal');
    expect(() => saveA11y('large', broken)).not.toThrow();
  });
});
