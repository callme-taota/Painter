# ButtonAuth：统一登录与 MFA 中台立项评估

评估日期：2026-10-29。依据 Painter 当前源码、[V7 鉴权与统一 MFA 评估](AUTH_REVIEW_V7.md)，并核对本文链接的协议与产品官方资料。本轮为静态评估；没有启动 ButtonAuth、迁移身份、修改登录入口或变更正在运行的 Painter。

## 结论与工作量

ButtonAuth 可作为独立的统一身份服务，Painter 是第一个接入项目。Painter 的业务用户、内容归属和权限体系已有适合联邦登录的落点，接入属于中等改造；完整中台的主要工作是 OIDC、认证因素和恢复、可信账号绑定、密钥及跨项目会话生命周期。

结合“低资源、本地服务器、Go、PostgreSQL、尽量单体”的既有目标，建议长期形态为：**独立 Go 模块化单体，前端构建后嵌入，PostgreSQL 18，标准 OIDC 接入，集中管理密码、GitHub 和 MFA；Painter 保留业务授权。** 数据库不使用 SQLite。协议、密码学和 WebAuthn 验证复用受维护的库，产品策略、持久化和运行保障由 ButtonAuth 实现并验证。

以下为一名熟悉 Go、PostgreSQL、OIDC 的工程师全职投入的估算，范围限定为自用、少量第一方应用；是工程判断，不是交付承诺。需求确认、安全复核、联调和恢复演练均会影响工期。

| 交付形态 | 估算 | 验收边界 |
| --- | --- | --- |
| 登录联通原型 | 3–5 个工作日 | 一个客户端完成 OIDC 跳转和本地用户映射；不能作为完整安全中台上线 |
| 成熟 IdP 内核 + ButtonAuth 品牌配置 + Painter 接入 | 2–4 工程师周 | 含 TOTP、恢复、绑定、会话联动、部署与基础回归；接受该产品的运行结构与定制边界 |
| Go 自有中台 + 成熟协议库 + Painter 完整接入 | 6–10 工程师周 | 含下文 V1 全范围；Painter 端约占其中 1–2 周，不重复相加 |
| 自有中台进一步加入 Passkey | 增量约 2–4 工程师周 | 多设备登记/使用、UV 策略、恢复、浏览器兼容及稳定域名验收 |

如果还要求公共多租户 SaaS、短信、企业组织/审批、复杂 RBAC、SAML、SCIM、风控或高可用，应另行估算。AI 可以加速实现和测试编写，协议兼容、安全验证和恢复演练仍是交付工作。

## 技术路线取舍

