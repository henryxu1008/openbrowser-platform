# 店铺浏览器 v1 合同

本文件所有新增接口、表和 DTO 均为 **PROPOSED / NOT_IMPLEMENTED**。仅供后续获授权的实现使用。基线、scope、authority 和 threat model 见 [README](README.md)；验收编号见 [实施任务](IMPLEMENTATION.md)。

## C1. 身份、企业与动作授权

### 1. 调用与身份来源

- 桌面继续使用原 PKCE 会话。当前实现保留内存 access token，但授权请求只有 `openid profile`；未证明该 Native client 的 token 可调用目标硕米 API。不得通过转发 ID token、Web secret、renderer bearer 或新建客户端来绕过此缺口。
- 新增 Go 内部 API client，从原会话在不可变 generation 下执行受限请求；不新增“取 token”Wails 方法。请求结束及副作用前再检查原 generation；不能等待后换用新账号的 token。
- 新增独立、严格校验的安装目录公开配置 `shuomi-public.json`，仅包含 `apiOrigin`；原 `oidc-public.json` 的两字段合同保持原样。发行时两者由维护者核对目标实例，本轮不创建配置。缺失/非法时店铺功能不可用，不退回匿名或其他实例；原独立功能仍按原门禁。
- apiOrigin 仅 HTTPS origin，无 userinfo、query、fragment 或额外路径；仅由受信任安装配置读取。renderer 不能传任意主机、路径、headers。API client 拒绝重定向，不将 token 发往代理探测、下载或商家主机。超时最多 15 秒；沿用现有 Store 响应上限 2 MiB；新增命令体不超过 16 KiB。
- 硕米服务器复用当前验证器、workbenchcontext 与 authz。主体取 verified `UserID`，成员取 `EffectiveMemberID`；issuer 取与该验证器绑定的服务端配置，不从请求 body/header 接受。现有 AuthenticatedIdentity 没有 issuer 字段，不为本版扩大全局身份 DTO。
- `X-Requested-Organization-ID` 只是待选企业，绝不作为权限证明；拒绝/忽略客户端 user、member、role、admin 与 tenant 声明。home organization 不是实际业务企业。DTO 必须返回已验证的 user/organization，桌面在原 generation/企业不匹配时丢弃迟到结果。

### 2. 现有读取接口及新增动作能力

直接复用 `GET /api/v1/workbench/context`、现有企业选择接口、`GET /api/v1/workbench/store-catalog`、Store detail/profile。保持原响应、服务端分页和实时 Store 访问策略；桌面不得依赖成员管理接口枚举全部企业店铺。

提议新增应用权限 `workbench.store.browser.use`，注册在现有 authz 的独立可分配模块 `store-browser`，不自动加入既有 `stores` 模块，不从“有店铺编辑权”隐式推导。不创建 ZITADEL 角色/client，不修改已有角色分配。默认关闭，管理员以后通过硕米现有企业角色模块配置明确分配；内建受保护角色的映射须在实现时显式测试，不硬编码 renderer 角色名。

| 操作 | 权限与范围 |
| --- | --- |
| 查看店铺投影 | 原 `workbench.store.read`，有效企业成员＋原 Store grant（保留原管理员分支） |
| 配置/打开受管环境 | 上述条件＋`workbench.store.browser.use`＋Store 记录 active＋本机 binding 属于当前 issuer/subject/企业 |
| 分配/撤销店铺 | 完整复用原 member-resource 管理门禁、CAS 与幂等回执；负责人字段不改变 grant |
| 查看店铺操作记录 | 原店铺读取门禁；普通成员仅自己的事件，既有授权管理员可读本企业该店铺全部事件 |
| 原操作的回执读取/结果报告 | 仅同一 verified issuer/subject 的原操作；允许 Store grant 被撤销后提交原结果，不能启动新动作，见 C3 |
| 撤权/注销后安全关闭 | 原 CloseOwnedBrowser 的进程/目录/端点证明；不授予 profile 读取、编辑、启动或新 CDP 指令 |

