// Package installer 提供测试工具自动安装功能
// 本文件包含 installer 包的单元测试
package installer

import (
	"strings"
	"testing"
	"time"
)

// ==================== NewToolInstaller 测试 ====================

func TestNewToolInstaller(t *testing.T) {
	tests := []struct {
		name        string
		verbose     bool
		autoInstall bool
		timeout     int
		expectTimeout int
	}{
		{"默认配置", false, false, 0, 300},
		{"自定义超时", true, true, 600, 600},
		{"负超时", false, false, -100, 300},
		{"详细模式", true, false, 120, 120},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer := NewToolInstaller(tt.verbose, tt.autoInstall, tt.timeout)

			if installer == nil {
				t.Fatal("NewToolInstaller() 返回 nil")
			}

			if installer.Verbose != tt.verbose {
				t.Errorf("Verbose = %v; 期望 %v", installer.Verbose, tt.verbose)
			}

			if installer.AutoInstall != tt.autoInstall {
				t.Errorf("AutoInstall = %v; 期望 %v", installer.AutoInstall, tt.autoInstall)
			}

			if installer.Timeout != tt.expectTimeout {
				t.Errorf("Timeout = %d; 期望 %d", installer.Timeout, tt.expectTimeout)
			}
		})
	}
}

// ==================== CheckTool 测试 ====================

func TestCheckTool_CommonTools(t *testing.T) {
	installer := &ToolInstaller{
		Verbose: false,
	}

	tests := []struct {
		name     string
		tool     string
		mustExist bool
	}{
		{"检查 dd", "dd", true},
		{"检查 curl", "curl", true},
		{"检查 sh", "sh", true},
		{"检查不存在的工具", "nonexistent_tool_xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := installer.CheckTool(tt.tool)

			if err != nil {
				t.Errorf("CheckTool() 意外错误：%v", err)
			}

			if info == nil {
				t.Fatal("CheckTool() 返回 nil")
			}

			if info.Name != tt.tool {
				t.Errorf("Name = %q; 期望 %q", info.Name, tt.tool)
			}

			if tt.mustExist && !info.Installed {
				t.Errorf("%s 应该已安装", tt.tool)
			}

			if !tt.mustExist && info.Installed {
				t.Errorf("%s 应该未安装", tt.tool)
			}

			if info.Installed && info.Path == "" {
				t.Error("已安装的工具应该有 Path")
			}
		})
	}
}

// ==================== getToolVersion 测试 ====================

func TestGetToolVersion(t *testing.T) {
	installer := &ToolInstaller{Verbose: false}

	tests := []struct {
		name string
		tool string
	}{
		{"dd 版本", "dd"},
		{"curl 版本", "curl"},
		{"sh 版本", "sh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := installer.getToolVersion(tt.tool)

			if version == "" {
				t.Errorf("%s 版本号不应该为空", tt.tool)
			}

			t.Logf("%s 版本：%s", tt.tool, version)
		})
	}
}

// ==================== detectPackageManager 测试 ====================

func TestDetectPackageManager(t *testing.T) {
	installer := &ToolInstaller{Verbose: false}

	pkgManager := installer.detectPackageManager()

	// 至少应该检测到一个包管理器（在 Linux 系统上）
	// 在非 Linux 系统上可能为空
	if pkgManager != "" {
		t.Logf("检测到包管理器：%s", pkgManager)

		// 验证是常见的包管理器之一
		validManagers := []string{"apt-get", "apt", "yum", "dnf", "zypper", "apk", "pacman"}
		isValid := false
		for _, valid := range validManagers {
			if pkgManager == valid {
				isValid = true
				break
			}
		}

		if !isValid {
			t.Errorf("检测到未知的包管理器：%s", pkgManager)
		}
	} else {
		t.Log("未检测到包管理器（可能在非 Linux 系统上）")
	}
}

// ==================== getInstallCommand 测试 ====================

func TestGetInstallCommand(t *testing.T) {
	installer := &ToolInstaller{Verbose: false}

	tests := []struct {
		name       string
		pkgManager string
		tool       string
		expect     string
	}{
		{"apt-get", "apt-get", "fio", "apt-get update && apt-get install -y fio"},
		{"yum", "yum", "iperf3", "yum install -y iperf3"},
		{"dnf", "dnf", "netperf", "dnf install -y netperf"},
		{"zypper", "zypper", "fio", "zypper install -y fio"},
		{"apk", "apk", "curl", "apk add --update curl"},
		{"pacman", "pacman", "vim", "pacman -Sy --noconfirm vim"},
		{"未知", "unknown", "fio", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := installer.getInstallCommand(tt.pkgManager, tt.tool)

			if cmd != tt.expect {
				t.Errorf("getInstallCommand(%q, %q) = %q; 期望 %q",
					tt.pkgManager, tt.tool, cmd, tt.expect)
			}
		})
	}
}

