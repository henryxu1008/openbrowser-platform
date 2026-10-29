# 店铺工作台实施交接

实施依据为本目录已合并的 README / CONTRACTS / UI / IMPLEMENTATION / REVIEW；执行 Issue 为浏览器 [#14](https://github.com/qq550723504/openbrowser-platform/issues/14) 和硕米 [#954](https://github.com/qq550723504/shuomi-ai-commerce/issues/954)。本文件说明代码入口与交接边界；固定 HEAD、CI 和独立审阅的滚动证据保留在实施 PR。

## 用户入口

安装包维护者确认并随包提供公开配置后，用户直接从应用快捷方式登录，在侧栏进入“店铺工作台”：

1. 选择已有企业；列表只使用硕米 Store catalog 返回的当前授权范围。
2. 进入店铺，点击“环境配置”；复用原浏览器表单的名称、内核、历史标签、内存、指纹和本地代理字段。保存后返回店铺，不自动打开。
3. 点击“打开环境”；首次服务端准入、持久化 attempted 标志后，调用原 Manager 和原两栈代理/内核启动流程。再次打开已运行环境只做获授权的当前观测。
4. 点击“正常关闭”；仅使用本机原 owned process/目录/端点证明，不按裸 PID 杀进程。关闭结果不返回完整环境资料。
5. “操作记录”显示 Store 授权回执、服务端准入及客户端报告。记录不等于商家页面操作成功；无清空、导出或重放入口。

路由为 `/stores`、`/stores/:storeId?tab=environment|records`、`/stores/:storeId/environment/edit`。保留原首页与独立入口；列表查询条件随返回链接保存。组织切换、原生 generation/revision 改变和迟到响应不重新绑定原动作。

企业切换先清空旧页面并替换为 `/stores`，再发布新上下文；不把上一企业的店铺 ID、记录页签或筛选带入新企业。选择当前企业不提交请求，未保存表单仍先确认放弃。旧编辑器离开后，即使原保存请求完成，也不能再导航回原店铺；原操作结果和归属仍由 Go/journal 保留。

企业上下文及店铺详情读取按本次请求标识发布结果，过期响应不能覆盖新 DTO、错误或加载状态。详情页离开后不继续发起下一段环境读取；读取与打开/关闭在页面内互斥，保留原生准入、CAS 与 UNKNOWN 规则。对应离线回归为 `frontend/test/store-workspace-lifecycle.test.jsx`，使用真实页面/Router 和受控 API Promise，不代表真实 Native 登录、店铺授权或内核验收。

## 原能力的接入位置

- `internal/shuomiapi`：Go-only 固定 HTTPS origin、原 Native 内存 token、既有 context/catalog/summary，加 C3/C4 四条接口；无 renderer token、任意 URL/header 或重定向。
- `backend/store_workbench.go`：店铺用例与原 AuthBoundApp；前端只调用固定命名方法。
- `backend/internal/browser/store_binding.go`：复用 Profile 原规范化及同一 ProfileDAO SQL，同库提交 Profile、immutable binding 与 journal 结果后才发布原 Manager 内存。
- SQLite migration 18 仅追加本机 binding、操作与报告表；PREPARED 意图在远端请求之前持久化，事务失败不会删除该证据。重启不自动执行旧操作。
- `BrowserEditPage` / `BrowserDetailPage` 的显式 managed adapter 不挂原全局 profile、tabs/CDP、日志清理或实时指纹检查 hooks。原 Modal 增补可访问名称、Escape、焦点循环和关闭焦点恢复。

## 失败、进程与审计合同补充

- 可信明确的 400/401/403/404/409 拒绝记 `failed / admission_rejected`，保留原证据；权限恢复后允许用户发起新动作，不自动重试。
- 超时、断连、无执行能力的重复回执、迟到准入及无法确定结果记 UNKNOWN，保留原 operationId；未决唯一槽位阻断换键继续。只读回执不会执行。
- 原操作回执按 verified issuer/subject、固定 apiOrigin、storeId/operationId 找本机原操作并派生原企业；即使当前企业选择或授权已丢失，也不改用当前企业冒充原归属。原会话 generation 必须仍有效，换主体拒绝；不产生新准入。
- 冷启动不把磁盘加载的 `Running=false` 当退出证明。有历史 attempted open 时，只有原目录进程枚举明确完整成功且没有匹配进程，才在原 journal 追加 `runtimeObservation / runtimeObservedAt` 并允许未来独立用户动作；原成功/UNKNOWN 结果不被覆盖。有残留、未就绪或查询失败保持未知，不清理、不杀、不重试。该跨进程核对目前只实现 Windows，其他平台保持未知。
- C4 响应增加只读 `canFilterActor`，来自服务端原 Store 管理员判定，用于决定是否显示操作人筛选；它不授予查询权限，服务器仍验证 scope/actor/cursor。

## 安装和发布前条件

沿用 `bat/publish.ps1` 和 NSIS；维护者为 Windows 发行同时提供 `-PublicIdentityConfig` 和 `-PublicStoreConfig`。用户不运行这些脚本。基础登录配置为 issuer/clientId；启用店铺功能时，身份文件还须包含维护者确认的公开 apiProjectId，沿原 PKCE 请求固定资源 scope，详见 CONTRACTS C1。后者仅为：

```json
{"apiOrigin":"https://confirmed-shuomi-api.example"}
```

示例不是发行目标。两配置由维护者在发布前核对；缺失或非法时店铺功能关闭，不回退到其他实例。未提供 secret 或新增 IAM client/角色。

硕米后端须先具备配套 C3/C4 代码及 Store-owned 两表。新增权限模块 `store-browser` 默认不分配，显式包含已有 store.read 与新增 browser.use，不包含 store.update；旧 `stores` 模块不继承 browser.use，原角色最多 32 模块的合同保持。

保留库安装由原 Store schema owner 的显式 `--browser-workbench --confirm-database <精确库名>` 模式负责；与原 `--align-core-contract` 互斥。此源码入口只添加新表及其 SELECT/INSERT，并核对实际权限；运行时不建表、不自动升级，当前没有执行该命令。

## 当前限制与验收边界

首版在本机存在受管绑定/操作证据后，对不能证明影响范围的旧全局 profile、代理/扩展、技术日志、自动化、备份/导入/恢复与共享内核写入口统一关闭；公开内核目录只读可用。旧资料保留，不迁移、不认领、不删除；受管表单只使用已安装内核和本机输入代理，不自动继承旧代理池秘密。共享内核维护仍通过维护者的安装包更新处理，本版不开放跨 owner 的通用修改/删除。

真实 Native token 到硕米 API 的兼容性、公开发行实例、真实 GUI/内核/商家流程、保留库安装和发布部署均未验证或未执行。离线测试/编译/CI 不能代替用户验收；后续试用须单独授权，并使用已有应用入口，不要求用户脚本启动。

## 2026-10-29：首次运行参数与正常关闭后续修复

上节未执行项保留其原交付时点。后端 owner 随后在用户单独授权下，用专用普通成员和单店最小权限，在原 `f1c9db0` 桌面与 `fb5389390` 后端组合完成配置、打开和报告保存。正常关闭返回 UNKNOWN；用户手动关闭后，原 owner 已核对目标进程为 0，原 UNKNOWN 关闭报告及配置保留。此阶段为局部试用，不是完整正常关闭或产品验收通过，管理员角色未改。

本次源码跟进确认：Store 为拒绝任意启动参数和旧代理池而跳过原 ApplyDefaults，导致首次运行参数也被丢弃；现场进程证明同样记录缺少 `--no-first-run`。在原 `prepareBrowserLaunchContext` 中仅对受管 Store 追加固定 `--no-first-run` 和 `--disable-background-mode`，不引入可变全局参数，不把这些内部参数持久化进受管 Profile。共享启动、Manager、代理连接栈和用户数据归属合同继续复用。

关闭仍须原始启动回执、当前 monitor/command、PID 创建时间、目录和 loopback 端点证明，仅发送一次 Browser.close。失败现在保留有限、脱敏的阶段，例如归属变化、端点未证明、CDP 交换未确认、进程退出未确认；不返回原生命令行、目录、端点或底层查询错误。报告仍使用既有 close_result / unknown / result_unknown，原始报告不可改写，未确认时不强退、不重试、不伪装已停止。

新增缺失参数、错误阶段丢失、确认 CDP 但进程不退出三项回归均先 RED，再在修复后 GREEN。相关 Store/AuthBound/Windows proof race 测试及 backend vet 通过。Windows 原生证明测试仅启动当前 Go 测试程序作为合成 HTTP/WebSocket 子进程，直接执行真实创建时间、CIM 目录和监听端口核验，没有启动 Chrome、产品、店铺环境或共享服务。

这些证据证明启动参数缺口与诊断丢失已修正，**没有证明旧 UNKNOWN 的唯一原因或新的真实 Chrome 正常关闭通过**。原试用目录、配置、报告、账号授权和管理员权限保持；受保护管理员授权的 B1 仍由后端 owner 单独处理。本次不部署、不操作现有环境、不关闭执行 Issue，新源码不能沿用原两 PR 的合并授权。

## 2026-10-29：详情只读刷新同时更新权限上下文

后续关闭复测和PR17合并已完成，记录见该PR及原owner回执；上节是当时源码修复阶段。新的桌面配套核对发现：详情只读刷新只重新读取Store/environment，按钮仍消费工作台初次加载的权限快照。用户已授权本轮本地修复、离线回归及只读核对后端最终交付，不授权部署、现有权限更改或店铺操作。

Design Basis：Reuse Existing Architecture。复用现有StoreWorkspace的context读取、requestEpoch、清空旧scope、错误后显式核对以及原详情加载流程；只把原“只读刷新”接到该owner，不增加第二份权限状态或新API。刷新先让旧详情退出，再按新context重建；查询参数和同企业深链保留，企业变化先回列表，主体不一致或读取失败不发布旧权限控件。读取epoch也参与Outlet key，覆盖React并发批处理下context立即返回且原生revision不变的情况。

旧回调和迟到响应按原epoch失效；打开/正常关闭进行中不发起上下文刷新，新context读取中不能用旧打开回调下发操作。刷新仅重读既有接口，不进行企业切换PUT、角色写入、配置/打开/关闭或原回执重发。UNKNOWN继续来自原持久化记录，重新加载后仍阻断新操作。服务端短缓存和每次操作的LiveWrite准入不变；这次修复保证消费服务器返回的权限，不宣称真实IAM即时投影。

六例新增权限/失败/上下文/UNKNOWN回归先RED后GREEN；并发React的同revision立即响应曾独立复现详情未重建，再以读取epoch修正。最终相关Store生命周期与受管UI两组测试共25例通过、无跳过，TypeScript和生产前端构建通过。工件在`build/dist/store-context-readonly-refresh-20261029-01/`；全部为模拟API/离线构建，未启动桌面、Chrome或店铺环境，未更新保留EXE、登录、权限、配置或数据。没有新Windows EXE实机验收、新远端CI、推送、合并或部署。
