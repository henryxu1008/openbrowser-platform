# Windows 内部隔离试用工具

本文脚本仅用于内部隔离试用，不是硕米终端用户的安装或日常启动入口。正式安装、原生登录与数据目录的交付约定见 [原生交付说明](../native-delivery/README.md)。

从仓库根使用 PowerShell 7。首次创建使用 `new-local-trial.ps1`，显式提供 EXE、完整预期 SHA256、公开 OIDC JSON 和一个**尚不存在**的绝对本地试用目录；以后使用 `open-local-trial.ps1` 指定第一次返回的同一 `TrialRoot`。

```powershell
.\scripts\new-local-trial.ps1 `
  -ExePath 'C:\Users\Henry\code\ant-browser\build\dist\auth-trial-main92bff35-20261025\ant-chrome.exe' `
  -ExpectedExeSha256 'e754a19c45b5be13365594b0ce50ce3c5c4cb8b0e4b9e5f38827fb1fb58c735b' `
  -PublicConfigPath 'C:\Users\Henry\code\ant-browser\build\dist\auth-trial-main92bff35-20261025\oidc-public.json' `
  -TrialRoot 'C:\Users\Henry\code\ant-browser\build\dist\local-user-trial-01'
```

这是用户手动调用示例，本项开发/测试没有执行目标 EXE。示例对应已验证 main `92bff352b161ff1d7e5072a8b76e97fc6a62dec8`、52,171,776 字节的固定包，不是原 upstream 或旧历史包。

公开 JSON 必须仅含两个字符串字段 `issuer`、`clientId`（Native public client，不是 secret）；使用已有 `https://localhost:35443` 对应登记，不重新注册。不要将账号、密码、token、私钥或私有配置交给脚本。

## 启动绑定和数据

生产 `main.go` 使用 EXE 目录作为 appRoot，单改 WorkingDirectory 不会隔离数据。因此入口校验源 SHA256，**仅复制这个 EXE**到新根，再核对复制后的 SHA；不复制旧 config、SQLite、profile、WebView 或备份。新根须位于源 EXE/公开配置目录之外，父目录须存在；重解析路径和已存在的根会拒绝。

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
  -ExpectedExeSha256 'e754a19c45b5be13365594b0ce50ce3c5c4cb8b0e4b9e5f38827fb1fb58c735b' `
  -PublicConfigPath 'C:\Users\Henry\code\ant-browser\build\dist\auth-trial-main92bff35-20261025\oidc-public.json' `
  -TrialRoot 'C:\Users\Henry\code\ant-browser\build\dist\local-user-trial-01'
```

重开入口只使用显式选定根中的 `ant-chrome.exe`，校验完整 SHA256、公开配置和已有 user/temp 目录，复用与首次启动相同的子进程环境。它不复制 EXE、不重建配置或 SQLite、不清空档案，不补建缺失目录。`config.yaml`、`data\app.db` 及 `user` 中状态留在同一根；异常直接停止，保留现场。

请选第一次创建得到的用户根，不要把源包的认证验收根或其他正式档案当作用户根。入口不搜索、接管或迁移旧目录；`new-local-trial.ps1` 仍拒绝已有根。要建立另一独立场景才选择另一个不存在的路径。两个脚本须保持同目录；不要用双击 EXE 替代环境绑定。

## 必要前置

- PowerShell 7（`pwsh`）和已有 WebView2；EXE 已嵌入前端，无需前端服务或再次构建。
- 原 Docker 身份环境可用：`task-processor-unified-20261010-identity-db-1`、`zitadel-api-1`、`zitadel-login-1` 需 healthy，`runtime-network-1` 需 running。入口不检查或启动服务、不探测 IdP；使用正常系统 TLS，不修改证书/安全设置。
- 登录由本人完成；应用 API 使用 loopback 19876，运行前须确认端口无冲突并保留其他现有会话，回调端口由 OS 分配。业务 API 仍需登录、有效 key 及本代次明确授权，入口不创建 key。
- 内核不捆绑、不下载。需要实例运行时，通过**上游原内核管理**使用用户指定版本 **148.0.7778.215**；不增加固定内核 manifest 或专用管理器。已有认证验收不等于内核/所有业务功能均已重验。

## 不运行产品的检查

```powershell
.\scripts\tests\new-local-trial.Tests.ps1
```

测试只用自建文本 EXE fixture 与临时公开配置，替换进程启动边界；检查首次创建、同根重开、配置/SQLite fixture 哈希保持和错误输入拒绝，不执行目标 EXE，不联系 IdP、启动内核或容器。两个新入口的真实产品启动及持久化重开验收仍为 NOT_RUN，不能移用旧手动场景 PASS。
