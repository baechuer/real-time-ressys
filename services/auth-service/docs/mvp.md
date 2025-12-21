Phase 0：对齐接口与 wiring（0.5–1h）

目标：保证 ports/service/handlers/router 的签名一致，bootstrap 能注入

 internal/application/auth/ports.go：最终版确认（UserRepo + PasswordHasher + TokenSigner + SessionStore + OneTimeTokenStore + EventPublisher）

 internal/application/auth/service.go：先写空壳 methods（Register/Login/Me…）只返回 domain.ErrInternal(nil) 之类，先让编译通过

 internal/transport/http/handlers/auth.go：先按 router 需要的方法签名写 stub（只为编译通过）

 internal/bootstrap/wire.go：把依赖都 new 出来并注入（可以先用 nil adapter + 注释未启用 routes）
验收：go test ./... 编译通过；/healthz 能跑

Phase 1：基础安全能力（JWT + password）（1–2h）

目标：Login/Register 的核心依赖先落地

1.1 PasswordHasher

文件：internal/infrastructure/security/password.go

 实现 PasswordHasher（bcrypt hash/compare）

 写单测（可放 internal/infrastructure/security/password_test.go）
验收：hash/compare 测试通过

1.2 TokenSigner (JWT)

文件：internal/infrastructure/security/jwt.go

 实现 TokenSigner（sign/verify，exp）

 错误映射到 domain（invalid/expired/sign_failed）

 单测
验收：签发→验证通过；过期 token → token_expired

Phase 2：UserRepo（Postgres）（2–4h）

目标：用户数据可持久化，支持 register/login/me/change password/verify

文件：internal/infrastructure/db/postgres/user_repo.go

 实现 UserRepo：

Create

GetByEmail

GetByID

UpdatePasswordHash

SetEmailVerified

LockUser（可先空实现或暂不使用）

 把 DB 错误转换成 domain error（重复 email→conflict；not found→not_found）
验收：本地连 DB 后：create→get 成功；重复 email 返回 email_already_exists

你现在 repo 里还没看到 migrations/ddl：建议你至少有一个 schema.sql 或迁移脚本（放哪里都行），不然后面调试会卡。

Phase 3：Service MVP（Register/Login/Me）（2–4h）

目标：业务闭环跑通（不含 refresh/邮件）

文件：internal/application/auth/service.go

 实现：

Register(ctx, email, password)：hash → repo.Create → sign access token

Login(ctx, email, password)：repo.GetByEmail → compare → sign

Me(ctx, userID)：repo.GetByID

 统一登录失败：只返回 domain.ErrInvalidCredentials()（防枚举）

 service 单测：fake UserRepo/Hasher/Signer（不连 DB）
验收：service 单测全过

Phase 4：HTTP handlers + router 打开核心路由（1–2h）

目标：HTTP 层可用（DTO/response 你已准备）

文件：internal/transport/http/handlers/auth.go

 Register/Login/Me handlers：

response.DecodeJSON + dto.Validate() + response.OK/Created + response.WriteError

文件：internal/transport/http/router/router.go

 打开：

POST /auth/v1/register

POST /auth/v1/login

GET /auth/v1/me
验收：curl：register→login→me 成功；错误输出结构一致

Phase 5：Auth middleware + RBAC（1–2h）

目标：保护路由、支持 admin

文件：internal/transport/http/middleware/authn.go

 Bearer token 解析

 调用 TokenSigner.VerifyAccessToken

 注入 ctx：userID/role

文件：internal/transport/http/middleware/rbac.go

 admin guard（role != admin → forbidden）

打开路由：GET /auth/v1/admin
验收：无 token 401；非 admin 403；admin 200

Phase 6：Refresh / Logout / Revoke sessions（Redis SessionStore）（3–6h）

目标：生产级 token 生命周期（refresh rotation）

文件：internal/infrastructure/redis/token_store.go

 实现 SessionStore：

CreateRefreshToken

RotateRefreshToken（旧 token 作废，检测重用）

GetUserIDByRefreshToken

RevokeRefreshToken

RevokeAll

文件：internal/application/auth/service.go

 增加业务：

Refresh（rotate → sign new access）

Logout（revoke current refresh）

SessionsRevoke（revoke all）

文件：handlers/auth.go：对接 endpoint
验收：refresh 可用；logout 后 refresh 不可用；revoke all 全失效

Phase 7：One-time token + Rabbit publisher（Verify email + Password reset）（3–6h）

目标：与你的 email-service 对接（auth 只发 MQ）

7.1 RabbitMQ Publisher

文件：internal/infrastructure/messaging/rabbitmq/publisher.go

 实现 EventPublisher：

PublishVerifyEmail(evt)

PublishPasswordReset(evt)
验收：能 publish 到 exchange/queue（至少本地打印确认/集成测试）

7.2 OneTimeTokenStore

建议位置：你现在已经有 internal/infrastructure/redis/token_store.go
你可以：

继续放 one-time token 在同一个文件里（不推荐会变大）

或新建：internal/infrastructure/redis/one_time_token_store.go（推荐）

 实现 Save/Consume/Peek

7.3 Verify email flows

service.go：

 VerifyEmailRequest(email)：生成 token → Save → 拼 URL → PublishVerifyEmail

 VerifyEmailConfirm(token)：Consume → repo.SetEmailVerified

handlers/auth.go：对应 endpoints

7.4 Password reset flows

service.go：

 PasswordResetRequest(email)：用户存在才发 MQ，但对外永远 200

 PasswordResetConfirm(token,newPw)：Consume → hash → UpdatePasswordHash →（可选）RevokeAll

 PasswordResetValidate(token)：Peek
验收：token 一次性；confirm 后密码更新；重复使用 token 失败

Phase 8：中间件补齐与上线质量（1–3h）

你已经有文件了（都在 internal/transport/http/middleware），建议按顺序启用：

recover.go（panic → 500）

request_id.go

logging.go（结合 zerolog）

security_headers.go

ratelimit.go（优先限制 login/reset）

验收：panic 不炸；日志有 request_id；安全头存在；限流生效

你现在最推荐的“下一步”

按你结构，最快路径是：

Phase 1 → Phase 2 → Phase 3 → Phase 4
也就是：先把 security/password.go、security/jwt.go、db/postgres/user_repo.go、application/auth/service.go(Register/Login/Me) 跑通，再接 HTTP。

如果你把 ports.go 和你目前 service.go 的函数签名贴一下，我可以把 Phase 3 的 service.go 直接按你接口写成“可编译可测试”的骨架（含 domain errors 的返回点），你照着填 repo/jwt/redis 即可。