| 路线 | 优势 | 对本项目的成本与判断 |
| --- | --- | --- |
| Keycloak 等成熟 IdP | 已有认证流程、管理功能和 OIDC 端点，较快形成完整可用版本 | 官方当前对较小生产部署建议 2 GB 容器内存限制；这不是所有部署的最低值，也不含整套 Painter/PG 的容量。与极低占用目标存在取舍，应实际压测。[官方容器说明](https://www.keycloak.org/server/containers)、[OIDC 端点](https://www.keycloak.org/docs/latest/server_admin/) |
| ZITADEL 完整产品 | Go 身份服务，支持 PostgreSQL，可评估现成功能 | 当前官方 Compose 包含 Go API、Next.js Login、代理和 PostgreSQL，部署形态应整体评估。[官方 Compose](https://zitadel.com/docs/self-hosting/deploy/compose)、[数据库支持](https://zitadel.com/docs/self-hosting/manage/requirements) |
| Go + OIDC Provider 库 | 可形成嵌入前端的独立单体，控制数据模型和运行依赖 | 协议库之外的用户、MFA、恢复、会话、审计和运维仍需建设。`zitadel/oidc/v3` 可作为 OP 候选，需通过最小 PoC、存储接口与退出兼容测试后锁定版本。[官方仓库](https://github.com/zitadel/oidc) |

推荐先用最小协议 PoC 验证 Go 路线的关键合同，再投入完整 V1。`coreos/go-oidc` 是客户端库，可用于 Painter 验证身份；它不提供 ButtonAuth 的 Provider 产品能力。[客户端官方仓库](https://github.com/coreos/go-oidc)

库的支持列表、示例或认证状态不能直接转化为 ButtonAuth 的安全/协议认证结论。尤其 `zitadel/oidc` README 明确说明的认证范围是其 RP，本文不据此宣称自建 OP 已获认证。

## 产品和数据责任

| ButtonAuth 负责 | Painter 负责 |
| --- | --- |
| 统一账号、登录名、密码和上游 GitHub 身份 | 既有 `users.id`、文章、评论、收藏、媒体、关注关系 |
| TOTP/后续 Passkey、恢复码、因素变更和救援 | `owner/author/reader`、对象归属与写事务中的权限复核 |
| 第一方应用登记、准确 callback、client 凭据 | OIDC 客户端、与原用户的身份映射、应用 Cookie/CSRF |
| 中心会话、认证等级/时间、全局撤销事件 | 本地会话、敏感操作再次认证、中心事件消费 |
| 认证安全审计、签名/加密密钥生命周期 | 内容业务审计、本站权限变更与本地撤销 |

中心管理员和 Painter 站长是不同权限域。ButtonAuth 身份不能自动获得 Painter owner；首次用户准入、默认 reader 和既有站长保留由 Painter 明确控制。

ButtonAuth 使用自己的数据库和最小权限角色。可以与 Painter 共用一个 PostgreSQL 实例以降低运维成本，但不互读表、不共享密码/会话表、不共享加密密钥。浏览器分别持有各站点的 host-only Cookie；跨项目 SSO 通过跳转中心完成。

## 建议的 V1 范围

1. 单部署、第一方应用登记，统一用户名/密码；默认关闭公开注册，提供受控邀请或管理员创建。开放注册时另外落实账号/IP 限速和滥用治理。
2. GitHub 作为中心的可选上游身份；中心统一提供因素策略，不能由上游登录名称推断本次已完成 MFA。
3. TOTP 登记、验证、替换、移除；高熵单次恢复码、重新生成及离线救援。密码策略补常见/泄漏值 blocklist。首位中心管理员安装流程受服务器安装凭据保护。
4. Authorization Code + S256 PKCE、OIDC discovery/JWKS、ID token、UserInfo、退出和 back-channel logout；仅启用本版用到的 grant，不开放动态客户端注册。[OAuth 安全 BCP](https://datatracker.ietf.org/doc/html/rfc9700)
5. 中心和应用会话列表、撤销、账号停用、近期 MFA 的敏感操作；账号/IP 的共享失败计数，不依赖可通过重启清空的进程内计数。
6. 应用、用户、因素和安全事件管理；签名密钥轮换、加密密钥版本、备份/恢复、部署脚本和协议契约测试。

Passkey 的模型和验证适配器保留扩展点，下一阶段落实完整生命周期。邮件找回在发信、地址验证和令牌流程验收后开放；V1 密码遗失使用明确的受控运维流程，不能把未验证邮箱当恢复依据。恢复码在本版用于已有第一因素后的 MFA 恢复，并不自动等价于密码重置凭据。

## 分层和存储落点

独立仓库建议采用 `cmd/buttonauth` 与 `internal/{identity,authentication,mfa,oidc,session,clients,audit,storage}`。模块通过显式接口协作，运行仍是一个进程；后台重试任务也由同一进程承载。初版使用 PostgreSQL 存储交易、挑战、限速和 outbox，无需额外引入 Redis 或消息队列。前端构建后嵌入 Go，不要求服务器运行 Node。

自研路线的初步表归属如下；这是建模清单，不是已交付 DDL，实施前仍需确认字段、外键、索引与保留期：

| 模块 | 主要数据 |
| --- | --- |
| Identity | `identities`、`password_credentials`、`external_identities`；身份不可变主键，上游 provider/issuer/subject 唯一 |
| Clients | `clients`、`client_redirect_uris`、客户端凭据版本与注册的 back-channel 地址；重定向准确匹配 |
| Authentication / OIDC | `auth_transactions`、`authorization_codes`、`center_sessions`、`client_session_links`；交易绑定客户端、浏览器、nonce、PKCE、请求策略和用途 |
| MFA | `authenticators`、`factor_enrollments`、`mfa_challenges`、`recovery_codes`；pending/active、过期、尝试次数、消费状态 |
| Operations | `rate_limit_buckets`、`audit_events`、`outbox_events`、`signing_key_metadata`；密钥材料与数据库分离管理或受独立主密钥保护 |

密码只存 Argon2id 摘要；恢复码和随机 bearer 凭据只存摘要。TOTP seed 需要加密保存以进行验证，不能简单哈希代替。签名私钥、TOTP 加密主密钥和各应用 client 凭据分别管理。签名轮换保留必要的旧公钥验证窗口，恢复演练同时验证数据库和独立密钥材料。

## 关键时序和原子性

| 场景 | 必须保持的时序与失败行为 |
| --- | --- |
| 登录 | Painter 创建浏览器绑定交易 → 跳转中心 → 中心验证第一因素和所需 MFA → 签发短期单次 code → Painter 服务端交换并验证 ID token → 事务内匹配身份与建立本地会话。只完成密码验证的 pending 状态不得获得可访问业务的完整会话 |
| 密码派生 | 有界并发计算在数据库长锁之外完成；拿到身份锁后重新核对凭据版本和停用状态，再建立会话 |
| TOTP 登记 | 近期身份确认 → 过期受控的 pending seed → 用户提供有效 OTP → 锁定登记并原子激活；未经验证的 seed 不计入有效因素 |
| TOTP 验证 | 锁定因素/认证交易，验证时间窗口与累计失败策略，消费时间步、推进交易并审计在同一事务；并发不能重复使用已接受 OTP，重新发起挑战不能清空累计失败次数。[TOTP 标准](https://www.rfc-editor.org/info/rfc6238/) |
| 恢复与因素替换 | 校验第一因素和恢复证明 → 单次消费恢复码 → 因素状态、旧中心会话撤销、安全审计与撤销 outbox 一次提交；下游投递可重试 |
| code 交换 | 绑定正确客户端、redirect URI 和 PKCE；短时有效、单次消费，交换时重查中心会话/账号状态。失败返回明确重新开始认证路径，不恢复已消费 code |
| 全局退出/停用 | 中心先持久化撤销和 outbox → 发送签名 logout token → 应用验证并事务性去重/撤销会话。失败重试，避免把网络成功当数据库提交；退出重放不能恢复会话。[Back-Channel Logout](https://openid.net/specs/openid-connect-backchannel-1_0.html) |

Painter 验证受信任 issuer、签名和允许算法、audience、有效期、nonce 及适用 azp；敏感操作另外核对约定的 `acr`、实际 `amr` 和 `auth_time`。这些证明要进入服务端会话，不能靠前端 `mfa=true`。[OIDC Core](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)

因素恢复、替换和移除需要独立的近期认证/恢复策略，记录安全审计和撤销相关会话；不能仅凭长期登录 Cookie 解除 MFA。安全通知需落实可达通道，V1 至少提供用户可查看的安全事件及运维告警；邮件通知在通道验收后接入。[OWASP MFA 生命周期](https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html)

## Painter 的实际改造量

当前源码与 V7 结论一致：已经有本地/GitHub 登录，但还没有通用 OIDC、MFA 和统一身份绑定。

- `migrations/001_initial.sql` 的 `oauth_accounts` 已有 provider/issuer/subject，适合新增映射；现有唯一键包含 provider，需要固定规范 provider 名称，并为 OIDC 明确 issuer/sub 唯一规则。`provider_login` 是非空显示字段，不可当身份主键。
- `internal/app/auth.go` 的路由、profile、subject、SQL 和 `config.go` 的端点限制均为 GitHub 专用。应增加独立 OIDC adapter 和配置，复用现有 state/PKCE 设计；直接修改 GitHub URL 无法完成接入。
- 新迁移增加 OIDC nonce、认证请求等级/时效、上游 sid 与已验证的认证元数据；按需要扩展当前固定 action 白名单。此评估没有生成迁移编号或改动已提交迁移。
- `sessions` 当前最长 30 天，`last_seen_at_ms` 只在插入时赋值，尚无真正闲置超时；接入时必须一起定义并实现期限、节流活动更新和中心撤销传播。
- `Principal`、写事务中的角色/撤销复核和本地 Cookie/CSRF 继续使用；在敏感操作增加 recent-auth，而不是让前端持有长期中心 token 调用全部业务 API。

### 旧账号迁移

1. 保留 Painter 原用户 ID、owner/author/reader 和全部内容外键，另加 ButtonAuth 身份映射；中心管理员不参与本站角色推断。
2. 现有用户先证明原账号控制权，再证明 ButtonAuth 账号控制权并完成中心 MFA，事务内确认双方未绑定冲突后建立关系。首次 owner 迁移需受控操作并演练回退。
3. 原站尚无 MFA。本地账号需验证近期原密码；GitHub-only 账号须独立证明原 GitHub 身份，但再次 OAuth 跳转不能证明近期重新认证或 MFA。高权限旧账号迁移须增加受控运维确认/线下校验，并完成中心 MFA；不能仅凭长期旧 Cookie 或再次跳转完成绑定。
4. 用户名、邮箱、头像和 GitHub 显示名均不能自动合并账号。ButtonAuth broker 后的 sub 和原 GitHub 数字 ID 是两套键；首次接入注册关闭时应进入绑定流程，不能为方便联调开放注册而生成重复 reader。
5. V1 优先显式绑定，不批量迁密码。后续导入现有 Argon2id 摘要时，再单独验证格式、策略和身份映射，不能假设任意 IdP 接受当前 hash。
6. 绑定覆盖和救援演练完成后，按项目策略收口本地/GitHub 普通登录；所有可进入后台的路径都须满足同一 MFA 要求。离线救援需审计并撤销旧会话。

## 低资源部署与失效边界

一台本地服务器可以承载 Painter、ButtonAuth 和同一 PG 实例中的两个隔离数据库，HTTPS 代理也可复用。这降低进程和运维数量，但仍具有共同机器故障域。Go 架构的实际内存、密码登录峰值和备份开销须测量，不给出未经实测的“几十 MB”或“1 GB 足够”承诺。

ButtonAuth 应使用长期稳定的 issuer 域名。局域网 DNS 可将域名指向本机私网 IP，同时为设备提供受信任的 HTTPS；Painter 的 callback 也应配置相应 HTTPS 地址。当前 `http://192.168.2.209:18080` 适合现有预览，不能直接作为 Passkey 的完整部署条件。WebAuthn 受安全上下文、origin 和 RP ID 约束，域名应在登记凭据前定下来。[WebAuthn 标准](https://www.w3.org/TR/webauthn-3/)

应用日常请求保留本地会话检查；登录、重新认证和定期中心状态确认才需要跨服务。建议 V1 采用 back-channel 事件加不超过 5 分钟的中心状态确认有效期作为普通私有会话的撤销边界；确认接口和应用认证方式须作为明确的内部合同实现，不冒充 OIDC 自带保证。状态确认绑定本地会话对应的中心 sid 和安全版本；仅查询账号启用状态不足以续期，已撤销会话必须拒绝续期，失败或重试不得延长原有效期。到期后中心不可用则停止续期私有会话，敏感操作不能因故障跳过再次认证。公开博客阅读继续独立运行。

普通退出只结束当前应用会话；“退出所有应用”结束中心和相关应用会话，文案与行为要分开。中心停用账号、因素恢复等事件触发全局撤销；网络分区下不能承诺零延迟。敏感写入在中心状态确认后仍需本地事务复核，在途请求竞态必须列入验收。

## 落地顺序与验收门槛

1. **协议 PoC**：独立临时 ButtonAuth、独立库、固定一个 Painter 测试客户端，验证 code/PKCE、ID token、JWKS 轮换、近期认证和 back-channel。确认库/部署路线后再进入产品实现。
2. **身份与因素闭环**：账号、GitHub、TOTP、恢复码、中心管理与安全审计，验证并发单次消费、失败计数和因素变更。
3. **Painter 接入**：明确旧账号绑定、原站长保留、注册关闭行为、敏感操作、闲置/绝对过期、撤销与中心故障。
4. **运维交付**：HTTPS、最小权限、安装/升级/备份/恢复、签名和加密密钥轮换，真实环境端到端测试与独立安全复核。
5. **复用验证**：第二个独立测试客户端验证跨 client 隔离、SSO、退出和接入文档；完成后沉淀小型接入包，再扩展 Passkey 和更多项目。

最低验收包含：错误 issuer/audience/算法/nonce、错误 PKCE/callback、重复 code、低等级/过旧认证拒绝、跨 client 混用、绑定冲突/TOTP/恢复码并发、因素移除、停用/改密/救援后的下游撤销、logout token 重放、JWKS 轮换、中心不可用与备份恢复。仅有登录演示成功不足以关闭这些验收项。
