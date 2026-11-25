// Package generator 根据 config.json 确定性生成 Swift 配置、XcodeGen 工程描述、Info.plist 与占位资源。
package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"com.glhpx.humming/tools/internal/config"
)

// Options 控制生成行为。
type Options struct {
	Root       string
	ConfigPath string
	// SkipIcon 若目标图标已存在则不覆盖。
	SkipExistingIcon bool
}

// Result 汇总生成产物。
type Result struct {
	Files []string
}

// Generate 读取配置并写出全部生成文件。
func Generate(opts Options) (*Result, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	if opts.Root == "" {
		return nil, fmt.Errorf("项目根目录不能为空")
	}

	out := &Result{}

	swiftPath := filepath.Join(opts.Root, "App", "Config", "GeneratedConfig.swift")
	if err := writeFile(swiftPath, renderGeneratedSwift(cfg)); err != nil {
		return nil, err
	}
	out.Files = append(out.Files, swiftPath)

	plistPath := filepath.Join(opts.Root, "App", "Resources", "Info.plist")
	plist, err := renderInfoPlist(cfg)
	if err != nil {
		return nil, err
	}
	if err := writeFile(plistPath, plist); err != nil {
		return nil, err
	}
	out.Files = append(out.Files, plistPath)

	ymlPath := filepath.Join(opts.Root, "project.yml")
	if err := writeFile(ymlPath, renderProjectYML(cfg)); err != nil {
		return nil, err
	}
	out.Files = append(out.Files, ymlPath)

	assets := []string{
		filepath.Join(opts.Root, "App", "Resources", "Assets.xcassets", "Contents.json"),
		filepath.Join(opts.Root, "App", "Resources", "Assets.xcassets", "AppIcon.appiconset", "Contents.json"),
		filepath.Join(opts.Root, "App", "Resources", "Assets.xcassets", "AccentColor.colorset", "Contents.json"),
	}
	for _, p := range assets {
		var content []byte
		switch {
		case strings.HasSuffix(p, "Assets.xcassets"+string(filepath.Separator)+"Contents.json") || strings.HasSuffix(p, "Assets.xcassets/Contents.json"):
			content = []byte(assetsContentsJSON)
		case strings.Contains(p, "AppIcon.appiconset"):
			content = []byte(appIconContentsJSON)
		case strings.Contains(p, "AccentColor.colorset"):
			content = []byte(accentColorContentsJSON)
		}
		if err := writeFile(p, content); err != nil {
			return nil, err
		}
		out.Files = append(out.Files, p)
	}

	iconPath := filepath.Join(opts.Root, "App", "Resources", "Assets.xcassets", "AppIcon.appiconset", "AppIcon.png")
	if opts.SkipExistingIcon {
		if _, err := os.Stat(iconPath); err == nil {
			out.Files = append(out.Files, iconPath)
		} else {
			pngData, err := renderPlaceholderIcon()
			if err != nil {
				return nil, err
			}
			if err := writeFile(iconPath, pngData); err != nil {
				return nil, err
			}
			out.Files = append(out.Files, iconPath)
		}
	} else {
		if _, err := os.Stat(iconPath); err == nil {
			// 不覆盖用户手工提供的图标。
			out.Files = append(out.Files, iconPath)
		} else {
			pngData, err := renderPlaceholderIcon()
			if err != nil {
				return nil, err
			}
			if err := writeFile(iconPath, pngData); err != nil {
				return nil, err
			}
			out.Files = append(out.Files, iconPath)
		}
	}

	launchPath := filepath.Join(opts.Root, "App", "Resources", "LaunchScreen.storyboard")
	if _, err := os.Stat(launchPath); err != nil {
		if err := writeFile(launchPath, []byte(launchScreenStoryboard)); err != nil {
			return nil, err
		}
	}
	out.Files = append(out.Files, launchPath)

	return out, nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func renderGeneratedSwift(cfg *config.Config) []byte {
	hosts := make([]string, len(cfg.WebView.AllowedHosts))
	for i, h := range cfg.WebView.AllowedHosts {
		hosts[i] = QuoteSwift(h)
	}
	origins := make([]string, len(cfg.Bridge.AllowedOrigins))
	for i, o := range cfg.Bridge.AllowedOrigins {
		origins[i] = QuoteSwift(o)
	}

	data := map[string]any{
		"Name":                QuoteSwift(cfg.App.Name),
		"BundleID":            QuoteSwift(cfg.App.BundleID),
		"Version":             QuoteSwift(cfg.App.Version),
		"Build":               cfg.App.Build,
		"MinimumIOS":          QuoteSwift(cfg.App.MinimumIOS),
		"URL":                 QuoteSwift(cfg.WebView.URL),
		"AllowedHosts":        strings.Join(hosts, ", "),
		"Fullscreen":          cfg.WebView.Fullscreen,
		"HideStatusBar":       cfg.WebView.HideStatusBar,
		"BackgroundColor":     QuoteSwift(cfg.WebView.BackgroundColor),
		"Bounce":              cfg.WebView.Bounce,
		"PullToRefresh":       cfg.WebView.PullToRefresh,
		"SwipeNavigation":     cfg.WebView.SwipeNavigation,
		"JavaScript":          cfg.WebView.JavaScript,
		"PersistentStorage":   cfg.WebView.PersistentStorage,
		"CustomUserAgent":     QuoteSwift(cfg.WebView.CustomUserAgent),
		"UserAgentSuffix":     QuoteSwift(cfg.WebView.UserAgentSuffix),
		"InlineMedia":         cfg.WebView.InlineMedia,
		"MediaAutoplay":       cfg.WebView.MediaAutoplay,
		"ExternalLinks":       QuoteSwift(cfg.WebView.ExternalLinks),
		"Portrait":            cfg.Screen.Portrait,
		"Landscape":           cfg.Screen.Landscape,
		"TimeoutSeconds":      cfg.Network.TimeoutSeconds,
		"AutoRetry":           cfg.Network.AutoRetry,
		"RetryInterval":       cfg.Network.RetryIntervalSeconds,
		"RetryMax":            cfg.Network.RetryMax,
		"BridgeEnabled":       cfg.Bridge.Enabled,
		"AllowedOrigins":      strings.Join(origins, ", "),
		"Camera":              cfg.Permissions.Camera,
		"Microphone":          cfg.Permissions.Microphone,
		"PhotoLibrary":        cfg.Permissions.PhotoLibrary,
		"Location":            cfg.Permissions.Location,
	}

	tmpl := template.Must(template.New("swift").Parse(generatedSwiftTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(err)
	}
	return normalizeNewlines(buf.Bytes())
}

func renderProjectYML(cfg *config.Config) []byte {
	data := map[string]any{
		"BundleID":   cfg.App.BundleID,
		"Version":    cfg.App.Version,
		"Build":      cfg.App.Build,
		"MinimumIOS": cfg.App.MinimumIOS,
	}
	tmpl := template.Must(template.New("yml").Parse(projectYMLTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(err)
	}
	return normalizeNewlines(buf.Bytes())
}

func renderInfoPlist(cfg *config.Config) ([]byte, error) {
	orientations := make([]string, 0, 4)
	if cfg.Screen.Portrait {
		orientations = append(orientations, "UIInterfaceOrientationPortrait")
	}
	if cfg.Screen.Landscape {
		orientations = append(orientations,
			"UIInterfaceOrientationLandscapeLeft",
			"UIInterfaceOrientationLandscapeRight",
		)
	}

	var extra strings.Builder
	if cfg.Permissions.Camera {
		extra.WriteString(`  <key>NSCameraUsageDescription</key>
  <string>用于在可信网页中调用相机功能</string>
`)
	}
	if cfg.Permissions.Microphone {
		extra.WriteString(`  <key>NSMicrophoneUsageDescription</key>
  <string>用于在可信网页中调用麦克风功能</string>
`)
	}
	if cfg.Permissions.PhotoLibrary {
		extra.WriteString(`  <key>NSPhotoLibraryUsageDescription</key>
  <string>用于在可信网页中访问相册</string>
`)
	}
	if cfg.Permissions.Location {
		extra.WriteString(`  <key>NSLocationWhenInUseUsageDescription</key>
  <string>用于在可信网页中提供定位服务</string>
`)
	}

	oriXML := ""
	for _, o := range orientations {
		oriXML += "    <string>" + o + "</string>\n"
	}

	data := map[string]any{
		"DisplayName":      escapeXML(cfg.App.Name),
		"BundleID":         escapeXML(cfg.App.BundleID),
		"Version":          escapeXML(cfg.App.Version),
		"Build":            cfg.App.Build,
		"MinimumIOS":       escapeXML(cfg.App.MinimumIOS),
		"Orientations":     strings.TrimRight(oriXML, "\n"),
		"PermissionKeys":   extra.String(),
		"HideStatusBar":    cfg.WebView.HideStatusBar,
	}
	tmpl := template.Must(template.New("plist").Parse(infoPlistTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return normalizeNewlines(buf.Bytes()), nil
}

func escapeXML(s string) string {
	r := strings.NewReplacer(
		`&`, `&amp;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
		`"`, `&quot;`,
		`'`, `&apos;`,
	)
	return r.Replace(s)
}

func normalizeNewlines(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return b
}

// ContentHash 计算确定性内容哈希，便于测试。
func ContentHash(data []byte) string {
	sum := sha256.Sum256(normalizeNewlines(data))
	return hex.EncodeToString(sum[:])
}

func renderPlaceholderIcon() ([]byte, error) {
	const size = 1024
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{R: 0x1A, G: 0x73, B: 0xE8, A: 0xFF}
	fg := color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	accent := color.RGBA{R: 0x0D, G: 0x47, B: 0xA1, A: 0xFF}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bg)
		}
	}
	// 中心圆形
	cx, cy, r := size/2, size/2, size/5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, fg)
			}
		}
	}
	// 底部横条
	for y := size*7/10; y < size*8/10; y++ {
		for x := size / 4; x < size*3/4; x++ {
			img.Set(x, y, accent)
		}
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const generatedSwiftTemplate = `// 本文件由 hummingctl generate 自动生成，禁止手工修改。
// Generated from config.json — do not edit.

import Foundation

enum GeneratedConfig {
    static let appName: String = {{.Name}}
    static let bundleID: String = {{.BundleID}}
    static let version: String = {{.Version}}
    static let build: Int = {{.Build}}
    static let minimumIOS: String = {{.MinimumIOS}}

    static let startURL: String = {{.URL}}
    static let allowedHosts: [String] = [{{.AllowedHosts}}]
    static let fullscreen: Bool = {{.Fullscreen}}
    static let hideStatusBar: Bool = {{.HideStatusBar}}
    static let backgroundColorHex: String = {{.BackgroundColor}}
    static let bounce: Bool = {{.Bounce}}
    static let pullToRefresh: Bool = {{.PullToRefresh}}
    static let swipeNavigation: Bool = {{.SwipeNavigation}}
    static let javaScriptEnabled: Bool = {{.JavaScript}}
    static let persistentStorage: Bool = {{.PersistentStorage}}
    static let customUserAgent: String = {{.CustomUserAgent}}
    static let userAgentSuffix: String = {{.UserAgentSuffix}}
    static let inlineMedia: Bool = {{.InlineMedia}}
    static let mediaAutoplay: Bool = {{.MediaAutoplay}}
    static let externalLinks: String = {{.ExternalLinks}}

    static let portrait: Bool = {{.Portrait}}
    static let landscape: Bool = {{.Landscape}}

    static let timeoutSeconds: Int = {{.TimeoutSeconds}}
    static let autoRetry: Bool = {{.AutoRetry}}
    static let retryIntervalSeconds: Int = {{.RetryInterval}}
    static let retryMax: Int = {{.RetryMax}}

    static let bridgeEnabled: Bool = {{.BridgeEnabled}}
    static let bridgeAllowedOrigins: [String] = [{{.AllowedOrigins}}]

    static let permissionCamera: Bool = {{.Camera}}
    static let permissionMicrophone: Bool = {{.Microphone}}
    static let permissionPhotoLibrary: Bool = {{.PhotoLibrary}}
    static let permissionLocation: Bool = {{.Location}}
}
`

const projectYMLTemplate = `# 本文件由 hummingctl generate 自动生成，禁止手工修改。
name: Humming
options:
  bundleIdPrefix: com.glhpx
  deploymentTarget:
    iOS: "{{.MinimumIOS}}"
  createIntermediateGroups: true
  groupSortPosition: top
settings:
  base:
    SWIFT_VERSION: "5.0"
    TARGETED_DEVICE_FAMILY: "1,2"
    IPHONEOS_DEPLOYMENT_TARGET: "{{.MinimumIOS}}"
    PRODUCT_BUNDLE_IDENTIFIER: {{.BundleID}}
    MARKETING_VERSION: "{{.Version}}"
    CURRENT_PROJECT_VERSION: {{.Build}}
    DEVELOPMENT_TEAM: ""
    CODE_SIGNING_ALLOWED: "NO"
    CODE_SIGNING_REQUIRED: "NO"
    CODE_SIGN_IDENTITY: ""
    ENABLE_BITCODE: "NO"
targets:
  Humming:
    type: application
    platform: iOS
    deploymentTarget: "{{.MinimumIOS}}"
    sources:
      - path: App
        excludes:
          - "**/*.md"
    settings:
      base:
        PRODUCT_NAME: Humming
        PRODUCT_BUNDLE_IDENTIFIER: {{.BundleID}}
        INFOPLIST_FILE: App/Resources/Info.plist
        GENERATE_INFOPLIST_FILE: false
        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon
        TARGETED_DEVICE_FAMILY: "1,2"
        SUPPORTS_MACCATALYST: false
        LD_RUNPATH_SEARCH_PATHS:
          - "$(inherited)"
          - "@executable_path/Frameworks"
        SWIFT_ACTIVE_COMPILATION_CONDITIONS: "$(inherited)"
  HummingTests:
    type: bundle.unit-test
    platform: iOS
    deploymentTarget: "{{.MinimumIOS}}"
    sources:
      - path: Tests
    dependencies:
      - target: Humming
    settings:
      base:
        PRODUCT_BUNDLE_IDENTIFIER: {{.BundleID}}.tests
        GENERATE_INFOPLIST_FILE: true
        TEST_HOST: "$(BUILT_PRODUCTS_DIR)/Humming.app/Humming"
        BUNDLE_LOADER: "$(TEST_HOST)"
schemes:
  Humming:
    build:
      targets:
        Humming: all
        HummingTests: [test]
    test:
      targets:
        - HummingTests
    run:
      config: Debug
    archive:
      config: Release
`

const infoPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<!-- 本文件由 hummingctl generate 自动生成，禁止手工修改。 -->
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>zh-Hans</string>
  <key>CFBundleDisplayName</key>
  <string>{{.DisplayName}}</string>
  <key>CFBundleExecutable</key>
  <string>$(EXECUTABLE_NAME)</string>
  <key>CFBundleIdentifier</key>
  <string>$(PRODUCT_BUNDLE_IDENTIFIER)</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>$(PRODUCT_NAME)</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>{{.Version}}</string>
  <key>CFBundleVersion</key>
  <string>{{.Build}}</string>
  <key>LSRequiresIPhoneOS</key>
  <true/>
  <key>MinimumOSVersion</key>
  <string>{{.MinimumIOS}}</string>
  <key>UILaunchStoryboardName</key>
  <string>LaunchScreen</string>
  <key>UIApplicationSupportsIndirectInputEvents</key>
  <true/>
  <key>UIStatusBarHidden</key>
  <{{if .HideStatusBar}}true{{else}}false{{end}}/>
  <key>UIViewControllerBasedStatusBarAppearance</key>
  <true/>
  <key>UISupportedInterfaceOrientations</key>
  <array>
{{.Orientations}}
  </array>
  <key>UISupportedInterfaceOrientations~ipad</key>
  <array>
{{.Orientations}}
  </array>
  <key>UIApplicationSceneManifest</key>
  <dict>
    <key>UIApplicationSupportsMultipleScenes</key>
    <false/>
    <key>UISceneConfigurations</key>
    <dict>
      <key>UIWindowSceneSessionRoleApplication</key>
      <array>
        <dict>
          <key>UISceneConfigurationName</key>
          <string>Default Configuration</string>
          <key>UISceneDelegateClassName</key>
          <string>$(PRODUCT_MODULE_NAME).SceneDelegate</string>
        </dict>
      </array>
    </dict>
  </dict>
  <key>NSAppTransportSecurity</key>
  <dict>
    <key>NSAllowsArbitraryLoads</key>
    <false/>
  </dict>
{{.PermissionKeys}}</dict>
</plist>
`

const assetsContentsJSON = `{
  "info" : {
    "author" : "xcode",
    "version" : 1
  }
}
`

const appIconContentsJSON = `{
  "images" : [
    {
      "filename" : "AppIcon.png",
      "idiom" : "universal",
      "platform" : "ios",
      "size" : "1024x1024"
    }
  ],
  "info" : {
    "author" : "xcode",
    "version" : 1
  }
}
`

const accentColorContentsJSON = `{
  "colors" : [
    {
      "color" : {
        "color-space" : "srgb",
        "components" : {
          "alpha" : "1.000",
          "blue" : "0.910",
          "green" : "0.451",
          "red" : "0.102"
        }
      },
      "idiom" : "universal"
    }
  ],
  "info" : {
    "author" : "xcode",
    "version" : 1
  }
}
`

const launchScreenStoryboard = `<?xml version="1.0" encoding="UTF-8"?>
<document type="com.apple.InterfaceBuilder3.CocoaTouch.Storyboard.XIB" version="3.0" toolsVersion="21701" targetRuntime="iOS.CocoaTouch" propertyAccessControl="none" useAutolayout="YES" launchScreen="YES" useTraitCollections="YES" useSafeAreas="YES" colorMatched="YES" initialViewController="01J-lp-oVM">
    <device id="retina6_1" orientation="portrait" appearance="light"/>
    <dependencies>
        <deployment identifier="iOS"/>
        <plugIn identifier="com.apple.InterfaceBuilder.IBCocoaTouchPlugin" version="21678"/>
        <capability name="Safe area layout guides" minToolsVersion="9.0"/>
        <capability name="documents saved in the Xcode 8 format" minToolsVersion="8.0"/>
    </dependencies>
    <scenes>
        <scene sceneID="EHf-IW-A2E">
            <objects>
                <viewController id="01J-lp-oVM" sceneMemberID="viewController">
                    <view key="view" contentMode="scaleToFill" id="Ze5-6b-2t3">
                        <rect key="frame" x="0.0" y="0.0" width="414" height="896"/>
                        <autoresizingMask key="autoresizingMask" widthSizable="YES" heightSizable="YES"/>
                        <subviews>
                            <label opaque="NO" userInteractionEnabled="NO" contentMode="left" text="Humming" textAlignment="center" lineBreakMode="tailTruncation" translatesAutoresizingMaskIntoConstraints="NO" id="brand-label">
                                <rect key="frame" x="20" y="430" width="374" height="36"/>
                                <fontDescription key="fontDescription" type="system" weight="semibold" pointSize="28"/>
                                <color key="textColor" red="0.1019607843" green="0.4509803922" blue="0.9098039216" alpha="1" colorSpace="custom" customColorSpace="sRGB"/>
                                <nil key="highlightedColor"/>
                            </label>
                        </subviews>
                        <viewLayoutGuide key="safeArea" id="Bcu-3y-fUS"/>
                        <color key="backgroundColor" red="1" green="1" blue="1" alpha="1" colorSpace="custom" customColorSpace="sRGB"/>
                        <constraints>
                            <constraint firstItem="brand-label" firstAttribute="centerX" secondItem="Ze5-6b-2t3" secondAttribute="centerX" id="cX"/>
                            <constraint firstItem="brand-label" firstAttribute="centerY" secondItem="Ze5-6b-2t3" secondAttribute="centerY" id="cY"/>
                            <constraint firstItem="brand-label" firstAttribute="leading" secondItem="Bcu-3y-fUS" secondAttribute="leading" constant="20" id="l"/>
                            <constraint firstItem="Bcu-3y-fUS" firstAttribute="trailing" secondItem="brand-label" secondAttribute="trailing" constant="20" id="t"/>
                        </constraints>
                    </view>
                </viewController>
                <placeholder placeholderIdentifier="IBFirstResponder" id="iYj-Kq-Ea1" userLabel="First Responder" sceneMemberID="firstResponder"/>
            </objects>
            <point key="canvasLocation" x="53" y="375"/>
        </scene>
    </scenes>
</document>
`
