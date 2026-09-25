// Обёртка над MAX Bridge (window.WebApp) с запасными путями для браузера.
// Факты о методах — hack-docs/dev-max/docs/webapps/bridge.md.

type Platform = 'ios' | 'android' | 'desktop' | 'web';

interface WebAppBackButton {
  show(): void;
  hide(): void;
  onClick(cb: () => void): void;
  offClick(cb: () => void): void;
}

interface WebApp {
  initData?: string;
  initDataUnsafe?: { start_param?: string };
  platform?: Platform;
  BackButton?: WebAppBackButton;
  shareMaxContent?(params: { text?: string; link?: string }): Promise<{ status: string }>;
  openLink?(url: string): void;
  openMaxLink?(url: string): void;
  openCodeReader?(fileSelect?: boolean): Promise<{ value: string }>;
  downloadFile?(url: string, fileName: string): Promise<{ status: 'downloading' | 'cancelled' }>;
  requestContact?(): Promise<{ phone: string; authDate: string | number; hash: string }>;
  enableClosingConfirmation?(): void;
  disableClosingConfirmation?(): void;
  DeviceStorage?: {
    setItem?(key: string, value: string): unknown;
    removeItem?(key: string): unknown;
  };
  HapticFeedback?: { notificationOccurred(type: 'success' | 'error' | 'warning'): unknown };
}

declare global {
  interface Window {
    WebApp?: WebApp;
  }
}

const wa = (): WebApp | undefined => window.WebApp;
const inMax = () => Boolean(wa()?.initData);
const mobile = () => wa()?.platform === 'ios' || wa()?.platform === 'android';

/** Имя бота: из него собираются ссылки на мини-приложение для шеринга. */
export const BOT_NAME: string = import.meta.env.VITE_BOT_NAME ?? 't105_hakaton_max_bot';

export const appLink = (startParam: string) => `https://max.ru/${BOT_NAME}?startapp=${startParam}`;

