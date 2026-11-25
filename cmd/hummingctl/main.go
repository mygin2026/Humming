// hummingctl 是 Humming 项目的配置生成、校验与 IPA 打包工具。
// 仅使用 Go 标准库，CGO_ENABLED=0。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"com.glhpx.humming/tools/internal/generator"
	"com.glhpx.humming/tools/internal/packaging"
)

func main() {
	if len(os.Args) < 2 {
		printRootHelp()
		os.Exit(2)
	}

	cmd := os.Args[1]
	switch cmd {
	case "generate":
		os.Exit(runGenerate(os.Args[2:]))
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "package":
		os.Exit(runPackage(os.Args[2:]))
	case "help", "-h", "--help":
		printRootHelp()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", cmd)
		printRootHelp()
		os.Exit(2)
	}
}

func printRootHelp() {
	fmt.Fprintf(os.Stdout, `hummingctl — Humming 配置生成与 IPA 打包工具

用法:
  hummingctl generate [--config path] [--root path]
  hummingctl validate [--config path] [--root path]
  hummingctl package --app <路径> --output <路径>
  hummingctl help

环境:
  建议设置 CGO_ENABLED=0，仅依赖 Go 标准库。
`)
}

func runGenerate(args []string) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "config.json", "配置文件路径")
	root := fs.String("root", ".", "项目根目录")
	help := fs.Bool("help", false, "显示帮助")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Fprintln(os.Stdout, "用法: hummingctl generate [--config path] [--root path]")
		return 0
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	cfg := *configPath
	if !filepath.IsAbs(cfg) {
		cfg = filepath.Join(absRoot, cfg)
	}

	result, err := generator.Generate(generator.Options{
		Root:       absRoot,
		ConfigPath: cfg,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate 失败: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "已生成 %d 个文件\n", len(result.Files))
	for _, f := range result.Files {
		rel, err := filepath.Rel(absRoot, f)
		if err != nil {
			rel = f
		}
		fmt.Fprintf(os.Stdout, "  - %s\n", rel)
	}
	return 0
}

func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "config.json", "配置文件路径")
	root := fs.String("root", ".", "项目根目录")
	help := fs.Bool("help", false, "显示帮助")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Fprintln(os.Stdout, "用法: hummingctl validate [--config path] [--root path]")
		return 0
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	cfg := *configPath
	if !filepath.IsAbs(cfg) {
		cfg = filepath.Join(absRoot, cfg)
	}

	if err := generator.ValidateProject(absRoot, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "validate 失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "校验通过")
	return 0
}

func runPackage(args []string) int {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	app := fs.String("app", "", ".app 目录路径")
	output := fs.String("output", "", "输出 IPA 路径")
	help := fs.Bool("help", false, "显示帮助")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Fprintln(os.Stdout, "用法: hummingctl package --app <路径> --output <路径>")
		return 0
	}
	if *app == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "错误: --app 与 --output 均为必填")
		fmt.Fprintln(os.Stderr, "用法: hummingctl package --app <路径> --output <路径>")
		return 2
	}

	result, err := packaging.PackageIPA(packaging.Options{
		AppPath:    *app,
		OutputPath: *output,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "package 失败: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "已生成 IPA: %s\n", result.IPAPath)
	fmt.Fprintf(os.Stdout, "SHA-256: %s\n", result.SHA256)
	fmt.Fprintf(os.Stdout, "校验文件: %s\n", result.SHA256Path)
	fmt.Fprintf(os.Stdout, "条目数: %d\n", result.EntryCount)
	return 0
}