不新增收费门槛。订阅、官方连接状态与浏览器动作权限分离。没有角色策略、授权依赖故障或身份不确定时，禁止新的配置/打开操作。

### 3. 准入、撤销与本机再次检查

服务端先按现有 LiveWrite 策略读取实时企业/角色，再在 Store DB 事务中按 **Store→member grant** 的原锁顺序检查店铺状态与成员授权，写操作回执及审计意图。复用/有界提取 Store owner 的授权事务能力，不能从新服务绕过 MemberScopedStoreRepository 直接查表。

事务内使用原 MemberScopedStoreRepository.LockRead 或同 owner 的等价只读授权 fence，再写本用例的操作/审计行；领域端口不暴露 GORM 事务对象。不要调用其 withWrite/authorize(write=true) 作为浏览器准入：该分支额外要求 store.update，而本版 store.read＋browser.use＋grant 就应足够，配置本机环境不等于修改 Store 记录。LockRead 必须借用同一事务，不能先普通 Get 检查后另开事务写回执。

成员店铺撤销与准入以该事务顺序为线性化点：撤销先提交则拒绝；准入先提交则已授权的单次操作可在最多 15 秒执行窗口内完成。外部 IAM 企业/角色变化沿用原实时解析语义，不虚构与本机 cmd.Start 的原子性。每一个新的配置/打开动作都重新准入。

桌面收到首次准入后，仍须匹配会话 generation、企业 revision、绑定版本及本地操作状态，并在代理预热/下载、文件写入、cmd.Start 等副作用入口检查。服务端准入后退出登录、切企业或超时的迟到响应不得触发执行。执行时限采用从请求发出起的本机单调计时保守限制，不依赖可调墙钟。已经准入且实际发生的结果属于原身份，不因注销而改写归属。

现有 useBrowserProfileActions 在 renderer 中先校验/预热代理、再调用启动。店铺入口不能直接套用这个回调，否则权限拒绝前已有网络副作用。复用原页面、提示、代理实现和启动顺序，受管模式把这些副作用的调度交给 Go 店铺用例，在同一原操作准入之后调用；不复制一个代理/启动实现，也不把 UI 隐藏当门禁。

## C2. 本机受管实例绑定与原子性

### 1. 不可变归属与持久化

在原 Profile SQLite 数据库内增加 `browser_store_bindings`；原 Profile 表、DAO、Manager 仍为唯一实例 owner。新增字段为：

| 字段 | 合同 |
| --- | --- |
| binding_id / profile_id | canonical UUID；各自唯一；profile_id 外键引用原 Profile |
| api_origin / issuer / subject / organization_id / store_id | 来自固定配置、verified 会话与服务端 Store 响应；保存后不可改绑；不得从 renderer 权威赋值 |
| binding_version | 正整数 CAS 版本；配置保存递增；打开不递增；与硕米 metadata 的 profileVersion 不同 |
| create_operation_id / created_at / updated_at | 本机原操作与时间；无 token、Cookie 或私密材料 |

同一 `(api_origin, issuer, subject, organization_id, store_id)` 在本机最多一个绑定；profile_id 在本机最多一个绑定，不设“全世界一家店铺只能一个实例”。不建立设备注册或服务器可信设备身份。成员 AuthorizationID 是每次动作的实时授权事实，不是长期本机 owner；退企重入不能继承旧 grant，能否继续原资料由新一轮真实授权决定。

首次配置在原 Manager 内复用配置规范化、默认内核/指纹、代理解析、UUID 和 ProfileDAO 的行为；增加同一 SQLite 事务提交 **Profile 行＋binding＋本机操作 journal**。原 Create→SaveProfiles 不是该事务，禁止“先创建再另存 binding”。只在提交后发布到原 Manager 内存；提交失败不留下可通过旧 API 启动的无归属 profile。必要目录在归属事务提交后、按原路径规则创建；失败保留已绑定资料和失败证据，不自动删除目录或重建另一实例。

