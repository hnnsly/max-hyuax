// Режим «Крупно и контрастно» (ADR-021): крупный текст, контрастные подписи, большие кнопки.
// Настройка живёт на устройстве; ссылка ?a11y=large включает режим для проверки.

export type A11yMode = 'normal' | 'large';

const KEY = 'dommax.a11y';

type Store = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

function parse(value: string | null | undefined): A11yMode | null {
  return value === 'large' || value === 'normal' ? value : null;
}

/** localStorage или undefined: в приватном режиме и в песочнице даже обращение к нему бросает. */
export function deviceStore(): Store | undefined {
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

/** Режим из ссылки (?a11y=large|normal), иначе сохранённый, иначе обычный. */
export function loadA11y(search: string, store: Store | undefined): A11yMode {
  const fromLink = parse(new URLSearchParams(search).get('a11y'));
  if (fromLink) return fromLink;
  try {
    return parse(store?.getItem(KEY)) ?? 'normal';
  } catch {
    return 'normal';
  }
}

/** Запоминает режим; обычный режим не хранится, чтобы не оставлять лишних записей. */
export function saveA11y(mode: A11yMode, store: Store | undefined): void {
  try {
    if (mode === 'large') store?.setItem(KEY, 'large');
    else store?.removeItem(KEY);
  } catch {
    /* хранилище недоступно: режим действует до закрытия приложения */
  }
}
