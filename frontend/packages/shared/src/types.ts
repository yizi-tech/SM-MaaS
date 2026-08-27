// 与后端 dto / model 对齐的 TypeScript 类型定义

export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data?: T;
}

export interface PageResult<T> {
  total: number;
  page: number;
  size: number;
  items: T[];
}

export interface UserInfo {
  id: number;
  email: string;
  nickname: string;
  avatar?: string;
  role: 'user' | 'admin';
  status: 'active' | 'disabled' | 'suspended';
  balance: string;
  token_credits: number;
  credit_used: number;
  token_alert_threshold: number;
  real_name_status: 'unverified' | 'pending' | 'verified' | 'rejected';
  phone?: string;
  qq?: string;
  created_at: string;
}

export interface LoginResponse {
  token: string;
  user: UserInfo;
}

export interface ModelCatalogEntry {
  id: string;
  provider: string;
  name: string;
  description: string;
  context: string;
  input_price: string;
  output_price: string;
  cache_read_price?: string;
  cache_write_price?: string;
  support_unlimited: boolean;
  unlimited_enabled: boolean;
  features: string[];
  status: string;
}

export interface Plan {
  id: number;
  name: string;
  description: string;
  price: string;
  currency: string;
  duration_days: number;
  rpm: number;
  tpm: number;
  included_tokens: number;
  concurrent_limit: number;
  model_access: string[];
  max_purchase: number;
}

export interface ApiKey {
  id: number;
  key_prefix: string;
  full_key?: string;
  name: string;
  model_access: string[];
  status: string;
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
}

export interface Subscription {
  id: number;
  plan_name: string;
  status: string;
  start_at: string;
  end_at: string;
  auto_renew: boolean;
  price: string;
  used_tokens: number;
  included_tokens: number;
}

export interface BillingRecord {
  id: number;
  request_id: string;
  model: string;
  provider: string;
  tokens_in: number;
  tokens_out: number;
  cached_tokens: number;
  tokens_cache_write?: number;
  cost: string;
  ttft_ms: number;
  duration_ms: number;
  detail?: string;
  billing_type: string;
  created_at: string;
}

export interface Transaction {
  id: number;
  transaction_no: string;
  type: string;
  amount: string;
  balance_before: string;
  balance_after: string;
  payment_method: string;
  status: string;
  description: string;
  created_at: string;
}

export interface TokenPackage {
  id: number;
  name: string;
  description: string;
  tokens: number;
  bonus_tokens: number;
  price: string;
  status: string;
  sort_order: number;
}

export interface ResetCoupon {
  id: number;
  code: string;
  status: 'unused' | 'used';
  note?: string;
  used_at?: string;
  created_at: string;
}

export interface Notification {
  id: number;
  title: string;
  content: string;
  type: string;
  is_read: boolean;
  read_at?: string;
  created_at: string;
}

export interface Invoice {
  id: number;
  amount: string;
  title_type: 'company' | 'personal';
  invoice_type: 'normal' | 'vat';
  title: string;
  tax_no?: string;
  bank_name?: string;
  bank_account?: string;
  address?: string;
  phone?: string;
  email?: string;
  status: 'pending' | 'issued' | 'rejected';
  invoice_no?: string;
  reject_reason?: string;
  remark?: string;
  issued_at?: string;
  created_at: string;
}

export interface InvoiceQuota {
  recharged: string;
  occupied: string;
  quota: string;
}

export interface CreditStatus {
  consumed_total: string;
  threshold: number;
  can_apply: boolean;
  application?: CreditApplication;
  credit_limit: number;
  credit_used: number;
  credit_available: number;
}

export interface CreditApplication {
  id: number;
  status: 'pending' | 'approved' | 'rejected';
  granted_tokens: number;
  reject_reason?: string;
  consumed_total: string;
  created_at: string;
  reviewed_at?: string;
}

export interface IdentityVerification {
  id: number;
  real_name?: string;
  status: string;
  reject_reason?: string;
  created_at?: string;
  updated_at?: string;
}

export interface ConversationLog {
  id: number;
  user_id: number;
  api_key_id?: number;
  request_id: string;
  model: string;
  messages: string;
  response: string;
  tokens_in: number;
  tokens_out: number;
  tokens_cached: number;
  cost: string;
  stream: boolean;
  status: string;
  created_at: string;
}