// ==================== QuickCheck 测试 ====================

func TestQuickCheck(t *testing.T) {
	installer := &ToolInstaller{Verbose: false}

	result := installer.QuickCheck()

	// 验证返回的 map 包含所有预期工具
	expectedTools := []string{"fio", "iperf3", "netperf", "dd", "curl"}
	for _, tool := range expectedTools {
		if _, ok := result[tool]; !ok {
			t.Errorf("QuickCheck() 结果缺少工具：%s", tool)
		}
	}

	// dd 和 curl 应该已安装
	if !result["dd"] {
		t.Log("警告：dd 未安装（这很不寻常）")
	}
	if !result["curl"] {
		t.Log("警告：curl 未安装")
	}

	// 输出完整报告
	t.Logf("工具检查结果：%+v", result)
}

// ==================== GetInstallReport 测试 ====================

func TestGetInstallReport(t *testing.T) {
	installer := &ToolInstaller{Verbose: false}

	tools := []string{"dd", "curl", "fio", "iperf3"}
	report := installer.GetInstallReport(tools)

	if report == "" {
		t.Error("GetInstallReport() 返回空字符串")
	}

	if !strings.Contains(report, "测试工具安装报告") {
		t.Error("报告应该包含标题")
	}

	t.Logf("\n%s", report)
}

// ==================== EnsureTool 测试（需要 sudo 权限） ====================

func TestEnsureTool_SkipNoSudo(t *testing.T) {
	// 跳过需要实际安装的测试
	t.Skip("需要 sudo 权限，跳过实际安装测试")

	installer := &ToolInstaller{
		Verbose:     true,
		AutoInstall: true,
		Timeout:     60,
	}

	// 这个测试在 CI/CD 环境中运行
	info, err := installer.EnsureTool("htop")
	if err != nil {
		t.Logf("EnsureTool() 结果：%v", err)
	} else {
		t.Logf("EnsureTool() 成功：%+v", info)
	}
}

// ==================== BatchEnsureTools 测试 ====================

func TestBatchEnsureTools(t *testing.T) {
	installer := &ToolInstaller{
		Verbose:     false,
		AutoInstall: false, // 不自动安装，只检查
	}

	tools := []string{"dd", "curl", "nonexistent_tool"}
	results, errors := installer.BatchEnsureTools(tools)

	// 验证结果
	if len(results) < 2 {
		t.Errorf("BatchEnsureTools() 应该至少返回 2 个成功结果")
	}

	// dd 和 curl 应该成功
	if _, ok := results["dd"]; !ok {
		t.Error("dd 应该在结果中")
	}
	if _, ok := results["curl"]; !ok {
		t.Error("curl 应该在结果中")
	}

	// nonexistent_tool 应该失败
	if len(errors) == 0 {
		t.Log("警告：期望至少一个错误（nonexistent_tool）")
	}

	t.Logf("成功：%d, 错误：%d", len(results), len(errors))
}

// ==================== WaitForInstallation 测试 ====================

func TestWaitForInstallation(t *testing.T) {
	installer := &ToolInstaller{Verbose: false}

	// 测试超时情况
	start := time.Now()
	err := installer.WaitForInstallation("nonexistent", 100*time.Millisecond, 300*time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Error("WaitForInstallation() 应该返回超时错误")
	}

	if duration < 300*time.Millisecond {
		t.Errorf("等待时间应该至少 300ms，实际：%v", duration)
	}

	t.Logf("超时测试耗时：%v", duration)
}

// ==================== 集成测试 ====================

func TestToolInstaller_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	installer := &ToolInstaller{
		Verbose:     true,
		AutoInstall: false,
	}

	// 检查常用工具
	tools := []string{"dd", "curl", "sh", "bash"}

	for _, tool := range tools {
		info, err := installer.CheckTool(tool)
		if err != nil {
			t.Errorf("检查 %s 失败：%v", tool, err)
			continue
		}

		t.Logf("%s: 已安装=%v, 路径=%s, 版本=%s",
			tool, info.Installed, info.Path, info.Version)
	}
}

// ==================== 基准测试 ====================

func BenchmarkCheckTool(b *testing.B) {
	installer := &ToolInstaller{Verbose: false}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		installer.CheckTool("dd")
	}
}

func BenchmarkQuickCheck(b *testing.B) {
	installer := &ToolInstaller{Verbose: false}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		installer.QuickCheck()
	}
}

func BenchmarkGetInstallCommand(b *testing.B) {
	installer := &ToolInstaller{Verbose: false}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		installer.getInstallCommand("apt-get", "fio")
	}
}
