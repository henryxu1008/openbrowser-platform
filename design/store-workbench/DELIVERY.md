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

沿用 `bat/publish.ps1` 和 NSIS；维护者为 Windows 发行同时提供 `-PublicIdentityConfig` 和 `-PublicStoreConfig`。用户不运行这些脚本。前者仍严格保持 issuer/clientId 两字段；后者仅为：

```json
{"apiOrigin":"https://confirmed-shuomi-api.example"}
```

示例不是发行目标。两配置由维护者在发布前核对；缺失或非法时店铺功能关闭，不回退到其他实例。未提供 secret 或新增 IAM client/角色。

硕米后端须先具备配套 C3/C4 代码及 Store-owned 两表。新增权限模块 `store-browser` 默认不分配，显式包含已有 store.read 与新增 browser.use，不包含 store.update；旧 `stores` 模块不继承 browser.use，原角色最多 32 模块的合同保持。

保留库安装由原 Store schema owner 的显式 `--browser-workbench --confirm-database <精确库名>` 模式负责；与原 `--align-core-contract` 互斥。此源码入口只添加新表及其 SELECT/INSERT，并核对实际权限；运行时不建表、不自动升级，当前没有执行该命令。

## 当前限制与验收边界

首版在本机存在受管绑定/操作证据后，对不能证明影响范围的旧全局 profile、代理/扩展、技术日志、自动化、备份/导入/恢复与共享内核写入口统一关闭；公开内核目录只读可用。旧资料保留，不迁移、不认领、不删除；受管表单只使用已安装内核和本机输入代理，不自动继承旧代理池秘密。共享内核维护仍通过维护者的安装包更新处理，本版不开放跨 owner 的通用修改/删除。

真实 Native token 到硕米 API 的兼容性、公开发行实例、真实 GUI/内核/商家流程、保留库安装和发布部署均未验证或未执行。离线测试/编译/CI 不能代替用户验收；后续试用须单独授权，并使用已有应用入口，不要求用户脚本启动。
