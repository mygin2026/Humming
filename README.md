# Humming

将远程 HTTPS H5 网站封装为轻量级 iOS 原生应用（Swift + UIKit + WKWebView）。  
编译与签名完全分离：CI 产出**未签名** `Humming-Unsigned.ipa`，供后续合法的企业内部签名与分发。

> **重要：未签名 IPA 不能直接安装到设备。** 安装需要有效的 Apple Developer Enterprise Program 证书、匹配 Bundle ID 的 In-House Provisioning Profile，以及正确的 entitlements。本仓库不包含任何证书、私钥或签名流程。

## 当前配置

| 项 | 值 |
| --- | --- |
| 显示名称 | Humming |
| Bundle ID | `com.glhpx.humming` |
| 启动 URL | `https://app.cs5.top/` |
| 主机白名单 | `app.cs5.top` |
| Bridge Origin | `https://app.cs5.top` |
| 最低系统 | iOS 15.0 |
| 架构 | arm64（真机） |

## 技术栈

- APP：Swift + UIKit + WKWebView（无 Flutter / RN / Cordova / Capacitor / CocoaPods）
- 工程：XcodeGen（`project.yml` 由工具生成，生成的 `.xcodeproj` 不作为配置源）
- 工具：Go 1.22+ 标准库（`hummingctl`），`CGO_ENABLED=0`，无第三方 Go 模块，无 Python

## 目录结构

```text
Humming/
├── App/                 # iOS 原生源码与资源
├── Tests/               # XCTest
├── cmd/hummingctl/      # Go CLI 入口
├── internal/            # 配置校验 / 生成 / IPA 打包
├── config.json          # 集中配置（唯一配置源）
├── project.yml          # XcodeGen 描述（自动生成）
└── .github/workflows/   # validate + build-ios
```

## 哪些文件应提交 Git

**提交：**

- 全部 Swift 源码、`Tests/`、`config.json`、`go.mod`、工作流、README
- 生成产物：`App/Config/GeneratedConfig.swift`、`App/Resources/Info.plist`、`project.yml`、`Assets.xcassets`、`LaunchScreen.storyboard`
- 占位 `AppIcon.png`（可被用户替换；生成器不会覆盖已存在图标）

**不提交：**

- `Humming.xcodeproj/`、`build/`、`dist/`、`*.ipa`、证书与 profile、私钥

生成文件顶部均标注「自动生成，禁止手工修改」。修改配置后请重新执行 `hummingctl generate`。

## Windows 本地工作流

仅需 Go、Git 与 Cursor，**无需** Xcode / Homebrew / Python / iOS SDK。

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

可选独立编译工具：

```powershell
$env:CGO_ENABLED = "0"
go build -o hummingctl.exe ./cmd/hummingctl
.\hummingctl.exe generate
.\hummingctl.exe validate
```

### hummingctl 子命令

| 命令 | 作用 |
| --- | --- |
| `generate` | 校验 `config.json`，生成 Swift 常量、`project.yml`、`Info.plist`、占位资源 |
| `validate` | 严格校验 JSON，并核对生成文件与源码完整性 |
| `package --app <路径> --output <路径>` | 将 `.app` 打成 IPA，并写出 SHA-256 |

## macOS / Xcode 本地打包

在已安装**官方 Xcode**（App Store → `/Applications/Xcode.app`）、Go、Homebrew 的 Mac 上：

```bash
brew install go xcodegen
# 若尚未选定开发者目录，执行一次：
sudo xcode-select -s /Applications/Xcode.app
chmod +x scripts/build-unsigned-ipa.sh
./scripts/build-unsigned-ipa.sh --skip-test
```

脚本默认使用 `/Applications/Xcode.app`，一般无需再传 `--xcode`。

常用选项：

```bash
./scripts/build-unsigned-ipa.sh --skip-test   # 跳过模拟器测试，更快出包
./scripts/build-unsigned-ipa.sh --clean       # 清理后全量重打包
```

产物：`dist/Humming-Unsigned.ipa` 与 `dist/Humming-Unsigned.ipa.sha256`（未签名，不可直接安装）。

## 配置字段说明

编辑根目录 `config.json`：

