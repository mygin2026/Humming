# Humming — Cursor Agent 完整开发提示词

## 1. 角色与任务

你是一名资深 iOS 原生开发工程师、WebKit 工程师和 CI/CD 工程师。请直接在当前工作区从零创建完整的 Humming 项目，而不是只输出教程、示例代码或伪代码。项目用于将远程 HTTPS H5 网站封装为轻量级 iOS 原生应用，支持后续合法的企业内部签名和分发。

开发者使用 Windows + Cursor，没有本地 macOS、Xcode 或 iOS SDK。所有源码编写、配置生成和静态校验应能在 Windows 完成；真实 iOS 编译由 GitHub Actions 的 macOS Runner 完成。不得声称在 Windows 上完成了 iOS 编译或真机测试。

## 2. 技术栈与强制约束

- 项目名、Target、Scheme、默认显示名称统一为 `Humming`。
- Swift + UIKit + WKWebView，最低部署目标 iOS 15.0，真机架构 arm64。
- 使用 XcodeGen 生成 Xcode 工程，生成的工程不作为配置源。
- 使用 JSON 集中管理应用配置；生成工具使用 Golang 标准库（Go 1.22 或更高版本），不依赖第三方 Go 模块、不使用 CGO。XcodeGen 自身的 project.yml 可由生成器输出 JSON 兼容的 YAML 内容。
- 不使用 Flutter、React Native、Cordova、Capacitor、CocoaPods 或第三方运行时 SDK。
- 不使用私有 API、不绕过 TLS 校验、不注入隐蔽跟踪代码、不收集非必要设备标识。
- 编译与签名完全分离；CI 输出未签名 IPA，不导入任何企业证书或私钥，不提供绕过系统签名验证的功能。
- 企业分发仅用于符合 Apple Developer Enterprise Program 条款的内部应用。
- 所有代码可维护、模块职责清晰，注释和 README 使用中文；Swift 标识符使用规范英文。

## 3. 项目目录

```text
Humming/
├── .github/workflows/
│   ├── validate.yml
│   └── build-ios.yml
├── App/
│   ├── Application/
│   │   ├── AppDelegate.swift
│   │   └── SceneDelegate.swift
│   ├── Config/
│   │   ├── AppConfig.swift
│   │   └── GeneratedConfig.swift
│   ├── Controllers/
│   │   └── WebViewController.swift
│   ├── WebKit/
│   │   ├── WebViewFactory.swift
│   │   ├── NavigationPolicy.swift
│   │   └── WebViewCoordinator.swift
│   ├── Bridge/
│   │   ├── JSBridge.swift
│   │   └── WeakScriptMessageHandler.swift
│   ├── Services/
│   │   ├── NetworkMonitor.swift
│   │   └── RetryController.swift
│   ├── Views/
│   │   ├── LoadingView.swift
│   │   └── ErrorView.swift
│   └── Resources/
│       ├── Assets.xcassets/
│       ├── LaunchScreen.storyboard
│       └── Info.plist
├── Tests/
│   ├── NavigationPolicyTests.swift
│   └── RetryControllerTests.swift
├── cmd/
│   └── hummingctl/
│       └── main.go
├── internal/
│   ├── config/
│   ├── generator/
│   └── packaging/
├── go.mod
├── config.json
├── project.yml
├── .gitignore
└── README.md
```

可根据实际实现补充文件，但不要无意义地拆分模块。生成文件必须标记为自动生成，禁止手工修改；使用确定性生成流程并在 README 说明哪些文件提交 Git。

## 4. 集中配置

创建以下默认 `config.json`，使用以下正式 URL 和 Bundle ID，不得替换为示例值：

```json
{
  "app": {
    "name": "Humming",
    "bundle_id": "com.glhpx.humming",
    "version": "1.0.0",
    "build": 1,
    "minimum_ios": "15.0"
  },
  "webview": {
    "url": "https://app.cs5.top/",
    "allowed_hosts": ["app.cs5.top"],
    "fullscreen": true,
    "hide_status_bar": false,
    "background_color": "#FFFFFF",
    "bounce": false,
    "pull_to_refresh": false,
    "swipe_navigation": true,
    "javascript": true,
    "persistent_storage": true,
    "custom_user_agent": "",
    "user_agent_suffix": "Humming/1.0.0",
    "inline_media": true,
    "media_autoplay": true,
    "external_links": "safari"
  },
  "screen": {
    "portrait": true,
    "landscape": false
  },
  "network": {
    "timeout_seconds": 30,
    "auto_retry": true,
    "retry_interval_seconds": 3,
    "retry_max": 5
  },
  "bridge": {
    "enabled": true,
    "allowed_origins": ["https://app.cs5.top"]
  },
  "permissions": {
    "camera": false,
    "microphone": false,
    "photo_library": false,
    "location": false
  }
}
```

