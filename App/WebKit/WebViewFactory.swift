import WebKit

enum WebViewFactory {
    static func makeConfiguration() -> WKWebViewConfiguration {
        let configuration = WKWebViewConfiguration()
        configuration.allowsInlineMediaPlayback = AppConfig.inlineMedia
        if AppConfig.mediaAutoplay {
            configuration.mediaTypesRequiringUserActionForPlayback = []
        } else {
            configuration.mediaTypesRequiringUserActionForPlayback = .all
        }

        let preferences = WKPreferences()
        preferences.javaScriptCanOpenWindowsAutomatically = false
        configuration.preferences = preferences

        let webpagePreferences = WKWebpagePreferences()
        webpagePreferences.allowsContentJavaScript = AppConfig.javaScriptEnabled
        configuration.defaultWebpagePreferences = webpagePreferences

        if AppConfig.persistentStorage {
            configuration.websiteDataStore = .default()
        } else {
            configuration.websiteDataStore = .nonPersistent()
        }

        configuration.allowsAirPlayForMediaPlayback = true
        configuration.suppressesIncrementalRendering = false

        if !AppConfig.userAgentSuffix.isEmpty {
            configuration.applicationNameForUserAgent = AppConfig.userAgentSuffix
        }

        return configuration
    }

    static func makeWebView(configuration: WKWebViewConfiguration) -> WKWebView {
        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.allowsBackForwardNavigationGestures = AppConfig.swipeNavigation
        webView.scrollView.bounces = AppConfig.bounce
        webView.scrollView.contentInsetAdjustmentBehavior = .never
        webView.backgroundColor = AppConfig.backgroundColor
        webView.isOpaque = true
        webView.scrollView.backgroundColor = AppConfig.backgroundColor

        if !AppConfig.customUserAgent.isEmpty {
            webView.customUserAgent = AppConfig.customUserAgent
        }

        #if DEBUG
        if #available(iOS 16.4, *) {
            webView.isInspectable = true
        }
        #endif

        return webView
    }
}
