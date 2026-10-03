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

Vite 会把 `/api/auth` 和 `/healthz` 同源代理至 `http://localhost:8088`。启用 `VF_EXPOSE_DEBUG_CODES` 后，开发环境的验证码接口会返回 `debugCode`，注册页会自动填入该值；生产环境不得启用此选项。

前端 Mock 只允许在 Vite 开发模式运行，生产构建始终调用真实 `/api/auth` 接口。

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
