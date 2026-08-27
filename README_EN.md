# MASS Platform (Mass AI SaaS Stack)

MASS is an **LLM API gateway + billing control-plane** for teams and businesses that need to expose AI capabilities with usage-based or subscription billing. The backend is built with Go / Gin / GORM / MySQL / Redis / JWT and provides model proxying (OpenAI / Anthropic compatible), token billing, subscriptions / top-up packs / credit lines, top-up payments (Epay / native WeChat Pay / native Alipay), KYC, invoicing, OpenID login, in-app notifications and data retention. The frontend is a React console split into a **user portal** and an **admin console**.

> 中文文档见 [README.md](./README.md)。Commercial licensing is covered in [LICENSE](./LICENSE).

---

## 1. Features

### User Portal (`/` landing, `/user` console)
- Auth: email / SMS-code registration & login, plus OpenID quick login and binding
- Dashboard: balance, token quota (credit line / top-up packs), subscription progress, recent transactions
- Model marketplace with pricing, "Unlimited Firepower" badges and eligibility hints
- API key management (with per-key model access control)
- Usage & billing: trend charts, itemized cost, transaction history, JSONL export
- Top-up center:
  - **Native WeChat Pay / Native Alipay** (QR code + order polling)
  - **Epay** (aggregated QR)
  - Callback amount reconciliation + scheduled gateway reconciliation (tamper-resistant, missed-callback recovery)
- Plans & subscriptions, token top-up packs, reset coupons
- **Low-balance alert**: users set a custom token threshold; an email is sent automatically when below it
- Notifications, invoicing (personal / enterprise, normal / special VAT), token credit lines, KYC
- Conversation history, feedback, personal settings

### Admin Console (`/admin`)
- Overview: users / active users / total revenue / today's requests / revenue trend
- User management (list / detail / edit / balance adjust), KYC review
- LLM channel management (**one-click model fetch** + connectivity test), pricing groups, model price table, plan management
- Model price **sync from channel** (dedupe + default price; disabled by default to avoid ¥0 polluting the marketplace)
- Order history, invoice review, credit-line approval, dunning, coupon issuance
- Notification sending, feedback handling, full conversation search & export
- System config (site branding / OpenID / Epay / **WeChat Pay** / **Alipay** groups; saved values take effect immediately, no restart)

### Payment & Billing Security
- All native callbacks are **signature / encryption verified** (WeChat AES-256-GCM decrypt, Alipay RSA2 verify)
- Both callbacks and proactive queries perform **amount reconciliation**; mismatches are rejected and alerted
- **Scheduled reconciliation**: every 2 minutes the gateway is queried to recover callbacks lost to network / downtime; unpaid orders auto-cancel after 30 minutes
- Idempotent settlement (`CompleteEpayPayment`) — duplicate callbacks never double-credit

---

## 2. Tech Stack

| Layer | Tech |
|------|------|
| Backend | Go / Gin / GORM / MySQL 8 / Redis / JWT / native WeChat Pay v3 / native Alipay RSA2 / Epay |
| Frontend | React 18 / Vite 5 / TypeScript / Ant Design 5 / React Router v6 / Axios / Zustand / dayjs |
| Gateway | OpenAI `/v1/*` & Anthropic compatible, SSE streaming, token-based billing |
| Ops | systemd / optional Docker Compose (MySQL + Redis + API + Nginx) / Nginx reverse proxy with rate limiting |

---

## 3. Project Structure

