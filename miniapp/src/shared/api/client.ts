// Клиент API: JSON, токен сессии, единый формат ошибок {"error": {"code", "message"}}.
import type {
  Category, CategoryHint, House, HouseDetails, Issue, IssueEvent, AssetObject, ReportInput, Session, Status, UkMetrics, User,
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

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`/api/v1${path}`, {
      method,
      headers: {
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(0, 'offline', OFFLINE);
  }
  if (res.status === 204) return undefined as T;
  const data: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string } } | null)?.error;
    if (res.status === 401) onUnauthorized();
    throw new ApiError(res.status, err?.code ?? 'http_error', err?.message ?? 'Что-то пошло не так. Попробуйте позже');
  }
  return data as T;
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
  myIssues: () => request<Issue[]>('GET', '/me/issues'),
  categories: () => request<Category[]>('GET', '/categories'),
  classify: (text: string) => request<CategoryHint>('POST', '/classify', { text }),
  appeal: (issueId: string) =>
    request<{ url: string; expires_at: string; file_name: string }>('POST', `/issues/${encodeURIComponent(issueId)}/appeal`),
  searchHouses: (query: string) => request<House[]>('GET', `/houses?${q({ query })}`),
  nearestHouses: (lat: number, lon: number) => request<House[]>('GET', `/houses/nearest?${q({ lat: String(lat), lon: String(lon) })}`),
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