两个首次配置请求可以各自生成候选 UUID，但唯一性必须由同库 owner/store 槽位约束仲裁，不能只按两个不同 bindingId 互斥。冲突请求的 Profile/binding/journal 一并回滚，不发布 Manager map、不创建目录、不自动换 UUID 重试；在原身份范围内返回 BINDING_VERSION_CONFLICT，用户只读刷新已提交绑定。配置与打开的本机未决检查同样覆盖 owner/store 槽位，不能以候选 binding 不同绕过。

受管模式必须使用同一 SQLite owner；数据库不可用时禁止退回 YAML/config 创建或启动。旧未绑定数据不迁移、不认领、不删除，不能靠同名、Code、路径或 externalStoreId 自动匹配。首次创建只使用 Manager 管理的新 UUID 目录。

### 2. 本机接口边界（命名为拟定合同）

| Go/Wails 动作 | 输入 | 输出/要求 |
| --- | --- | --- |
| StoreWorkbenchContext/List/Get | generation、企业选择、受限筛选 | 通过 Go 获取原服务端 DTO；不返回 token，缺后端不得回退 mock 店铺 |
| StoreEnvironmentSave | storeId、generation、企业 revision、operationId、可编辑配置、expectedBindingVersion（首次为 0） | Go 生成/读取 binding/profile 身份，先服务端 configure 准入，再原子本地保存；成功返回绑定版本 |
| StoreEnvironmentOpen | storeId、generation、企业 revision、operationId、expectedBindingVersion | Go 解析 profile；先新准入，再复用原启动链；已运行时不重复启动 |
| StoreEnvironmentState | storeId 与当前上下文 | 只返回当前用户授权绑定的本机状态；历史结果与当前观测分开 |
| StoreEnvironmentClose | storeId 与当前上下文 | 正常关闭复用原链；无业务权限时仅可转入原本机 owned cleanup，不扩大其参数/证明 |

上述接口不接受调用者指定的 owner、目录、PID、端口、任意命令或 shell。配置页复用 BrowserEditPage 的正常 core/proxy/指纹字段，受管目录不可编辑；launch/fingerprint 参数须按既有参数解析方式拒绝覆盖 user-data-dir、调试监听、代理栈或访问其他档案的逃逸参数，不能仅做字符串黑名单。

### 3. 所有旧入口都消费同一受管边界

原 Manager 加有界 action policy，保持原 appState 指针、锁和启动协议；所有可触达 profile 的 Wails、HTTP、CDP、automation、后台计划、Code/别名解析最终须检查 canonical profile 的 binding。禁止在 renderer 先筛选后继续调用任意旧方法。

该 policy 必须在原 startupBusiness 装配的定时器/后台执行者启用前注入；不能先启动旧计划再等待店铺页面安装策略。数据库/binding 状态不可读时，受管数据相关动作关闭。只读加载到私有 Manager 内存不等于授予对外访问。

- 当前账号/企业之外的受管 profile 在列表、搜索、标签、回收站、日志、启动 Code、代理/扩展引用投影中不返回；不能按裸 profileId 读取。
- 共享代理/扩展引用、全局日志和批量写入同样检查其受管影响集合：不能向另一 owner 返回相关秘密或更改其受管实例使用的配置。无法证明只影响当前有权范围时，拒绝敏感读取/写入或仅返回原有安全公共字段；不新增另一资源权限系统。原内核版本等无私密信息的公共只读目录保持可用。
- 受管 profile 的配置/启动只能通过上述店铺用例。旧直接启动、克隆、删除/恢复、导出/导入、自动化和原始 CDP 控制入口首版拒绝，即使是管理员也不能用它们解除归属。
- 原启动链内部为一次受准入 open 进行的既有进程管理/debug 接管仍可复用，不向调用者泄漏新 CDP 能力。代理校验后的“直连启动”确认也是新动作，须重新检查上下文/准入，不能绕过绑定。
- 全量/选中备份、S3/OpenList 定时备份、恢复和迁移若会包含/覆盖受管实例、数据库或其目录，首版在预检阶段拒绝，不悄悄跳过数据然后报“全量成功”。无受管数据的原范围不变。运行中的旧计划同样检查，不能只有新 UI 限制。
- 未绑定旧 profile 的通用入口不得选用受管目录或其符号链接/junction/规范化别名；受管目录也不能指向旧 profile。沿用原维护互斥与路径规范化，保护集合包含所有 binding，与当前列表可见性无关。
- 不修改 OS 权限/加密模型，不将本机文件被特权用户修改纳入应用防篡改承诺。

