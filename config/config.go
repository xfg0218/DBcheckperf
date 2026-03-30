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
	// TestHardware 硬件信息收集
	TestHardware TestType = "H"
	// TestLatency 延迟和 IOPS 测试
	TestLatency TestType = "l"
	// TestIOStat IO 统计
	TestIOStat TestType = "i"
	// TestNUMA NUMA 信息
	TestNUMA TestType = "u"
	// TestKernel 内核参数
	TestKernel TestType = "k"
	// TestNetQuality 网络质量测试
	TestNetQuality TestType = "q"
	// TestDiskInfo 磁盘详细信息
	TestDiskInfo TestType = "I"
)

// SSHAuthInfo SSH 认证信息
type SSHAuthInfo struct {
	// Username SSH 用户名
	Username string
	// Password SSH 密码
	Password string
	// Port SSH 端口
	Port int
}

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
	// RandBlockSize 随机读写块大小（KB）
	RandBlockSize int
	// LatencyBlockSize 延迟测试块大小（KB）
	LatencyBlockSize int
	// IOStatInterval IO 统计采样间隔（秒），字符串格式如 "1s"
	IOStatInterval string
	// IOStatDevices IO 统计设备列表
	IOStatDevices []string
	// NetQualityTarget 网络质量测试目标主机
	NetQualityTarget string
	// ReportFormat 报告格式（html, json, table）
	ReportFormat string
	// ReportOutput 报告输出文件路径
	ReportOutput string
	// SSHAuthFile SSH 认证配置文件路径
	SSHAuthFile string
	// SSHAuthMap 主机认证信息映射（从 SSHAuthFile 解析）
	SSHAuthMap map[string]SSHAuthInfo
	// Iterations 测试迭代次数（取平均值）
	Iterations int
	// UseDirectIO 是否使用直接 IO（oflag=direct）
	UseDirectIO bool
	// UseFsync 是否使用 fsync（conv=fsync）
	UseFsync bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		BlockSize:        32,       // 默认 32KB，与 Greenplum 页面大小一致
		FileSize:         "2xRAM",  // 默认使用 2 倍 RAM 大小
		Duration:         15 * time.Second,
		BufferSize:       8,        // 默认 8KB 发送缓冲区
		TestTypes:        []TestType{TestDisk, TestStream, TestNetworkSerial},
		RandBlockSize:    0,        // 随机读写使用 -B 参数指定的块大小
		LatencyBlockSize: 4,        // 延迟测试默认 4KB
		IOStatInterval:   "1s",     // IO 统计默认 1 秒间隔
		Iterations:       1,        // 默认测试 1 次
		UseDirectIO:      true,     // 默认使用直接 IO
		UseFsync:         true,     // 默认使用 fsync
	}
}

// Validate 验证配置是否有效
func (c *Config) Validate() error {
	// 纯本地测试类型（不需要 -d 和 -h/-f/-F）
	// s=内存带宽，i=IO 统计，u=NUMA, k=内核参数，q=网络质量，H=硬件信息
	localOnlyTypes := []TestType{TestStream, TestIOStat, TestNUMA, TestKernel, TestNetQuality, TestHardware}
	
	// 检查是否仅包含纯本地测试类型
	isLocalOnly := true
	for _, t := range c.TestTypes {
		found := false
		for _, localType := range localOnlyTypes {
			if t == localType {
				found = true
				break
			}
		}
		if !found {
			isLocalOnly = false
			break
		}
	}
	
	// 如果仅包含纯本地测试类型，不需要验证主机和目录
	if isLocalOnly && len(c.TestTypes) > 0 {
		return nil
	}

	// 硬件信息收集模式不需要测试目录和主机
	if hasTestType(c.TestTypes, TestHardware) && len(c.TestTypes) == 1 {
		return nil
	}

	// 网络测试（n/N/M）需要主机和临时目录
	networkTypes := []TestType{TestNetworkSerial, TestNetworkParallel, TestNetworkMatrix}
	hasNetwork := false
	for _, t := range c.TestTypes {
		for _, nt := range networkTypes {
			if t == nt {
				hasNetwork = true
				break
			}
		}
		if hasNetwork {
			break
		}
	}

	if hasNetwork {
		if len(c.Hosts) == 0 && c.HostFile == "" {
			return ErrNoHostsSpecified
		}
		if len(c.TestDirs) == 0 {
			return ErrNoTestDirsSpecified
		}
		return nil
	}

	// 其他测试类型（d, l, I）需要测试目录，但主机可选（默认本机）
	if len(c.TestDirs) == 0 {
		return ErrNoTestDirsSpecified
	}

	return nil
}

// hasTestType 检查是否包含指定的测试类型
func hasTestType(types []TestType, target TestType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

// 错误定义
var (
	// ErrNoHostsSpecified 未指定主机
	ErrNoHostsSpecified = os.ErrInvalid
	// ErrNoTestDirsSpecified 未指定测试目录
	ErrNoTestDirsSpecified = os.ErrInvalid
)
