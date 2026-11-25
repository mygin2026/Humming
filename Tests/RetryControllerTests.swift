import XCTest
@testable import Humming

final class RetryControllerTests: XCTestCase {
    func testRespectsRetryLimit() {
        let controller = RetryController(maxRetries: 2, interval: 0.01)
        let exp1 = expectation(description: "retry1")
        let exp2 = expectation(description: "retry2")

        XCTAssertTrue(controller.schedule {
            exp1.fulfill()
        })

        wait(for: [exp1], timeout: 1.0)

        XCTAssertTrue(controller.schedule {
            exp2.fulfill()
        })

        wait(for: [exp2], timeout: 1.0)
        XCTAssertFalse(controller.canRetry)
        XCTAssertFalse(controller.schedule { })
    }

    func testPreventsDuplicateScheduling() {
        let controller = RetryController(maxRetries: 5, interval: 0.2)
        let exp = expectation(description: "once")
        exp.expectedFulfillmentCount = 1

        XCTAssertTrue(controller.schedule {
            exp.fulfill()
        })
        XCTAssertFalse(controller.schedule {
            exp.fulfill()
        })

        wait(for: [exp], timeout: 1.0)
    }

    func testResetClearsAttempts() {
        let controller = RetryController(maxRetries: 1, interval: 0.01)
        let exp1 = expectation(description: "first")
        XCTAssertTrue(controller.schedule { exp1.fulfill() })
        wait(for: [exp1], timeout: 1.0)
        XCTAssertFalse(controller.canRetry)

        controller.reset()
        XCTAssertTrue(controller.canRetry)

        let exp2 = expectation(description: "after-reset")
        XCTAssertTrue(controller.schedule { exp2.fulfill() })
        wait(for: [exp2], timeout: 1.0)
    }

    func testCancelStopsPendingWork() {
        let controller = RetryController(maxRetries: 3, interval: 0.3)
        var fired = false
        XCTAssertTrue(controller.schedule {
            fired = true
        })
        controller.cancel()
        let exp = expectation(description: "wait")
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) {
            exp.fulfill()
        }
        wait(for: [exp], timeout: 1.0)
        XCTAssertFalse(fired)
    }
}
