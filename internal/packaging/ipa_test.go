package packaging

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackageIPAAndValidate(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "Humming.app")
	if err := os.MkdirAll(filepath.Join(app, "Frameworks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte(`<?xml version="1.0"?><plist></plist>`), 0o644); err != nil {
		t.Fatal(err)
	}
	execPath := filepath.Join(app, "Humming")
	if err := os.WriteFile(execPath, []byte("fake-mach-o"), 0o755); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "dist", "Humming-Unsigned.ipa")
	res, err := PackageIPA(Options{
		AppPath:    app,
		OutputPath: out,
		FixedTime:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("package: %v", err)
	}
	if res.SHA256 == "" {
		t.Fatal("缺少 SHA256")
	}
	if _, err := os.Stat(res.SHA256Path); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	foundExec := false
	for _, f := range zr.File {
		if f.Name == "Payload/Humming.app/Humming" {
			foundExec = true
			mode := f.Mode()
			if mode&0o111 == 0 {
				t.Fatalf("可执行文件权限丢失: %v", mode)
			}
		}
		if f.Name == "../evil" || f.Name == "Payload/../evil" {
			t.Fatal("不应包含穿越路径")
		}
	}
	if !foundExec {
		t.Fatal("缺少 Payload/Humming.app/Humming")
	}
}

func TestRejectNonAppDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "NotAnApp")
	_ = os.MkdirAll(dir, 0o755)
	_, err := PackageIPA(Options{AppPath: dir, OutputPath: filepath.Join(root, "a.ipa")})
	if err == nil {
		t.Fatal("应拒绝非 .app 目录")
	}
}

func TestValidateRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	ipa := filepath.Join(root, "bad.ipa")
	f, err := os.Create(ipa)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	_, _ = zw.Create("Payload/Humming.app/Info.plist")
	_, _ = zw.Create("Payload/../evil.txt")
	_ = zw.Close()
	_ = f.Close()
	if err := ValidateIPAStructure(ipa, "Humming.app"); err == nil {
		t.Fatal("应拒绝路径穿越")
	}
}
