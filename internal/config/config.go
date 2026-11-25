// Package config 定义 Humming 应用集中配置结构与严格校验逻辑。
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Config 是 config.json 的完整结构。
type Config struct {
	App         AppConfig         `json:"app"`
	WebView     WebViewConfig     `json:"webview"`
	Screen      ScreenConfig      `json:"screen"`
	Network     NetworkConfig     `json:"network"`
	Bridge      BridgeConfig      `json:"bridge"`
	Permissions PermissionsConfig `json:"permissions"`
}

type AppConfig struct {
	Name        string `json:"name"`
	BundleID    string `json:"bundle_id"`
	Version     string `json:"version"`
	Build       int    `json:"build"`
	MinimumIOS  string `json:"minimum_ios"`
}

type WebViewConfig struct {
	URL               string   `json:"url"`
	AllowedHosts      []string `json:"allowed_hosts"`
	Fullscreen        bool     `json:"fullscreen"`
	HideStatusBar     bool     `json:"hide_status_bar"`
	BackgroundColor   string   `json:"background_color"`
	Bounce            bool     `json:"bounce"`
	PullToRefresh     bool     `json:"pull_to_refresh"`
	SwipeNavigation   bool     `json:"swipe_navigation"`
	JavaScript        bool     `json:"javascript"`
	PersistentStorage bool     `json:"persistent_storage"`
	CustomUserAgent   string   `json:"custom_user_agent"`
	UserAgentSuffix   string   `json:"user_agent_suffix"`
	InlineMedia       bool     `json:"inline_media"`
	MediaAutoplay     bool     `json:"media_autoplay"`
	ExternalLinks     string   `json:"external_links"`
}

type ScreenConfig struct {
	Portrait  bool `json:"portrait"`
	Landscape bool `json:"landscape"`
}

type NetworkConfig struct {
	TimeoutSeconds        int  `json:"timeout_seconds"`
	AutoRetry             bool `json:"auto_retry"`
	RetryIntervalSeconds  int  `json:"retry_interval_seconds"`
	RetryMax              int  `json:"retry_max"`
}

type BridgeConfig struct {
	Enabled        bool     `json:"enabled"`
	AllowedOrigins []string `json:"allowed_origins"`
}

type PermissionsConfig struct {
	Camera       bool `json:"camera"`
	Microphone   bool `json:"microphone"`
	PhotoLibrary bool `json:"photo_library"`
	Location     bool `json:"location"`
}

