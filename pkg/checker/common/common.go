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

	"golang.org/x/crypto/ssh"
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

// RunSSHCommandWithAuth 通过 SSH 密码认证执行远程命令并返回输出
// 使用 golang.org/x/crypto/ssh 实现原生 SSH 客户端
func RunSSHCommandWithAuth(hostname, username, password string, port int, command string) (string, error) {
	return RunSSHCommandWithAuthTimeout(hostname, username, password, port, command, 30*time.Second)
}

// RunSSHCommandWithAuthTimeout 通过 SSH 密码认证执行远程命令，带超时控制
func RunSSHCommandWithAuthTimeout(hostname, username, password string, port int, command string, timeout time.Duration) (string, error) {
	// 创建 SSH 配置
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 连接地址
	addr := fmt.Sprintf("%s:%d", hostname, port)

	// 建立 SSH 连接
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("SSH 连接失败：%v", err)
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH 会话创建失败：%v", err)
	}
	defer session.Close()

	// 设置超时
	done := make(chan struct {
		output []byte
		err    error
	}, 1)

	go func() {
		output, err := session.CombinedOutput(command)
		done <- struct {
			output []byte
			err    error
		}{output, err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			return "", fmt.Errorf("SSH 命令执行失败：%v", result.err)
		}
		return string(result.output), nil
	case <-time.After(timeout):
		client.Close()
		return "", fmt.Errorf("SSH 命令执行超时（%v）", timeout)
	}
}
