# VerdantFlare Login

VerdantFlare Center 的独立认证服务，包含 Vue 登录前端和 Go HTTP 后端。它负责邮箱注册、邮箱验证、密码登录、会话、密码重置和退出，不承载组织、套餐、应用权限或 API Key 等业务数据。

## 当前实现边界

- Go 1.25 标准库 HTTP 服务。
- Argon2id 密码哈希，参数为 64 MiB、3 次迭代、4 路并行。
- 32 字节随机会话令牌；浏览器只持有 `HttpOnly`、`SameSite=Lax` Cookie。
- 会话、验证码、密码重置令牌在存储中只保存摘要。
- Go 1.25 `CrossOriginProtection` 拦截跨站写请求。
- 登录、验证码、注册、找回密码接口带进程内速率限制。
- 配置 `VF_DATABASE_URL` 后使用 PostgreSQL 持久化；开发环境未配置时回退到内存存储。
- 企业 SSO 路由已保留，但在 IdP 与 OIDC Client 确定前返回 `501`。
- PostgreSQL 迁移由部署仓库中的一次性数据库初始化 Job 执行，应用本身不在启动时修改表结构。
- 生产环境使用 Resend Email API 投递验证码和密码重置邮件；注册验证码仅在服务商接受后返回成功。密码找回接口仍返回通用响应，避免暴露邮箱是否已注册；投递失败会留在服务端错误日志中。生产环境禁止只记日志的邮件模式和调试验证码。

## 本地全栈开发

终端一，启动 Go 后端：

```bash
VF_EXPOSE_DEBUG_CODES=true go run ./cmd/server
```

终端二，关闭前端 Mock 并启动 Vue：

```bash
npm install
VITE_USE_MOCK=false npm run dev
```

Vite 会把 `/api/auth` 和 `/healthz` 同源代理至 `http://localhost:8088`。启用 `VF_EXPOSE_DEBUG_CODES` 后，开发环境的验证码接口会返回 `debugCode`，注册页会自动填入该值；生产环境拒绝此选项。开发环境默认使用只记录元信息的 `log` 邮件模式，仅供本地测试，不代表真实投递。

## Resend 邮件投递

在 Resend 验证发件域名后，设置 `VF_MAIL_PROVIDER=resend`、`VF_MAIL_FROM`（如 `VerdantFlare <no-reply@example.com>`）和 `VF_RESEND_API_KEY`。Login 通过 Resend Email API 发送纯文本邮件，无需在前端或 new-api 中配置 Key。发件地址必须属于已验证域名；Resend 返回邮件 ID 仅证明服务商接受，实际送达还需核对 Resend 投递记录及收件箱。

生产部署清单从 `verdantflare/login-mail` Secret 读取 `VF_MAIL_FROM` 与 `VF_RESEND_API_KEY`。请通过受控渠道在集群创建 Secret，勿把 Key 写入 Git、命令历史、文档或应用日志。先确认 Secret 与域名验证，再部署新镜像；未配置时 Login API 会拒绝启动。测试 Key 使用完应轮换。

联调管理端入口时，在本机启动 Control Service 并设置 `CONTROL_TRUST_AUTH_HEADERS=true`。Vite 的 `/api/control/context` 开发代理先向 Login 验证会话，再将登录主体传给 Control；未登录或未绑定业务角色的用户不会看到管理端入口。

前端 Mock 只允许在 Vite 开发模式运行，生产构建始终调用真实 `/api/auth` 接口。

登录成功页通过同源的 `/api/control/context` 读取当前组织角色。具备 `app_ops_admin` 或 `customer_success_admin` 的用户会看到“进入管理端”，分别进入 Hub 的应用发布或客户组织页面。该接口由 Login Web 的 Nginx 先验证 Login 会话，再把身份传给 Control Service；Hub 和 Control Service 仍会校验页面及操作权限。开发模式的 Mock 仅用 `VITE_DEV_ADMIN_EMAIL`（默认 `admin@verdantflarehub.com`）模拟运营管理员。

## API

| 方法 | 路由 | 说明 |
| --- | --- | --- |
| `POST` | `/api/auth/verification-code` | 发送注册邮箱验证码 |
| `POST` | `/api/auth/sign-up` | 验证邮箱验证码并创建账号 |
| `POST` | `/api/auth/sign-in` | 密码登录并设置会话 Cookie |
| `GET` | `/api/auth/session` | 读取当前登录会话 |
| `POST` | `/api/auth/forgot-password` | 请求密码重置邮件；始终返回通用结果 |
| `POST` | `/api/auth/reset-password` | 使用一次性令牌更新密码并撤销旧会话 |
| `POST` | `/api/auth/logout` | 撤销当前会话并清除 Cookie |
| `GET` | `/api/auth/sso/authorize` | 预留企业 SSO 入口，当前未配置 |
| `GET` | `/healthz` | 存活检查 |

`returnTo` 仅接受 Hub 内部相对路径。外部 URL、协议相对 URL 和反斜杠路径都会回退到 Hub 首页。

## 数据库

PostgreSQL 表结构位于 `migrations/postgres/`。生产环境必须设置 `VF_DATABASE_URL`；连接池默认最多 10 个连接、保留 5 个空闲连接。注册与验证码消费、密码重置与会话撤销均在短事务内完成。

默认管理员的密码哈希由管理员本地生成，明文密码不得提交。优先使用隐藏输入并通过标准输入传给哈希工具，避免密码进入命令历史：

```bash
read -r -s -p 'Bootstrap password: ' VF_BOOTSTRAP_PASSWORD </dev/tty
printf '\n' >/dev/tty
printf %s "$VF_BOOTSTRAP_PASSWORD" | go run ./cmd/password-hash
unset VF_BOOTSTRAP_PASSWORD
```

## 验证

```bash
go test ./...
go vet ./...
npm run build
```