生成器必须校验必填字段、类型、未知字段、Bundle ID、版本号、HTTPS URL、主机白名单、可信 Origin、颜色、重试参数和至少一种屏幕方向。禁止默认允许任意域名；禁止 URL 中的用户名密码。生成 Swift 常量时正确转义字符串，避免配置内容变成可执行源码。根据权限开关生成 Info.plist 对应说明；未实现的权限功能不得假装可用。配置文件内不得保存任何私钥、密码、API Token。

### 4.1 Go 工具链实现要求

- 根目录初始化 `go.mod`，module 为 `com.glhpx.humming/tools`，最低 Go 1.22；统一使用 `CGO_ENABLED=0`，仅允许 Go 标准库。禁止引入 Python、pip、Node.js、GCC 或其他额外脚本运行时。
- `hummingctl generate`：读取并验证 config.json，生成 GeneratedConfig.swift、project.yml、Info.plist 与必要占位资源；使用 encoding/json、text/template（配合安全 Swift 字符串转义）、encoding/xml、image/png、os、path/filepath 等标准库。生成内容应确定性一致；失败退出非零状态，不覆盖用户手工提供的图标。
- `hummingctl validate`：严格验证 JSON（拒绝未知字段和尾随数据），检查必需资源、生成文件与配置一致性；使用 Go 单元测试验证合法及非法输入。
- `hummingctl package --app <路径> --output <路径>`：使用 archive/zip、io/fs、crypto/sha256 实现 IPA 打包与 SHA-256 文件生成，保留 Unix 可执行权限、资源和必要的符号链接，拒绝路径穿越，检查 ZIP 内部结构。在 macOS CI 中另用 plutil、lipo、vtool 等系统工具检查实际产物的 Info.plist、架构及平台（不依赖 Go 第三方 plist 库）。
- 使用 filepath 处理本地路径、path 处理 ZIP 条目；兼容 Windows 和 macOS。所有子命令支持 --help、明确错误信息和非零失败退出码。不要将 Go 工具编译进 iOS APP；Go 仅用于开发与构建辅助，APP 仍采用 Swift 原生实现。
- README 提供可选独立工具编译命令：`go build -o hummingctl.exe ./cmd/hummingctl`（Windows）。GitHub Actions 中通过 actions/setup-go 读取 go.mod 版本；执行 go test ./... 和 go vet ./...，无需 Python。

## 5. 原生应用与 WebView

- 使用 UIKit 程序化创建 UIWindow、UIViewController 和 WKWebView，使用 Scene 生命周期，禁止主界面依赖 storyboard；LaunchScreen 可使用 storyboard。
- APP 无原生导航栏和 TabBar。支持全屏内容、状态栏隐藏开关、安全区域、刘海屏和灵动岛适配。默认内容避开系统遮挡，H5 可通过 CSS `env(safe-area-inset-*)` 自行适配；全屏与安全区行为应明确定义。
- 支持竖屏及可配置横屏、iPad 适配、系统键盘、前后台切换。不在每次回到前台时强制刷新。
- 通过 WKWebViewConfiguration 初始化媒体策略、JavaScript、WKWebsiteDataStore.default() 或 nonPersistent()。持久模式支持 Cookie、LocalStorage、IndexedDB，具体行为遵循 WebKit 限制。
- 支持 HTTPS HLS/m3u8、HTML5 视频、内联及全屏播放；自动播放配置不代表能绕过系统策略。使用 allowsInlineMediaPlayback 和 mediaTypesRequiringUserActionForPlayback。
- 默认保留 WebKit 原生 User-Agent，通过 applicationNameForUserAgent 追加版本标记；只有显式配置才替换 UA。
- 支持手势返回、可选下拉刷新和滚动回弹。加载进度采用 estimatedProgress，正确注销观察者。
- 处理 target=_blank、window.open：同一可信站点在当前 WebView 打开，外部 HTTPS 依策略交给系统浏览器或阻止，不为不可信页面开放原生桥接。
- 处理 Web 内容进程终止，有限重载恢复，不产生无限刷新循环。
- 所有 UI 更新在主线程进行；避免闭包、WKScriptMessageHandler、delegate 的循环引用。

## 6. 网络、错误页与安全策略

