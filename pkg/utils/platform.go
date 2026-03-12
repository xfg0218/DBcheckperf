// Package utils 提供通用工具函数
package utils

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// OSInfo 操作系统信息
type OSInfo struct {
	// Name 操作系统名称
	Name string
	// Version 版本号
	Version string
	// Arch 架构 (x86_64, aarch64, arm64)
	Arch string
	// Kernel 内核版本
	Kernel string
	// IsLinux 是否 Linux
	IsLinux bool
	// IsFreeBSD 是否 FreeBSD
	IsFreeBSD bool
	// HasSystemd 是否有 systemd
	HasSystemd bool
}

// GetOSInfo 获取操作系统信息
func GetOSInfo() *OSInfo {
	info := &OSInfo{
		Arch:      runtime.GOARCH,
		IsLinux:   runtime.GOOS == "linux",
		IsFreeBSD: runtime.GOOS == "freebsd",
	}

	// 标准化架构名称
	switch info.Arch {
	case "amd64":
		info.Arch = "x86_64"
	case "arm64":
		info.Arch = "aarch64"
	}

	// 获取内核版本
	info.Kernel = getKernelVersion()

	// 检测操作系统
	if info.IsLinux {
		info.Name, info.Version = getLinuxOSInfo()
		info.HasSystemd = hasSystemd()
	} else if info.IsFreeBSD {
		info.Name, info.Version = getFreeBSDInfo()
	}

	return info
}

// getKernelVersion 获取内核版本
func getKernelVersion() string {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}
	return "Unknown"
}

// getLinuxOSInfo 获取 Linux 发行版信息
func getLinuxOSInfo() (name, version string) {
	// 尝试读取 /etc/os-release (大多数现代 Linux)
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		return parseOSRelease(string(data))
	}

	// 尝试读取 /etc/redhat-release (RHEL 系旧版本)
	data, err = os.ReadFile("/etc/redhat-release")
	if err == nil {
		return parseRedhatRelease(string(data))
	}

	// 尝试读取 /etc/debian_version (Debian)
	data, err = os.ReadFile("/etc/debian_version")
	if err == nil {
		return "Debian", strings.TrimSpace(string(data))
	}

	// 尝试使用 lsb_release 命令
	cmd := exec.Command("lsb_release", "-d")
	output, err := cmd.Output()
	if err == nil {
		return parseLsbRelease(string(output))
	}

	return "Linux", "Unknown"
}

// parseOSRelease 解析 /etc/os-release
func parseOSRelease(content string) (name, version string) {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			pretty := strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			// 从 PRETTY_NAME 提取名称和版本
			parts := strings.SplitN(pretty, " ", 2)
			if len(parts) > 0 {
				name = parts[0]
			}
			if len(parts) > 1 {
				version = parts[1]
			}
			return
		}
		if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), `"`)
		}
		if strings.HasPrefix(line, "VERSION=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION="), `"`)
		}
	}
	return
}

// parseRedhatRelease 解析 /etc/redhat-release
func parseRedhatRelease(content string) (name, version string) {
	line := strings.TrimSpace(content)
	if strings.Contains(line, "CentOS") {
		name = "CentOS"
	} else if strings.Contains(line, "Red Hat") {
		name = "Red Hat Enterprise Linux"
	} else if strings.Contains(line, "Oracle") {
		name = "Oracle Linux"
	} else if strings.Contains(line, "Rocky") {
		name = "Rocky Linux"
	} else if strings.Contains(line, "AlmaLinux") {
		name = "AlmaLinux"
	} else if strings.Contains(line, "openEuler") {
		name = "openEuler"
	} else if strings.Contains(line, "CTyunOS") {
		name = "CTyunOS"
	}

	// 提取版本号
	parts := strings.Fields(line)
	for i, part := range parts {
		if part == "release" && i+1 < len(parts) {
			version = parts[i+1]
			break
		}
	}

	return
}

// parseLsbRelease 解析 lsb_release 输出
func parseLsbRelease(content string) (name, version string) {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "Description:") {
			desc := strings.TrimSpace(strings.TrimPrefix(line, "Description:"))
			parts := strings.SplitN(desc, " ", 2)
			if len(parts) > 0 {
				name = parts[0]
			}
			if len(parts) > 1 {
				version = parts[1]
			}
		}
	}
	return
}

// getFreeBSDInfo 获取 FreeBSD 信息
func getFreeBSDInfo() (name, version string) {
	cmd := exec.Command("freebsd-version")
	output, err := cmd.Output()
	if err == nil {
		version = strings.TrimSpace(string(output))
	}
	return "FreeBSD", version
}

// hasSystemd 检测是否有 systemd
func hasSystemd() bool {
	// 检测 /run/systemd/system 目录
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return true
	}
	// 检测 systemd 命令
	cmd := exec.Command("systemctl", "--version")
	return cmd.Run() == nil
}

// CommandExists 检查命令是否存在
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// IsRoot 检查是否以 root 权限运行
func IsRoot() bool {
	return os.Geteuid() == 0
}