var (
	bundleIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*(\.[a-zA-Z][a-zA-Z0-9-]*)+$`)
	versionPattern  = regexp.MustCompile(`^\d+(\.\d+){1,2}$`)
	iosVersionPattern = regexp.MustCompile(`^\d+(\.\d+)?$`)
	colorPattern    = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	hostPattern     = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)
)

// knownTopLevelKeys 用于拒绝未知顶层字段。
var knownTopLevelKeys = map[string]struct{}{
	"app": {}, "webview": {}, "screen": {}, "network": {}, "bridge": {}, "permissions": {},
}

var knownAppKeys = map[string]struct{}{
	"name": {}, "bundle_id": {}, "version": {}, "build": {}, "minimum_ios": {},
}

var knownWebViewKeys = map[string]struct{}{
	"url": {}, "allowed_hosts": {}, "fullscreen": {}, "hide_status_bar": {},
	"background_color": {}, "bounce": {}, "pull_to_refresh": {}, "swipe_navigation": {},
	"javascript": {}, "persistent_storage": {}, "custom_user_agent": {}, "user_agent_suffix": {},
	"inline_media": {}, "media_autoplay": {}, "external_links": {},
}

var knownScreenKeys = map[string]struct{}{
	"portrait": {}, "landscape": {},
}

var knownNetworkKeys = map[string]struct{}{
	"timeout_seconds": {}, "auto_retry": {}, "retry_interval_seconds": {}, "retry_max": {},
}

var knownBridgeKeys = map[string]struct{}{
	"enabled": {}, "allowed_origins": {},
}

var knownPermissionKeys = map[string]struct{}{
	"camera": {}, "microphone": {}, "photo_library": {}, "location": {},
}

// Load 读取并严格解析配置文件。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	return Parse(data)
}

// Parse 严格解析配置 JSON：拒绝未知字段与尾随数据。
func Parse(data []byte) (*Config, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("配置为空")
	}

	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("JSON 存在尾随数据")
	}

	for key := range raw {
		if _, ok := knownTopLevelKeys[key]; !ok {
			return nil, fmt.Errorf("未知顶层字段: %s", key)
		}
	}
	required := []string{"app", "webview", "screen", "network", "bridge", "permissions"}
	for _, key := range required {
		if _, ok := raw[key]; !ok {
			return nil, fmt.Errorf("缺少必填字段: %s", key)
		}
	}

	if err := rejectUnknownKeys(raw["app"], knownAppKeys, "app"); err != nil {
		return nil, err
	}
	if err := rejectUnknownKeys(raw["webview"], knownWebViewKeys, "webview"); err != nil {
		return nil, err
	}
	if err := rejectUnknownKeys(raw["screen"], knownScreenKeys, "screen"); err != nil {
		return nil, err
	}
	if err := rejectUnknownKeys(raw["network"], knownNetworkKeys, "network"); err != nil {
		return nil, err
	}
	if err := rejectUnknownKeys(raw["bridge"], knownBridgeKeys, "bridge"); err != nil {
		return nil, err
	}
	if err := rejectUnknownKeys(raw["permissions"], knownPermissionKeys, "permissions"); err != nil {
		return nil, err
	}

	var cfg Config
	strict := json.NewDecoder(bytes.NewReader(trimmed))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("配置结构解析失败: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func rejectUnknownKeys(raw json.RawMessage, known map[string]struct{}, section string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("%s 必须是对象: %w", section, err)
	}
	for key := range obj {
		if _, ok := known[key]; !ok {
			return fmt.Errorf("未知字段: %s.%s", section, key)
		}
	}
	return nil
}

// Validate 校验业务规则。
func (c *Config) Validate() error {
	if strings.TrimSpace(c.App.Name) == "" {
		return fmt.Errorf("app.name 不能为空")
	}
	if !bundleIDPattern.MatchString(c.App.BundleID) {
		return fmt.Errorf("app.bundle_id 格式无效: %s", c.App.BundleID)
	}
	if !versionPattern.MatchString(c.App.Version) {
		return fmt.Errorf("app.version 格式无效: %s", c.App.Version)
	}
	if c.App.Build < 1 {
		return fmt.Errorf("app.build 必须 >= 1")
	}
	if !iosVersionPattern.MatchString(c.App.MinimumIOS) {
		return fmt.Errorf("app.minimum_ios 格式无效: %s", c.App.MinimumIOS)
	}
	minMajor, err := parseIOSMajor(c.App.MinimumIOS)
	if err != nil {
		return err
	}
	if minMajor < 15 {
		return fmt.Errorf("app.minimum_ios 不得低于 15.0")
	}

	if err := validateHTTPSURL(c.WebView.URL, "webview.url"); err != nil {
		return err
	}
	if len(c.WebView.AllowedHosts) == 0 {
		return fmt.Errorf("webview.allowed_hosts 不能为空，禁止默认允许任意域名")
	}
	for i, host := range c.WebView.AllowedHosts {
		if !isValidHost(host) {
			return fmt.Errorf("webview.allowed_hosts[%d] 无效: %s", i, host)
		}
	}
	startHost, err := hostFromURL(c.WebView.URL)
	if err != nil {
		return err
	}
	if !containsHost(c.WebView.AllowedHosts, startHost) {
		return fmt.Errorf("webview.url 主机 %s 不在 allowed_hosts 中", startHost)
	}
	if !colorPattern.MatchString(c.WebView.BackgroundColor) {
		return fmt.Errorf("webview.background_color 必须是 #RRGGBB: %s", c.WebView.BackgroundColor)
	}
	switch c.WebView.ExternalLinks {
	case "safari", "block":
	default:
		return fmt.Errorf("webview.external_links 仅支持 safari 或 block")
	}

	if !c.Screen.Portrait && !c.Screen.Landscape {
		return fmt.Errorf("screen 必须至少启用一种方向")
	}

	if c.Network.TimeoutSeconds < 1 || c.Network.TimeoutSeconds > 300 {
		return fmt.Errorf("network.timeout_seconds 必须在 1-300 之间")
	}
	if c.Network.RetryIntervalSeconds < 1 || c.Network.RetryIntervalSeconds > 60 {
		return fmt.Errorf("network.retry_interval_seconds 必须在 1-60 之间")
	}
	if c.Network.RetryMax < 0 || c.Network.RetryMax > 20 {
		return fmt.Errorf("network.retry_max 必须在 0-20 之间")
	}

	if c.Bridge.Enabled {
		if len(c.Bridge.AllowedOrigins) == 0 {
			return fmt.Errorf("bridge.enabled 时 allowed_origins 不能为空")
		}
		for i, origin := range c.Bridge.AllowedOrigins {
			if err := validateOrigin(origin); err != nil {
				return fmt.Errorf("bridge.allowed_origins[%d]: %w", i, err)
			}
		}
	}

	// 未实现的权限不得开启。
	if c.Permissions.Camera {
		return fmt.Errorf("permissions.camera 当前未实现，不得开启")
	}
	if c.Permissions.Microphone {
		return fmt.Errorf("permissions.microphone 当前未实现，不得开启")
	}
	if c.Permissions.PhotoLibrary {
		return fmt.Errorf("permissions.photo_library 当前未实现，不得开启")
	}
	if c.Permissions.Location {
		return fmt.Errorf("permissions.location 当前未实现，不得开启")
	}

	return nil
}

func parseIOSMajor(v string) (int, error) {
	parts := strings.Split(v, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("app.minimum_ios 解析失败: %w", err)
	}
	return major, nil
}

func validateHTTPSURL(raw, field string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s 不是合法 URL: %w", field, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%s 必须使用 HTTPS", field)
	}
	if u.User != nil {
		return fmt.Errorf("%s 禁止包含用户名或密码", field)
	}
	if u.Host == "" {
		return fmt.Errorf("%s 缺少主机", field)
	}
	host := u.Hostname()
	if host == "" || !isValidHost(host) {
		return fmt.Errorf("%s 主机无效", field)
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%s 端口无效", field)
		}
	}
	return nil
}

func validateOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("不是合法 Origin: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("Origin 必须为 HTTPS")
	}
	if u.User != nil {
		return fmt.Errorf("Origin 禁止包含用户名或密码")
	}
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("Origin 不得包含路径")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("Origin 不得包含查询或片段")
	}
	if u.Hostname() == "" || !isValidHost(u.Hostname()) {
		return fmt.Errorf("Origin 主机无效")
	}
	return nil
}

func isValidHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || strings.Contains(host, ":") || strings.Contains(host, "/") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return false
	}
	return hostPattern.MatchString(host)
}

func hostFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return strings.ToLower(u.Hostname()), nil
}

func containsHost(hosts []string, target string) bool {
	target = strings.ToLower(target)
	for _, h := range hosts {
		if strings.ToLower(h) == target {
			return true
		}
	}
	return false
}
