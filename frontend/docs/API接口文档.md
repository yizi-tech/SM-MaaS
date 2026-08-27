# MASS 平台前端 API 接口文档

> 基础路径：`/api/v1`（除 LLM 网关兼容接口为 `/v1`）。鉴权：除公开接口外均需请求头 `Authorization: Bearer <JWT>`；管理端还需 `role=admin`。
> 前端视觉层已建立共享企业主题 Token（`packages/shared/src/theme.ts`），不会改变以下 API 路径、请求体、响应结构或鉴权规则。

## 1. 公开接口

### 1.1 认证 `/auth`

| 方法 | 路径 | 说明 | 请求体 |
|------|------|------|--------|
| POST | `/auth/register` | 注册 | `{email, password(≥6), nickname, verify_code, verify_method(默认email), phone?}` |
| POST | `/auth/login` | 登录 | `{email, password}` → `{token, user}` |
| POST | `/auth/send-code` | 发送验证码 | `{method: email\|sms, email?, phone?}` |
| GET | `/auth/openid/config` | 亦 OpenID 是否启用 | → `{enabled, server, redirect_uri}` |
| GET | `/auth/openid/authorize?intent=login\|bind` | 302 跳转 OAuth 授权页 | — |
| GET | `/auth/openid/callback?code=&state=` | OAuth 回调 | 登录：302 → `/user?oauth_token=<jwt>`；绑定：302 → `/user?oauth_bound=1` |

### 1.2 其他公开

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/plans` | 可用套餐列表 `Plan[]` |
| GET | `/models` | 模型市场（已定价且渠道可用的模型）`ModelCatalogEntry[]` |
| GET | `/site-config` | 站点品牌配置 `{site_name, site_logo, site_description, site_icp, site_footer, ...}` |
| GET | `/health` | 健康检查 |

## 2. 用户端 `/user`（JWT）

### 2.1 资料与安全

| 方法 | 路径 | 说明 | 请求体 |
|------|------|------|--------|
| GET | `/user/profile` | 我的资料 `UserInfo` | — |
| PUT | `/user/profile` | 更新资料 | `{nickname, phone, qq, avatar}` |
| PUT | `/user/password` | 修改密码（需验证码） | `{old_password, new_password, verify_method, verify_code}` |
| POST | `/user/password/send-code` | 发送改密验证码 | `{method, email?, phone?}` |
| POST | `/user/upload` | 上传图片（multipart `file`，≤5MB） | → `{url}` |

### 2.2 API Keys

| 方法 | 路径 | 说明 | 请求体 |
|------|------|------|--------|
| GET | `/user/api-keys` | Key 列表 `ApiKey[]` | — |
| POST | `/user/api-keys` | 创建 Key | `{name, model_access[]}` → `{full_key}`（仅此一次） |
| DELETE | `/user/api-keys/:id` | 删除 Key | — |

### 2.3 用量与账单

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/user/usage?start=&end=` | 用量汇总 `{start,end,total_tokens,total_cost,daily:[{date,tokens,cost}]}` |
| GET | `/user/billing-records?page=&size=` | 计费明细（分页） |
| GET | `/user/billing-records/:id` | 单条计费详情 |
| GET | `/user/transactions?page=&size=` | 交易流水（分页） |

### 2.4 充值（易支付）

| 方法 | 路径 | 说明 | 请求体 |
|------|------|------|--------|
| GET | `/user/payment-config` | 可用支付方式 `{methods:["balance","epay"]}` | — |
| POST | `/user/recharge/epay` | 创建充值订单 | `{amount(¥1-50000), pay_type: alipay\|wxpay\|qqpay}` → `{out_trade_no, pay_url, amount}` |
| GET | `/user/recharge/epay/status?out_trade_no=` | 订单本地状态 `{status}` | — |
| POST | `/user/recharge/epay/query` | 主动查询网关并入账 | `{out_trade_no}` → `{status}` |

### 2.5 套餐 / 订阅 / 加油包 / 重置券

| 方法 | 路径 | 说明 | 请求体 |
|------|------|------|--------|
| POST | `/user/subscribe` | 开通订阅（扣余额） | `{plan_id}` |
| GET | `/user/subscriptions` | 我的订阅 `Subscription[]` | — |
| POST | `/user/subscriptions/:id/cancel` | 取消订阅 | — |
| GET | `/user/token-packages` | 加油包列表 | — |
| POST | `/user/token-packages/:id/purchase` | 余额购买加油包 | → `{transaction, token_credits}` |
| GET | `/user/reset-coupons` | 我的重置券 | — |
| POST | `/user/reset-coupons/:id/redeem` | 兑换（重置已用额度） | → `{coupon_id, reset_count}` |

