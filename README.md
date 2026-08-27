<p align="center">
  <img src="frontend/assets/MaaSLOGO.png" alt="SM-MaaS" width="320">
</p>

# MASS 平台（Mass AI SaaS Stack）

MASS 是一个 **LLM API 网关 + 计费控制台** 平台，面向需要对外提供 AI 能力并按量/按订阅计费的企业与团队。后端基于 Go / Gin / GORM / MySQL / Redis / JWT，提供模型转发（兼容 OpenAI / Anthropic 协议）、额度计费、订阅 / 加油包 / 授信、充值支付（易支付 / 原生微信支付 / 原生支付宝）、实名认证、发票、OpenID 登录、站内通知与数据留存等能力；前端为 React 控制台，分为 **用户门户** 与 **管理后台** 两个应用。

> 配套英文文档见 [README_EN.md](./README_EN.md)，商用授权见 [LICENSE](./LICENSE)。

---

## 1. 功能一览

### 用户门户（`/` 落地页、`/user` 控制台）
- 认证：邮箱 / 手机验证码注册登录，亦支持 OpenID 快捷登录与绑定
- 控制台总览：余额、Token 额度（授信 / 加油包）、订阅进度、最近交易
- 模型市场：浏览可用模型与价格，支持「无限火力」标识与资格提示
- API Keys 管理（含模型访问控制）
- 用量账单：趋势图、计费明细、交易流水、JSONL 导出
- 充值中心：
  - **原生微信支付 / 原生支付宝**（扫码，订单状态轮询）
  - **易支付（Epay）** 聚合码支付
  - 回调金额对账 + 定时网关对账（防篡改、补漏单）
- 套餐订阅、Token 加油包、重置券
- **低额预警**：用户可自定义 Token 余额阈值，低于阈值自动邮件提醒
- 站内通知、发票申请（企业 / 个人、普票 / 专票）、Token 授信、实名认证
- 对话记录、问题反馈、个人设置

### 管理后台（`/admin`）
- 数据概览：用户数 / 活跃用户 / 总收入 / 今日请求 / 收入趋势
- 用户管理（列表 / 详情 / 编辑 / 余额调整）、实名认证审核
- LLM 渠道管理（**一键拉取渠道模型** + 连通性测试）、定价分组、模型价格表、套餐管理
- 模型价格 **从渠道同步**（自动去重 + 默认价，默认停用防止 ¥0 污染广场）
- 订单流水、发票审核、Token 授信审批、催账、重置券发放
- 通知发送、反馈处理、全量对话记录查询与导出
- 系统配置（站点品牌 / OpenID 登录 / 易支付 / **微信支付** / **支付宝**，分组保存，保存后立即生效无需重启）

### 支付与计费安全
- 所有原生支付（微信 / 支付宝）回调均做 **签名 / 加密验真**（微信 AES-256-GCM 解密、支付宝 RSA2 验签）
- 回调与主动查询均做 **金额对账**：网关金额与本地订单不一致时拒绝结算并记录告警
- **定时对账任务**：每 2 分钟主动查询网关，补偿因网络 /  downtime 漏掉的回调；未支付订单 30 分钟自动取消
- 充值流水有幂等结算（`CompleteEpayPayment`），重复回调不会重复入账

---

## 2. 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go / Gin / GORM / MySQL 8 / Redis / JWT / 原生微信支付 v3 / 原生支付宝 RSA2 / 易支付 |
| 前端 | React 18 / Vite 5 / TypeScript / Ant Design 5 / React Router v6 / Axios / Zustand / dayjs |
| 网关 | 兼容 OpenAI `/v1/*` 与 Anthropic 协议，SSE 流式转发，按 Token 计费 |
| 运维 | systemd 托管 / 可选 Docker Compose（MySQL + Redis + API + Nginx）/ Nginx 反代限流 |

---

## 3. 项目结构

