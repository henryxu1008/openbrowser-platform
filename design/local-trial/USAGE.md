# Windows 内部隔离试用工具

本文脚本仅用于内部隔离试用，不是硕米终端用户的安装或日常启动入口。正式安装、原生登录与数据目录的交付约定见 [原生交付说明](../native-delivery/README.md)。

从仓库根使用 PowerShell 7。首次创建使用 `new-local-trial.ps1`，显式提供 EXE、完整预期 SHA256、公开 OIDC JSON 和一个**尚不存在**的绝对本地试用目录；以后使用 `open-local-trial.ps1` 指定第一次返回的同一 `TrialRoot`。

## 当前候选与历史样例

不要复用历史 EXE 的哈希或内核版本。先由维护者记录本次候选的完整 Git HEAD、前端构建结果、Windows 生产编译结果与最终 EXE SHA256；源码版本号不代替提交和文件哈希。PR 的 CI 编译成功不等于已发布安装包，也不代表存在可直接下载的 EXE 工件。

以下变量必须来自**同一次受控构建及已确认目标实例**，不从旧试用目录或历史示例猜填。`Get-FileHash` 仅记录产物身份，不独立证明产物可信；须核对构建来源与预期记录。命令会启动产品，只能在另行取得运行授权后执行：

```powershell
# $CandidateExe、$CandidatePublicIdentity、$CandidatePublicApi、$NewTrialRoot 由维护者按当前构建记录填写。
# $ExpectedCandidateSha256 为该次构建记录中的完整 SHA256，不是历史常量。
Get-FileHash -LiteralPath $CandidateExe -Algorithm SHA256
.\scripts\new-local-trial.ps1 `
  -ExePath $CandidateExe `
  -ExpectedExeSha256 $ExpectedCandidateSha256 `
  -PublicConfigPath $CandidatePublicIdentity `
  -ApiConfigPath $CandidatePublicApi `
  -TrialRoot $NewTrialRoot
```

历史证据（不可作为当前运行命令）：旧认证样例 main `92bff352b161ff1d7e5072a8b76e97fc6a62dec8`，EXE 52,171,776 字节，SHA256 `e754a19c45b5be13365594b0ce50ce3c5c4cb8b0e4b9e5f38827fb1fb58c735b`。旧认证样例内核需求 `148.0.7778.215` 也不作为当前平台目录选择指令。

公开身份 JSON 必须含字符串 `issuer`、`clientId`，可选有界公开项目 ID `apiProjectId`（1–128 位 ASCII 字母、数字、`_`、`-`）；不接受额外字段。基础身份试用可用两字段；Store 配套必须提供目标实例已有 API 项目的 `apiProjectId`。Native client ID 与 API project ID 不是同一个概念，不能猜填；不新建客户端、不加入 client secret、密码、token 或私钥。脚本将公开 JSON 写入新根供原生读取。正式交付契约见 [原生交付说明](../native-delivery/README.md)。

### Store / 平台内核试用的额外准入

Store 与平台内核 API 共同使用 EXE 根内的 `shuomi-public.json`：仅 `apiOrigin` 一个 HTTPS origin 字段，无路径、账号、查询或 secret，文件最多 4096 字节。`-ApiConfigPath $CandidatePublicApi` 必须来自已核对的目标实例公开配置，无默认 URL；脚本不会探测服务或验证部署是否正确。

身份配置含 `apiProjectId` 时，首次创建与重开都必须显式提供 `ApiConfigPath`。新建在创建目录前验证公开配置，再写入两份 JSON，最后调用原进程启动边界；输入无效不创建目录、不启动。复制/写入失败不启动、不重试，部分新根保留。重开只核对该根已有 API origin 与显式配置完全相同，不覆盖、不补建、不改绑定；缺失、不一致或尝试用两字段身份配置打开已有 Store 根都拒绝。

仅基础身份试用可省略 `ApiConfigPath`（以及身份 JSON 中的 `apiProjectId`），保留旧调用兼容；这不证明 Store 可用。两份公开配置与真实身份、Store、内核及网络运行必须分别取得执行授权。具体代码边界见 [Store 交付说明](../store-workbench/DELIVERY.md)、[当前准备体验范围](../store-workbench/ISSUE21-CANDIDATE.md)。

## 启动绑定和数据

生产 `main.go` 使用 EXE 目录作为 appRoot，单改 WorkingDirectory 不会隔离数据。因此入口校验源 SHA256，**仅复制这个 EXE 并写入已校验的公开配置**到新根，再核对复制后的 SHA；不复制旧 config、SQLite、profile、WebView 或备份。新根须位于源 EXE/公开配置目录之外，父目录须存在；重解析路径和已存在的根会拒绝。

通过 `ProcessStartInfo` 启动，`UseShellExecute=false`，工作目录与 EXE 目录均为新根。环境先 Clear，仅保留系统六项 `SystemRoot/WINDIR/SystemDrive/ComSpec/PATH/PATHEXT`，再设置：

| 子环境键 | 值 |
|---|---|
| USERPROFILE、HOME | 新根/user |
| APPDATA | 新根/user/Roaming |
| LOCALAPPDATA | 新根/user/Local |
| TEMP、TMP | 新根/temp |
| ABFREE_OIDC_ISSUER、ABFREE_OIDC_CLIENT_ID | 两项公开配置 |

不设置 User/Machine 持久变量，不回显公开配置内容。验证/复制/启动异常直接报错，不覆盖、删除、换路径或重试；部分新目录保留供检查。入口不监控、代办登录、自动退出或强杀。

程序获用户明确运行后可在新根生成配置、SQLite/default 元数据、日志、WebView 和本地备份设置。这是独立试用数据，不是用户正式档案。默认系统浏览器仍可能复用或维护 SSO/Cookie；新 appRoot 不代表系统浏览器完全隔离。结束时用应用内“仅退出应用”。

## 再次打开同一份数据

先正常退出该根的应用，再执行：

```powershell
.\scripts\open-local-trial.ps1 `
  -ExpectedExeSha256 $ExpectedCandidateSha256 `
  -PublicConfigPath $CandidatePublicIdentity `
  -ApiConfigPath $CandidatePublicApi `
  -TrialRoot $NewTrialRoot
