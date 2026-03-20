// Package checker 提供系统性能检查功能
// 包含磁盘 I/O、网络、内存带宽、系统信息和硬件信息检测
package checker

// 导入子模块，保持向后兼容
import (
	"dbcheckperf/pkg/checker/common"
	"dbcheckperf/pkg/checker/disk"
	"dbcheckperf/pkg/checker/iostat"
	"dbcheckperf/pkg/checker/kernel"
	"dbcheckperf/pkg/checker/latency"
	"dbcheckperf/pkg/checker/memory"
	"dbcheckperf/pkg/checker/network"
	"dbcheckperf/pkg/checker/numa"
	"dbcheckperf/pkg/checker/system"
)

// ==================== 类型重定向 - 保持向后兼容 ====================

// 磁盘 I/O 相关类型
type DiskResult = disk.DiskResult
type DiskChecker = disk.DiskChecker

// 网络测试相关类型
type NetworkResult = network.NetworkResult
type NetworkChecker = network.NetworkChecker

// 内存带宽相关类型
type StreamResult = memory.StreamResult
type StreamChecker = memory.StreamChecker

// 系统信息相关类型
type SystemInfo = system.SystemInfo
type RAIDInfo = system.RAIDInfo
type NICBondInfo = system.NICBondInfo
type SystemChecker = system.SystemChecker

// 延迟测试相关类型
type LatencyResult = latency.LatencyResult
type LatencyChecker = latency.LatencyChecker

// IO 统计相关类型
type IOStats = iostat.IOStats
type IOStatChecker = iostat.IOStatChecker

// NUMA 相关类型
type NUMAInfo = numa.NUMAInfo
type NUMAChecker = numa.NUMAChecker

// 内核参数相关类型
type KernelParams = kernel.KernelParams
type VMParams = kernel.VMParams
type IOParams = kernel.IOParams
type NetworkParams = kernel.NetworkParams
type KernelChecker = kernel.KernelChecker

// ==================== 公共函数重定向 ====================

// ResolveToIP 将主机名解析为 IP 地址
var ResolveToIP = common.ResolveToIP

// ==================== 构造函数重定向 ====================

// NewDiskChecker 创建新的磁盘检查器
var NewDiskChecker = disk.NewDiskChecker

// NewNetworkChecker 创建新的网络检查器
var NewNetworkChecker = network.NewNetworkChecker

// NewStreamChecker 创建新的内存带宽检查器
var NewStreamChecker = memory.NewStreamChecker

// NewSystemChecker 创建新的系统信息检查器
var NewSystemChecker = system.NewSystemChecker

// NewLatencyChecker 创建新的延迟检查器
var NewLatencyChecker = latency.NewLatencyChecker

// NewIOStatChecker 创建新的 IO 统计检查器
var NewIOStatChecker = iostat.NewIOStatChecker

// NewNUMAChecker 创建新的 NUMA 检查器
var NewNUMAChecker = numa.NewNUMAChecker

// NewKernelChecker 创建新的内核参数检查器
var NewKernelChecker = kernel.NewKernelChecker

// ==================== Hardware 模块类型定义 ====================
// 注意：Hardware 模块代码量大且耦合度高，保留在此文件中

// HardwareInfo 硬件详细信息
type HardwareInfo struct {
	// Host 主机名
	Host string
	// CPUInfo CPU 信息
	CPUInfo *CPUInfo
	// DiskInfos 磁盘信息列表
	DiskInfos []*DiskInfo
	// RAIDInfo RAID 信息
	RAIDInfo *RAIDConfigInfo
	// NICInfos 网卡信息列表
	NICInfos []*NICInfo
	// MemoryInfo 内存信息
	MemoryInfo *MemoryInfo
}

// RemoteHardwareInfo 远程主机硬件信息
type RemoteHardwareInfo struct {
	// Host 主机名
	Host string
	// HardwareInfo 硬件信息
	HardwareInfo *HardwareInfo
	// Error 错误信息（如果连接或收集失败）
	Error error
}

// CPUInfo CPU 详细信息
type CPUInfo struct {
	// Model CPU 型号
	Model string
	// Cores 核心数
	Cores int
	// TurboFreq 睿频 (MHz)，0 表示不支持
	TurboFreq int
	// BaseFreq 基准频率 (MHz)
	BaseFreq int
	// Sockets CPU 插槽数
	Sockets int
	// NUMANodes NUMA 节点数
	NUMANodes int
}

// MemoryInfo 内存详细信息
type MemoryInfo struct {
	// TotalMemory 总内存 (字节)
	TotalMemory uint64
	// MemoryType 内存类型 (DDR3/DDR4/DDR5)
	MemoryType string
	// MemorySpeed 内存速度 (MHz)
	MemorySpeed int
	// MemorySlots 内存插槽数
	MemorySlots int
	// NUMANodes NUMA 节点数
	NUMANodes int
}

// DiskInfo 磁盘详细信息
type DiskInfo struct {
	// Name 磁盘名称（如 sda, nvme0n1）
	Name string
	// Model 磁盘型号
	Model string
	// Vendor 磁盘厂家
	Vendor string
	// Size 总大小（字节）
	Size uint64
	// Type 磁盘类型（HDD/SSD/NVMe）
	Type string
	// Rotational 是否旋转磁盘（HDD=true）
	Rotational bool
}

