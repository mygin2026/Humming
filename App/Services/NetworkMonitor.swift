import Foundation
import Network

final class NetworkMonitor {
    private let monitor = NWPathMonitor()
    private let queue = DispatchQueue(label: "com.glhpx.humming.network")
    private let lock = NSLock()

    private var _isConnected = true
    private var _isExpensive = false
    private var _isConstrained = false

    var onStatusChange: ((Bool) -> Void)?

    var isConnected: Bool {
        lock.lock()
        defer { lock.unlock() }
        return _isConnected
    }

    var isExpensive: Bool {
        lock.lock()
        defer { lock.unlock() }
        return _isExpensive
    }

    var isConstrained: Bool {
        lock.lock()
        defer { lock.unlock() }
        return _isConstrained
    }

    func start() {
        monitor.pathUpdateHandler = { [weak self] path in
            guard let self = self else { return }
            let connected = path.status == .satisfied
            self.lock.lock()
            let previous = self._isConnected
            self._isConnected = connected
            self._isExpensive = path.isExpensive
            self._isConstrained = path.isConstrained
            self.lock.unlock()

            if previous != connected {
                DispatchQueue.main.async {
                    self.onStatusChange?(connected)
                }
            }
        }
        monitor.start(queue: queue)
    }

    func stop() {
        monitor.cancel()
    }

    deinit {
        monitor.cancel()
    }
}
