package config

import (
	"strings"
	"testing"
)

func validConfigJSON() string {
	return `{
  "app": {
    "name": "Humming",
    "bundle_id": "com.glhpx.humming",
    "version": "1.0.0",
    "build": 1,
    "minimum_ios": "15.0"
  },
  "webview": {
    "url": "https://app.cs5.top/",
    "allowed_hosts": ["app.cs5.top"],
    "fullscreen": true,
    "hide_status_bar": false,
    "background_color": "#FFFFFF",
    "bounce": false,
    "pull_to_refresh": false,
    "swipe_navigation": true,
    "javascript": true,
    "persistent_storage": true,
    "custom_user_agent": "",
    "user_agent_suffix": "Humming/1.0.0",
    "inline_media": true,
    "media_autoplay": true,
    "external_links": "safari"
  },
  "screen": {
    "portrait": true,
    "landscape": false
  },
  "network": {
    "timeout_seconds": 30,
    "auto_retry": true,
    "retry_interval_seconds": 3,
    "retry_max": 5
  },
  "bridge": {
    "enabled": true,
    "allowed_origins": ["https://app.cs5.top"]
  },
  "permissions": {
    "camera": false,
    "microphone": false,
    "photo_library": false,
    "location": false
  }
}`
}

func TestParseValidConfig(t *testing.T) {
	cfg, err := Parse([]byte(validConfigJSON()))
	if err != nil {
		t.Fatalf("合法配置应通过: %v", err)
	}
	if cfg.App.BundleID != "com.glhpx.humming" {
		t.Fatalf("unexpected bundle id: %s", cfg.App.BundleID)
	}
}

func TestRejectUnknownField(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), `"location": false`, `"location": false, "nfc": true`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝未知字段")
	}
}

func TestRejectTrailingData(t *testing.T) {
	raw := validConfigJSON() + "\n{}"
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝尾随数据")
	}
}

func TestRejectHTTPURL(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), "https://app.cs5.top/", "http://app.cs5.top/", 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝 HTTP URL")
	}
}

func TestRejectURLWithCredentials(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), "https://app.cs5.top/", "https://user:pass@app.cs5.top/", 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝带凭证的 URL")
	}
}

func TestRejectEmptyAllowedHosts(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), `"allowed_hosts": ["app.cs5.top"]`, `"allowed_hosts": []`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝空白名单")
	}
}

func TestRejectHostNotInWhitelist(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), `"allowed_hosts": ["app.cs5.top"]`, `"allowed_hosts": ["other.example.com"]`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("URL 主机不在白名单时应失败")
	}
}

func TestRejectNoOrientation(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), `"portrait": true`, `"portrait": false`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应至少启用一种方向")
	}
}

func TestRejectInvalidColor(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), "#FFFFFF", "white", 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝非法颜色")
	}
}

func TestRejectEnabledUnimplementedPermission(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), `"camera": false`, `"camera": true`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("未实现权限不得开启")
	}
}

func TestRejectInvalidBundleID(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), "com.glhpx.humming", "invalid", 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝非法 Bundle ID")
	}
}

func TestRejectInvalidExternalLinks(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), `"external_links": "safari"`, `"external_links": "chrome"`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝非法 external_links")
	}
}

func TestRejectInvalidOrigin(t *testing.T) {
	raw := strings.Replace(validConfigJSON(), `"allowed_origins": ["https://app.cs5.top"]`, `"allowed_origins": ["https://app.cs5.top/path"]`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("应拒绝带路径的 Origin")
	}
}