### 2.6 通知 / 发票 / 授信

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/user/notifications?page=&size=` | 通知列表 `{items,total,page,size}` |
| GET | `/user/notifications/unread-count` | 未读数 `{unread}` |
| PUT | `/user/notifications/:id/read` | 标记已读 |
| PUT | `/user/notifications/read-all` | 全部已读 |
| GET | `/user/invoice-quota` | 开票额度 `{recharged,occupied,quota}` |
| POST | `/user/invoices` | 申请发票 `{amount,title_type(company\|personal),invoice_type(normal\|vat),title,tax_no,bank_name,bank_account,address,phone,email,remark}` |
| GET | `/user/invoices?page=&size=` | 我的发票 |
| GET | `/user/credit/status` | 授信状态 `{consumed_total,threshold,can_apply,application,credit_limit,credit_used,credit_available}` |
| POST | `/user/credit/apply` | 申请授信（累计消费≥¥5000） |
| POST | `/user/credit/repay` | 授信还款 `{tokens}` |

### 2.7 实名 / 对话 / 反馈 / OpenID

| 方法 | 路径 | 说明 | 请求体 |
|------|------|------|--------|
| POST | `/user/identity-verification` | 提交实名 | `{real_name, id_number, id_card_front, id_card_back}` |
| GET | `/user/identity-verification` | 实名状态 `{status, reject_reason, ...}` | — |
| GET | `/user/conversations?page=&size=&model=` | 对话记录 `{items,total,models}` | — |
| GET | `/user/conversations/:id` | 对话详情 | — |
| GET | `/user/conversations/export.jsonl` | 导出 JSONL（下载） | — |
| POST | `/user/feedback` | 提交反馈 | `{type: bug\|suggestion\|other, title, content, contact}` |
| GET | `/user/feedback?page=&size=` | 我的反馈 | — |
| GET | `/user/feedback/:id` | 反馈详情 | — |
| GET | `/user/openid/status` | OpenID 绑定状态 | — |
| POST | `/user/openid/bind` | 绑定（302 跳转授权） | — |
| POST | `/user/openid/unbind` | 解绑 | — |

## 3. GUI 桌面端联动接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/gui/login` | 登录+会话快照 `{token,user,quota,models:{default_model,models[]}}` |
| GET | `/gui/session` | 会话快照（含额度/模型） |
| GET | `/gui/sync` | 毫秒级额度同步 `{server_time,subscription,token_credits,balance,credit}` |
| GET | `/gui/models` | 可用模型列表 |

> `quota.subscription`：`{id,plan_name,status,start_at,end_at,auto_renew,price,included_tokens,used_tokens,remaining_tokens,rpm,tpm,concurrent_limit,model_access}`
> `quota.credit`：`{limit, used, available}`

## 4. 管理端 `/admin`（JWT + admin）

### 4.1 用户 / 实名

| 方法 | 路径 | 说明 | 请求体 |
|------|------|------|--------|
| GET | `/admin/users?page=&size=&keyword=` | 用户列表 `UserInfo[]`（分页） | — |
| GET | `/admin/users/:id` | 用户详情 `{...,subscriptions,api_keys}` | — |
| PUT | `/admin/users/:id` | 更新用户 `{nickname,phone,role,status,real_name_status,balance,balance_adjust,balance_note}` | — |
| PUT | `/admin/users/:id/status` | 更新状态 `{status}` | — |
| GET | `/admin/identity-verifications?page=&size=&status=` | 实名申请列表 | — |
| POST | `/admin/identity-verifications/:id/review` | 审核 `{action:approve\|reject, reason}` | — |

### 4.2 渠道 / 定价

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/admin/channels` | 渠道列表 / 新增 `{name,type(openai\|anthropic),base_url,api_key,models[],priority,enabled,remark}` |
| POST | `/admin/channels/test` | 渠道连通性测试 `{type,base_url,api_key,model}` |
| PUT/DELETE | `/admin/channels/:id` | 更新 / 删除 |
| GET/POST | `/admin/pricing-groups` | 定价分组列表 / 新增 `{name,multiplier,models[],enabled,remark}` |
| PUT/DELETE | `/admin/pricing-groups/:id` | 更新 / 删除 |
| GET/POST | `/admin/model-prices` | 价格表（¥/百万token）/ 新增 `{model,input_price,output_price,cache_read_price,cache_write_price,enabled,remark}` |
| PUT/DELETE | `/admin/model-prices/:id` | 更新 / 删除 |

### 4.3 套餐 / 订单 / 对话

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/admin/plans` | 套餐列表 / 新增 `{name,description,price,currency,duration_days,rpm,tpm,included_tokens,concurrent_limit,model_access,sort_order}` |
| PUT/DELETE | `/admin/plans/:id` | 更新 / 删除 |
| GET | `/admin/orders?page=&size=&type=&status=&user_id=` | 订单流水（分页） |
| GET | `/admin/conversations?page=&size=&model=&status=&user_id=` | 全量调用记录 `{items,total,models}` |
| GET | `/admin/conversations/:id` | 调用详情 |
| GET | `/admin/conversations/export.jsonl` | 导出 JSONL |