```
mass/
├── backend/                     # Go 后端（Gin + GORM + MySQL + Redis）
│   ├── cmd/server/              # 服务入口
│   ├── cmd/seed-demo-users/     # 演示账号种子（可选）
│   ├── internal/
│   │   ├── api/                 # 路由 / handler / middleware / dto
│   │   ├── billing/             # 计费与额度核心（含对账 / 过期 / 结算）
│   │   ├── llm/                 # LLM 渠道代理（OpenAI / Anthropic）
│   │   ├── payment/             # 易支付 / 微信 / 支付宝 客户端
│   │   ├── repository/          # 数据访问层
│   │   ├── service/             # 验证码 / OpenID 等
│   │   └── model/               # GORM 数据模型
│   └── pkg/                     # 通用工具（响应封装、日志等）
├── frontend/                    # React + Vite 前端（npm workspaces monorepo）
│   ├── user/                    # 用户门户构建产物（后端静态托管）
│   ├── admin/                   # 管理后台构建产物
│   ├── packages/
│   │   ├── shared/              # 共享包（Axios 封装、类型、主题）
│   │   ├── user-app/            # 用户门户源码
│   │   └── admin-app/           # 管理后台源码
│   └── docs/                    # 前端开发计划、API 文档
├── docker/                      # Dockerfile + docker-compose.yml + .env.example
├── nginx.conf                   # 生产 Nginx 反代（限流 / 安全头 / SSE）
├── scripts/                     # build.sh / package.sh / start-server.sh
├── setup_mass.sql               # 遗留 Postgres 建库示例（**非 MySQL 必用**）
├── docs/                        # 设计 / 架构 / 接口 等文档
└── README.md / README_EN.md / LICENSE
```

---

## 4. 环境依赖

| 组件 | 版本 / 说明 |
|------|------------|
| Go | 1.22+（编译后端） |
| Node.js | 20.x（构建前端） |
| MySQL | 8.0（utf8mb4），需先建空库 `mass`，**应用启动自动迁移建表** |
| Redis | 7.x，**必须设置密码** |
| Nginx | 1.27+（生产反代，可选） |

> 说明：`setup_mass.sql` 是早期 Postgres 建库示例，**与当前 MySQL 部署不符**，请勿直接用于生产；建库请用下面的 SQL 片段。

```sql
CREATE DATABASE mass CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'mass_user'@'%' IDENTIFIED BY '你的强密码';
GRANT ALL PRIVILEGES ON mass.* TO 'mass_user'@'%';
FLUSH PRIVILEGES;
```

---

## 5. 部署（生产推荐：MySQL + systemd）

### 5.1 后端编译
```bash
cd backend
go build -o ../mass-server ./cmd/server
```

### 5.2 环境变量
后端通过环境变量配置（参考 `backend/internal/config/config.go`）。**缺少 `JWT_SECRET` / `DB_PASSWORD` / `REDIS_PASSWORD` 时拒绝启动。**

| 变量 | 默认 | 说明 |
|------|------|------|
| `SERVER_PORT` | 8080 | 监听端口 |
| `SERVER_MODE` | release | 运行模式 |
| `DB_HOST` / `DB_PORT` | localhost / 3306 | MySQL 地址 |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | mass / mass123 / mass | 数据库凭据 |
| `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD` / `REDIS_DB` | localhost / 6379 / 空 / 0 | Redis |
| `JWT_SECRET` | 空（**必须**设 ≥32 字节随机串） | JWT 签发密钥 |
| `MASS_FRONTEND_DIR` | 无 | 前端静态目录（设为 `../frontend` 由 Gin 托管） |
| `UPLOAD_DIR` | uploads | 上传目录 |
| `LOG_LEVEL` / `LOG_OUTPUT` | info / stdout | 日志级别 / 输出（file 落盘） |
| `OPENAI_BASE_URL` / `OPENAI_API_KEY` | api.openai.com | 默认 OpenAI 兼容渠道 |
| `ANTHROPIC_BASE_URL` / `ANTHROPIC_API_KEY` | api.anthropic.com | Anthropic 渠道 |
| `SMTP_*` | 见 config | 邮件（余额预警 / 发票通知） |

> 开发可用 `MASS_ALLOW_INSECURE_DEFAULTS=true` 跳过弱密钥校验，**生产严禁**。

### 5.3 前端构建
```bash
cd frontend
npm install
npm run build      # 产物输出到 frontend/user 与 frontend/admin
```

### 5.4 以 systemd 托管
示例单元 `/etc/systemd/system/mass.service`（工作目录 `/opt/mass`，执行 `/opt/mass/mass-server`，设置上述环境变量，依赖 `mysql.service` / `redis-server.service`）。
```bash
systemctl daemon-reload
systemctl enable --now mass
systemctl status mass
```

### 5.5 Nginx 反代（可选，推荐）
使用仓库根目录 `nginx.conf`：将前端 `frontend/` 挂载为 `/usr/share/nginx/html`，API 反代到 `127.0.0.1:8080`。已内置认证接口限流、LLM 网关 SSE 长连接优化与安全响应头。

---

