import Foundation

/// 主框架导航策略：精确主机白名单比较，禁止简单字符串包含匹配。
enum NavigationDecision: Equatable {
    case allow
    case openExternal
    case block
}

struct NavigationPolicy {
    let allowedHosts: Set<String>
    let externalLinks: ExternalLinksPolicy

    init(allowedHosts: [String], externalLinks: ExternalLinksPolicy) {
        self.allowedHosts = Set(allowedHosts.map { NavigationPolicy.normalizeHost($0) })
        self.externalLinks = externalLinks
    }

    func decide(for url: URL, isMainFrame: Bool, userInitiated: Bool) -> NavigationDecision {
        guard let scheme = url.scheme?.lowercased() else {
            return .block
        }

        // 禁止自定义 scheme / file / javascript 进入主框架。
        switch scheme {
        case "https":
            break
        case "about":
            // about:blank 等仅允许非主框架或短暂空白页
            return isMainFrame ? .block : .allow
        default:
            return .block
        }

        guard let host = url.host, !host.isEmpty else {
            return .block
        }

        let normalizedHost = NavigationPolicy.normalizeHost(host)
        if !isExactAllowedHost(normalizedHost) {
            if isMainFrame {
                if userInitiated, externalLinks == .safari, scheme == "https" {
                    return .openExternal
                }
                return .block
            }
            // 子资源不在此做防火墙；主框架白名单 != 子资源防火墙。
            return .allow
        }

        // 限制非默认端口：仅允许缺省 443 或 URL 未显式声明其他端口。
        if let port = url.port, port != 443 {
            return .block
        }

        return .allow
    }

    func isTrustedOrigin(scheme: String?, host: String?, port: Int?) -> Bool {
        guard let scheme = scheme?.lowercased(), scheme == "https" else {
            return false
        }
        guard let host = host, !host.isEmpty else {
            return false
        }
        if let port = port, port != 443 {
            return false
        }
        return isExactAllowedHost(NavigationPolicy.normalizeHost(host))
    }

    func isTrustedPageURL(_ url: URL?) -> Bool {
        guard let url = url else { return false }
        return decide(for: url, isMainFrame: true, userInitiated: false) == .allow
    }

    private func isExactAllowedHost(_ host: String) -> Bool {
        allowedHosts.contains(host)
    }

    static func normalizeHost(_ host: String) -> String {
        var value = host.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        if value.hasSuffix(".") {
            value.removeLast()
        }
        return value
    }

    /// 解析并规范化 URL，拒绝含用户名密码的地址。
    static func parseURL(_ string: String) -> URL? {
        guard let components = URLComponents(string: string) else {
            return nil
        }
        if components.user != nil || components.password != nil {
            return nil
        }
        return components.url
    }
}
