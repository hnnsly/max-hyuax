// Типы ответов API. Источник истины — api/openapi.yaml; при правке контракта меняются здесь же.

export type Status = 'sent' | 'accepted' | 'in_progress' | 'done' | 'rejected';
export type Role = 'resident' | 'uk_operator' | 'district';

export interface User {
  id: number;
  first_name: string;
  role: Role;
  house_id?: string;
  organization_id?: string;
  /** Для роли района: какой район он смотрит. */
  district?: string;
  has_consent: boolean;
  consent_version: string;
  /** Житель оставил телефон для мастера; сам номер не приходит. */
  phone_shared: boolean;
  /** Житель председатель совета своего дома: у него есть папка предложений. */
  chairman?: boolean;
  /** На сервере включена роль для проверки: показывать переключатель. */
  role_switch?: boolean;
}

/** Участник заявки, оставивший телефон; видит только сотрудник ответственной УК. */
export interface Contact {
  first_name: string;
  phone: string;
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

export type ProposalStatus = 'new' | 'accepted' | 'declined';

/** Предложение председателю совета. Автора нет в ответе ни для кого. */
export interface Proposal {
  id: string;
  text: string;
  status: ProposalStatus;
  answer?: string;
  created_at: string;
  answered_at?: string;
}

/** Опрос дома без юридической силы; my_vote — номер варианта жителя или null. */
export interface Poll {
  id: string;
  proposal_id?: string;
  question: string;
  options: { text: string; votes: number }[];
  total: number;
  my_vote: number | null;
  open: boolean;
  created_at: string;
  closes_at: string;
}

/** Адрес точки по данным OpenStreetMap; без адреса геокодер выключен или ничего не нашёл. */
export interface GeoPlace {
  address?: string;
  attribution?: string;
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
  /** Проверка ремонта: сколько участников подтвердили текущее «выполнено». */
  confirmed_count: number;
  /** Ответ текущего пользователя на текущее «выполнено». */
  my_answer: 'fixed' | null;
  /** Оценка ремонта текущим пользователем 1–5; нет, если не оценивал. */
  my_rating?: number;
  /** До какого момента можно подтвердить или вернуть; null, если заявка не выполнена. */
  answer_until: string | null;
  /** Когда жители в последний раз вернули заявку в работу. */
  reopened_at?: string;
  /** Только в карточке и только для сотрудника ответственной УК. */
  contacts?: Contact[];
}

export interface IssueEvent {
  kind: 'created' | 'joined' | 'status_changed' | 'overdue' | 'confirmed' | 'reopened';
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
  /** Фото приложил текущий пользователь: его можно убрать. */
  mine: boolean;
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

/** УК в сравнении по району; длительности в минутах, null — нет данных. */
export interface DistrictOrg {
  id: string;
  name: string;
  first_response_median_min: number | null;
  issues_total: number;
  open_total: number;
  overdue_open: number;
  closed_total: number;
  closed_on_time: number;
  confirmed_by_residents: number;
  reopened_by_residents: number;
  /** Средняя оценка ремонтов жителями 1–5; null — оценок нет. */
  rating_avg: number | null;
  ratings: number;
  sample_data: boolean;
}

/** Кабинет района: УК района за период. */
export interface DistrictMetrics {
  district: string;
  period_days: number;
  sample_data: boolean;
  organizations: DistrictOrg[];
}

/** Подписи под коллективным обращением в жилинспекцию (ADR-023). */
export interface AppealSummary {
  count: number;
  mine: boolean;
  /** Текущий житель подписал с ФИО. */
  named: boolean;
}

/** УК в рейтинге района (ADR-022); score null — мало данных. */
export interface RatingOrg {
  id: string;
  name: string;
  score: number | null;
  rating_avg: number | null;
  ratings: number;
  closed_total: number;
  closed_on_time: number;
  confirmed_by_residents: number;
  overdue_open: number;
}

/** Рейтинг УК района для жителей. */
export interface DistrictRating {
  district: string;
  period_days: number;
  my_org_id?: string;
  sample_data: boolean;
  organizations: RatingOrg[];
}

/** Дом на карте кабинета УК или района: открытые и просроченные заявки. */
export interface MapHouse {
  id: string;
  address: string;
  lat: number;
  lon: number;
  open: number;
  overdue: number;
  max_overdue_days: number; // 0 — просрочек нет
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
  /** Из выполненных за период жители подтвердили ремонт. */
  confirmed_by_residents: number;
  /** Жители вернули в работу за период. */
  reopened_by_residents: number;
  open_total: number;
  overdue_open: number;
  /** Средняя оценка ремонтов жителями 1–5; null — оценок нет. */
  rating_avg: number | null;
  ratings: number;
  sample_data: boolean;
}

/** Плановые работы и отключения в доме (GEN_V4, ADR-024). */
export interface MaintenanceAlert {
  id: string;
  house_id: string;
  address?: string;
  category: string;
  category_title: string;
  title: string;
  description?: string;
  starts_at: string;
  ends_at: string;
  created_at: string;
}

/** Должностное лицо УК, ведущее личный приём граждан (ПП РФ № 416 п. 28). */
export interface Specialist {
  code: string;
  title: string;
  description: string;
  schedule: string;
}

/** Запись жителя на личный приём в УК. */
export interface Appointment {
  id: string;
  house_id: string;
  address?: string;
  user_name?: string;
  specialist: string;
  specialist_title: string;
  topic: string;
  slot_at: string;
  status: 'booked' | 'cancelled' | 'completed';
  created_at: string;
}
