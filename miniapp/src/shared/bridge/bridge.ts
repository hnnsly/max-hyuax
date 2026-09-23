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
  HapticFeedback?: { notificationOccurred(type: 'success' | 'error' | 'warning'): unknown };
}

declare global {
  interface Window {
    WebApp?: WebApp;
  }
}

const wa = (): WebApp | undefined => window.WebApp;
const mobile = () => wa()?.platform === 'ios' || wa()?.platform === 'android';

/** Имя бота: из него собираются ссылки на мини-приложение для шеринга. */
export const BOT_NAME: string = import.meta.env.VITE_BOT_NAME ?? 't105_hakaton_max_bot';

export const appLink = (startParam: string) => `https://max.ru/${BOT_NAME}?startapp=${startParam}`;

export const bridge = {
  /** Открыто ли приложение внутри MAX: там всегда есть подписанный initData. */
  inMax: () => Boolean(wa()?.initData),
  initData: () => wa()?.initData ?? '',

  startParam(): string {
    const fromMax = wa()?.initDataUnsafe?.start_param;
    if (fromMax) return fromMax;
    const params = new URLSearchParams(window.location.search);
    return params.get('WebAppStartParam') ?? params.get('startapp') ?? '';
  },

  backButton: {
    show: () => wa()?.BackButton?.show(),
    hide: () => wa()?.BackButton?.hide(),
    onClick: (cb: () => void) => wa()?.BackButton?.onClick(cb),
    offClick: (cb: () => void) => wa()?.BackButton?.offClick(cb),
  },

  /** Сканер QR есть только в мобильном клиенте MAX. */
  canScan: () => Boolean(wa()?.openCodeReader) && mobile(),
  scan: async () => (await wa()!.openCodeReader!(false)).value,

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

  /** Тактильный отклик есть только на телефонах; в остальных местах молча пропускаем. */
  hapticSuccess() {
    if (mobile()) wa()?.HapticFeedback?.notificationOccurred('success');
  },
};
