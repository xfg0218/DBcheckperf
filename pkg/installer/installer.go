// Package installer 提供测试工具自动安装功能
// 支持自动检测和安装 fio、iperf3、netperf 等性能测试工具
package installer

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"dbcheckperf/pkg/checker/common"
)

// ToolInstaller 测试工具安装器
type ToolInstaller struct {
	// Verbose 是否显示详细输出
	Verbose bool
	// Timeout 安装超时时间（秒）
	Timeout int
	// AutoInstall 是否自动安装
	AutoInstall bool
}

// NewToolInstaller 创建新的工具安装器
func NewToolInstaller(verbose, autoInstall bool, timeout int) *ToolInstaller {
	if timeout <= 0 {
		timeout = 300 // 默认 5 分钟超时
	}
	return &ToolInstaller{
		Verbose:     verbose,
		Timeout:     timeout,
		AutoInstall: autoInstall,
	}
}

// ToolInfo 工具信息
type ToolInfo struct {
	// Name 工具名称
	Name string
	// Installed 是否已安装
	Installed bool
	// Path 安装路径
	Path string
	// Version 版本号
	Version string
}

// CheckTool 检查工具是否已安装
func (ti *ToolInstaller) CheckTool(toolName string) (*ToolInfo, error) {
	path, err := exec.LookPath(toolName)
	if err != nil {
		return &ToolInfo{
			Name:      toolName,
			Installed: false,
		}, nil
	}

	// 获取版本号
	version := ti.getToolVersion(toolName)

	return &ToolInfo{
		Name:      toolName,
		Installed: true,
		Path:      path,
		Version:   version,
	}, nil
}

// getToolVersion 获取工具版本号
func (ti *ToolInstaller) getToolVersion(toolName string) string {
	cmd := exec.Command(toolName, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown"
	}

	// 解析版本输出（取第一行）
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		versionLine := lines[0]
		// 尝试提取版本号
		re := regexp.MustCompile(`\d+\.\d+(\.\d+)?`)
		matches := re.FindString(versionLine)
		if matches != "" {
			return matches
		}
		return strings.TrimSpace(versionLine)
	}

	return "unknown"
}

// InstallTool 安装工具
func (ti *ToolInstaller) InstallTool(toolName string) error {
	if ti.Verbose {
		fmt.Printf("正在安装 %s...\n", toolName)
	}

	// 检测包管理器
	pkgManager := ti.detectPackageManager()
	if pkgManager == "" {
		return fmt.Errorf("未检测到支持的包管理器")
	}

	if ti.Verbose {
		fmt.Printf("检测到包管理器：%s\n", pkgManager)
	}

	// 执行安装命令
	installCmd := ti.getInstallCommand(pkgManager, toolName)
	if ti.Verbose {
		fmt.Printf("执行命令：%s\n", installCmd)
	}

	cmd := exec.Command("sh", "-c", installCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("安装失败：%v, %s", err, string(output))
	}

	// 验证安装
	info, err := ti.CheckTool(toolName)
	if err != nil {
		return err
	}

	if !info.Installed {
		return fmt.Errorf("安装完成但无法验证 %s 是否成功", toolName)
	}

	if ti.Verbose {
		fmt.Printf("%s 安装成功，版本：%s\n", toolName, info.Version)
	}

	return nil
}

// detectPackageManager 检测包管理器
func (ti *ToolInstaller) detectPackageManager() string {
	managers := []string{"apt-get", "yum", "dnf", "zypper", "apk", "pacman"}

	for _, manager := range managers {
		if _, err := exec.LookPath(manager); err == nil {
			return manager
		}
	}

	return ""
}

// getInstallCommand 获取安装命令
func (ti *ToolInstaller) getInstallCommand(pkgManager, toolName string) string {
	switch pkgManager {
	case "apt-get", "apt":
		return fmt.Sprintf("%s update && %s install -y %s", pkgManager, pkgManager, toolName)
	case "yum", "dnf":
		return fmt.Sprintf("%s install -y %s", pkgManager, toolName)
	case "zypper":
		return fmt.Sprintf("%s install -y %s", pkgManager, toolName)
	case "apk":
		return fmt.Sprintf("%s add --update %s", pkgManager, toolName)
	case "pacman":
		return fmt.Sprintf("%s -Sy --noconfirm %s", pkgManager, toolName)
	default:
		return ""
	}
}

// EnsureTool 确保工具已安装，如未安装则自动安装
func (ti *ToolInstaller) EnsureTool(toolName string) (*ToolInfo, error) {
	// 检查是否已安装
	info, err := ti.CheckTool(toolName)
	if err != nil {
		return nil, err
	}

	if info.Installed {
		if ti.Verbose {
			fmt.Printf("%s 已安装，版本：%s\n", toolName, info.Version)
		}
		return info, nil
	}

	// 未安装，检查是否需要自动安装
	if !ti.AutoInstall {
		return info, fmt.Errorf("%s 未安装且未启用自动安装", toolName)
	}

	// 自动安装
	if err := ti.InstallTool(toolName); err != nil {
		return nil, err
	}

	// 重新检查
	return ti.CheckTool(toolName)
}

// EnsureFio 确保 fio 已安装
func (ti *ToolInstaller) EnsureFio() (*ToolInfo, error) {
	return ti.EnsureTool("fio")
}

// EnsureIperf3 确保 iperf3 已安装
func (ti *ToolInstaller) EnsureIperf3() (*ToolInfo, error) {
	return ti.EnsureTool("iperf3")
}

// EnsureNetperf 确保 netperf 已安装
func (ti *ToolInstaller) EnsureNetperf() (*ToolInfo, error) {
	return ti.EnsureTool("netperf")
}

