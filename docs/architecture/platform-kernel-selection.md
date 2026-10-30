# 平台内核发行与桌面选择

用户授权范围：源码、必要离线回归及 Draft PR。此实现没有部署、实际发行制品、从 CDN 下载真实内核、修改现有权限或配置，也没有操作保留应用、实例或店铺环境。

## 用户结果

内核页面、普通实例编辑及店铺环境编辑使用同一个版本选择器。版本来自平台目录，用户只选择固定发行版本；未缓存的选择自动准备，失败保留原绑定。准备缺失的当前版本也可显式重试。平台推荐仅作为标签，不自动选中或升级。

准备完成只产生公共制品缓存，不创建或保存实例、店铺配置，也不启动环境。编辑器保留原保存/取消入口和店铺 CAS。准备期间阻止重复选择及保存；登录、页面、店铺修订或当前绑定变化后，旧响应不得调用新上下文的选择回调。事件只带 releaseId、generation、固定阶段与百分比。

旧的任意下载地址、导入、扫描、编辑路径、删除和默认版本维护入口已从用户流程移除。AuthBound 原生边界也拒绝旧 BrowserCore 能力（仅保留去除路径的就绪摘要），不能通过构造调用恢复。实例列表的通用浏览器设置仍保留，原内核维护表改为版本页入口。

## 后端合同

配套后端：[shuomi-ai-commerce PR #1014](https://github.com/henryxu1008/shuomi-ai-commerce/pull/1014)，合同为该仓库 `docs/architecture/browser-kernel-release-catalog-v1.md`。

- `GET /api/v1/browser/kernel-releases`，原生附带 os、arch、clientVersion；limit/cursor 原生控制。
- `GET /api/v1/browser/kernel-releases/{releaseId}/artifact`，同样附带兼容性查询。
- 直接 JSON，没有 data 包装。目录为 releases/nextCursor；错误为固定 code。
- releaseId 为规范小写 UUID，也是不可变制品身份；version 为显示版本；兼容边界为完整三段 SemVer，最大版本上界不包含自身。
- 空兼容页仍可能有 nextCursor。客户端继续翻页，校验严格单调 UUID 顺序，最多 32 页 / 1000 项，拒绝不完整结果。
- 元数据大小上限 2 GiB，SHA256 为小写 64 位十六进制，格式为 zip、tar.gz、tar.xz。
- downloadOrigins 是部署批准的 1..8 个规范 HTTPS origin。初始 URL 和最多三次跳转均须精确命中；仅跳转目标允许临时 query。初始地址无 query、userinfo、fragment。制品请求没有 API bearer 或 cookie；传输错误、地址与临时 query 不进入前端、回执或日志。

目录读取只需当前有效登录，不附企业上下文、店铺 ID 或 Store/use 权限。发行权限由后端现有平台管理员权限控制，桌面没有发布能力。

## 本机事实与执行边界

`apiconfig` 复用现有 shuomi-public.json 严格解析；Store 客户端的原错误与请求行为保持。`kernelapi` 单独绑定平台 API origin，复用 desktopauth 的原生请求授权。没有 bridge 或发行配置时明确失败，没有模拟可用版本或本地扫描兜底。

平台目录拥有发行事实；`StateRoot/platform-kernels/{releaseId}/{installationUUID}` 及其私有回执拥有本机就绪事实。回执绑定 API origin、固定元数据、相对可执行文件和其摘要，不保存 URL、下载来源列表、签名 query 或身份凭据。Core lookup 直接读取受控安装视图，不复制到 CoreDAO。

准备流程为新建专属暂存目录、限长下载、大小/SHA 校验、归档预检查、复用已有解压库、识别可执行文件、写回执，最后在 WithIdentity 内短暂执行目录发布。网络、解压和长哈希都不占 identity 锁。identity 提交不取得 browser manager Mutex，避免与店铺保存的反向锁序。新内核能力不占店铺通用锁；同一 releaseId 合并同代际下载，其他代际不能加入旧请求。

安装目录不可覆盖。文件缺失或损坏时另建安装目录，旧目录保留；失败只清理本次专属暂存目录。原本的任意本地目录、config/DAO 内核和环境迁移不自动登记成平台发行版本。没有明确绑定或版本不可用时，运行解析失败，不回退到默认或第一个版本。

Windows 路径比较使用已有 Win32 GetLongPathName 规范短文件名拼写，并处理长路径；各路径分量仍显式拒绝符号链接和目录 junction。短名称别名不再被误判为目录逃逸，也不会借规范化采用链接目标。API 行为见 [Microsoft GetLongPathNameW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getlongpathnamew)。合成并发夹具会在准备提前失败时直接报错，避免等不到下载起点而挂住。

新保存必须仍在发行并已准备；保留同一个已有就绪绑定可在撤回后继续保存其他字段。普通实例保存会在锁内再次核对身份和原绑定，避免网络读取期间的绑定替换。店铺仍执行原权限、CAS、操作回执和启动校验。刷新目录不会改绑定、覆盖运行文件或自动切换版本。

制品下载使用现有 HTTP client factory 的直连分支，不增加用户代理参数，不切换 Xray/sing-box 与 Mihomo 连接栈。

## 新绑定消费者闭包

复制的三个桌面入口和自动化模板新建都复用同一原生选择校验：复制属于新绑定，即使源仍可使用，下架后也不能复制；查询在 manager 锁外，提交时在锁内核对原身份及源 CoreId。本机 HTTP 通过现有 profileCreator/profileUpdater 接口的私有适配器进入同一原生写入路径，不新增 Wails 入口；平台模式的无准入 manager Create/Update/Copy 明确拒绝，无法静默 fallback。

实例包新建和覆盖均使用新 UUID，必须对包中已有 releaseId 做发行及就绪校验，长解压后在提交前再次校验。平台模式不恢复包内 CoreDAO 配置，不按路径或旧 ID 做映射。现有导入选项没有替换版本，本轮不增加替换交互、格式兼容或迁移；有效平台 ID 的 V1/V2 包继续复用原导入、关联数据、覆盖及回滚协议。只有短的本地提交阶段持 manager 锁，核对身份和覆盖目标；SQL 提交前仅检查固定代际身份，避免在单连接事务中再次查询 Store 数据。失败恢复原绑定及目录，不在回滚时重新查询发行状态或选择版本。

## 首版包形状及验收限制

发行方须制作自包含的普通文件/目录包。首版拒绝符号链接、硬链接、特殊文件、路径越界、Windows 设备/别名路径、大小写重复目标；展开上限 8 GiB、100000 项。含链接的常规 macOS bundle 须另制包，不能由此实现宣称其已兼容。解压不会执行制品。现有解压器的单顶层文件误剥离问题一并修正，并由合成包回归覆盖。

本次验证是离线源码验证：合成 HTTP 传输、包、登录代际与临时数据库；包括真实原生保存路径、撤回、损坏修复、迟到响应和重复选择。生产编译不等于真实内核、跨操作系统或店铺验收。实际发行、真实 CDN 下载/安装及保留环境验收均 NOT_RUN，须另行安排。
