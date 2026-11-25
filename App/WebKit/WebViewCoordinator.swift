import UIKit
import WebKit

protocol WebViewCoordinatorDelegate: AnyObject {
    func coordinatorDidStartLoading(_ coordinator: WebViewCoordinator)
    func coordinator(_ coordinator: WebViewCoordinator, didUpdateProgress progress: Double)
    func coordinatorDidFinishLoading(_ coordinator: WebViewCoordinator)
    func coordinator(_ coordinator: WebViewCoordinator, didFail error: WebLoadError)
    func coordinatorDidTerminateContentProcess(_ coordinator: WebViewCoordinator)
    func coordinator(_ coordinator: WebViewCoordinator, openExternalURL url: URL)
}

enum WebLoadError: Equatable {
    case offline
    case timeout
    case cancelled
    case http(Int)
    case tls
    case processTerminated
    case unknown(String)

    var userMessage: String {
        switch self {
        case .offline:
            return "网络不可用，请检查连接后重试"
        case .timeout:
            return "加载超时，请稍后重试"
        case .cancelled:
            return "加载已取消"
        case .http(let code):
            return "服务器返回错误（\(code)）"
        case .tls:
            return "安全连接失败，已拒绝不安全的证书"
        case .processTerminated:
            return "页面进程异常终止"
        case .unknown(let message):
            return message.isEmpty ? "页面加载失败" : message
        }
    }

    var isRetryable: Bool {
        switch self {
        case .offline, .timeout, .http, .processTerminated, .unknown:
            return true
        case .cancelled, .tls:
            return false
        }
    }
}

final class WebViewCoordinator: NSObject {
    weak var delegate: WebViewCoordinatorDelegate?
    private(set) weak var webView: WKWebView?
    private let navigationPolicy: NavigationPolicy
    private var progressObservation: NSKeyValueObservation?

    init(navigationPolicy: NavigationPolicy) {
        self.navigationPolicy = navigationPolicy
        super.init()
    }

    func attach(to webView: WKWebView) {
        self.webView = webView
        webView.navigationDelegate = self
        webView.uiDelegate = self

        progressObservation = webView.observe(\.estimatedProgress, options: [.new]) { [weak self] webView, _ in
            guard let self = self else { return }
            DispatchQueue.main.async {
                self.delegate?.coordinator(self, didUpdateProgress: webView.estimatedProgress)
            }
        }
    }

    func detach() {
        progressObservation?.invalidate()
        progressObservation = nil
        webView?.navigationDelegate = nil
        webView?.uiDelegate = nil
        webView = nil
    }

    deinit {
        detach()
    }

    private func mapError(_ error: Error) -> WebLoadError {
        let nsError = error as NSError
        if nsError.domain == NSURLErrorDomain {
            switch nsError.code {
            case NSURLErrorCancelled:
                return .cancelled
            case NSURLErrorTimedOut:
                return .timeout
            case NSURLErrorNotConnectedToInternet, NSURLErrorNetworkConnectionLost, NSURLErrorDataNotAllowed:
                return .offline
            case NSURLErrorServerCertificateUntrusted,
                 NSURLErrorServerCertificateHasBadDate,
                 NSURLErrorServerCertificateHasUnknownRoot,
                 NSURLErrorServerCertificateNotYetValid,
                 NSURLErrorClientCertificateRejected,
                 NSURLErrorClientCertificateRequired,
                 NSURLErrorSecureConnectionFailed:
                return .tls
            default:
                break
            }
        }
        // 不打印敏感细节
        return .unknown("页面加载失败")
    }
}

extension WebViewCoordinator: WKNavigationDelegate {
    func webView(
        _ webView: WKWebView,
        decidePolicyFor navigationAction: WKNavigationAction,
        decisionHandler: @escaping (WKNavigationActionPolicy) -> Void
    ) {
        guard let url = navigationAction.request.url else {
            decisionHandler(.cancel)
            return
        }

        let isMainFrame = navigationAction.targetFrame?.isMainFrame ?? true
        let userInitiated = navigationAction.navigationType == .linkActivated

        switch navigationPolicy.decide(for: url, isMainFrame: isMainFrame, userInitiated: userInitiated) {
        case .allow:
            decisionHandler(.allow)
        case .openExternal:
            delegate?.coordinator(self, openExternalURL: url)
            decisionHandler(.cancel)
        case .block:
            decisionHandler(.cancel)
        }
    }

    func webView(
        _ webView: WKWebView,
        decidePolicyFor navigationResponse: WKNavigationResponse,
        decisionHandler: @escaping (WKNavigationResponsePolicy) -> Void
    ) {
        if navigationResponse.isForMainFrame,
           let http = navigationResponse.response as? HTTPURLResponse,
           !(200...399).contains(http.statusCode) {
            DispatchQueue.main.async { [weak self] in
                guard let self = self else { return }
                self.delegate?.coordinator(self, didFail: .http(http.statusCode))
            }
            decisionHandler(.cancel)
            return
        }
        decisionHandler(.allow)
    }

    func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) {
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            self.delegate?.coordinatorDidStartLoading(self)
        }
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            self.delegate?.coordinatorDidFinishLoading(self)
        }
    }

    func webView(
        _ webView: WKWebView,
        didFail navigation: WKNavigation!,
        withError error: Error
    ) {
        handleFailure(error)
    }

    func webView(
        _ webView: WKWebView,
        didFailProvisionalNavigation navigation: WKNavigation!,
        withError error: Error
    ) {
        handleFailure(error)
    }

    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            self.delegate?.coordinatorDidTerminateContentProcess(self)
        }
    }

    func webView(
        _ webView: WKWebView,
        didReceive challenge: URLAuthenticationChallenge,
        completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void
    ) {
        // 不绕过 TLS，使用系统默认校验。
        completionHandler(.performDefaultHandling, nil)
    }

    private func handleFailure(_ error: Error) {
        let mapped = mapError(error)
        if mapped == .cancelled {
            return
        }
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            self.delegate?.coordinator(self, didFail: mapped)
        }
    }
}

extension WebViewCoordinator: WKUIDelegate {
    func webView(
        _ webView: WKWebView,
        createWebViewWith configuration: WKWebViewConfiguration,
        for navigationAction: WKNavigationAction,
        windowFeatures: WKWindowFeatures
    ) -> WKWebView? {
        // target=_blank / window.open：可信站点在当前 WebView 打开。
        guard let url = navigationAction.request.url else {
            return nil
        }
        let decision = navigationPolicy.decide(for: url, isMainFrame: true, userInitiated: true)
        switch decision {
        case .allow:
            webView.load(navigationAction.request)
        case .openExternal:
            delegate?.coordinator(self, openExternalURL: url)
        case .block:
            break
        }
        return nil
    }

    @available(iOS 15.0, *)
    func webView(
        _ webView: WKWebView,
        requestMediaCapturePermissionFor origin: WKSecurityOrigin,
        initiatedByFrame frame: WKFrameInfo,
        type: WKMediaCaptureType,
        decisionHandler: @escaping (WKPermissionDecision) -> Void
    ) {
        // 默认未实现相机/麦克风，拒绝授权。
        decisionHandler(.deny)
    }
}