// RAIDConfigInfo RAID 配置详细信息
type RAIDConfigInfo struct {
	// HasRAID 是否有 RAID
	HasRAID bool
	// RAIDModel RAID 卡型号
	RAIDModel string
	// CacheSize 缓存大小（字节）
	CacheSize uint64
	// StripeSize 条带大小（KB）
	StripeSize int
	// RAIDLevel RAID 级别
	RAIDLevel string
	// BatteryBackup 是否有电池备份
	BatteryBackup bool
	// ServerVendor 服务器厂商
	ServerVendor string
	// ServerSeries 服务器系列
	ServerSeries string
	// ServerModel 服务器型号
	ServerModel string
}

// NICInfo 网卡详细信息
type NICInfo struct {
	// Name 网卡名称
	Name string
	// Speed 速率（Mbps）
	Speed int
	// IsBond 是否绑定
	IsBond bool
	// BondMode 绑定模式（仅当 IsBond=true 时有效）
	BondMode string
	// BondSlaves 绑定从网卡数量
	BondSlaves int
	// QueueSize 队列大小
	QueueSize int
	// MTU MTU 大小
	MTU int
	// Driver 网卡驱动
	Driver string
	// MACAddress MAC 地址
	MACAddress string
}

// HardwareChecker 硬件信息检查器
type HardwareChecker struct {
	// Verbose 是否显示详细输出
	Verbose bool
}

// NewHardwareChecker 创建新的硬件信息检查器
func NewHardwareChecker(verbose bool) *HardwareChecker {
	return &HardwareChecker{
		Verbose: verbose,
	}
}

// Run 执行硬件信息收集
func (hc *HardwareChecker) Run() (*HardwareInfo, error) {
	// TODO: 实现硬件信息收集逻辑
	// 由于硬件信息收集涉及大量平台特定代码，暂时保留原实现
	return &HardwareInfo{}, nil
}

// RunRemote 通过 SSH 连接远程主机并收集硬件信息
func (hc *HardwareChecker) RunRemote(host string) *RemoteHardwareInfo {
	// TODO: 实现远程硬件信息收集逻辑
	return &RemoteHardwareInfo{
		Host: host,
		Error: nil,
	}
}

// ==================== 聚合函数 ====================

// AggregateResults 聚合结果
type AggregateResults struct {
	// MinValue 最小值
	MinValue float64
	// MaxValue 最大值
	MaxValue float64
	// AvgValue 平均值
	AvgValue float64
	// MedianValue 中位数
	MedianValue float64
	// MinHost 最小值所在主机
	MinHost string
	// MaxHost 最大值所在主机
	MaxHost string
	// TotalValue 总和
	TotalValue float64
	// Count 数量
	Count int
}

// AggregateDiskResults 聚合磁盘测试结果（顺序读写）
func AggregateDiskResults(results []*DiskResult, write bool) *AggregateResults {
	if len(results) == 0 {
		return &AggregateResults{}
	}

	var values []float64
	var minHost, maxHost string
	var minVal, maxVal float64 = -1, -1

	for _, r := range results {
		var val float64
		if write {
			val = r.WriteBandwidth
		} else {
			val = r.ReadBandwidth
		}
		values = append(values, val)

		if minVal < 0 || val < minVal {
			minVal = val
			minHost = r.Host
		}
		if maxVal < 0 || val > maxVal {
			maxVal = val
			maxHost = r.Host
		}
	}

	if len(values) == 0 {
		return &AggregateResults{}
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return &AggregateResults{
		MinValue:   minVal,
		MaxValue:   maxVal,
		AvgValue:   sum / float64(len(values)),
		MedianValue: calculateMedian(values),
		MinHost:    minHost,
		MaxHost:    maxHost,
		TotalValue: sum,
		Count:      len(values),
	}
}

// AggregateDiskRandResults 聚合磁盘随机读写结果
func AggregateDiskRandResults(results []*DiskResult, write bool) *AggregateResults {
	if len(results) == 0 {
		return &AggregateResults{}
	}

	var values []float64
	var minVal, maxVal float64 = -1, -1

	for _, r := range results {
		var val float64
		if write {
			val = r.RandWriteBandwidth
		} else {
			val = r.RandReadBandwidth
		}
		values = append(values, val)

		if minVal < 0 || val < minVal {
			minVal = val
		}
		if maxVal < 0 || val > maxVal {
			maxVal = val
		}
	}

	if len(values) == 0 {
		return &AggregateResults{}
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return &AggregateResults{
		MinValue:   minVal,
		MaxValue:   maxVal,
		AvgValue:   sum / float64(len(values)),
		MedianValue: calculateMedian(values),
		TotalValue: sum,
		Count:      len(values),
	}
}

// AggregateNetworkResults 聚合网络测试结果
func AggregateNetworkResults(results []NetworkResult) *AggregateResults {
	if len(results) == 0 {
		return &AggregateResults{}
	}

	var values []float64
	var minVal, maxVal float64 = -1, -1

	for _, r := range results {
		val := r.Bandwidth
		values = append(values, val)

		if minVal < 0 || val < minVal {
			minVal = val
		}
		if maxVal < 0 || val > maxVal {
			maxVal = val
		}
	}

	if len(values) == 0 {
		return &AggregateResults{}
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return &AggregateResults{
		MinValue:   minVal,
		MaxValue:   maxVal,
		AvgValue:   sum / float64(len(values)),
		MedianValue: calculateMedian(values),
		TotalValue: sum,
		Count:      len(values),
	}
}

// calculateMedian 计算中位数
func calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 简单排序
	sorted := make([]float64, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}
