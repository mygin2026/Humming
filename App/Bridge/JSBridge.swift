import UIKit
import WebKit

/// 版本化 H5 ↔ Native 桥接。仅暴露白名单方法。
final class JSBridge: NSObject, WKScriptMessageHandler {
    static let handlerName = "humming"
    static let protocolVersion = 1
    static let maxMessageBytes = 8 * 1024
    static let maxIDLength = 64
    static let maxMethodLength = 64

    private weak var webView: WKWebView?
    private let navigationPolicy: NavigationPolicy
    private var pendingIDs: Set<String> = []
    private var weakHandler: WeakScriptMessageHandler?
    private let networkMonitor: NetworkMonitor

    init(navigationPolicy: NavigationPolicy, networkMonitor: NetworkMonitor) {
        self.navigationPolicy = navigationPolicy
        self.networkMonitor = networkMonitor
        super.init()
    }

    func attach(to webView: WKWebView) {
        detach()
        self.webView = webView
        let proxy = WeakScriptMessageHandler(delegate: self)
        weakHandler = proxy
        webView.configuration.userContentController.add(proxy, name: Self.handlerName)
    }

    func detach() {
        if let webView = webView {
            webView.configuration.userContentController.removeScriptMessageHandler(forName: Self.handlerName)
        }
        weakHandler = nil
        webView = nil
        pendingIDs.removeAll()
    }

    func clearPending() {
        pendingIDs.removeAll()
    }

    deinit {
        detach()
    }

    func userContentController(
        _ userContentController: WKUserContentController,
        didReceive message: WKScriptMessage
    ) {
        guard AppConfig.bridgeEnabled else { return }
        guard message.name == Self.handlerName else { return }
        guard message.frameInfo.isMainFrame else { return }

        let origin = message.frameInfo.securityOrigin
        guard navigationPolicy.isTrustedOrigin(
            scheme: origin.protocol,
            host: origin.host,
            port: Int(origin.port)
        ) else {
            return
        }

        guard navigationPolicy.isTrustedPageURL(webView?.url) else {
            return
        }

        guard isAllowedBridgeOrigin(origin) else {
            return
        }

        let body = message.body
        if JSONSerialization.isValidJSONObject(body),
           let data = try? JSONSerialization.data(withJSONObject: body, options: []),
           data.count > Self.maxMessageBytes {
            return
        }

        guard let dict = body as? [String: Any] else { return }
        guard let id = dict["id"] as? String,
              !id.isEmpty,
              id.count <= Self.maxIDLength,
              isSafeToken(id) else {
            return
        }
        guard let method = dict["method"] as? String,
              !method.isEmpty,
              method.count <= Self.maxMethodLength,
              isSafeToken(method) else {
            reply(id: id, ok: false, errorCode: "invalid_method", errorMessage: "非法方法名")
            return
        }

        let params = dict["params"] as? [String: Any] ?? [:]
        pendingIDs.insert(id)

        switch method {
        case "getAppInfo":
            reply(id: id, ok: true, result: [
                "name": AppConfig.appName,
                "version": AppConfig.version,
                "build": AppConfig.build,
                "protocolVersion": Self.protocolVersion
            ])
        case "getNetworkStatus":
            reply(id: id, ok: true, result: [
                "online": networkMonitor.isConnected,
                "expensive": networkMonitor.isExpensive,
                "constrained": networkMonitor.isConstrained
            ])
        case "getCapabilities":
            reply(id: id, ok: true, result: [
                "bridge": true,
                "camera": false,
                "microphone": false,
                "photoLibrary": false,
                "location": false,
                "openExternalURL": true
            ])
        case "openExternalURL":
            handleOpenExternalURL(id: id, params: params)
        default:
            reply(id: id, ok: false, errorCode: "unknown_method", errorMessage: "未知方法")
        }
    }

    private func handleOpenExternalURL(id: String, params: [String: Any]) {
        guard let raw = params["url"] as? String,
              let url = NavigationPolicy.parseURL(raw),
              let scheme = url.scheme?.lowercased(),
              scheme == "https" else {
            reply(id: id, ok: false, errorCode: "invalid_url", errorMessage: "仅允许 HTTPS URL")
            return
        }

        let decision = navigationPolicy.decide(for: url, isMainFrame: true, userInitiated: true)
        guard decision == .openExternal || decision == .allow else {
            reply(id: id, ok: false, errorCode: "blocked", errorMessage: "URL 被策略拒绝")
            return
        }

        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            guard let host = self.webView?.window?.rootViewController else {
                self.reply(id: id, ok: false, errorCode: "unavailable", errorMessage: "无法展示确认框")
                return
            }
            let alert = UIAlertController(
                title: "打开外部链接",
                message: "是否在系统浏览器中打开？\n\(url.host ?? "")",
                preferredStyle: .alert
            )
            alert.addAction(UIAlertAction(title: "取消", style: .cancel, handler: { [weak self] _ in
                self?.reply(id: id, ok: false, errorCode: "cancelled", errorMessage: "用户取消")
            }))
            alert.addAction(UIAlertAction(title: "打开", style: .default, handler: { [weak self] _ in
                UIApplication.shared.open(url, options: [:]) { success in
                    if success {
                        self?.reply(id: id, ok: true, result: ["opened": true])
                    } else {
                        self?.reply(id: id, ok: false, errorCode: "open_failed", errorMessage: "无法打开链接")
                    }
                }
            }))
            host.present(alert, animated: true)
        }
    }

    private func isAllowedBridgeOrigin(_ origin: WKSecurityOrigin) -> Bool {
        let host = origin.host.lowercased()
        let scheme = origin.protocol.lowercased()
        let port = Int(origin.port)
        let originString: String
        if port == 0 || port == 443 {
            originString = "\(scheme)://\(host)"
        } else {
            originString = "\(scheme)://\(host):\(port)"
        }
        return AppConfig.bridgeAllowedOrigins.contains { allowed in
            let normalized = allowed.trimmingCharacters(in: CharacterSet(charactersIn: "/")).lowercased()
            return normalized == originString
        }
    }

    private func isSafeToken(_ value: String) -> Bool {
        let allowed = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-_."))
        return value.unicodeScalars.allSatisfy { allowed.contains($0) }
    }

    private func reply(
        id: String,
        ok: Bool,
        result: [String: Any]? = nil,
        errorCode: String? = nil,
        errorMessage: String? = nil
    ) {
        guard pendingIDs.contains(id) else { return }
        pendingIDs.remove(id)

        var payload: [String: Any] = [
            "id": id,
            "ok": ok
        ]
        if ok {
            payload["result"] = result ?? [:]
        } else {
            payload["error"] = [
                "code": errorCode ?? "error",
                "message": errorMessage ?? "错误"
            ]
        }

        guard let webView = webView else { return }

        // 使用结构化参数 API，避免拼接不可信 JavaScript。
        let script = "window.dispatchEvent(new CustomEvent('humming://bridge', { detail: payload }));"
        DispatchQueue.main.async {
            webView.callAsyncJavaScript(
                script,
                arguments: ["payload": payload],
                in: nil,
                in: .page,
                completionHandler: nil
            )
        }
    }
}
