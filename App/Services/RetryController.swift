import Foundation

/// 有上限的自动重试调度，避免重复调度与无限循环。
final class RetryController {
    private let maxRetries: Int
    private let interval: TimeInterval
    private var attempt = 0
    private var workItem: DispatchWorkItem?
    private var isScheduling = false

    var canRetry: Bool {
        attempt < maxRetries
    }

    var currentAttempt: Int { attempt }

    init(maxRetries: Int, interval: TimeInterval) {
        self.maxRetries = max(0, maxRetries)
        self.interval = max(0.1, interval)
    }

    /// 安排一次重试。若已在调度中或已达上限则返回 false。
    @discardableResult
    func schedule(on queue: DispatchQueue = .main, execute: @escaping () -> Void) -> Bool {
        guard canRetry else { return false }
        guard !isScheduling else { return false }

        isScheduling = true
        attempt += 1
        let item = DispatchWorkItem { [weak self] in
            self?.isScheduling = false
            execute()
        }
        workItem = item
        queue.asyncAfter(deadline: .now() + interval, execute: item)
        return true
    }

    func cancel() {
        workItem?.cancel()
        workItem = nil
        isScheduling = false
    }

    func reset() {
        cancel()
        attempt = 0
    }
}