```

重开入口只使用显式选定根中的 `ant-chrome.exe`，校验完整 SHA256、公开配置和已有 user/temp 目录，复用与首次启动相同的子进程环境。它不复制 EXE、不重建配置或 SQLite、不清空档案，不补建缺失目录。`config.yaml`、`data\app.db` 及 `user` 中状态留在同一根；异常直接停止，保留现场。

请选第一次创建得到的用户根，不要把源包的认证验收根或其他正式档案当作用户根。入口不搜索、接管或迁移旧目录；`new-local-trial.ps1` 仍拒绝已有根。要建立另一独立场景才选择另一个不存在的路径。两个脚本须保持同目录；不要用双击 EXE 替代环境绑定。

## 必要前置

- PowerShell 7（`pwsh`）和已有 WebView2；EXE 已嵌入前端，无需前端服务或再次构建。
- 目标身份服务、硕米 API 与所需数据库必须由维护者确认可用且公开配置匹配；旧本地容器名称不作为当前实例检查依据。入口不检查或启动服务、不探测 IdP；使用正常系统 TLS，不修改证书/安全设置。
- 登录由本人完成；应用 API 使用 loopback 19876，运行前须确认端口无冲突并保留其他现有会话，回调端口由 OS 分配。业务 API 仍需登录、有效 key 及本代次明确授权，入口不创建 key。
- 试用脚本不准备内核。当前 Store 通过现有平台版本选择器显式选择适配客户端 OS/架构/版本的已发布 release，再校验和准备；保存不触发下载或打开。服务端必须已提供授权目录及符合 hash/size/origin 契约的制品。推荐标签不是自动选择授权；不回退旧路径内核。已有认证验收不等于真实 CDN 或 Store 整链验收。

## 不运行产品的检查

```powershell
.\scripts\tests\new-local-trial.Tests.ps1
```

测试只用自建文本 EXE fixture 与临时公开配置，替换进程启动边界；检查首次创建、同根重开、配置/SQLite fixture 哈希保持、Store API 配置先于启动落盘、缺失或不一致拒绝及错误输入零启动，不执行目标 EXE，不联系 IdP、启动内核或容器。两个新入口的真实产品启动及持久化重开验收仍为 NOT_RUN，不能移用旧手动场景 PASS。
