// Типы ответов API. Источник истины — api/openapi.yaml; при правке контракта меняются здесь же.

export type Status = 'sent' | 'accepted' | 'in_progress' | 'done' | 'rejected';
export type Role = 'resident' | 'uk_operator';

export interface User {
  id: number;
  first_name: string;
  role: Role;
  house_id?: string;
  organization_id?: string;
  has_consent: boolean;
  consent_version: string;
}

export interface Session {
  token: string;
  expires_at: string;
  user: User;
  start_param?: string;
}

export interface Category {
  code: string;
  title: string;
  responsible: string;
  basis: string;
  business_days: number;
}

export interface Organization {
  id: string;
  type: string;
  name: string;
  phone_office?: string;
  phone_dispatcher?: string;
  phone_emergency?: string;
  schedule?: string;
}

export interface House {
  id: string;
  address: string;
  district?: string;
  year_built?: number;
  floors?: number;
  entrances_count: number;
  organization_id: string;
  source: string;
}

export interface AssetObject {
  id: string;
  house_id: string;
  entrance_id?: string;
  category: string;
  label: string;
  qr_code: string;
}

export interface HouseDetails extends House {
  organization: Organization;
  entrances: { id: string; number: number }[];
  objects: AssetObject[];
}

export interface Issue {
  id: string;
  number: number;
  house_id: string;
  address?: string;
  object_id?: string;
  place?: string;
  category: string;
  category_title: string;
  title: string;
  description: string;
  status: Status;
  status_at: string;
  status_comment?: string;
  created_at: string;
  deadline: string;
  overdue: boolean;
  participant_count: number;
  joined: boolean;
  responsible?: Organization;
  basis?: string;
}

export interface IssueEvent {
  kind: 'created' | 'joined' | 'status_changed' | 'overdue';
  status: Status;
  comment?: string;
  at: string;
}

export interface ReportInput {
  house_id: string;
  object_id?: string;
  category?: string;
  title?: string;
  description?: string;
}

export const isClosed = (s: Status) => s === 'done' || s === 'rejected';

/** Фото к заявке; url отдаёт JPEG только с токеном сессии. */
export interface Photo {
  id: string;
  url: string;
  width: number;
  height: number;
  created_at: string;
}

/** Подписанная ссылка на PDF-обращение: живёт 10 минут и привязана к пользователю. */
export interface AppealLink {
  url: string;
  expires_at: string;
  file_name: string;
}

/** Подсказка категории по тексту; все поля null, если не узнали. */
export interface CategoryHint {
  category: string | null;
  title: string | null;
  source: 'llm' | 'rules' | null;
}

export interface DayMedian {
  date: string; // ГГГГ-ММ-ДД, день подачи по Москве
  median_min: number | null;
}

/** Метрики УК; длительности в минутах, null — нет данных. */
export interface UkMetrics {
  first_response_median_min: number | null;
  prev_week_median_min: number | null;
  first_response_by_day: DayMedian[];
  period_days: number;
  issues_total: number;
  reports_per_issue: number;
  closed_total: number;
  closed_on_time: number;
  open_total: number;
  overdue_open: number;
  sample_data: boolean;
}