### 4.4 发票 / 授信 / 催账

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/invoices?page=&size=&status=` | 发票列表 |
| POST | `/admin/invoices/:id/issue` | 开具 `{invoice_no}` |
| POST | `/admin/invoices/:id/reject` | 驳回 `{reason}` |
| GET | `/admin/credit-applications?page=&size=&status=` | 授信申请列表 |
| POST | `/admin/credit-applications/:id/approve` | 通过 `{granted_tokens}` |
| POST | `/admin/credit-applications/:id/reject` | 驳回 `{reason}` |
| GET | `/admin/credit-collections?page=&size=` | 催账列表（待还用户） |
| POST | `/admin/credit-collect` | 发起催收 `{user_id, note?}` |

### 4.5 重置券 / 通知 / 反馈

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/reset-coupons` | 已发放重置券 |
| POST | `/admin/reset-coupons` | 发放 `{user_id(0=全员), count(1-10), note}` |
| GET/POST | `/admin/notifications` | 通知列表 / 发送 `{user_id(0=全员), title, content, type}` |
| GET | `/admin/feedback?page=&size=&status=` | 反馈列表 |
| PUT | `/admin/feedback/:id/status` | 更新反馈 `{status, admin_note}` |

### 4.6 分析 / 系统

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/analytics/overview` | 概览 `{total_users,active_users,total_revenue,today_requests}` |
| GET | `/admin/analytics/revenue?start_date=&end_date=` | 收入趋势 `[{date,amount,count}]` |
| GET | `/admin/config` | 全部系统配置 `SystemConfig[]` |
| PUT | `/admin/config` | 单条更新 `{key,value,group}` |
| PUT | `/admin/config/batch` | 分组批量保存 `{group, items:[{key,value}]}` |
| GET | `/admin/logs?page=&size=&level=&module=&start_date=&end_date=` | 系统日志（分页） |
| GET | `/admin/metrics?start_date=&end_date=` | 系统指标 |
| GET | `/admin/health` | 健康检查 `{database,redis,api}` |

## 5. LLM 网关（API Key 鉴权）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/chat/completions`（及 `/api/v1/llm/chat/completions`） | OpenAI 兼容对话（SSE 流式） |
| POST | `/v1/completions` | OpenAI 补全 |
| GET | `/v1/models` | 可用模型列表 |
| POST | `/v1/messages` | Anthropic Messages 兼容 |

> 鉴权：请求头 `X-API-Key: sk-xxx` 或 `Authorization: Bearer sk-xxx`。

## 6. 通用类型（TypeScript 对齐）

```ts
interface UserInfo {
  id: number; email: string; nickname: string; avatar?: string;
  role: 'user' | 'admin'; status: 'active' | 'disabled' | 'suspended';
  balance: string; token_credits: number; credit_used: number;
  real_name_status: string; phone?: string; qq?: string; created_at: string;
}
interface PageResult<T> { total: number; page: number; size: number; items: T[] }
interface ModelCatalogEntry {
  id: string; provider: string; name: string; description: string; context: string;
  input_price: string; output_price: string; cache_read_price?: string;
  cache_write_price?: string; features: string[]; status: string;
}
interface Plan {
  id: number; name: string; description: string; price: string; currency: string;
  duration_days: number; rpm: number; tpm: number; included_tokens: number;
  concurrent_limit: number; model_access: string[];
}
interface ApiKey {
  id: number; key_prefix: string; full_key?: string; name: string;
  model_access: string[]; status: string; last_used_at?: string; created_at: string;
}
interface Subscription {
  id: number; plan_name: string; status: string; start_at: string; end_at: string;
  auto_renew: boolean; price: string; used_tokens: number; included_tokens: number;
}
interface BillingRecord {
  id: number; request_id: string; model: string; provider: string;
  tokens_in: number; tokens_out: number; cached_tokens: number; cost: string;
  ttft_ms: number; duration_ms: number; detail?: string; billing_type: string; created_at: string;
}
interface Transaction {
  id: number; transaction_no: string; type: string; amount: string;
  balance_before: string; balance_after: string; payment_method: string;
  status: string; description: string; created_at: string;
}
interface LLMChannel {
  id: number; name: string; type: string; base_url: string; api_key?: string;
  models: string[]; priority: number; enabled: boolean; remark?: string; created_at: string;
}
interface PricingGroup {
  id: number; name: string; multiplier: string; models: string[];
  enabled: boolean; remark?: string; created_at: string;
}
interface ModelPrice {
  id: number; model: string; input_price: string; output_price: string;
  cache_read_price?: string; cache_write_price?: string;
  enabled: boolean; remark?: string; created_at: string;
}
```