### 4. 本机操作 journal 与 UNKNOWN

在相同 SQLite owner 增加 `browser_store_operations`：operationId、bindingId/profileId、原身份/企业、action、请求指纹、原绑定版本、阶段、单次执行 attempted 标记、安全结果及报告状态。仅此业务需要的有界 journal，不增加通用队列或后台重试服务。

执行顺序：持久化 PREPARED → 请求服务端准入 → 保存首次准入 → 原 generation/版本检查 → **持久化 attempted 后**执行一次本机动作 → 固定本机结果 → 一次结果报告。任一持久化失败，禁止开始尚未发生的副作用。相同 operationId 不执行第二次；同时同一 binding 只允许一个未决配置/打开动作。

进程启动、提交或响应丢失不能确认时记 UNKNOWN，保留本机 journal、已有诊断和 operationId；重启不重放 PREPARED/ADMITTED/attempted，不替换 operationId 自动继续。只允许主动只读核对原回执和原进程/目录证明；无法判断则继续阻断该 binding 的新打开。明确观察到不存在活动进程且无未决本机写入后，可以记录核对证据，由用户发起一个新的独立动作；旧 UNKNOWN 不被覆盖成成功。已运行则返回当前观测，不重复启动。不能凭空进程列表或只看 PID 就宣布无副作用。

## C3. 服务端动作准入与回执 API

### 1. 新增端口与路由

新路由归 `internal/storecenter/httpapi`；业务合同和用例归 Store owner，app/httpapi/currentapplication 只组装既有认证、权限、Store DB 和时钟。无新服务、端口、worker、消息系统或 legacy 管理依赖。

| 方法与路径 | 认证/企业策略 | 合同 |
| --- | --- | --- |
| POST `/api/v1/workbench/stores/:store_id/browser-operations` | CurrentIdentity＋LiveWrite＋browser.use；用例再校验 store.read/Store grant | configure/open 单次准入 |
| GET `/api/v1/workbench/stores/:store_id/browser-operations/:operation_id` | CurrentIdentity＋None；用例仅按当前 verified issuer/subject 查原回执 | 仅结果恢复读取，无执行能力 |
| POST `/api/v1/workbench/stores/:store_id/browser-operations/:operation_id/reports` | 同上；必须绑定原动作与原主体 | 固定类型的本机结果报告 |
| GET `/api/v1/workbench/stores/:store_id/audit` | CurrentIdentity＋LiveWrite＋store.read；用例执行 Store 范围检查 | 见 C4，只读分页 |

