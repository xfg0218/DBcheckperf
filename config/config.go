// Package config 提供配置管理功能
// 用于解析和管理 dbcheckperf 工具的运行配置参数
package config

import (
	"os"
	"time"
)

// TestType 定义测试类型
type TestType string

const (
	// TestDisk 磁盘 I/O 测试
	TestDisk TestType = "d"
	// TestStream 内存带宽测试
	TestStream TestType = "s"
	// TestNetworkSerial 网络串行测试
	TestNetworkSerial TestType = "n"
	// TestNetworkParallel 网络并行测试
	TestNetworkParallel TestType = "N"
	// TestNetworkMatrix 网络全矩阵测试
	TestNetworkMatrix TestType = "M"
)

// Config 配置结构体，包含所有运行参数
type Config struct {
	// Hosts 主机列表
	Hosts []string
	// HostFile 主机文件路径
	HostFile string
	// TestDirs 测试目录列表
	TestDirs []string
	// TempDir 临时目录（用于网络和流测试）
	TempDir string
	// TestTypes 要运行的测试类型
	TestTypes []TestType
	// BlockSize 磁盘 I/O 测试的块大小（KB）
	BlockSize int
	// FileSize 磁盘 I/O 测试的文件大小
	FileSize string
	// Duration 网络测试持续时间
	Duration time.Duration
	// DisplayPerHost 是否显示每台主机的结果
	DisplayPerHost bool
	// Verbose 详细模式
	Verbose bool
	// VeryVerbose 非常详细模式
	VeryVerbose bool
	// UseNetperf 是否使用 netperf 进行网络测试
	UseNetperf bool
	// BufferSize 网络测试的缓冲区大小（KB）
	BufferSize int
	// TestRandom 是否测试随机读写
	TestRandom bool
	// RandBlockSize 随机读写块大小（KB）
	RandBlockSize int
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		BlockSize:     32,    // 默认 32KB，与 Greenplum 页面大小一致
		FileSize:      "2xRAM", // 默认使用 2 倍 RAM 大小
		Duration:      15 * time.Second,
		BufferSize:    8,     // 默认 8KB 发送缓冲区
		TestTypes:     []TestType{TestDisk, TestStream, TestNetworkSerial},
		TestRandom:    false, // 默认不测试随机读写
		RandBlockSize: 4,     // 随机读写默认 4KB
	}
}

// Validate 验证配置是否有效
func (c *Config) Validate() error {
	if len(c.Hosts) == 0 && c.HostFile == "" {
		return ErrNoHostsSpecified
	}
	if len(c.TestDirs) == 0 {
		return ErrNoTestDirsSpecified
	}
	return nil
}

// 错误定义
var (
	// ErrNoHostsSpecified 未指定主机
	ErrNoHostsSpecified = os.ErrInvalid
	// ErrNoTestDirsSpecified 未指定测试目录
	ErrNoTestDirsSpecified = os.ErrInvalid
)