- `app`：名称、Bundle ID、版本、build、最低 iOS
- `webview`：启动 URL、`allowed_hosts`（精确主机，禁止任意域名）、全屏/状态栏、背景色、回弹、下拉刷新、手势返回、JS、持久存储、UA、媒体、外部链接策略（`safari` / `block`）
- `screen`：至少启用竖屏或横屏之一
- `network`：超时、自动重试间隔与次数上限
- `bridge`：开关与可信 Origin 列表
- `permissions`：当前**均未实现**，必须保持 `false`（开启会被校验拒绝）

安全约束：

- 仅允许 HTTPS；URL 禁止用户名密码
- 不设置 `NSAllowsArbitraryLoads`，不忽略证书错误
- **主框架导航白名单 ≠ 子资源防火墙**；H5 仍需自行配置 CSP、Cookie 属性与服务端鉴权

## 替换图标与启动页

1. 准备 **1024×1024、无透明通道** 的 PNG
2. 覆盖 `App/Resources/Assets.xcassets/AppIcon.appiconset/AppIcon.png`
3. 修改 `LaunchScreen.storyboard` 或背景色（`webview.background_color`）
4. 修改显示名：改 `config.json` 的 `app.name` 后执行 `generate`

## JavaScript Bridge

H5 调用：

```javascript
window.webkit.messageHandlers.humming.postMessage({
  id: "req-001",
  method: "getAppInfo",
  params: {}
});
```

返回通过 `humming://bridge` CustomEvent 派发：`{id, ok, result}` 或 `{id, ok, error}`。

已实现方法：`getAppInfo`、`getNetworkStatus`、`getCapabilities`、`openExternalURL`（需用户确认）。  
最小 Promise 封装见 `docs/bridge-example.js`。

未实现能力（`getCapabilities` 返回 `false`）：相机、麦克风、相册、定位。

## GitHub Actions

### 校验（`validate.yml`）

在 `windows-latest` 与 `ubuntu-latest` 矩阵上：`generate` → `validate` → `go test` → `go vet`。

### 编译（`build-ios.yml`）

- 触发：`workflow_dispatch`，或 `main` 分支相关路径 push
- Runner：`macos-15`（可改为 `macos-26` / `macos-latest`）
- 步骤：生成校验 → 安装 XcodeGen → 生成工程 → Simulator XCTest → Release arm64 真机编译 → IPA 打包与结构校验 → 上传 Artifact

下载产物：GitHub 仓库 → Actions → 对应运行 → Artifacts → `Humming-Unsigned`。

本地无法在 Windows 上完成 iOS 编译或真机测试；真实编译依赖 GitHub macOS Runner。

### 推送到自己的 GitHub 私有仓库

```powershell
git remote add origin git@github.com:<你的用户名>/Humming.git
git push -u origin main
```

然后在 Actions 中手动运行 **Build iOS**，或向 `main` 推送触发。

## 真机测试清单

推送并由企业证书签名安装后，建议验证：

- [ ] 冷启动进入 `https://app.cs5.top/`
- [ ] HTTPS 页面与 Cookie / LocalStorage 登录保持（持久存储开启时）
- [ ] HLS / HTML5 视频内联播放
- [ ] 键盘弹出与安全区
- [ ] 竖屏（及配置开启时的横屏）
- [ ] 断网错误页、手动重试、网络恢复
- [ ] 外部链接按策略打开或拦截
- [ ] 不可信来源的 Bridge 调用被拒绝
- [ ] 前后台切换不强制刷新

## 已知限制

- Windows 仅能完成配置生成、静态校验与 Go 测试；不能本地编译 IPA
- 未签名包不可安装；本项目不实施企业签名
- 自动播放受系统策略限制，配置无法绕过
- Bridge 仅主框架 + 可信 Origin；不提供任意原生 API / 文件 / Shell
- 子资源、XHR、WebSocket 不在原生白名单防火墙范围内
- 相机 / 麦克风 / 相册 / 定位未实现

## 企业签名所需条件（需自行准备）

1. 有效的 Apple Developer Enterprise Program 账号与分发证书
2. Bundle ID 为 `com.glhpx.humming` 的 In-House Provisioning Profile
3. 匹配的 entitlements
4. 使用 `codesign` / Xcode Organizer 等官方流程签名后分发

本仓库 CI **不会**导入证书或私钥。