```
mass/
├── backend/                     # Go backend (Gin + GORM + MySQL + Redis)
│   ├── cmd/server/              # service entrypoint
│   ├── cmd/seed-demo-users/     # demo account seeder (optional)
│   ├── internal/
│   │   ├── api/                 # routes / handlers / middleware / dto
│   │   ├── billing/             # billing core (reconcile / expire / settle)
│   │   ├── llm/                 # LLM channel proxy (OpenAI / Anthropic)
│   │   ├── payment/             # Epay / WeChat / Alipay clients
│   │   ├── repository/          # data access
│   │   ├── service/             # captcha / OpenID etc.
│   │   └── model/               # GORM models
│   └── pkg/                     # shared utils (response, logging)
├── frontend/                    # React + Vite (npm workspaces monorepo)
│   ├── user/                    # user portal build output (static)
│   ├── admin/                   # admin console build output
│   ├── packages/{shared,user-app,admin-app}
│   └── docs/                    # dev plan, API docs
├── docker/                      # Dockerfile + docker-compose.yml + .env.example
├── nginx.conf                   # production Nginx (rate limit / security headers / SSE)
├── scripts/                     # build.sh / package.sh / start-server.sh
├── setup_mass.sql               # legacy Postgres example (NOT for MySQL)
├── docs/                        # design / architecture / API docs
└── README.md / README_EN.md / LICENSE
```

---

## 4. Requirements

| Component | Version / Notes |
|-----------|-----------------|
| Go | 1.22+ (build backend) |
| Node.js | 20.x (build frontend) |
| MySQL | 8.0 (utf8mb4); create empty `mass` DB — **schema auto-migrates on startup** |
| Redis | 7.x, **password required** |
| Nginx | 1.27+ (production proxy, optional) |

> `setup_mass.sql` is an early Postgres example and does **not** match the MySQL deployment. Create the DB with:

```sql
CREATE DATABASE mass CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'mass_user'@'%' IDENTIFIED BY 'your-strong-password';
GRANT ALL PRIVILEGES ON mass.* TO 'mass_user'@'%';
FLUSH PRIVILEGES;
```

---

## 5. Deployment (Production: MySQL + systemd)

### 5.1 Build backend
```bash
cd backend
go build -o ../mass-server ./cmd/server
```

### 5.2 Environment variables
Configured via env (see `backend/internal/config/config.go`). The service **refuses to start** without `JWT_SECRET` / `DB_PASSWORD` / `REDIS_PASSWORD`.

| Var | Default | Notes |
|-----|---------|-------|
| `SERVER_PORT` | 8080 | listen port |
| `SERVER_MODE` | release | run mode |
| `DB_HOST` / `DB_PORT` | localhost / 3306 | MySQL address |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | mass / mass123 / mass | DB credentials |
| `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD` / `REDIS_DB` | localhost / 6379 / "" / 0 | Redis |
| `JWT_SECRET` | "" (**required**, ≥32-byte random) | JWT signing key |
| `MASS_FRONTEND_DIR` | unset | static dir (set to `../frontend` to serve via Gin) |
| `UPLOAD_DIR` | uploads | upload dir |
| `LOG_LEVEL` / `LOG_OUTPUT` | info / stdout | log level / output (file) |
| `OPENAI_BASE_URL` / `OPENAI_API_KEY` | api.openai.com | default OpenAI-compatible channel |
| `ANTHROPIC_BASE_URL` / `ANTHROPIC_API_KEY` | api.anthropic.com | Anthropic channel |
| `SMTP_*` | see config | email (low-balance alert / invoices) |

> Dev only: `MASS_ALLOW_INSECURE_DEFAULTS=true` skips weak-key checks — **never** in production.

### 5.3 Build frontend
```bash
cd frontend
npm install
npm run build      # outputs to frontend/user and frontend/admin
```

### 5.4 systemd
Example unit `/etc/systemd/system/mass.service` (workdir `/opt/mass`, exec `/opt/mass/mass-server`, env as above, `After=mysql.service redis-server.service`).
```bash
systemctl daemon-reload
systemctl enable --now mass
systemctl status mass
```

### 5.5 Nginx (recommended)
Use the repo `nginx.conf`: mount `frontend/` at `/usr/share/nginx/html` and proxy API to `127.0.0.1:8080`. Includes auth rate limiting, LLM SSE optimizations and security headers.

---

## 6. Payment Configuration (WeChat / Alipay / Epay)

All three are configured in **Admin → System Config** by group, and **take effect immediately (no restart)**:

