// Package packaging 使用标准库将 .app 打包为未签名 IPA（ZIP）。
package packaging

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Options IPA 打包参数。
type Options struct {
	AppPath    string
	OutputPath string
	// FixedTime 若非零，用于确定性时间戳（测试用）。
	FixedTime time.Time
}

// Result 打包结果。
type Result struct {
	IPAPath    string
	SHA256Path string
	SHA256     string
	EntryCount int
}

// PackageIPA 将 .app 目录打包为 Payload/<AppName>.app/ 结构的 IPA。
func PackageIPA(opts Options) (*Result, error) {
	appPath := filepath.Clean(opts.AppPath)
	info, err := os.Stat(appPath)
	if err != nil {
		return nil, fmt.Errorf("读取 .app 失败: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf(".app 路径必须是目录: %s", appPath)
	}
	appName := filepath.Base(appPath)
	if !strings.HasSuffix(appName, ".app") {
		return nil, fmt.Errorf("应用目录名必须以 .app 结尾: %s", appName)
	}
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("输出路径不能为空")
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return nil, err
	}

	outFile, err := os.Create(opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("创建 IPA 失败: %w", err)
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	entryCount := 0
	modTime := opts.FixedTime
	if modTime.IsZero() {
		modTime = time.Now()
	}

	err = filepath.WalkDir(appPath, func(full string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(appPath, full)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			// 根目录条目
			hdr := &zip.FileHeader{
				Name:     path.Join("Payload", appName) + "/",
				Method:   zip.Store,
				Modified: modTime,
			}
			hdr.SetMode(0o755 | os.ModeDir)
			_, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			entryCount++
			return nil
		}

		// 拒绝路径穿越
		if strings.Contains(rel, "..") || path.IsAbs(rel) {
			return fmt.Errorf("拒绝路径穿越: %s", rel)
		}
		zipPath := path.Join("Payload", appName, rel)

		fi, err := d.Info()
		if err != nil {
			return err
		}

		// 符号链接
		if fi.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(full)
			if err != nil {
				return err
			}
			slashTarget := filepath.ToSlash(target)
			if filepath.IsAbs(target) || path.IsAbs(slashTarget) || strings.Contains(slashTarget, "..") {
				return fmt.Errorf("拒绝危险符号链接: %s -> %s", rel, target)
			}
			hdr := &zip.FileHeader{
				Name:     zipPath,
				Method:   zip.Store,
				Modified: modTime,
			}
			hdr.SetMode(fi.Mode())
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(w, target); err != nil {
				return err
			}
			entryCount++
			return nil
		}

		if d.IsDir() {
			hdr := &zip.FileHeader{
				Name:     zipPath + "/",
				Method:   zip.Store,
				Modified: modTime,
			}
			hdr.SetMode(0o755 | os.ModeDir)
			_, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			entryCount++
			return nil
		}

		hdr := &zip.FileHeader{
			Name:     zipPath,
			Method:   zip.Deflate,
			Modified: modTime,
		}
		hdr.SetMode(ipaFileMode(fi.Mode(), rel, appName))
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(full)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		entryCount++
		return nil
	})
	if err != nil {
		_ = zw.Close()
		_ = outFile.Close()
		_ = os.Remove(opts.OutputPath)
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := outFile.Close(); err != nil {
		return nil, err
	}

	if err := ValidateIPAStructure(opts.OutputPath, appName); err != nil {
		_ = os.Remove(opts.OutputPath)
		return nil, err
	}

	sum, err := fileSHA256(opts.OutputPath)
	if err != nil {
		return nil, err
	}
	shaPath := opts.OutputPath + ".sha256"
	line := sum + "  " + filepath.Base(opts.OutputPath) + "\n"
	if err := os.WriteFile(shaPath, []byte(line), 0o644); err != nil {
		return nil, err
	}

	return &Result{
		IPAPath:    opts.OutputPath,
		SHA256Path: shaPath,
		SHA256:     sum,
		EntryCount: entryCount,
	}, nil
}

// ValidateIPAStructure 检查 ZIP 内部结构是否合法。
func ValidateIPAStructure(ipaPath, expectedAppName string) error {
	r, err := zip.OpenReader(ipaPath)
	if err != nil {
		return fmt.Errorf("打开 IPA 失败: %w", err)
	}
	defer r.Close()

	prefix := "Payload/" + expectedAppName + "/"
	hasPayload := false
	hasInfoPlist := false
	for _, f := range r.File {
		name := path.Clean("/" + f.Name)
		name = strings.TrimPrefix(name, "/")
		if name != f.Name && !strings.HasSuffix(f.Name, "/") {
			// Clean 可能改变路径，进一步检查穿越
		}
		if strings.Contains(f.Name, "..") {
			return fmt.Errorf("IPA 含路径穿越条目: %s", f.Name)
		}
		if f.Name == "Payload/" || strings.HasPrefix(f.Name, "Payload/") {
			hasPayload = true
		}
		if !strings.HasPrefix(f.Name, "Payload/") && f.Name != "Payload" {
			// 允许其他元数据，但主体必须在 Payload 下；这里仅警告核心检查
			continue
		}
		if strings.HasPrefix(f.Name, prefix) || f.Name == "Payload/"+expectedAppName || f.Name == "Payload/"+expectedAppName+"/" {
			if strings.HasSuffix(f.Name, "Info.plist") && !f.FileInfo().IsDir() {
				hasInfoPlist = true
			}
		}
	}
	if !hasPayload {
		return fmt.Errorf("IPA 缺少 Payload/ 目录")
	}
	if !hasInfoPlist {
		return fmt.Errorf("IPA 缺少 Payload/%s/Info.plist", expectedAppName)
	}
	return nil
}

// ipaFileMode 在 Windows 等不保留 Unix 权限的环境上，按 IPA 约定恢复可执行位。
func ipaFileMode(src os.FileMode, relSlash string, appName string) os.FileMode {
	if src&0o111 != 0 {
		return 0o755
	}
	base := path.Base(relSlash)
	appBinary := strings.TrimSuffix(appName, ".app")
	if relSlash == appBinary || base == appBinary {
		return 0o755
	}
	if strings.HasSuffix(base, ".dylib") {
		return 0o755
	}
	// Foo.framework/Foo
	if strings.Contains(relSlash, ".framework/") && !strings.Contains(base, ".") {
		return 0o755
	}
	return 0o644
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
