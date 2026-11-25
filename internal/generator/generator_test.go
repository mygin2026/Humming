package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEscapeSwiftString(t *testing.T) {
	in := "a\"b\\c\n"
	got := EscapeSwiftString(in)
	want := `a\"b\\c\n`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestGenerateDeterministic(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.json")
	src, err := os.ReadFile(filepath.Join("..", "..", "config.json"))
	if err != nil {
		// 回退内嵌合法配置
		src = []byte(`{
  "app": {"name":"Humming","bundle_id":"com.glhpx.humming","version":"1.0.0","build":1,"minimum_ios":"15.0"},
  "webview": {"url":"https://app.cs5.top/","allowed_hosts":["app.cs5.top"],"fullscreen":true,"hide_status_bar":false,"background_color":"#FFFFFF","bounce":false,"pull_to_refresh":false,"swipe_navigation":true,"javascript":true,"persistent_storage":true,"custom_user_agent":"","user_agent_suffix":"Humming/1.0.0","inline_media":true,"media_autoplay":true,"external_links":"safari"},
  "screen": {"portrait":true,"landscape":false},
  "network": {"timeout_seconds":30,"auto_retry":true,"retry_interval_seconds":3,"retry_max":5},
  "bridge": {"enabled":true,"allowed_origins":["https://app.cs5.top"]},
  "permissions": {"camera":false,"microphone":false,"photo_library":false,"location":false}
}`)
	}
	if err := os.WriteFile(cfgPath, src, 0o644); err != nil {
		t.Fatal(err)
	}

	r1, err := Generate(Options{Root: root, ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("generate 1: %v", err)
	}
	swift1, err := os.ReadFile(filepath.Join(root, "App", "Config", "GeneratedConfig.swift"))
	if err != nil {
		t.Fatal(err)
	}
	h1 := ContentHash(swift1)

	// 删除生成文件再生成一次
	_ = os.Remove(filepath.Join(root, "App", "Config", "GeneratedConfig.swift"))
	r2, err := Generate(Options{Root: root, ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("generate 2: %v", err)
	}
	swift2, err := os.ReadFile(filepath.Join(root, "App", "Config", "GeneratedConfig.swift"))
	if err != nil {
		t.Fatal(err)
	}
	h2 := ContentHash(swift2)
	if h1 != h2 {
		t.Fatalf("生成不具确定性: %s vs %s", h1, h2)
	}
	if len(r1.Files) == 0 || len(r2.Files) == 0 {
		t.Fatal("应生成文件列表")
	}
}

func TestDoesNotOverwriteExistingIcon(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.json")
	src := []byte(`{
  "app": {"name":"Humming","bundle_id":"com.glhpx.humming","version":"1.0.0","build":1,"minimum_ios":"15.0"},
  "webview": {"url":"https://app.cs5.top/","allowed_hosts":["app.cs5.top"],"fullscreen":true,"hide_status_bar":false,"background_color":"#FFFFFF","bounce":false,"pull_to_refresh":false,"swipe_navigation":true,"javascript":true,"persistent_storage":true,"custom_user_agent":"","user_agent_suffix":"Humming/1.0.0","inline_media":true,"media_autoplay":true,"external_links":"safari"},
  "screen": {"portrait":true,"landscape":false},
  "network": {"timeout_seconds":30,"auto_retry":true,"retry_interval_seconds":3,"retry_max":5},
  "bridge": {"enabled":true,"allowed_origins":["https://app.cs5.top"]},
  "permissions": {"camera":false,"microphone":false,"photo_library":false,"location":false}
}`)
	if err := os.WriteFile(cfgPath, src, 0o644); err != nil {
		t.Fatal(err)
	}
	iconDir := filepath.Join(root, "App", "Resources", "Assets.xcassets", "AppIcon.appiconset")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	iconPath := filepath.Join(iconDir, "AppIcon.png")
	if err := os.WriteFile(iconPath, custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(Options{Root: root, ConfigPath: cfgPath}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(iconPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Fatal("不应覆盖用户提供的图标")
	}
}
