import UIKit

/// 运行时配置门面，读取自动生成的 GeneratedConfig。
enum AppConfig {
    static var appName: String { GeneratedConfig.appName }
    static var bundleID: String { GeneratedConfig.bundleID }
    static var version: String { GeneratedConfig.version }
    static var build: Int { GeneratedConfig.build }

    static var startURL: URL? {
        URL(string: GeneratedConfig.startURL)
    }

    static var allowedHosts: [String] {
        GeneratedConfig.allowedHosts.map { $0.lowercased() }
    }

    static var fullscreen: Bool { GeneratedConfig.fullscreen }
    static var hideStatusBar: Bool { GeneratedConfig.hideStatusBar }
    static var bounce: Bool { GeneratedConfig.bounce }
    static var pullToRefresh: Bool { GeneratedConfig.pullToRefresh }
    static var swipeNavigation: Bool { GeneratedConfig.swipeNavigation }
    static var javaScriptEnabled: Bool { GeneratedConfig.javaScriptEnabled }
    static var persistentStorage: Bool { GeneratedConfig.persistentStorage }
    static var customUserAgent: String { GeneratedConfig.customUserAgent }
    static var userAgentSuffix: String { GeneratedConfig.userAgentSuffix }
    static var inlineMedia: Bool { GeneratedConfig.inlineMedia }
    static var mediaAutoplay: Bool { GeneratedConfig.mediaAutoplay }
    static var externalLinksPolicy: ExternalLinksPolicy {
        ExternalLinksPolicy(rawValue: GeneratedConfig.externalLinks) ?? .safari
    }

    static var portrait: Bool { GeneratedConfig.portrait }
    static var landscape: Bool { GeneratedConfig.landscape }

    static var timeoutSeconds: TimeInterval {
        TimeInterval(GeneratedConfig.timeoutSeconds)
    }

    static var autoRetry: Bool { GeneratedConfig.autoRetry }
    static var retryIntervalSeconds: TimeInterval {
        TimeInterval(GeneratedConfig.retryIntervalSeconds)
    }

    static var retryMax: Int { GeneratedConfig.retryMax }

    static var bridgeEnabled: Bool { GeneratedConfig.bridgeEnabled }
    static var bridgeAllowedOrigins: [String] { GeneratedConfig.bridgeAllowedOrigins }

    static var backgroundColor: UIColor {
        UIColor(hex: GeneratedConfig.backgroundColorHex) ?? .white
    }

    static var supportedOrientations: UIInterfaceOrientationMask {
        var mask: UIInterfaceOrientationMask = []
        if portrait {
            mask.insert(.portrait)
        }
        if landscape {
            mask.insert(.landscapeLeft)
            mask.insert(.landscapeRight)
        }
        if mask.isEmpty {
            mask = .portrait
        }
        return mask
    }
}

enum ExternalLinksPolicy: String {
    case safari
    case block
}

extension UIColor {
    convenience init?(hex: String) {
        var value = hex.trimmingCharacters(in: .whitespacesAndNewlines)
        if value.hasPrefix("#") {
            value.removeFirst()
        }
        guard value.count == 6, let int = UInt32(value, radix: 16) else {
            return nil
        }
        let r = CGFloat((int >> 16) & 0xFF) / 255.0
        let g = CGFloat((int >> 8) & 0xFF) / 255.0
        let b = CGFloat(int & 0xFF) / 255.0
        self.init(red: r, green: g, blue: b, alpha: 1.0)
    }
}
