import XCTest
@testable import Humming

final class NavigationPolicyTests: XCTestCase {
    private var policy: NavigationPolicy!

    override func setUp() {
        super.setUp()
        policy = NavigationPolicy(
            allowedHosts: ["app.cs5.top"],
            externalLinks: .safari
        )
    }

    func testAllowsTrustedHost() {
        let url = URL(string: "https://app.cs5.top/home")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .allow)
    }

    func testBlocksLookalikeDomain() {
        let url = URL(string: "https://app.cs5.top.evil.com/")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .block)
    }

    func testBlocksSubstringConfusion() {
        let url = URL(string: "https://notapp.cs5.top/")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .block)
    }

    func testBlocksSubdomain() {
        let url = URL(string: "https://evil.app.cs5.top/")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .block)
    }

    func testBlocksNonDefaultPort() {
        let url = URL(string: "https://app.cs5.top:8443/")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .block)
    }

    func testBlocksHTTP() {
        let url = URL(string: "http://app.cs5.top/")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .block)
    }

    func testBlocksJavaScriptScheme() {
        let url = URL(string: "javascript:alert(1)")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: true), .block)
    }

    func testBlocksFileScheme() {
        let url = URL(string: "file:///etc/passwd")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .block)
    }

    func testExternalLinkOpensSafariWhenUserInitiated() {
        let url = URL(string: "https://example.com/page")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: true), .openExternal)
    }

    func testExternalLinkBlockedWithoutUserGesture() {
        let url = URL(string: "https://example.com/page")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .block)
    }

    func testExternalLinkBlockedWhenPolicyIsBlock() {
        let blocked = NavigationPolicy(allowedHosts: ["app.cs5.top"], externalLinks: .block)
        let url = URL(string: "https://example.com/page")!
        XCTAssertEqual(blocked.decide(for: url, isMainFrame: true, userInitiated: true), .block)
    }

    func testTrustedOriginRequiresHTTPSAndExactHost() {
        XCTAssertTrue(policy.isTrustedOrigin(scheme: "https", host: "app.cs5.top", port: 443))
        XCTAssertTrue(policy.isTrustedOrigin(scheme: "https", host: "app.cs5.top", port: nil))
        XCTAssertFalse(policy.isTrustedOrigin(scheme: "http", host: "app.cs5.top", port: nil))
        XCTAssertFalse(policy.isTrustedOrigin(scheme: "https", host: "evil.app.cs5.top", port: nil))
        XCTAssertFalse(policy.isTrustedOrigin(scheme: "https", host: "app.cs5.top", port: 8443))
    }

    func testParseURLRejectsCredentials() {
        XCTAssertNil(NavigationPolicy.parseURL("https://user:pass@app.cs5.top/"))
        XCTAssertNotNil(NavigationPolicy.parseURL("https://app.cs5.top/"))
    }

    func testHostNormalizationIgnoresTrailingDotAndCase() {
        let url = URL(string: "https://APP.CS5.TOP./")!
        XCTAssertEqual(policy.decide(for: url, isMainFrame: true, userInitiated: false), .allow)
    }
}