使用 NWPathMonitor 监听网络；网络不可用时展示原生中文错误页和手动重试按钮。首次加载和明确的临时网络错误可执行有上限的重试，网络恢复后按当前状态决定是否重载；避免重复调度。不得自动重放 POST、支付、表单提交等非幂等请求。区分用户取消导航、离线、超时、HTTP 错误、TLS 错误、内容进程终止，TLS 错误不得通过重试降低安全性。

导航策略要求：

- 仅允许可信 HTTPS 主机进行主框架内部导航，使用 URLComponents 解析并进行规范化后的精确主机比较；禁止简单字符串包含匹配。重定向也应检查，限制非默认端口或显式配置端口。
- 外部链接仅在符合配置及明确用户操作时交由系统处理。禁止任意自定义 Scheme、file、javascript 等进入主框架；确有需要的 tel/mailto 等后续单独实现并验证。
- 不设置 NSAllowsArbitraryLoads，不忽略证书错误，不启用任意文件访问。禁止为了兼容 HTTP 而关闭 ATS。
- 主框架导航白名单并不等于所有子资源防火墙；H5 站点应配置 CSP、正确的 Cookie 属性及服务端权限控制。在 README 明确这一点。
- Release 默认不启用 Web Inspector；Debug 如需启用，使用系统版本检查。
- 日志不得打印 Cookie、Token、请求正文、完整敏感 URL 或个人信息。

## 7. JavaScript Bridge

使用 `window.webkit.messageHandlers.humming.postMessage(...)` 实现 H5 到 Native 的通信；设计版本化协议，包含 `id`、`method`、`params`。例如：

```javascript
window.webkit.messageHandlers.humming.postMessage({
  id: "req-001",
  method: "getAppInfo",
  params: {}
});
```

返回协议为 `{id, ok, result}` 或 `{id, ok, error:{code,message}}`，通过安全 JSON 序列化返回给 H5；优先使用支持结构化参数的 WebKit API，避免拼接不可信 JavaScript。H5 侧提供一个最小 Promise 封装示例，包含超时及错误处理。

默认仅实现 `getAppInfo`（名称、版本、build）、`getNetworkStatus`、`getCapabilities`；可实现需要用户确认的 `openExternalURL`，必须复用导航 URL 安全策略。禁止暴露任意原生方法、Shell、文件读写、任意 HTTP 代理、设备永久唯一标识或任意 JavaScript 执行接口。

每次调用均验证 bridge 开关、message.frameInfo.isMainFrame、frame securityOrigin（scheme、host、有效端口）及当前顶层可信页面。限制请求大小、方法名称、参数类型和 ID；拒绝未知方法与不可信 iframe。页面切换时清理旧请求，防止结果发送到其他页面。通过弱引用代理注册 handler，并在销毁时移除。

## 8. 权限及资源

默认不申请相机、麦克风、相册、定位或通知权限。只有实际功能、配置开关和系统 API 均支持时才按用户操作触发请求；为开启的功能生成明确的中文用途说明。WebKit 权限 delegate 需校验可信来源，不能无条件授权。对 iOS 版本差异使用可用性检查；未实现能力在 getCapabilities 中返回不支持，并在 README 标注。

创建有效的 Assets.xcassets、AppIcon 和 LaunchScreen，提供可替换的 1024×1024 无透明通道占位 PNG（可由 Go 标准库 image、image/color、image/png 生成简单几何图案）。不要下载来路不明的品牌素材。启动页简洁，符合系统规范，不添加固定等待时间。README 说明如何替换图标、修改颜色、名称和启动页。

## 9. Windows 本地工作流

本地仅需 Go、Git 和 Cursor。无需安装 Python。

README 必须提供 PowerShell 命令：

```powershell
$env:CGO_ENABLED = "0"
go run ./cmd/hummingctl generate
go run ./cmd/hummingctl validate
go test ./...
go vet ./...
git init
git add .
git commit -m "Initialize Humming"
```

Go CLI 应实际支持上述命令；补充 Go 单元测试（testing 包），覆盖配置错误、生成确定性、路径及 IPA 包装逻辑。不得要求 Windows 安装 Xcode、Homebrew 或 Swift iOS SDK。文档说明推送至用户自己的 GitHub 私有仓库的方式，但未经授权不得自行创建或推送远程仓库。

## 10. GitHub Actions 自动编译

创建 `validate.yml`：在 push/pull_request 时使用 Windows 和 Ubuntu Runner 矩阵，通过 actions/setup-go 安装 go.mod 指定的 Go 版本，运行配置生成、静态校验及 Go 单元测试。

