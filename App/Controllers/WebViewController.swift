import UIKit
import WebKit

final class WebViewController: UIViewController {
    private var webView: WKWebView!
    private let loadingView = LoadingView()
    private let errorView = ErrorView()
    private var refreshControl: UIRefreshControl?

    private let networkMonitor = NetworkMonitor()
    private lazy var navigationPolicy = NavigationPolicy(
        allowedHosts: AppConfig.allowedHosts,
        externalLinks: AppConfig.externalLinksPolicy
    )
    private lazy var coordinator = WebViewCoordinator(navigationPolicy: navigationPolicy)
    private lazy var bridge = JSBridge(navigationPolicy: navigationPolicy, networkMonitor: networkMonitor)
    private lazy var retryController = RetryController(
        maxRetries: AppConfig.retryMax,
        interval: AppConfig.retryIntervalSeconds
    )

    private var hasLoadedOnce = false
    private var isLoading = false
    private var processTerminateCount = 0
    private let maxProcessReloads = 2
    private var loadTimeoutWorkItem: DispatchWorkItem?

    override var prefersStatusBarHidden: Bool {
        AppConfig.hideStatusBar
    }

    override var preferredStatusBarStyle: UIStatusBarStyle {
        .darkContent
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = AppConfig.backgroundColor
        setupWebView()
        setupOverlays()
        setupNetwork()
        loadInitial()
    }

    deinit {
        loadTimeoutWorkItem?.cancel()
        retryController.cancel()
        coordinator.detach()
        bridge.detach()
        networkMonitor.stop()
    }

