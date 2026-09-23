import { useCallback, useEffect, useState } from 'react';
import { ApiError } from './client';

export interface Resource<T> {
  data?: T;
  error?: ApiError;
  loading: boolean;
  reload(): void;
}

/** Загрузка данных для экрана с состояниями «загрузка», «ошибка», «данные». */
export function useResource<T>(load: () => Promise<T>, deps: readonly unknown[]): Resource<T> {
  const [state, setState] = useState<{ data?: T; error?: ApiError; loading: boolean }>({ loading: true });
  const [tick, setTick] = useState(0);
  const reload = useCallback(() => setTick((t) => t + 1), []);

  useEffect(() => {
    let alive = true;
    setState((s) => ({ ...s, loading: true, error: undefined }));
    load().then(
      (data) => alive && setState({ data, loading: false }),
      (err: unknown) =>
        alive &&
        setState((s) => ({
          ...s,
          loading: false,
          error: err instanceof ApiError ? err : new ApiError(0, 'unknown', 'Что-то пошло не так. Попробуйте позже'),
        })),
    );
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- зависимости передаёт вызывающий
  }, [...deps, tick]);

  return { ...state, reload };
}
