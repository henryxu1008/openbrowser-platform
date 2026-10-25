# 最小实施任务、验收与阻塞

此文件只列产品实施任务，不派工、不创建产品执行 Issue、不执行产品测试。用户本轮另已授权补齐 UI 设计依据及提交设计文档 PR；这不授权产品实施、合并或部署。实施使用一个用户结果的连续交付，跨两个仓库各保留必要 PR，不按文件/测试数量机械拆分；不重复派发硕米已有 Store、组织权限或浏览器管理实现。

## 1. 实施任务与依赖

| ID | 用户可见结果/最小工作 | 现有 owner 与注入路径 | 准入与依赖 |
| --- | --- | --- | --- |
| T0 | 确认本版设计及 UI 增量；记录当前产品指令、scope、Design Basis、owner | 本设计＋[UI.md](UI.md) 的 Figma 节点/现有页面/桌面差异映射；执行 Issue 引用，不重做旧页设计 | UI 依据本轮补齐；架构增量审阅结论见 REVIEW；实现仍需用户实施授权及执行 Issue |
| T1 | 桌面可取得当前企业及获授权店铺 | 现有 workbenchcontext/store-catalog → 原认证中间件；浏览器 desktopauth → 受限 Go API client → Wails 投影 | C1；Native token 兼容先用受控 fixture，真实实例核对留后续授权阶段 |
| T2 | 店铺动作与授权、审计准入闭环 | Store 用例/port → Store DB 原事务与 AuditRepository → storecenter/httpapi → app 组装；现有 authz 新增默认关闭模块 | C1/C3/C4；原 Store→grant 锁顺序；同步现有 route descriptor 白名单/装配校验而不移除 guard；不得新建 IAM/服务 |
| T3 | 首次配置持久绑定、再次打开复用原实例 | 原 Manager/ProfileDAO 增量事务＋同库 binding/journal → 原 AuthBoundApp/调度 → 原配置/启停页面 | T1/T2；C2；拒绝 Create 后补 binding 的不原子方案 |
| T4 | 权限在直接 API、后台与数据传递入口一致 | 原 action policy/调度、Code selector、备份/恢复/目录检查 | T3；所有受管读写入口完成枚举；任何遗漏为阻塞，不能仅做 UI 隐藏 |
| T5 | 店铺工作台与操作记录可用 | UI U2..U6；现有 Store 列表/详情语义、BrowserEdit/List/Detail 完整流程、原授权对话框 | T1..T4；受管容器适配 scope/动作/返回与异常，不能只换标题挂旧 hook；缺真实数据展示不可用，不能 mock 成功 |
| T6 | 验收与交付审阅 | 原离线测试、现有 CI、最终差异与架构约束 | 只运行与实际实现有关的检查；真实产品/IAM/发布仍另行授权 |

新增公共合同冻结在 CONTRACTS.md；实现中仅替换端口的具体接线，不在 app 组装层编写 Store 业务。若必须变更事实 owner、权限/撤销语义、绑定所有权或 UNKNOWN 协议，先以具体 BLOCKER 回到本设计，不用新框架补洞。

## 2. 验收矩阵（均为待实现，当前 NOT_RUN）

