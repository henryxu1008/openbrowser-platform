# Issue #22 — 当前店铺自有代理参数草稿

需求来源：openbrowser-platform #22。现有受管编辑页要求完整代理 URI，普通用户拿到地址、端口和凭据后需要手工编码。此切片只增加参数填写入口，复用 `buildDirectImportCandidate`、现有 Form/Modal 和 Store 门禁。

## 行为边界

- 在既有“店铺网络”区域保留 URI 粘贴，增加 HTTP / HTTPS / SOCKS5 参数短表单。沿用 v4 已有网络表单的端口与地址分组、共享控件，以及已对齐的 Store portal 主题；不添加地区、采购或网络就绪模拟值。
- “应用到草稿”是同步纯本地更新，切换到显式自有代理方式并进入原未保存确认流程。不会调用全局代理池、保存、内核准备、探测、打开或关闭环境。
- 保存仍由原 `ManagedBrowserEditor.save` / `StoreEnvironmentSave` 按既有身份、企业 revision、Store 与 binding CAS 提交；本次不更改任何原生接口或权限。
- URI 编辑区和密码输入遮蔽；不输出完整 URI、凭据预览或未知异常。账号首尾空格拒绝，密码保留原样；SOCKS5 认证遵守原生解析器的 255 字节上限。
- 取消/关闭销毁参数草稿。重复应用只消费一次；Store/企业/身份、编辑上下文、底层 URI 或可编辑状态变化后，旧回调失效。填写尚未应用的参数不改变原配置。

## 验证

真实 Store 页面/Router/适配器配合 mock native bridge：凭据编码、非法参数、取消、重复应用、URI 修改、Store/企业/身份切换、权限读取失败、保存期间、卸载、原 CAS 与零副作用。另有独立表单组件上下文替换与禁用恢复回归。

浏览器夹具 `test/fixtures/store-proxy-draft.html` 使用真实外壳、认证门、Store 页面和编辑器，原生副作用与 fetch 均拒绝；仅保留方法计数，不记录载荷。实际 v4 HTML 原型、1280px/390px 浅色、深色、Tab/Shift+Tab、Escape 返回焦点与 Enter 应用的像素/交互证据在私有本地核验目录。

运行验证仅使用离线夹具，不部署、执行真实代理或启动产品。公开候选的准确 HEAD、CI 与评审结果以对应 Draft PR 为准。发布方检查目录继续为空；参数格式有效不证明连接、地区或商家登录可用。
