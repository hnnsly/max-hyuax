// Клиент API: JSON, токен сессии, единый формат ошибок {"error": {"code", "message"}}.
import type {
  AppealLink, Category, CategoryHint, DistrictMetrics, GeoPlace, House, HouseDetails, Issue, IssueEvent, AssetObject, Photo, ReportInput, Session, Status, UkMetrics, User,
} from './types';

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

const OFFLINE = 'Нет связи с сервером. Проверьте интернет и попробуйте ещё раз.';

let token = '';
let onUnauthorized: () => void = () => {};

export function setToken(t: string) {
  token = t;
}

/** Вызывается при 401: сессия истекла, приложение показывает вход заново. */
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

/** Запрос с токеном сессии. FormData уходит как есть: границу multipart выставляет браузер. */
async function send(method: string, path: string, body?: unknown): Promise<Response> {
  const form = body instanceof FormData;
  let res: Response;
  try {
    res = await fetch(`/api/v1${path}`, {
      method,
      headers: {
        ...(body !== undefined && !form ? { 'Content-Type': 'application/json' } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: form ? body : body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(0, 'offline', OFFLINE);
  }
  if (!res.ok) {
    const data: unknown = await res.json().catch(() => null);
    const err = (data as { error?: { code?: string; message?: string } } | null)?.error;
    if (res.status === 401) onUnauthorized();
    throw new ApiError(res.status, err?.code ?? 'http_error', err?.message ?? 'Что-то пошло не так. Попробуйте позже');
  }
  return res;
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await send(method, path, body);
  if (res.status === 204) return undefined as T;
  return (await res.json().catch(() => null)) as T;
}

const q = (params: Record<string, string | undefined>) => {
  const s = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v) s.set(k, v);
  return s.toString();
};

export const api = {
  loginMax: (initData: string) => request<Session>('POST', '/auth/max', { init_data: initData }),
  loginDemo: (role: string) => request<Session>('POST', '/auth/demo', { role }),
  me: () => request<User>('GET', '/me'),
  acceptConsent: (version: string) => request<User>('POST', '/me/consent', { version }),
  setHouse: (houseId: string) => request<User>('POST', '/me/house', { house_id: houseId }),
  deleteAccount: () => request<void>('DELETE', '/me'),
  sharePhone: (c: { phone: string; auth_date: string; hash: string }) => request<User>('POST', '/me/phone', c),
  districtMetrics: () => request<DistrictMetrics>('GET', '/district/metrics'),
  districtOverdue: () => request<Issue[]>('GET', '/district/overdue'),
  hidePhone: () => request<User>('DELETE', '/me/phone'),
  myIssues: () => request<Issue[]>('GET', '/me/issues'),
  categories: () => request<Category[]>('GET', '/categories'),
  classify: (text: string) => request<CategoryHint>('POST', '/classify', { text }),
  photos: (issueId: string) => request<Photo[]>('GET', `/issues/${encodeURIComponent(issueId)}/photos`),
  uploadPhotos: (issueId: string, files: File[]) => {
    const form = new FormData();
    for (const f of files) form.append('photo', f);
    return request<Photo[]>('POST', `/issues/${encodeURIComponent(issueId)}/photos`, form);
  },
  /** Фото отдаются только с токеном, поэтому грузятся через fetch, а не <img src>. */
  photoBlob: async (photoId: string) => (await send('GET', `/photos/${encodeURIComponent(photoId)}`)).blob(),
  removePhoto: (photoId: string) => request<void>('DELETE', `/photos/${encodeURIComponent(photoId)}`),
  confirmRepair: (issueId: string) => request<Issue>('POST', `/issues/${encodeURIComponent(issueId)}/confirm`),
  reopenRepair: (issueId: string, comment: string) =>
    request<Issue>('POST', `/issues/${encodeURIComponent(issueId)}/reopen`, { comment }),
  appeal: (issueId: string) =>
    request<AppealLink>('POST', `/issues/${encodeURIComponent(issueId)}/appeal`),
  searchHouses: (query: string) => request<House[]>('GET', `/houses?${q({ query })}`),
  nearestHouses: (lat: number, lon: number) => request<House[]>('GET', `/houses/nearest?${q({ lat: String(lat), lon: String(lon) })}`),
  reverseGeocode: (lat: number, lon: number) => request<GeoPlace>('GET', `/geo/reverse?${q({ lat: String(lat), lon: String(lon) })}`),
  house: (id: string) => request<HouseDetails>('GET', `/houses/${encodeURIComponent(id)}`),
  houseIssues: (id: string) => request<Issue[]>('GET', `/houses/${encodeURIComponent(id)}/issues`),
  objectByCode: (code: string) => request<{ object: AssetObject; house: House }>('GET', `/objects/${encodeURIComponent(code)}`),
  similar: (houseId: string, category: string, objectId?: string) =>
    request<Issue[]>('GET', `/issues/similar?${q({ house_id: houseId, category, object_id: objectId })}`),
  report: (input: ReportInput) => request<Issue>('POST', '/issues', input),
  issue: (id: string) => request<Issue>('GET', `/issues/${encodeURIComponent(id)}`),
  timeline: (id: string) => request<IssueEvent[]>('GET', `/issues/${encodeURIComponent(id)}/timeline`),
  join: (id: string) => request<Issue>('POST', `/issues/${encodeURIComponent(id)}/join`),
  changeStatus: (id: string, status: Status, comment: string) =>
    request<Issue>('POST', `/issues/${encodeURIComponent(id)}/status`, { status, comment }),
  ukQueue: () => request<Issue[]>('GET', '/uk/issues'),
  ukMetrics: () => request<UkMetrics>('GET', '/uk/metrics'),
  ukHouses: () => request<House[]>('GET', '/uk/houses'),
};