| 编号 | 必须保持的行为 | 最小、确定性证据 |
| --- | --- | --- |
| A01 身份/API | 同一 generation 的 Go token 调用固定目标；不向 renderer/其他 origin 泄漏；缺配置不能假数据解锁 | 现有会话/HTTP fake transport：重定向、非法 origin、超时、换账号途中返回；无真实 IdP |
| A02 企业与成员 | A/B 企业隔离、home org 不代替选择值；未分配店铺无列表/详情/分页泄漏；退企重入不继承旧 grant | 复用现有成员授权 fixture，加企业/成员与 query/cursor case |
| A03 动作权限 | store.read、browser.use 与 Store grant 共同生效；不隐式要求 store.update；负责人与授权独立；自定义企业角色按现有投影判定 | Casbin/role policy fixture，只有 read＋use＋grant 无 update 也能准入；撤销该 grant 则拒绝；默认不开通、新模块非隐式继承、依赖失效关闭 |
| A04 撤销顺序 | Store grant 撤销先提交拒绝；准入先提交只允许原短时单次动作；退出/切企业后的迟到准入不执行 | channel/barrier 控制原 Store 事务及 native admission seam；不靠 sleep、真实登录或进程 |
| A05 绑定原子性 | Profile、binding、journal 同事务；保存失败不留下可通过旧入口使用的无归属实例；两次首次配置用不同候选 bindingId 仍只提交一个 Profile＋binding，失败请求不污染 Manager；重启读回同一绑定 | 临时 SQLite/fault injection＋并发 barrier；复用默认配置/DAO；受控文件 fixture，不创建产品窗口 |
| A06 目录/所有权 | 同用户同店铺复用，其他 subject/org 不可读写；目录别名/修改参数/旧 profile 不能访问受管数据 | 原路径解析、junction/参数解析单元 seam；不扫描真实档案或改 OS ACL |
| A07 全入口覆盖 | Wails、HTTP、Code、后台、clone、trash、import/export、全量/定时备份均检查同一受管 policy | 原真实 facade/router/调度 seam 的表驱动测试；受管页面挂载/运行事件不得触发旧 tabs/CDP、全局资料 hooks 或日志清空/轮询；不能只测试新 detached helper |
| A08 单次/UNKNOWN | 双击/并发相同 op 仅一次；重复 POST/GET 无执行能力；attempted 前后崩溃、响应丢失不重放 | 本机 journal 及 HTTP/launcher seam 故障注入；断言 cmd.Start/预热调用计数，不运行内核 |
| A09 回执与审计 | 原主体/原操作不可改写；相同报告幂等、不同载荷冲突；撤权后本人能报告旧结果但不能新开 | 真实 router/middleware 清除组织 header 后仍按 path＋verified主体取得原行，伪造 header 无效；Store 同事务 fixture；原成员授权/no-op 回执、分页源排序、跨范围 cursor、来源不可用 |
| A10 事实与脱敏 | 服务端准入、本机结果、当前运行观测、商家状态区分；缺报告非成功；输出无 token/Cookie/绝对目录/命令行 | DTO/allowlist 与 mock 结果组合；检查审计和错误输出 |
| A11 关闭与保留 | 撤权不杀旧窗口；安全关闭只接受原 owned proof；未知关闭不自动重试；历史未绑定资料保留 | 原 cleanup proof/monitor seam；PID 复用、端点不匹配；不操作真实进程 |
| A12 原流程 | 原 Manager、appState、内核发现/安装路径、代理两栈、现有存储生命周期不被新实现复制或绕过 | 对相关原回归/现有 CI 按实际 diff 复用和增量验证；不再运行已冻结产品验收 |
| A13 用户路径 | 登录→选企业/店铺→首次配置→再次打开→正常关闭→查看记录；返回筛选、无权/空/失败/UNKNOWN 状态明确 | 离线 UI fixture；补原 Modal 的可访问名称、Escape/关闭焦点恢复，不能假定原组件已满足；覆盖无旧 hook 副作用；真实 GUI/内核/商家页验收另行授权，当前 NOT_RUN |

只为本次安全/持久化边界补必要回归，不新增专项验收平台或要求用户跑脚本。测试文件、fixture 和检查命令在实现时依据实际改动确定；不能把本表当已通过测试数量。

## 3. 阻塞分层与退出条件

| ID | 当前状态/影响 | 解除方式与责任 | 不阻塞的工作 |
| --- | --- | --- | --- |
| G1 实施授权 | `NOT_AUTHORIZED`；当前用户只授权文档与审阅，不允许产品编码 | 用户后续明确实施；执行 Issue 记录设计依据及范围 | 本轮文档、审阅、任务分解 |
| G2 新 UI authority | **`RESOLVED_DESIGN`**；本轮实际核对现有 Figma，并在 UI.md 补齐三流程的复用/桌面差异，增量独立审阅通过 | REVIEW 已记录，本设计基线为 IMPLEMENTATION_READY；UI-R1/R2 由 A07/A13 的实施回归承接，不重画 Figma 或修改原合同/事实 owner | 设计文档 PR；正式实施仍受 G1 限制 |
| G3 Native API token | `UNVERIFIED`；当前 Native 登录通过不证明 access token 在目标硕米 API 可用；若不兼容会阻塞真实用户路径 | 复用现有 Native client/API verifier 的公开参数与受控合同验证；真实 token 交互须另有运行授权；不擅自改 IAM/换客户端 | 本轮合同及后续受控 fixture；不能冒充真实联调 PASS |
| G4 生产目标 | `DEFERRED_BY_USER`；对外 issuer/clientId/API origin 发布前确定 | 依原决定在发布前核对维护者公开配置 | 设计、获授权的代码与离线测试；不作为本轮设计 BLOCKER |

G1 是授权边界，G2 是本轮补齐并增量审阅的 UI 设计依据，G3 是真实联调/试用阻塞，G4 是发布条件；不要把它们混写成“设计因缺公网配置而无法完成”。最终设计准入与实施授权分别记录在 REVIEW.md；设计就绪也不授权修改正式生产路径。

## 4. 证据复用与完成状态

可复用：固定版本的 Store/企业权限源码合同，原浏览器登录、Manager、内核、代理与 cleanup 的已有离线证据。须注明原 SHA/边界；旧登录 GUI 证据不证明 Store API token、新绑定或审计的可用性。

本轮检查 Figma 只读依据、文档引用/一致性、diff 范围及架构增量，并按用户授权提交设计文档 PR。产品实现、schema 安装、本机离线产品回归、真实 GUI/内核/IAM/商家站点、发布与部署均 `NOT_RUN`。原仓库 PR CI 会按既有工作流独立触发离线测试/编译，状态以实际 run 为准，不等于产品验收；不修改工作流，不手动触发发布或真实运行。