    private func setupWebView() {
        let configuration = WebViewFactory.makeConfiguration()
        webView = WebViewFactory.makeWebView(configuration: configuration)
        webView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(webView)

        if AppConfig.fullscreen {
            // 全屏铺满；H5 通过 env(safe-area-inset-*) 自行适配。
            NSLayoutConstraint.activate([
                webView.topAnchor.constraint(equalTo: view.topAnchor),
                webView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                webView.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                webView.bottomAnchor.constraint(equalTo: view.bottomAnchor)
            ])
        } else {
            NSLayoutConstraint.activate([
                webView.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
                webView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                webView.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                webView.bottomAnchor.constraint(equalTo: view.safeAreaLayoutGuide.bottomAnchor)
            ])
        }

        coordinator.delegate = self
        coordinator.attach(to: webView)

        if AppConfig.bridgeEnabled {
            bridge.attach(to: webView)
        }

        if AppConfig.pullToRefresh {
            let control = UIRefreshControl()
            control.addTarget(self, action: #selector(handlePullToRefresh), for: .valueChanged)
            webView.scrollView.refreshControl = control
            refreshControl = control
        }
    }

    private func setupOverlays() {
        loadingView.translatesAutoresizingMaskIntoConstraints = false
        errorView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(loadingView)
        view.addSubview(errorView)

        NSLayoutConstraint.activate([
            loadingView.topAnchor.constraint(equalTo: view.topAnchor),
            loadingView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            loadingView.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            loadingView.bottomAnchor.constraint(equalTo: view.bottomAnchor),

            errorView.topAnchor.constraint(equalTo: view.topAnchor),
            errorView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            errorView.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            errorView.bottomAnchor.constraint(equalTo: view.bottomAnchor)
        ])

        errorView.onRetry = { [weak self] in
            self?.manualRetry()
        }
    }

    private func setupNetwork() {
        networkMonitor.onStatusChange = { [weak self] connected in
            guard let self = self else { return }
            if connected {
                if !self.hasLoadedOnce || self.errorView.isHidden == false {
                    self.retryController.reset()
                    self.reloadCurrent(reason: .networkRestored)
                }
            } else if self.isLoading == false && self.hasLoadedOnce == false {
                self.showError(.offline)
            }
        }
        networkMonitor.start()
    }

    private func loadInitial() {
        guard networkMonitor.isConnected else {
            showError(.offline)
            return
        }
        guard let url = AppConfig.startURL else {
            showError(.unknown("启动地址无效"))
            return
        }
        load(url: url, resetRetry: true)
    }

    private enum ReloadReason {
        case manual
        case autoRetry
        case networkRestored
        case processTerminated
        case pullToRefresh
    }

    private func reloadCurrent(reason: ReloadReason) {
        bridge.clearPending()
        if let url = webView.url, navigationPolicy.isTrustedPageURL(url) {
            load(url: url, resetRetry: reason == .manual || reason == .pullToRefresh)
        } else if let start = AppConfig.startURL {
            load(url: start, resetRetry: true)
        }
    }

    private func load(url: URL, resetRetry: Bool) {
        if resetRetry {
            retryController.reset()
        }
        errorView.hide()
        isLoading = true
        loadingView.start()
        scheduleTimeout()

        var request = URLRequest(url: url, cachePolicy: .useProtocolCachePolicy, timeoutInterval: AppConfig.timeoutSeconds)
        request.httpMethod = "GET"
        webView.load(request)
    }

    private func scheduleTimeout() {
        loadTimeoutWorkItem?.cancel()
        let item = DispatchWorkItem { [weak self] in
            guard let self = self, self.isLoading else { return }
            self.webView.stopLoading()
            self.handleLoadFailure(.timeout, allowAutoRetry: true)
        }
        loadTimeoutWorkItem = item
        DispatchQueue.main.asyncAfter(deadline: .now() + AppConfig.timeoutSeconds, execute: item)
    }

    private func manualRetry() {
        retryController.reset()
        processTerminateCount = 0
        reloadCurrent(reason: .manual)
    }

    @objc private func handlePullToRefresh() {
        reloadCurrent(reason: .pullToRefresh)
    }

    private func showError(_ error: WebLoadError) {
        isLoading = false
        loadTimeoutWorkItem?.cancel()
        loadingView.stop()
        refreshControl?.endRefreshing()
        errorView.show(message: error.userMessage)
    }

    private func handleLoadFailure(_ error: WebLoadError, allowAutoRetry: Bool) {
        isLoading = false
        loadTimeoutWorkItem?.cancel()
        loadingView.stop()
        refreshControl?.endRefreshing()

        if error == .cancelled {
            return
        }

        // TLS 错误不得通过重试降低安全性。
        if allowAutoRetry,
           AppConfig.autoRetry,
           error.isRetryable,
           error != .tls,
           !hasLoadedOnce {
            let scheduled = retryController.schedule { [weak self] in
                self?.reloadCurrent(reason: .autoRetry)
            }
            if scheduled {
                loadingView.start()
                return
            }
        }

        showError(error)
    }

    override func viewWillTransition(to size: CGSize, with coordinator: UIViewControllerTransitionCoordinator) {
        super.viewWillTransition(to: size, with: coordinator)
    }
}

extension WebViewController: WebViewCoordinatorDelegate {
    func coordinatorDidStartLoading(_ coordinator: WebViewCoordinator) {
        isLoading = true
        errorView.hide()
        loadingView.start()
    }

    func coordinator(_ coordinator: WebViewCoordinator, didUpdateProgress progress: Double) {
        loadingView.setProgress(progress)
    }

    func coordinatorDidFinishLoading(_ coordinator: WebViewCoordinator) {
        isLoading = false
        hasLoadedOnce = true
        processTerminateCount = 0
        loadTimeoutWorkItem?.cancel()
        retryController.reset()
        loadingView.stop()
        refreshControl?.endRefreshing()
        errorView.hide()
        bridge.clearPending()
    }

    func coordinator(_ coordinator: WebViewCoordinator, didFail error: WebLoadError) {
        handleLoadFailure(error, allowAutoRetry: true)
    }

    func coordinatorDidTerminateContentProcess(_ coordinator: WebViewCoordinator) {
        processTerminateCount += 1
        if processTerminateCount <= maxProcessReloads {
            handleLoadFailure(.processTerminated, allowAutoRetry: false)
            reloadCurrent(reason: .processTerminated)
        } else {
            showError(.processTerminated)
        }
    }

    func coordinator(_ coordinator: WebViewCoordinator, openExternalURL url: URL) {
        UIApplication.shared.open(url, options: [:], completionHandler: nil)
    }
}