| Group | Key config (system_configs keys) |
|-------|----------------------------------|
| Epay | `pay_epay_enabled`, `pay_epay_gateway`, `pay_epay_pid`, `pay_epay_key`, `pay_epay_sign_upper` |
| WeChat Pay | `pay_wechat_enabled`, `pay_wechat_appid`, `pay_wechat_mchid`, `pay_wechat_api_key` (APIv3), `pay_wechat_serial`, `pay_wechat_private_key`, `pay_wechat_notify_url` |
| Alipay | `pay_alipay_enabled`, `pay_alipay_appid`, `pay_alipay_private_key`, `pay_alipay_public_key`, `pay_alipay_notify_url`, `pay_alipay_gateway` |

- WeChat notify: `POST /api/v1/user/pay/wechat/notify`
- Alipay notify: `POST /api/v1/user/pay/alipay/notify`
- Top-up: `POST /api/v1/user/recharge/wechat`, `/recharge/alipay`; status: `GET /api/v1/user/recharge/status`
- `pay_wechat_notify_url` / `pay_alipay_notify_url` must be public, HTTPS, reachable by the gateway.

---

## 7. Initial Admin

Demo accounts are created by `backend/cmd/seed-demo-users` (idempotent):

```bash
cd backend
export MASS_ALLOW_DEMO_SEED=true
export MASS_ADMIN_PASSWORD='your-admin-pass'
export MASS_DEMO_PASSWORD='your-demo-pass'
export DB_HOST=127.0.0.1 DB_PORT=3306 DB_USER=mass_user DB_PASSWORD=xxx DB_NAME=mass
go run ./cmd/seed-demo-users
```

| Role | Email | Starting balance |
|------|-------|-----------------:|
| Admin | `admin@mass-platform.com` | ¥10,000.00 |
| User | `demo@mass-platform.com` | ¥100.00 |

> Use strong random passwords in production and create a dedicated admin; do not reuse demo credentials.

---

## 8. Docker (optional)

```bash
cp docker/.env.example docker/.env   # set JWT_SECRET / DB_PASSWORD / REDIS_PASSWORD
cd frontend && npm install && npm run build && cd ..
docker compose -f docker/docker-compose.yml up -d --build
```

> Fixed: `docker-compose.yml` now uses **MySQL 8** (older version wrongly used Postgres). Build the frontend on the host first; nginx mounts `frontend/`.

---

## 9. Development

```bash
# backend
cd backend && go run ./cmd/server

# frontend (hot reload)
cd frontend && npm install
npm run dev:user    # http://localhost:5173 (proxies /api -> 127.0.0.1:8080)
npm run dev:admin   # http://localhost:5174
```

---

## 10. Security & Ops
- JWT secret must be ≥32 random bytes; compromise allows admin forgery.
- DB / Redis passwords must not use defaults; Redis must require a password.
- Enable Nginx rate limiting and security headers (bundled in `nginx.conf`).
- Top-ups use "signature/encryption verification + amount reconciliation + scheduled reconciliation + idempotent settlement".
- Logs cover top-up / settlement / reconciliation audit; amount anomalies write a `reconcile` alert for manual review.

---

## 11. Known Items / Pre-release Checklist
1. **Payment callbacks & public model gateways**: code is complete and compiles/runs in this environment, but **real merchant callbacks and public gateway proxying** should be validated in a sandbox with actual keys and outbound network. Before launch, run a small sandbox order end-to-end (scan → callback → credited) and verify the scheduled reconciliation auto-recovers a missed callback within 2 minutes.
2. `docker-compose.yml` and `setup_mass.sql` previously used Postgres; this docs and `docker-compose.yml` are now MySQL-aligned.
3. Frontend large-chunk warning (>1.5MB) is from Ant Design and is harmless; code-splitting can be added later.

---

## 12. License
This project is **proprietary / commercial software**. No commercial distribution or sublicensing is permitted without written authorization from the copyright holder. See [LICENSE](./LICENSE) (commercial license terms).
