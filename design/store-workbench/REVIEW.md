# 架构审阅记录

状态：**`IMPLEMENTATION_READY`（仅设计）**。2026-10-25 已完成原合同两轮有界独立审阅，随后按用户新授权完成 UI 依据增量独立审阅；未解决的设计 BLOCKER 为 **0**，G2 已解除。G1 产品实施仍 **NOT_AUTHORIZED**；设计就绪不授权编码、合并、真实运行、IAM 或部署。

## 1. 范围与评审依据

首次审阅本目录 README、CONTRACTS、IMPLEMENTATION 中的授权、绑定、事务/副作用、审计、幂等与 UNKNOWN 合同；复用 README 固定的两仓库源码树。本次新增 UI.md 后仅审阅 UI 复用与桌面适配增量，不重新审阅未变化的合同。不运行任何产品、测试、身份/商家交互，不修改 IAM、源代码或 schema。

采用硕米当前 [Architecture Review Checklist](https://github.com/qq550723504/shuomi-ai-commerce/blob/d8d30f6b0e8dfa903e02b002ef12f5747610c28f/docs/architecture/architecture-review-checklist.md)与 [AGENTS.md](https://github.com/qq550723504/shuomi-ai-commerce/blob/d8d30f6b0e8dfa903e02b002ef12f5747610c28f/AGENTS.md) 的 finding 分类和最多两轮正常审阅规则。仅对真实影响本版 Must 的越权、数据损坏、重复副作用、错误身份或核心路径不可完成等问题判 BLOCKER；不把明确 Out of Scope 的 OS 防篡改、远程硬撤销或商家页面全审计升级为新需求。

## 2. 审阅身份、轮次与冻结摘要

独立 reviewer：本次会话的只读审阅子代理 `/root/store_design_review`，与文档 Writer 分开；不是 GitHub PR review、外部人工批准或运行验收。其未写文件、运行测试/产品或访问真实 IAM/商家流程。此单次委派依据为硕米 AGENTS 对本版新增授权/持久化边界的独立 Architecture Review 要求，不建立常驻多 Writer。

第一轮检查完整范围、实际 owner 与固定源码，未发现架构 BLOCKER，提出 R1–R3 三个实施接线细节；Writer 在同一份合同中补正。第二轮仅复核这三处及对应验收映射，确认没有未解决合同 BLOCKER；不启动第三轮泛化评审。

第二轮实际读到的三个正文文件 SHA256（工作树原始 UTF-8/LF 字节）：

| 文件 | SHA256 |
| --- | --- |
| README.md | `B3075A4CC043B8826DD63CE384ECD4A95A5AE563C2DCB73F98EFFFA63AC50B32` |
| CONTRACTS.md | `37B9835D5A18722437543AC68B9BE288DA64444A1191C5FB45DCD12C2E6A8CFE` |
| IMPLEMENTATION.md | `9132642F29E773A03C79E187B501C75AA518667C6DC3861E01E6DE1FF2CA718A` |

本 REVIEW 是评审结果记录，不参与自己的摘要，避免自引用 hash。以上为首次合同评审时的读本；随后用户授权 UI 增量，README/IMPLEMENTATION 因此更新，CONTRACTS 始终保持该摘要。固定源码树见 README，不将文档 hash 冒称源码 HEAD/CI 证据。

## 3. Findings 与处理结论

| ID | Finding / 当前 Must 与后果 | 分类与阻塞层级 | 合同补正及验收 |
| --- | --- | --- | --- |
| R1 | 原回执使用 None 中间件时企业 header/scope 被清除，依赖它定位会让撤权后的结果无法报告 | `IMPLEMENTATION_TEST`；合同澄清已完成，真实 router 回归待实现；无剩余设计 BLOCKER | [C3](CONTRACTS.md#c3-服务端动作准入与回执-api)：只用 path 的全局 storeId/opId 与 verified issuer/subject 定位原行，企业从原行取得；[A09](IMPLEMENTATION.md#2-验收矩阵均为待实现当前-not_run) |
| R2 | 原 Store withWrite 还检查 store.update，机械复用会错误拒绝只有读取与 browser.use 的合法用户 | `IMPLEMENTATION_TEST`；合同澄清已完成，权限组合回归待实现；无剩余设计 BLOCKER | [C1](CONTRACTS.md#c1-身份企业与动作授权)：同事务 LockRead/等价 fence，不隐式要求 update；A03 覆盖合法准入及撤销 |
| R3 | 两个首次配置各有不同候选 bindingId，单按 bindingId 互斥会重复实例或留下无归属数据 | `IMPLEMENTATION_TEST`；合同澄清已完成，SQLite 并发/fault fixture 待实现；无剩余设计 BLOCKER | [C2](CONTRACTS.md#c2-本机受管实例绑定与原子性)：同库 owner/store 唯一约束＋Profile/binding/journal 全回滚，提交前不发布 Manager/建目录；A05/A08 |

审阅同时确认：完整复用原 owner 而非新建 Store/IAM/浏览器管理器；代理预热在授权准入后；所有 profile 入口和批量数据边界覆盖受管 policy；旧回执不给执行能力；报告主体固定；无报告/UNKNOWN 不冒充成功、不自动重试；服务器事实和客户端报告来源分开。上述为合同评审判断，落实情况由 A01–A13 的实现回归证明。

## 4. 就绪、授权与外部条件

| 项目 | 最终结论 |
| --- | --- |
| 授权、实例绑定、审计接口合同 | 本轮静态独立审阅通过，无未解决合同级 BLOCKER |
| 整批 Implementation Ready | **IMPLEMENTATION_READY（设计基线）**；UI.md 的实际 Figma/现有页面映射与桌面差异已完成增量独立审阅，G2 解除；执行 Issue 仍须后续授权与范围登记才能正式开工 |
| 产品代码实施 | **NOT_AUTHORIZED**；用户本轮仅授权设计文档与审阅，即使后续就绪也不能据本记录自行编码 |
| Native access token → 硕米 API | **UNVERIFIED**；G3 是真实联调/试用验证项，旧桌面登录证据不能替代；不重建 IAM 或以 ID token 绕过 |
| 生产 issuer/clientId/API origin | **DEFERRED_BY_USER**；G4 仅发布条件，不阻塞合同设计完成 |
| 实际产品、schema、IAM、真实服务与部署 | **NOT_RUN**；无状态变更，无新服务 |

本设计不宣称商业站点或商家登录/操作验收通过。后续只对真实 diff、组合风险和上述必要实施回归复核；非 Blocker hardening 不扩成本版前置。

## 5. UI 增量独立审阅与 G2 解除

用户新增指令明确要求复用已有 Figma 规范与浏览器页面补齐 UI 设计依据，并提交文档 PR。Writer 实际只读核对 UI U1 的可见节点与截图，记录旧子账户/AI/示例数据的剔除依据、三条完整路径、字段/状态/返回及可访问性；未编辑画布。独立 reviewer 复用本会话 `/root/store_design_review`，只读审查新增文档与实际页面源码；Figma 实际读取证据来自主会话，reviewer 未重复访问 Figma。此为新授权增量，未启动第三轮旧合同泛化审阅。

UI 审阅时读本：

| 文件 | SHA256 |
| --- | --- |
| UI.md | `8126A2F9E8AA0C2180F89B5A985A5BA2AF3F817708F72D9057CA2F0D844DB722` |
| README.md | `1D3D6470ADA20C11F81E3A7D04D8B5BA2C80781D5B02264755645CD140596906` |
| IMPLEMENTATION.md | `93A193620E1C1320AE9F99C1911D9533454491B26017930D326F25537E1223AD` |
| CONTRACTS.md | `37B9835D5A18722437543AC68B9BE288DA64444A1191C5FB45DCD12C2E6A8CFE` |

审阅结论：未解决 BLOCKER 0；用户批准的复用边界、已核对 Figma/源码与明确的桌面差异足以解除 G2，设计基线可标 IMPLEMENTATION_READY。该结论不要求重新绘制三张 Figma 成品页，也不声称其已绘制。上述读本之后只写回就绪状态、本审阅记录及下列实施检查，未改变 UI 流程或 C1–C4。

| Finding | 分类/处理 | 验收承接 |
| --- | --- | --- |
| UI-R1 | `IMPLEMENTATION_TEST`：原 Modal 尚未保证 Escape、关闭焦点恢复及可访问名称；实施时在原组件/容器补齐所需行为，不能把复用当成已通过 | A13 |
| UI-R2 | `IMPLEMENTATION_TEST`：受管挂载及运行事件不得触发原 BrowserDetail tabs/CDP、BrowserEdit 全局资料 hooks 或 BrowserLogs 清空/轮询 | A07/A13 |

三流程职责、受管 adapter、保存与打开分离、预热后置、CAS/UNKNOWN/迟到响应、只读审计范围及现有窗口范围均满足本版设计 Must。G1 仍禁止编码，G3 真实 API token 未验证，G4 公开目标仍留发布前确定；这些不改写为设计 BLOCKER 或虚构 PASS。

## 6. 本轮文档验证与 PR 范围

检查五份文档的本地相对引用/锚点、JSON 示例、Markdown 围栏、审阅摘要与 `git diff --check`；设计 PR 的差异限定为本目录五个 Markdown 文件。PR 以实际 main 为基线，不因旧实现分支的 squash 历史把产品代码重新带入范围。原工作流可独立触发 PR 离线测试/编译，实际状态以 run 为准，不是产品验收。

本机未运行产品回归、编译、GUI、内核、登录、服务或发布/部署；未改 IAM、schema、运行配置或产品源码，未创建产品执行 Issue。文档检查证据留在当前会话，不写成产品 PASS；设计 PR 提交/CI 状态按远端实际结果报告，不从本记录推断合并授权。