export const bridge = {
  /** Открыто ли приложение внутри MAX: там всегда есть подписанный initData. */
  inMax,
  initData: () => wa()?.initData ?? '',

  startParam(): string {
    const fromMax = wa()?.initDataUnsafe?.start_param;
    if (fromMax) return fromMax;
    const params = new URLSearchParams(window.location.search);
    return params.get('WebAppStartParam') ?? params.get('startapp') ?? '';
  },

  // Вне MAX у скрипта Bridge нет транспорта: вызовы только сыплют предупреждениями, пропускаем их.
  backButton: {
    show(): void {
      if (inMax()) wa()?.BackButton?.show();
    },
    hide(): void {
      if (inMax()) wa()?.BackButton?.hide();
    },
    onClick(cb: () => void): void {
      if (inMax()) wa()?.BackButton?.onClick(cb);
    },
    offClick(cb: () => void): void {
      if (inMax()) wa()?.BackButton?.offClick(cb);
    },
  },

  /** Номер телефона отдаёт только клиент MAX (requestContact); в браузере его взять неоткуда. */
  canRequestContact: () => inMax() && Boolean(wa()?.requestContact),

  /**
   * Запросить номер в нативном окне MAX (dev-max/docs/webapps/bridge.md). null — пользователь
   * отказался; при сетевой ошибке клиента промис отклоняется.
   */
  async requestContact(): Promise<{ phone: string; auth_date: string; hash: string } | null> {
    try {
      const r = await wa()!.requestContact!();
      return { phone: r.phone, auth_date: String(r.authDate), hash: r.hash };
    } catch (err) {
      // Отказ приходит объектом {error: {code}} или экземпляром Error: JSON.stringify(Error) даёт «{}».
      const text = `${JSON.stringify(err ?? '')} ${err instanceof Error ? err.message : ''}`;
      if (text.includes('user_refused')) return null;
      throw err;
    }
  },

  /** Печать из WebView телефона не гарантирована: кнопку показываем на компьютере и в браузере. */
  canPrint: () => !mobile() && typeof window.print === 'function',

  /** Сканер QR есть только в мобильном клиенте MAX. */
  canScan: () => Boolean(wa()?.openCodeReader) && mobile(),
  scan: async () => (await wa()!.openCodeReader!(false)).value,

  /**
   * Открыть ссылку max.ru внутри MAX (dev-max/docs/webapps/bridge.md, openMaxLink): например, чат с ботом.
   * В браузере — новая вкладка.
   */
  openMaxLink(url: string): void {
    const w = wa();
    if (w?.initData && w.openMaxLink) {
      w.openMaxLink(url);
      return;
    }
    window.open(url, '_blank', 'noopener');
  },

  /**
   * Поделиться в чат MAX. Внутри MAX — нативный выбор чата, в браузере — ссылка на шеринг MAX.
   * Вызывать только из обработчика нажатия.
   */
  async share(text: string, link: string): Promise<void> {
    const w = wa();
    if (w?.initData && w.shareMaxContent) {
      await w.shareMaxContent({ text, link });
      return;
    }
    const url = `https://max.ru/:share?text=${encodeURIComponent(`${text}\n${link}`)}`;
    window.open(url, '_blank', 'noopener');
  },

  /**
   * Скачать файл. В MAX ссылки href не скачиваются, нужен downloadFile с абсолютным https-адресом,
   * и MAX проверяет, что перед вызовом было нажатие (dev-max/docs/webapps/bridge.md): вызывать сразу
   * из обработчика, без сетевых запросов перед ним. В браузере — обычная ссылка с атрибутом download.
   */
  async download(path: string, fileName: string): Promise<'downloading' | 'cancelled'> {
    const url = new URL(path, window.location.origin).href;
    const w = wa();
    if (w?.initData && w.downloadFile) {
      return (await w.downloadFile(url, fileName)).status;
    }
    const a = document.createElement('a');
    a.href = url;
    a.download = fileName;
    a.click();
    return 'downloading';
  },

  /** Тактильный отклик есть только на телефонах; в остальных местах молча пропускаем. */
  hapticSuccess() {
    if (mobile()) wa()?.HapticFeedback?.notificationOccurred('success');
  },

  /** Защита от случайного закрытия формы свайпом вниз в MAX (FR-ISSUE-05). */
  closingConfirmation: {
    enable(): void {
      if (inMax()) wa()?.enableClosingConfirmation?.();
    },
    disable(): void {
      if (inMax()) wa()?.disableClosingConfirmation?.();
    },
  },

  /** Черновик новой заявки по дому: localStorage + дублирование в DeviceStorage MAX. */
  draft: {
    load(houseId: string): { category: string; objectId: string; text: string } | null {
      if (!houseId) return null;
      try {
        const raw = localStorage.getItem(`dommax.draft.${houseId}`);
        if (!raw) return null;
        const parsed = JSON.parse(raw) as { category?: string; objectId?: string; text?: string };
        return {
          category: typeof parsed.category === 'string' ? parsed.category : '',
          objectId: typeof parsed.objectId === 'string' ? parsed.objectId : '',
          text: typeof parsed.text === 'string' ? parsed.text : '',
        };
      } catch {
        return null;
      }
    },
    save(houseId: string, d: { category: string; objectId: string; text: string }): void {
      if (!houseId) return;
      const key = `dommax.draft.${houseId}`;
      try {
        if (!d.category && !d.objectId && !d.text.trim()) {
          localStorage.removeItem(key);
          if (inMax()) wa()?.DeviceStorage?.removeItem?.(key);
          return;
        }
        const raw = JSON.stringify(d);
        localStorage.setItem(key, raw);
        if (inMax()) wa()?.DeviceStorage?.setItem?.(key, raw);
      } catch {
        /* хранилище недоступно */
      }
    },
    clear(houseId: string): void {
      if (!houseId) return;
      const key = `dommax.draft.${houseId}`;
      try {
        localStorage.removeItem(key);
        if (inMax()) wa()?.DeviceStorage?.removeItem?.(key);
      } catch {
        /* хранилище недоступно */
      }
    },
  },
};