None 仅用于原操作回执/报告，不能复制到新动作。现有 [server_auth 中间件](https://github.com/qq550723504/shuomi-ai-commerce/blob/d8d30f6b0e8dfa903e02b002ef12f5747610c28f/internal/app/httpapi/server_auth.go) 会清空企业 scope/roles/grants 并删除 X-Requested-Organization-ID，因此这两条接口完全不依赖该 header 或 EffectiveOrganizationID。由 path 的全局唯一 store_id＋operation_id＋verified issuer/subject 定位原操作，original organization 只从固定操作行取得；不从当前角色构造新企业权限，不重新调用需要 Store grant 的店铺 Get 来定位。原主体不匹配统一 404。

Store grant/成员被撤销后允许同一有效身份报告原结果；token 无效/注销后不可联网补报，先留本机证据，之后仅本人重新登录并主动报告原结果，不自动重试或重新执行。新的 membership ID 不改写旧 ActorMemberID。管理员不能冒充原操作者报结果。

### 2. POST 准入

要求唯一 `Idempotency-Key`，为 canonical UUID 且等于本机 operationId；store_id 同样为 canonical UUID。请求使用明确字段，拒绝额外字段和 null：

```json
{
  "schemaVersion": "store-browser-v1",
  "action": "open_environment",
  "bindingId": "11111111-1111-4111-8111-111111111111",
  "profileId": "22222222-2222-4222-8222-222222222222",
  "bindingVersion": "1"
}
```

action 仅 `configure_environment` 或 `open_environment`；bindingVersion 为正整数十进制字符串；配置首次使用已生成候选 binding/profile 和版本 1，后续用拟提交的新版本。profile/binding 引用只用于关联，服务器不把它们视为可信设备或商家绑定。请求不含用户资料、目录、Cookie、配置内容或商家凭据。

Store DB 新增 owner 内的 `workbench_store_browser_operations`，唯一键 `(organization_id, operation_id)`；固定 verified issuer、subject、ActorMemberID、storeId、action、客户端 binding/profile/version、服务器准入时间/窗口、规范化请求指纹和结果状态。同一事务写 `browser_operation_admitted` Store 审计事件。新增表是该用例的原始操作回执，不是第二个 Store、grant 或启动状态 owner；schema 仍由原 Store schema owner 管理，禁止运行时借本轮文档安装。

首次成功返回 201，包含原 userId/organizationId、operationId、action、admittedAt、executionWindowMs=15000 和 `executionAuthorized: true`；仅当前请求能获此首次执行响应。重复相同 key/主体/载荷返回 200 原回执，且 **executionAuthorized: false**；任何 GET 永远 false。主体不同按 404 处理，原主体同 key 不同载荷 409。重复 POST 仍经过当前实时授权，撤权后不能用幂等命中取得执行能力。

客户端必须持久化并 CAS 消费首次响应；并发响应或重复 UI 点击不能重复执行。超时/响应丢失即 UNKNOWN，不自动 POST 第二次；GET 查到“已准入”仍不足以执行。每次执行只能使用当前操作的首次响应，不能把回执查询当可恢复的启动令牌。

拒绝返回安全 code，不泄漏未授权店铺名称/存在性。已验证主体的范围内拒绝可写固定字段的 `browser_operation_denied` 审计；其目标只表示尝试引用，不证明店铺存在。身份无法验证则沿用认证安全日志。拒绝日志写入失败不放行副作用，不能把“无记录”展示为“无人尝试”。

### 3. 结果报告

```json
{
  "schemaVersion": "store-browser-v1",
  "kind": "open_result",
  "outcome": "succeeded",
  "reasonCode": "process_started",
  "observedAt": "2026-10-25T12:00:00Z"
}
```

kind 仅 `configure_result`、`open_result`、`close_result`。configure_result 只能对应 configure 操作；open_result/close_result 只能对应原 open 操作。outcome 仅 succeeded/failed/unknown。reasonCode 为固定枚举：configuration_saved、configuration_failed、process_started、already_running、launch_failed、context_changed、execution_window_expired、owned_close_confirmed、owned_close_failed、result_unknown；按 kind/outcome 校验组合。observedAt 为客户端声明，服务器另存 receivedAt；自由文本、PID、URL、命令行和扩展 payload 拒绝。

| kind | succeeded reason | failed reason（有明确未完成证据） | unknown reason |
| --- | --- | --- | --- |
| configure_result | configuration_saved | configuration_failed、context_changed、execution_window_expired | result_unknown |
| open_result | process_started、already_running | launch_failed、context_changed、execution_window_expired | result_unknown |
| close_result | owned_close_confirmed | owned_close_failed | result_unknown |

未列出的组合返回 400。configuration_failed/launch_failed 只用于确认未提交/未启动的失败；已经写入或启动但无法判断结果时必须 unknown，不能借错误返回值假装已回滚。owned_close_failed 只表示明确未关闭，端点/进程身份不确定必须 unknown。

唯一键 `(organization_id, operation_id, kind)`；原主体完全相同报告幂等返回原结果，不同载荷 409，不覆盖原 UNKNOWN/失败。报告与 Store 审计事件在一个 Store DB 事务提交。没有报告时投影为 `result_unconfirmed`，不改成失败，也不自动补发本机操作。close_result 是原窗口的客户端关闭观察，不要求旧 open_result 必须成功，允许已启动但原结果回传丢失的安全关闭证据。

服务端只能证明身份验证、准入/拒绝和报告收到；不能证明进程已启动、窗口已关闭或商家操作成功。接收结果报告不检查 browser.use，也不产生新执行授权。服务端 Record 的现有通用幂等比较不固定 actor，故本用例须先在原操作行比较完整主体与规范载荷，不能仅依赖 AuditRepository.Record。

### 4. 错误与重试

| HTTP/code | 含义与客户端行为 |
| --- | --- |
| 400 INVALID_REQUEST | 字段、UUID、版本或枚举错误；不执行 |
| 401 AUTHENTICATION_REQUIRED | 原会话失效；清空业务视图，不自动重新登录 |
| 403 PERMISSION_DENIED | 有效企业内缺少动作能力；不执行 |
| 404 STORE_OR_OPERATION_NOT_FOUND | 资源不可见、原主体不匹配或目标不存在；不泄漏详情 |
| 409 OPERATION_CONFLICT / BINDING_VERSION_CONFLICT | 幂等载荷或本机版本冲突；刷新核对，不自动覆盖/重放 |
| 503 AUTHORIZATION_UNAVAILABLE / AUDIT_UNAVAILABLE | 依赖未确认；不执行；请求可能提交但未获响应时为 UNKNOWN |
| 超时、连接中断、响应解析失败 | 未获首次有效准入则不执行；保留 operationId，主动只读查证，不自动重试 |

新动作、报告、查询均禁止 HTTP 客户端自动重试；UI 明确的“查询原结果”只做 GET，不能偷偷重新 POST。一次本机动作成功但报告失败，保留“本机成功／服务器报告未确认”两列事实，不重复动作来补审计。

## C4. 按店铺的业务审计查询

Store owner 对外提供有界 AuditHistoryReader，从原成员授权 operations 和 Store 审计/浏览器 operation/report 读取原始事实。新浏览器 action、source、operation reference、profile/binding reference 使用固定类型扩展现有审计 schema；不借 SafeFieldNames/任意 JSON 塞新 payload，不更改旧事件含义。授权行的当前 active/version 不用来重建历史，原 actor、版本、时间和 grant/no-op 回执保持。

GET audit 参数仅 `limit`（1..100，默认 20）、`cursor`、`actor`、`action`、`outcome`、`from`、`to`；拒绝重复参数和未知筛选。时间 UTC、闭开区间，首次未提供时间时服务端固定 to=当前时刻，默认 from=to-30天，最多 90 天窗口；cursor 固定同一窗口。普通成员由服务端强制 actor=self，不能查询别人；管理员采用现有组织/Store 管理门禁，不因前端传 admin=true 提权。已删除店铺的历史仅同企业授权管理员按 Store tombstone/原审计范围读取；普通成员无 grant 不再有全量审计读取权，但自己的原操作 GET 仍按 C3 可查。

响应：schemaVersion、verified userId/organizationId、storeId、items、nextCursor、coverage。每项仅 eventId、operationId、actorSubject/memberRef、action、serverTime/clientObservedAt（明确来源）、outcome、reasonCode、profileRef/bindingRef 和 factSource；不带绝对目录或原配置。coverage 明确只含已有成员店铺授权回执和本版应用命令，不声称覆盖商家网页活动、旧技术日志或所有历史行为。

分页采用 owner 提供的稳定倒序 `(recordedAt, sourceRank, sourceEventKey)` 位置，cursor 绑定 issuer/subject、企业、store、actor、筛选窗口和源位置；不能跨企业、店铺、用户或筛选复用，非法 cursor 返回 400。先服务端授权/过滤再分页，不把其他人的记录/总数取回客户端再隐藏。读取来源失败返回 unavailable，不能当作空列表；事件的业务时间与收录时间分别保留，不宣称跨 owner 全局因果序。

原 profile-delete-audit.jsonl、LaunchCallRecord 和 stderr 继续是技术诊断，不自动变成具有 verified actor 的业务审计。账户审计 `/api/v1/account/audit` 的既有响应和 source_account.read 门禁保持；本版不将店铺事件塞入该接口。
