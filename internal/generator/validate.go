package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"com.glhpx.humming/tools/internal/config"
)

// ValidateProject 验证配置、必需资源以及生成文件与配置的一致性。
func ValidateProject(root, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	required := []string{
		filepath.Join(root, "App", "Application", "AppDelegate.swift"),
		filepath.Join(root, "App", "Application", "SceneDelegate.swift"),
		filepath.Join(root, "App", "Config", "AppConfig.swift"),
		filepath.Join(root, "App", "Controllers", "WebViewController.swift"),
		filepath.Join(root, "App", "WebKit", "WebViewFactory.swift"),
		filepath.Join(root, "App", "WebKit", "NavigationPolicy.swift"),
		filepath.Join(root, "App", "WebKit", "WebViewCoordinator.swift"),
		filepath.Join(root, "App", "Bridge", "JSBridge.swift"),
		filepath.Join(root, "App", "Bridge", "WeakScriptMessageHandler.swift"),
		filepath.Join(root, "App", "Services", "NetworkMonitor.swift"),
		filepath.Join(root, "App", "Services", "RetryController.swift"),
		filepath.Join(root, "App", "Views", "LoadingView.swift"),
		filepath.Join(root, "App", "Views", "ErrorView.swift"),
		filepath.Join(root, "App", "Resources", "LaunchScreen.storyboard"),
		filepath.Join(root, "Tests", "NavigationPolicyTests.swift"),
		filepath.Join(root, "Tests", "RetryControllerTests.swift"),
		filepath.Join(root, "cmd", "hummingctl", "main.go"),
		filepath.Join(root, "go.mod"),
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("缺少必需文件: %s", p)
		}
	}

	expectedSwift := renderGeneratedSwift(cfg)
	swiftPath := filepath.Join(root, "App", "Config", "GeneratedConfig.swift")
	actualSwift, err := os.ReadFile(swiftPath)
	if err != nil {
		return fmt.Errorf("缺少生成文件 GeneratedConfig.swift，请先运行 hummingctl generate")
	}
	if !bytes.Equal(normalizeNewlines(actualSwift), expectedSwift) {
		return fmt.Errorf("GeneratedConfig.swift 与 config.json 不一致，请重新 generate")
	}

	expectedPlist, err := renderInfoPlist(cfg)
	if err != nil {
		return err
	}
	plistPath := filepath.Join(root, "App", "Resources", "Info.plist")
	actualPlist, err := os.ReadFile(plistPath)
	if err != nil {
		return fmt.Errorf("缺少生成文件 Info.plist，请先运行 hummingctl generate")
	}
	if !bytes.Equal(normalizeNewlines(actualPlist), expectedPlist) {
		return fmt.Errorf("Info.plist 与 config.json 不一致，请重新 generate")
	}

	expectedYML := renderProjectYML(cfg)
	ymlPath := filepath.Join(root, "project.yml")
	actualYML, err := os.ReadFile(ymlPath)
	if err != nil {
		return fmt.Errorf("缺少生成文件 project.yml，请先运行 hummingctl generate")
	}
	if !bytes.Equal(normalizeNewlines(actualYML), expectedYML) {
		return fmt.Errorf("project.yml 与 config.json 不一致，请重新 generate")
	}

	iconPath := filepath.Join(root, "App", "Resources", "Assets.xcassets", "AppIcon.appiconset", "AppIcon.png")
	if _, err := os.Stat(iconPath); err != nil {
		return fmt.Errorf("缺少 AppIcon.png")
	}

	return nil
}
