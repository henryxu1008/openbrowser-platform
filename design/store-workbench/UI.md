# 店铺工作台 UI 设计依据与桌面适配 v1

状态：**`UI_DESIGN_BASIS_READY`**；本轮增量独立审阅通过，仅设计，尚无页面实现或产品运行验收。就绪与授权分开，见 [REVIEW](REVIEW.md)。本文件解除原 G2，不改变 [授权、绑定与审计合同](CONTRACTS.md)。

## U1. Authority、调查与取舍

产品依据：2026-10-25 用户明确要求“先复用现有 Figma 规范和浏览器页面，补齐店铺工作台、环境配置及操作记录的 UI 设计依据，更新架构审阅与实施就绪结论，并把设计文档提交 PR”。这是本版桌面适配的产品范围，**不是产品编码、真实运行、IAM、合并或部署授权**。

设计依据链：该用户决定 → 硕米[最终 UI/IA authority](https://github.com/qq550723504/shuomi-ai-commerce/blob/d8d30f6b0e8dfa903e02b002ef12f5747610c28f/docs/product/final-ui-ia-authority.md)与[Store 页面已批准取舍](https://github.com/qq550723504/shuomi-ai-commerce/blob/d8d30f6b0e8dfa903e02b002ef12f5747610c28f/docs/architecture/store-center-figma-v1.md) → 本轮实际读取的可见 Figma 节点 → 浏览器现有完整页面 → 本文件的桌面差异及状态。新页面不以包名、示例数字或旧截图中的未开放能力决定产品范围。

2026-10-25 只读核对 Figma 文件 `tg48P46SSXl6TBy9lZwg63`、页面 `31:463`「硕米官网」的以下可见节点；已读取结构/文字并查看店铺深浅主题、设置、操作记录及浅色详情截图。没有写入 Figma、创建新画布/组件或启动浏览器产品。**本文件是复用依据上的桌面适配规范，不声称新增三张 Figma 成品页已经绘制。**

| Evidence | 本轮实际节点与名称 | 采用的结构 | 本版明确不采用 |
| --- | --- | --- | --- |
| F1 | [1560:359](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1560-359) / [1560:716](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1560-716)，我的店铺优化版深/浅色，1440×900 | 标题→紧凑筛选→店铺资产列表→明确入口，浅深主题相同层次 | 四个示例经营概览、样例计数、IP 正常、AI 文案、店铺 CRUD/同步按钮；负责人不变成授权 |
| F2 | [1569:359](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1569-359)，店铺概览，1440×900 | 返回店铺列表、店铺标题、平台/站点、详情页签与状态区域 | 虚构销售/利润/AI 任务数字，不新增网络页签 |
| F3 | [1569:676](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1569-676) / [1569:975](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1569-975)，授权信息/数据同步 | 记录状态、官方连接与业务动作分开表达 | 节点中的旧子账户密码/验证码，以及“官方 API 暂未开放”；依当前 Store 官方连接合同，只读展示已有事实 |
| F4 | [1569:1604](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1569-1604)，店铺设置，1440×900 | 明确分区、字段标签、集中保存，返回位置稳定 | AI 自动执行开关、经营策略及店铺资料编辑；环境配置不是 Store 设置的第二写入源 |
| F5 | [1532:525](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1532-525) / [1532:857](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1532-857)，企业操作记录深/浅色，1440×900 | 范围说明→筛选→记录→只读详情；状态文字与标签并用 | 导出记录、汇总卡、关键词搜索、总页数/末页、重放或清空；C4 未提供这些合同 |
| F6 | [8075:2331](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=8075-2331) / [8075:2423](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=8075-2423)，操作记录右侧详情深/浅色 | 只读详情的时间、操作者、对象、结果、关联信息和关闭行为 | 样例人名/已完成记录不进入产品；桌面用原 Modal 承接短详情，不新建 Drawer 系统 |

F1/F3/F4 的旧语义与当前合同不一致之处已经由现有 Store 产品决定收敛；复用布局不恢复它们。本轮硕米 main 从调查基线 d8d30f6 前进至 `ac54a11fed2ffdb72b3a5e8e2bd5c3575500830b`，只涉及 Amazon 采集币种与对应文档/测试；固定的 Store/权限/UI 依据仍适用。浏览器 main 仍为 `32fafdee392dae08c61e459bd67aa7f3d4420cf7`。

## U2. 完整页面复用与适配

| 本版流程 | 现有页面/组件依据 | 直接复用 | 需要的上下文适配 |
| --- | --- | --- | --- |
| 店铺工作台 | F1/F2；[BrowserListPage](../../frontend/src/modules/browser/pages/BrowserListPage.tsx)、[BrowserListLayout](../../frontend/src/modules/browser/components/BrowserListLayout.tsx) | 原标题/筛选/列表/忙态与返回结构，服务端真实分页 | 数据源从任意本机 profile 变为获授权 Store 投影；不继承导入、回收站、全局备份和批处理按钮 |
| 店铺详情/本机环境 | F2；[BrowserDetailPage](../../frontend/src/modules/browser/pages/BrowserDetailPage.tsx) | 概览层次、运行状态、启停反馈、既有 owned cleanup | 只使用 storeId→Go 验证绑定的安全 DTO；不照搬调试端口、原始代理、URL 打开、扩展/标签页控制、重启等入口 |
| 环境配置 | F4；[BrowserEditPage](../../frontend/src/modules/browser/pages/BrowserEditPage.tsx) | 原完整创建/编辑流程、基础/代理/指纹分区、校验、dirty-leave 确认、保存与错误输入保留 | 受管模式固定 owner/store/目录，改用 StoreEnvironmentSave；成功返回原店铺，不能继续原 navigate('/browser/list')；无 live 指纹检测或自动启动 |
| 店铺操作记录 | F5/F6；硕米现有 [AuditPage](https://github.com/qq550723504/shuomi-ai-commerce/blob/d8d30f6b0e8dfa903e02b002ef12f5747610c28f/web/listingkit-ui/src/components/workbench/account/audit/audit-page.tsx)；浏览器 Table/Modal | 已有范围隔离、筛选、游标翻页、只读详情与失效清空流程 | 独立 Store C4 查询适配，不能复用其账户 BFF/source_account 权限或所有 hook 副作用；不将 BrowserLogs 当业务审计 |

BrowserEditPage、BrowserDetailPage 当前自行加载/调用通用实例 API，AuditPage 也自行读取账户上下文/API；不能只换标题继续挂旧 hook。实施时在原页面模块中增加受管容器/模式或有界抽取完整视图，显式注入已验证 scope、读取、保存、启停、返回和错误策略。不能复制一套页面/Manager，也不能让展示组件顺带安装额外全局 binding。可复用流程中的自动刷新必须遵守 C3：店铺动作/结果查询不自动重试，窗口重新聚焦/网络恢复不触发隐式重发。

原 [BrowserLogsPage](../../frontend/src/modules/browser/pages/BrowserLogsPage.tsx) 有“清空”和技术筛选，不直接作为新记录页；它继续保留原职责。当前没有需要在本批退休的合格页面，受管接线不完整前不能删除原独立流程。

## U3. 导航、工作台与详情

沿用原 [Layout](../../frontend/src/shared/layout/Layout.tsx)、[Sidebar](../../frontend/src/shared/layout/Sidebar.tsx) 和 [AppRoutes](../../frontend/src/routes/AppRoutes.tsx) 的壳；新增一个“我的店铺”入口，不复制硕米整套网页导航，不新增账户/组织/权限管理菜单。以下为未来桌面路由合同，当前尚不存在：

| 路由 | 主职责与离开目标 |
| --- | --- |
| `/stores` | 选择企业、查找获授权店铺；每项“查看店铺”进入详情，不在列表写配置或直接批量启动 |
| `/stores/:storeId?tab=environment`（默认） | 当前店铺的本机环境与主操作；返回保存的列表筛选/位置 |
| `/stores/:storeId?tab=records` | 同一店铺的只读操作记录；切换页签不发起业务动作 |
| `/stores/:storeId/environment/edit` | 首次配置/后续编辑共用原配置流程；保存成功回环境页，取消不提交 |

不修改原全局根路由或独立实例入口的默认选择。硕米网页“进入店铺”继续进入店铺商品页；桌面明确使用“打开店铺环境”。URL 的 storeId、tab、query 都是未验证选择值，不能充当 owner；返回目标只从本地已知路由构造，拒绝任意 returnUrl。原 generation、企业 revision 改变时不恢复旧店铺/表单。

```text
我的店铺  [当前企业 ▾]
[店铺名称/ID________] [平台 ▾] [筛选] [重置] [刷新]
店铺名称 · 平台/站点   记录状态 · 本机环境状态       [查看店铺]
…只显示服务端授权后的当前页…                   [上一页] [下一页]

← 返回我的店铺     店铺名称 · 平台/站点
[本机环境]  [操作记录]
店铺记录 / 官方 API 连接 / 服务状态（各自标注）
本机环境名称、内核、代理安全摘要、本机观测时间、环境状态
主操作依状态为：[配置店铺环境] / [打开店铺环境] / [正常关闭]
```

列表仅搜索名称/ID、平台和原服务端分页。未取得完整 Store DTO/本机观测时分别显示“读取中/暂不可用”，不把失败判成“未配置/已停止”。列表数量采用服务端获授权集合的总数，不拿当前页运行数当全企业统计；首版不增加经营概览。详情中的名称、分组、负责人都是 Store 只读值，环境名称只是本机别名。

| 环境状态 | 页面主操作/反馈 |
| --- | --- |
| 未配置且可使用 | 配置店铺环境 |
| 已配置、明确未运行且可使用 | 打开店铺环境；配置入口为次要操作 |
| 启动/停止/保存未决 | 原操作进行中，禁用重复提交；可查看原操作引用，不能换 key 再开 |
| 明确运行中 | 正常关闭；再次“打开”只返回/聚焦可核验的已有窗口，不发起第二次启动 |
| UNKNOWN / 本机状态不可核验 | 结果待确认；仅“查询原结果/查看记录”，不出现“重试启动” |
| 无 browser.use、店铺停用/删除或已撤权 | 明确说明原因，禁止配置/新开；现有窗口只保留 C1/C2 允许的 owned cleanup |

注销不把现有商家窗口显示成“已注销商家账号”；安全关闭证明失败时显示“关闭结果待确认”，保留原证据，不提供强制停止/自动再次关闭。

## U4. 环境配置的字段与保存闭环

```text
← 返回店铺       配置店铺环境                         [保存配置]
当前企业 / 当前店铺（只读，不可改绑）
基础配置：环境名称、内核、历史标签策略、内存上限
代理与定位：原代理来源/选择、安全状态与原配置提示
指纹配置：原分区和校验，较长内容保持原折叠结构
```

原 BrowserEditPage 的表单、配置对象规范化、代理解析和指纹字段继续复用。受管模式不显示可修改目录、目录选择器/打开目录、通用 launchArgs、克隆/导入、本机标签/分组管理、浏览器内指纹检测等入口；Store 分组/负责人不能被本机表单覆盖。目录显示“由本机自动管理”，内核缺失导航至原内核管理后回到表单；本页不下载、预热或启动内核。

代理秘密不在摘要回显；本次明确编辑所需的代理输入只交原 Go 用例处理，不写审计。读不到有权代理时保留不可用状态，不自动改为直连。源页面中的支持性校验可以复用，实际网络探测、定位/预热等副作用遵守 C1 的动作准入；纯视图打开不得触发它们。不复制代理栈实现。

保存顺序：静态输入校验→固定原 scope/operationId/CAS→Go 配置准入→C2 原子保存→显示本机结果与报告状态→回到该店铺环境页。**保存不自动打开环境**；“打开”是下一次明确动作，有独立新准入。保存失败保留当前输入与错误；409 提示“配置已变化，请核对最新版本”，不静默刷新覆盖输入；UNKNOWN 锁定提交并提供只读查证，取消/返回不宣称回滚。

dirty-leave 复用原离开确认：继续编辑/放弃本次未提交输入。已提交未确定的操作不能被“放弃输入”取消；返回详情仍保留 pending/UNKNOWN。保存成功返回目标只能是当前 storeId；若身份/企业已变，丢弃返回回调和旧输入，回到当前有效企业店铺选择。

## U5. 店铺操作记录

记录页复用 F5 的层次，短详情采用现有 [Modal](../../frontend/src/shared/components/Modal.tsx) 与 [Table](../../frontend/src/shared/components/Table.tsx)，无需新增 Drawer/日志平台。保留一个只读列表，不混入授权/配置提交表单。

```text
店铺名称  [本机环境] [操作记录]
记录范围：当前店铺的已有成员授权回执及本版应用命令；不含商家网页点击
[时间范围] [动作 ▾] [结果 ▾] [操作者（仅有权管理员）] [筛选] [重置] [刷新]
时间 | 操作者 | 动作 | 来源 | 结果 | [详情]
…原始事实，不用当前状态倒推历史…              [上一批] [下一批]
```

筛选严格采用 C4 的 from/to、actor、action、outcome 与 cursor；不增加 Figma 中未有合同支持的关键词搜索、导出、清空、汇总、总页数或末页。普通成员强制只看本人，前端不加载其他人的记录后再隐藏；操作者是身份引用，未核验名字时不冒充人名。下一批使用服务端游标，上一批只使用本 scope 已访问的游标栈；改筛选重置游标与旧详情。

结果展示分开：服务器“已准入/已拒绝”、本机“报告成功/失败/未知”、服务器“报告未确认”。不得统一画成绿色“已完成”。字段缺失或来源失败不等于零记录。详情仅展示 C4 的操作 ID、店铺/环境引用、原操作者、服务端时间、明确标注的客户端观察时间、动作/来源、安全结果/原因码；没有补报、重放、授权、再次启动按钮。

原操作仍未确认时，“查询原结果”只调用 C3 GET，不从审计记录还原新的执行授权。本人撤权后的原回执查证使用 C3 的窄身份合同，不能借它继续读取完整店铺记录；切企业时完整记录表和详情立即清空。

## U6. 统一异常、视觉和可访问性

| 状态 | 必须展示的行为 |
| --- | --- |
| 企业未选择/切换中 | 原 context 选择规则；清除旧店铺/表单/记录，不能默认用 home org 冒充选择 |
| 公开店铺 API 配置缺失/认证不兼容 | “店铺服务未配置/不可用”，不回退 mock、脚本或新登录；不影响原登录门禁的权威性 |
| 加载/无数据/无权限/依赖失败 | 各自独立状态；“没有符合筛选的店铺/记录”仅用于已成功读取的空集合 |
| 已过期/已撤权/换账号的迟到响应 | 忽略旧 scope 的 DTO、toast、保存跳转和抽屉/弹窗；不显示前一企业的残留内容 |
| 网络失败/报告失败 | 已知本机结果与服务器未确认分别展示，保留操作引用；无自动重试/重新登录/自动切换代理 |

视觉沿用浏览器 [theme tokens](../../frontend/src/shared/theme/themes/base.css)、[深色主题](../../frontend/src/shared/theme/themes/dark.css)、[全局字体与焦点](../../frontend/src/index.css)及原 Button/Form/Card/Badge/Modal/Table。Figma 的浅深层次、边框与状态层级作为语义依据，桌面不另造一套硬编码 Figma 色值；现有主题继续生效。字体保留现有 system→Noto Sans SC fallback；标题 text-xl、正文 text-sm、主控件 h-9、紧凑按钮 h-8、间距 gap-4，均取已有组件，不下载字体。

1440×900 是参考画布，不强制桌面窗口尺寸。布局检查沿用 [现有默认最小窗口 1200×700](../../backend/internal/config/config_defaults.go)及[小屏启动尺寸适配](../../internal/windowsizing/window_sizing.go)，不新增手机断点验收或修改窗口配置。按原 responsive 方式，在产品支持的较窄内容区字段折为一列，表格只在其容器横向滚动，标题/主操作/返回不能被挤出可达范围。短详情 Modal 适配原宽度及滚动，关闭后焦点回触发按钮。

每个输入有可访问名称、必填/错误关联；复用 FormItem 时仍为 input/select 提供 label/id 或 aria-label，不能因原包装是 span 就认为已有 label 关联。页签用原模式并补正确键盘/选中语义，列表名称/详情链接和状态文字可读；颜色不作为唯一信息。提交中禁用、焦点、Escape/关闭与 dirty-leave 的顺序可预测，原 Button 的 loading 状态直接复用。

## U7. 实施映射与本轮证据边界

| UI 必须满足 | 对应现有验收 |
| --- | --- |
| 身份/企业变化、只显示授权集合与安全字段 | A01/A02/A06/A07 |
| 只读与可使用、管理员记录范围、负责人不授权 | A03/A09 |
| 保存后返回、独立打开、同实例复用、代理预热在准入后 | A04/A05/A08/A12/A13 |
| UNKNOWN 与未确认分开，查证不能重放；安全关闭 | A08/A10/A11 |
| 三条完整路径、深浅主题、窄屏/键盘及返回/错误状态 | A13，离线 UI fixture；真实产品仍 NOT_RUN |

本轮只有 Figma 只读设计证据与源码阅读，**没有**产品截图、实际点击验收或新功能可用性证明。后续实现不能把这份 UI 依据当成运行 PASS；无需为本范围重画全部 Figma、建设组件库或引入新视觉框架。