export interface Feedback {
  id: number;
  type: 'bug' | 'suggestion' | 'other';
  title: string;
  content: string;
  contact?: string;
  status: 'pending' | 'processing' | 'resolved' | 'closed';
  admin_note?: string;
  resolved_at?: string;
  created_at: string;
  updated_at: string;
}

// ---------- GUI 联动 ----------

export interface GuiSubscriptionView {
  id: number;
  plan_name: string;
  status: string;
  start_at: string;
  end_at: string;
  auto_renew: boolean;
  price: string;
  included_tokens: number;
  used_tokens: number;
  remaining_tokens: number;
  rpm: number;
  tpm: number;
  concurrent_limit: number;
  model_access: string[];
}

export interface GuiQuota {
  server_time: number;
  subscription?: GuiSubscriptionView;
  token_credits: number;
  balance: string;
  credit: { limit: number; used: number; available: number };
}

export interface GuiModelView {
  id: string;
  provider: string;
  name: string;
  context: string;
  input_price_per_m: string;
  output_price_per_m: string;
  cache_read_price_per_m: string;
  features: string[];
  status: string;
}

export interface GuiSession {
  user: UserInfo;
  quota: GuiQuota;
  models: { default_model: string; models: GuiModelView[] };
}

// ---------- 管理端 ----------

export interface LLMChannel {
  id: number;
  name: string;
  type: 'openai' | 'anthropic';
  base_url: string;
  api_key?: string;
  models: string[];
  priority: number;
  enabled: boolean;
  remark?: string;
  created_at: string;
}

export interface PricingGroup {
  id: number;
  name: string;
  multiplier: string;
  models: string[];
  enabled: boolean;
  remark?: string;
  created_at: string;
}

export interface ModelPrice {
  id: number;
  model: string;
  input_price: string;
  output_price: string;
  cache_read_price?: string;
  cache_write_price?: string;
  enabled: boolean;
  remark?: string;
  support_unlimited: boolean;
  unlimited_enabled: boolean;
  created_at: string;
}

export interface UserDetail extends UserInfo {
  last_login_at?: string;
  last_login_ip?: string;
  updated_at: string;
  subscriptions: Subscription[];
  api_keys: ApiKey[];
}

export interface IdentityVerificationAdmin {
  id: number;
  user_id: number;
  real_name: string;
  id_number: string;
  id_card_front: string;
  id_card_back: string;
  status: string;
  reject_reason?: string;
  created_at: string;
  user?: UserInfo;
}

export interface SystemConfig {
  id: number;
  key: string;
  value: string;
  group: string;
  created_at: string;
  updated_at: string;
}

export interface SiteConfig {
  site_name?: string;
  site_url?: string;
  site_logo?: string;
  site_description?: string;
  site_icp?: string;
  site_footer?: string;
  legal_terms?: string;
  legal_privacy?: string;
}

export interface SystemLog {
  id: number;
  level: string;
  module: string;
  action: string;
  user_id?: number;
  ip?: string;
  request_id: string;
  message: string;
  details?: string;
  created_at: string;
}

export interface SystemMetrics {
  id: number;
  timestamp: string;
  total_requests: number;
  success_requests: number;
  failed_requests: number;
  total_tokens: number;
  avg_latency_ms: number;
  active_users: number;
  active_keys: number;
  revenue: number;
  created_at: string;
}

export interface AnalyticsOverview {
  total_users: number;
  active_users: number;
  total_revenue: string;
  today_requests: number;
  active_subscriptions: number;
  today_revenue: string;
  today_new_users: number;
  pending_verifications: number;
  total_requests: number;
}

export interface RevenueAnalyticsItem {
  date: string;
  amount: string;
  count: number;
}

export interface DailyAnalyticsItem {
  date: string;
  revenue: string;
  requests: number;
  new_users: number;
  new_subs: number;
}

export interface SystemHealth {
  database: boolean;
  redis: boolean;
  api: boolean;
}

export interface CreditCollection {
  user_id: number;
  email: string;
  nickname: string;
  credit_used: number;
  credit_limit: number;
}