// InstallRemoteTool 在远程主机上安装工具
func (ti *ToolInstaller) InstallRemoteTool(host, toolName string) error {
	if ti.Verbose {
		fmt.Printf("[%s] 正在安装 %s...\n", host, toolName)
	}

	// 检查远程主机是否已安装
	checkCmd := fmt.Sprintf("which %s", toolName)
	output, err := common.RunSSHCommand(host, checkCmd)
	if err == nil && strings.TrimSpace(output) != "" {
		if ti.Verbose {
			fmt.Printf("[%s] %s 已安装：%s\n", host, toolName, strings.TrimSpace(output))
		}
		return nil
	}

	// 未安装，执行自动安装
	if !ti.AutoInstall {
		return fmt.Errorf("[%s] %s 未安装且未启用自动安装", host, toolName)
	}

	// 检测远程主机的包管理器
	detectCmd := "which apt-get || which yum || which dnf || which zypper || which apk || which pacman"
	pkgManager, err := common.RunSSHCommand(host, detectCmd)
	if err != nil {
		return fmt.Errorf("[%s] 无法检测包管理器：%v", host, err)
	}

	pkgManager = strings.TrimSpace(strings.Split(pkgManager, "\n")[0])
	if pkgManager == "" {
		return fmt.Errorf("[%s] 未检测到支持的包管理器", host)
	}

	// 执行安装
	installCmd := ti.getInstallCommand(pkgManager, toolName)
	if ti.Verbose {
		fmt.Printf("[%s] 执行安装命令：%s\n", host, installCmd)
	}

	output, err = common.RunSSHCommand(host, installCmd)
	if err != nil {
		return fmt.Errorf("[%s] 安装失败：%v, %s", host, err, output)
	}

	// 验证安装
	verifyCmd := fmt.Sprintf("which %s && %s --version", toolName, toolName)
	output, err = common.RunSSHCommand(host, verifyCmd)
	if err != nil {
		return fmt.Errorf("[%s] 安装完成但验证失败：%v", host, err)
	}

	if ti.Verbose {
		lines := strings.Split(output, "\n")
		fmt.Printf("[%s] %s 安装成功：%s\n", host, toolName, lines[len(lines)-1])
	}

	return nil
}

// EnsureRemoteFio 确保远程主机 fio 已安装
func (ti *ToolInstaller) EnsureRemoteFio(host string) error {
	return ti.InstallRemoteTool(host, "fio")
}

// EnsureRemoteIperf3 确保远程主机 iperf3 已安装
func (ti *ToolInstaller) EnsureRemoteIperf3(host string) error {
	return ti.InstallRemoteTool(host, "iperf3")
}

// EnsureRemoteNetperf 确保远程主机 netperf 已安装
func (ti *ToolInstaller) EnsureRemoteNetperf(host string) error {
	return ti.InstallRemoteTool(host, "netperf")
}

// BatchEnsureTools 批量确保多个工具已安装
func (ti *ToolInstaller) BatchEnsureTools(toolNames []string) (map[string]*ToolInfo, []error) {
	results := make(map[string]*ToolInfo)
	var errors []error

	for _, toolName := range toolNames {
		info, err := ti.EnsureTool(toolName)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %v", toolName, err))
		} else {
			results[toolName] = info
		}
	}

	return results, errors
}

// BatchEnsureRemoteTools 批量确保远程主机多个工具已安装
func (ti *ToolInstaller) BatchEnsureRemoteTools(hosts []string, toolNames []string) map[string][]error {
	results := make(map[string][]error)

	for _, host := range hosts {
		var hostErrors []error
		for _, toolName := range toolNames {
			var err error
			switch toolName {
			case "fio":
				err = ti.EnsureRemoteFio(host)
			case "iperf3":
				err = ti.EnsureRemoteIperf3(host)
			case "netperf":
				err = ti.EnsureRemoteNetperf(host)
			default:
				err = ti.InstallRemoteTool(host, toolName)
			}

			if err != nil {
				hostErrors = append(hostErrors, err)
			}
		}
		results[host] = hostErrors
	}

	return results
}

// QuickCheck 快速检查所有必需工具
func (ti *ToolInstaller) QuickCheck() map[string]bool {
	tools := map[string]bool{
		"fio":     false,
		"iperf3":  false,
		"netperf": false,
		"dd":      false,
		"curl":    false,
	}

	for tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			tools[tool] = true
		}
	}

	return tools
}

// GetInstallReport 获取安装报告
func (ti *ToolInstaller) GetInstallReport(tools []string) string {
	var report strings.Builder

	report.WriteString("测试工具安装报告\n")
	report.WriteString("====================\n\n")

	for _, toolName := range tools {
		info, err := ti.CheckTool(toolName)
		if err != nil {
			report.WriteString(fmt.Sprintf("%s: 检查失败 - %v\n", toolName, err))
			continue
		}

		if info.Installed {
			report.WriteString(fmt.Sprintf("%s: ✓ 已安装 (版本 %s)\n", toolName, info.Version))
		} else {
			report.WriteString(fmt.Sprintf("%s: ✗ 未安装\n", toolName))
		}
	}

	return report.String()
}

// WaitForInstallation 等待安装完成（用于后台安装）
func (ti *ToolInstaller) WaitForInstallation(toolName string, interval, timeout time.Duration) error {
	start := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if time.Since(start) > timeout {
			return fmt.Errorf("等待 %s 安装超时", toolName)
		}

		info, err := ti.CheckTool(toolName)
		if err != nil {
			continue
		}

		if info.Installed {
			return nil
		}
	}

	return nil
}