## 6. 支付配置（原生微信 / 支付宝 / 易支付）

三种支付均在 **管理后台 → 系统配置** 中按分组填写并保存，**保存后立即生效（无需重启）**：

| 分组 | 关键配置项（system_configs 键） |
|------|-------------------------------|
| 易支付 | `pay_epay_enabled`、`pay_epay_gateway`、`pay_epay_pid`、`pay_epay_key`、`pay_epay_sign_upper` |
| 微信支付 | `pay_wechat_enabled`、`pay_wechat_appid`、`pay_wechat_mchid`、`pay_wechat_api_key`(APIv3)、`pay_wechat_serial`、`pay_wechat_private_key`、`pay_wechat_notify_url` |
| 支付宝 | `pay_alipay_enabled`、`pay_alipay_appid`、`pay_alipay_private_key`、`pay_alipay_public_key`、`pay_alipay_notify_url`、`pay_alipay_gateway` |

- 微信回调：`POST /api/v1/user/pay/wechat/notify`
- 支付宝回调：`POST /api/v1/user/pay/alipay/notify`
- 发起充值：`POST /api/v1/user/recharge/wechat`、`/recharge/alipay`；状态轮询：`GET /api/v1/user/recharge/status`
- `pay_wechat_notify_url` / `pay_alipay_notify_url` 需为公网可访问、可达后端的 HTTPS 地址。

---

## 7. 初始管理员

演示账号由 `backend/cmd/seed-demo-users` 创建（幂等、可重复执行）：

```bash
cd backend
export MASS_ALLOW_DEMO_SEED=true
export MASS_ADMIN_PASSWORD='你的管理员密码'
export MASS_DEMO_PASSWORD='你的演示用户密码'
export DB_HOST=127.0.0.1 DB_PORT=3306 DB_USER=mass_user DB_PASSWORD=xxx DB_NAME=mass
go run ./cmd/seed-demo-users
```

| 角色 | 邮箱 | 初始余额 |
|------|------|---------:|
| 管理员 | `admin@mass-platform.com` | ¥10,000.00 |
| 普通用户 | `demo@mass-platform.com` | ¥100.00 |

> 生产环境请使用强随机密码，并创建独立管理员账号，切勿复用演示口令。

---

## 8. Docker 部署（可选）

```bash
cp docker/.env.example docker/.env   # 填入 JWT_SECRET / DB_PASSWORD / REDIS_PASSWORD
cd frontend && npm install && npm run build && cd ..   # 先构建前端
docker compose -f docker/docker-compose.yml up -d --build
```

> 已修正：`docker-compose.yml` 现使用 **MySQL 8**（旧版误用 Postgres，与后端 DSN 不符）。前端需先在宿主机构建后由 nginx 挂载 `frontend/`。

---

## 9. 开发

```bash
# 后端
cd backend && go run ./cmd/server

# 前端（开发热更新）
cd frontend
npm install
npm run dev:user    # http://localhost:5173 （代理 /api → 127.0.0.1:8080）
npm run dev:admin   # http://localhost:5174
```

---

## 10. 安全与运维要点
- JWT 密钥必须 ≥32 字节随机串；泄露可伪造管理员。
- 数据库 / Redis 密码不得使用默认值；Redis 必须启用密码。
- 生产启用 Nginx 限流与安全头（已内置于 `nginx.conf`）。
- 充值采用「签名 / 加密验真 + 金额对账 + 定时对账 + 幂等结算」四重防护。
- 日志含充值 / 结算 / 对账审计；异常金额会写 `reconcile` 告警便于人工复核。

---

## 11. 已知事项 / 发布前建议
1. **支付回调与公共模型网关**：本仓库代码已完整实现并在当前环境编译 / 运行通过，但**真实商户回调报文与公网模型网关转发**需在具备密钥与外网的环境中做沙箱验证。上线前请用沙箱商户号发一笔小额订单，确认：扫码 → 回调到账 → 流水正确；并验证断网时「定时对账」能在 2 分钟内自动补单。
2. `docker-compose.yml` 与 `setup_mass.sql` 早期曾误用 Postgres，本文档与 `docker-compose.yml` 已统一为 MySQL。
3. 前端大包提示（>1.5MB）为 Ant Design 体积所致，不影响运行；如需可后续做代码分割。

---

## 12. 许可
本项目为**专有 / 商业软件**，未经著作权人书面授权不得用于商业分发或再许可。详见 [LICENSE](./LICENSE)（商用授权说明）。