创建 `build-ios.yml`：支持 workflow_dispatch 手动构建，以及 main 分支 push（可按路径过滤）。使用 GitHub 当前公开支持且能提供所需 Xcode/iOS SDK 的 macOS Runner；优先选择明确的 Runner 标签，并在 README 写明如何更换。若无法联网核实标签或 Action 版本，标记待 CI 验证，不凭空指定版本。打印 Xcode、SDK、Swift 版本。

构建步骤：checkout → setup-go → Go 配置验证与生成（go run ./cmd/hummingctl validate 和 go run ./cmd/hummingctl generate） → 安装 XcodeGen → 生成工程 → 检查 scheme → 编译 → IPA 包装 → 结构验证 → 上传 artifact。默认权限 `contents: read`，设置合理 timeout，禁止输出秘密。

核心命令（根据实际工程路径调整）：

```bash
xcodegen generate
xcodebuild -project Humming.xcodeproj -scheme Humming \
  -configuration Release -sdk iphoneos \
  -destination 'generic/platform=iOS' -derivedDataPath build \
  ARCHS=arm64 CODE_SIGNING_ALLOWED=NO CODE_SIGNING_REQUIRED=NO \
  CODE_SIGN_IDENTITY='' build
go run ./cmd/hummingctl package --app build/Build/Products/Release-iphoneos/Humming.app --output dist/Humming-Unsigned.ipa
```

IPA 必须是合法 ZIP 容器，根目录为 `Payload/Humming.app/`。保留文件权限和资源，检查 Info.plist、CFBundleExecutable、Bundle ID、版本号和 arm64 真机 Mach-O；使用 macOS 工具检查架构与平台，不能把模拟器产物当真机包。生成 SHA-256 校验文件，上传 IPA 和校验文件。未签名 IPA 不可直接安装，README 必须醒目标明。

测试阶段在 macOS 上选择实际可用的 iOS Simulator（通过 xcodebuild -showdestinations 或 simctl 查询），运行 XCTest。不要硬编码不存在的设备名称。构建失败需保留必要日志，不隐藏失败、不使用 continue-on-error 掩盖关键检查。

## 11. 测试与验收标准

- [ ] 所有目录与源码真实存在，无 TODO 代替核心功能、空函数或伪代码。
- [ ] Windows 可以生成和校验配置，Go 测试通过。
- [ ] XcodeGen 配置包含 Humming Target/Scheme、UIKit 入口、资源、iOS 15.0 部署目标及测试 Target。
- [ ] macOS CI 可以编译 arm64 真机 Release 应用，并生成 Humming-Unsigned.ipa 和 SHA-256。
- [ ] XCTest 覆盖可信主机、恶意相似域名、子域名、端口、非 HTTPS、外部链接策略、重试上限和桥接来源判断。
- [ ] 真机测试清单包括启动、HTTPS 页面、Cookie 登录保持、HLS、键盘、安全区、横竖屏、断网重连、手动重试、外部链接、桥接拒绝及前后台切换。
- [ ] 无第三方运行依赖、私钥、证书、签名秘密或关闭 TLS 校验的逻辑。
- [ ] README 包含配置字段说明、Windows 使用流程、GitHub Actions 下载产物步骤、已知限制和后续企业签名所需条件。
- [ ] 企业签名需要有效的企业分发证书、匹配 Bundle ID 的 In-House provisioning profile 及正确 entitlements；本项目不实施签名，也不承诺未签名包可以安装。

## 12. Cursor Agent 执行指令

1. 检查当前工作区，保护已有用户文件，不无故覆盖或删除。
2. 按上述需求创建完整项目文件、配置生成器、资源、测试和工作流。
3. 在 Windows 可用环境运行 Go 校验和测试，并修复所有实际发现的问题。
4. 对 Swift API、生命周期、内存管理、iOS 15 兼容性及安全策略进行代码审查，优先使用官方 Apple 文档验证不确定 API。
5. 如果没有 macOS 或 GitHub 执行权限，明确将真实编译、模拟器和真机测试列为未验证，不伪造成功日志；不得擅自部署或推送。
6. 最后输出简洁的交付总结：创建的文件、已实际运行的命令及结果、尚未验证的事项、已配置的 H5 URL/白名单/Bundle ID、首次云端构建步骤。

现在开始直接实施 Humming，不要只回复设计方案。除非遇到会造成数据丢失或必须取得用户凭据的操作，否则采用上述默认配置继续完成开发。
