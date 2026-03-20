// Package common 提供公共辅助函数
// 包含 SSH 命令执行、主机名解析等通用功能
package common

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ResolveToIP 将主机名解析为 IP 地址
// 如果已经是 IP 地址或解析失败，返回原始值
// 导出版本，供外部包使用
func ResolveToIP(host string) string {
	// 尝试解析为主机名
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		// 解析失败，返回原始值
		return host
	}

	// 返回第一个 IPv4 地址
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}

	// 如果没有 IPv4 地址，返回第一个 IP
	return ips[0].String()
}

// resolveToIP 将主机名解析为 IP 地址（内部使用）
func resolveToIP(host string) string {
	return ResolveToIP(host)
}

// getHostname 获取本地主机名
func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return name
}

// GetHostname 获取本地主机名（导出版本）
func GetHostname() string {
	return getHostname()
}

// runSSHCommand 通过 SSH 执行远程命令并返回输出（辅助函数）
// 使用统一的 SSH 选项，确保连接稳定性和安全性
func RunSSHCommand(host string, command string) (string, error) {
	return runSSHCommandWithTimeout(host, command, 30*time.Second)
}

// runSSHCommand 通过 SSH 执行远程命令并返回输出（辅助函数）
// 使用统一的 SSH 选项，确保连接稳定性和安全性
func runSSHCommand(host string, command string) (string, error) {
	return runSSHCommandWithTimeout(host, command, 30*time.Second)
}

// runSSHCommandWithTimeout 通过 SSH 执行远程命令，带超时控制
func runSSHCommandWithTimeout(host string, command string, timeout time.Duration) (string, error) {
	cmd := exec.Command("ssh",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		host, command)

	// 设置命令执行超时
	if timeout > 0 {
		cmd.WaitDelay = timeout
	}

	output, err := cmd.Output()
	if err != nil {
		// 区分超时错误和其他错误
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "killed") {
			return "", fmt.Errorf("SSH 命令执行超时（%v）：%v", timeout, err)
		}
		return "", fmt.Errorf("SSH 命令执行失败：%v", err)
	}
	return string(output), nil
}
