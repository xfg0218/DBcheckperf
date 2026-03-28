// Package checker 提供系统性能检查功能
// 包含磁盘 I/O、网络、内存带宽、系统信息和硬件信息检测
package checker

// 导入子模块，保持向后兼容
import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"dbcheckperf/pkg/checker/common"
	"dbcheckperf/pkg/checker/disk"
	"dbcheckperf/pkg/checker/iostat"
	"dbcheckperf/pkg/checker/kernel"
	"dbcheckperf/pkg/checker/latency"
	"dbcheckperf/pkg/checker/memory"
	"dbcheckperf/pkg/checker/network"
	"dbcheckperf/pkg/checker/numa"
	"dbcheckperf/pkg/checker/system"
	"dbcheckperf/pkg/utils"
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
	info := &HardwareInfo{
		Host: common.GetHostname(),
	}

	// 收集 CPU 信息
	info.CPUInfo = hc.getCPUInfo()

	// 收集内存信息
	info.MemoryInfo = hc.getMemoryInfo()

	// 收集磁盘信息
	info.DiskInfos = hc.getDiskInfos()

	// 收集 RAID 信息
	info.RAIDInfo = hc.getRAIDInfo()

	// 收集网卡信息
	info.NICInfos = hc.getNICInfos()

	return info, nil
}

// getCPUInfo 获取 CPU 信息
func (hc *HardwareChecker) getCPUInfo() *CPUInfo {
	cpu := &CPUInfo{
		Model:      "Unknown",
		Cores:      1,
		Sockets:    1,
		NUMANodes:  1,
	}

	// 读取 /proc/cpuinfo
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return cpu
	}

	lines := strings.Split(string(data), "\n")
	processorCount := 0
	physicalIds := make(map[string]bool)

	for _, line := range lines {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				cpu.Model = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "processor") {
			processorCount++
		}
		if strings.HasPrefix(line, "physical id") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				physicalIds[strings.TrimSpace(parts[1])] = true
			}
		}
		if strings.HasPrefix(line, "cpu MHz") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				mhz, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				cpu.BaseFreq = int(mhz)
			}
		}
	}

	cpu.Cores = processorCount
	if processorCount > 0 {
		cpu.Sockets = len(physicalIds)
		if cpu.Sockets == 0 {
			cpu.Sockets = 1
		}
	}

	// 获取睿频
	if cpu.BaseFreq > 0 {
		data, err = os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
		if err == nil {
			maxFreq, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if maxFreq > 0 {
				cpu.TurboFreq = maxFreq / 1000
			}
		}
	}

	// 获取 NUMA 节点数
	cpu.NUMANodes = hc.getNumANodes()

	return cpu
}

// getMemoryInfo 获取内存信息
func (hc *HardwareChecker) getMemoryInfo() *MemoryInfo {
	mem := &MemoryInfo{
		MemoryType:   "Unknown",
		MemorySpeed:  0,
		MemorySlots:  0,
		NUMANodes:    1,
	}

	// 获取总内存
	total, err := utils.GetTotalRAM()
	if err == nil {
		mem.TotalMemory = total
	}

	// 获取 NUMA 节点数
	mem.NUMANodes = hc.getNumANodes()

	// 尝试使用 dmidecode 获取内存类型和速度
	cmd := exec.Command("dmidecode", "-t", "memory")
	output, err := cmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Type:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					memType := strings.TrimSpace(parts[1])
					if memType != "" && memType != "Unknown" {
						mem.MemoryType = memType
					}
				}
			}
			if strings.Contains(line, "Speed:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					speed, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
					if speed > 0 {
						mem.MemorySpeed = speed
					}
				}
			}
			if strings.Contains(line, "Locator:") && strings.Contains(line, "DIMM") {
				mem.MemorySlots++
			}
		}
	}

	return mem
}

// getDiskInfos 获取磁盘信息列表
func (hc *HardwareChecker) getDiskInfos() []*DiskInfo {
	var diskInfos []*DiskInfo

	// 获取块设备列表
	blockDir := "/sys/block"
	entries, err := os.ReadDir(blockDir)
	if err != nil {
		return diskInfos
	}

	for _, entry := range entries {
		name := entry.Name()
		// 跳过 loop 设备和分区
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "sr") {
			continue
		}
		// 只处理 sd* 和 nvme* 设备
		if !strings.HasPrefix(name, "sd") && !strings.HasPrefix(name, "nvme") && !strings.HasPrefix(name, "vd") {
			continue
		}

		diskInfo := hc.getSingleDiskInfo(name)
		if diskInfo != nil {
			diskInfos = append(diskInfos, diskInfo)
		}
	}

	return diskInfos
}

// getSingleDiskInfo 获取单个磁盘信息
func (hc *HardwareChecker) getSingleDiskInfo(name string) *DiskInfo {
	diskInfo := &DiskInfo{
		Name:       name,
		Model:      "Unknown",
		Vendor:     "Unknown",
		Type:       "Unknown",
		Rotational: true,
	}

	// 获取磁盘大小
	sizePath := fmt.Sprintf("/sys/block/%s/size", name)
	data, err := os.ReadFile(sizePath)
	if err == nil {
		sectors, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		diskInfo.Size = sectors * 512 // 每扇区 512 字节
	}

	// 获取磁盘类型（是否旋转）
	rotPath := fmt.Sprintf("/sys/block/%s/queue/rotational", name)
	data, err = os.ReadFile(rotPath)
	if err == nil {
		rotational := strings.TrimSpace(string(data))
		diskInfo.Rotational = rotational == "1"
		if rotational == "0" {
			// NVMe 设备
			if strings.HasPrefix(name, "nvme") {
				diskInfo.Type = "NVMe"
			} else {
				diskInfo.Type = "SSD"
			}
		} else {
			diskInfo.Type = "HDD"
		}
	}

	// 使用 hdparm 获取磁盘型号
	cmd := exec.Command("hdparm", "-I", "/dev/"+name)
	output, err := cmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Model Number:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					diskInfo.Model = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// 尝试从 /sys/block/name/device 获取厂家信息
	vendorPath := fmt.Sprintf("/sys/block/%s/device/vendor", name)
	data, err = os.ReadFile(vendorPath)
	if err == nil {
		diskInfo.Vendor = strings.TrimSpace(string(data))
	}

	return diskInfo
}

// getRAIDInfo 获取 RAID 信息
func (hc *HardwareChecker) getRAIDInfo() *RAIDConfigInfo {
	raid := &RAIDConfigInfo{
		HasRAID:      false,
		StripeSize:   64, // 默认条带大小
		BatteryBackup: false,
	}

	// 检测软件 RAID
	data, err := os.ReadFile("/proc/mdstat")
	if err == nil && strings.Contains(string(data), "active") {
		raid.HasRAID = true
		raid.RAIDModel = "Linux Software RAID (mdraid)"
		raid.StripeSize = 512
		return raid
	}

	// 使用 lspci 检测 RAID 卡
	cmd := exec.Command("lspci")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "raid") || strings.Contains(lineLower, "sas") || strings.Contains(lineLower, "storage controller") {
				raid.HasRAID = true
				if idx := strings.Index(line, ":"); idx != -1 {
					raid.RAIDModel = strings.TrimSpace(line[idx+1:])
				}
				break
			}
		}
	}

	// 尝试使用 megacli/storcli 获取 RAID 信息
	cmd = exec.Command("storcli", "/c0", "show")
	output, err = cmd.Output()
	if err == nil {
		raid.HasRAID = true
		if raid.RAIDModel == "" {
			raid.RAIDModel = "LSI/Broadcom RAID"
		}
		// 解析缓存和条带大小（简化处理）
		raid.CacheSize = 1024 * 1024 * 1024 // 1GB
		raid.StripeSize = 64
	}

	return raid
}

// getNICInfos 获取网卡信息列表
func (hc *HardwareChecker) getNICInfos() []*NICInfo {
	var nicInfos []*NICInfo

	netPath := "/sys/class/net"
	entries, err := os.ReadDir(netPath)
	if err != nil {
		return nicInfos
	}

	for _, entry := range entries {
		name := entry.Name()
		// 跳过 lo 和虚拟接口
		if name == "lo" || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "veth") {
			continue
		}

		nicInfo := hc.getSingleNICInfo(name)
		if nicInfo != nil {
			nicInfos = append(nicInfos, nicInfo)
		}
	}

	return nicInfos
}

// getSingleNICInfo 获取单个网卡信息
func (hc *HardwareChecker) getSingleNICInfo(name string) *NICInfo {
	nic := &NICInfo{
		Name:     name,
		Speed:    0,
		IsBond:   false,
		BondMode: "",
		BondSlaves: 0,
		MTU:      1500,
		Driver:   "Unknown",
		MACAddress: "",
	}

	// 获取网卡速率
	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", name)
	data, err := os.ReadFile(speedPath)
	if err == nil {
		speed, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if speed > 0 {
			nic.Speed = speed
		}
	}

	// 获取 MTU
	mtuPath := fmt.Sprintf("/sys/class/net/%s/mtu", name)
	data, err = os.ReadFile(mtuPath)
	if err == nil {
		mtu, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if mtu > 0 {
			nic.MTU = mtu
		}
	}

	// 获取 MAC 地址
	macPath := fmt.Sprintf("/sys/class/net/%s/address", name)
	data, err = os.ReadFile(macPath)
	if err == nil {
		nic.MACAddress = strings.TrimSpace(string(data))
	}

	// 获取驱动信息
	driverPath := fmt.Sprintf("/sys/class/net/%s/device/driver", name)
	if _, err := os.Stat(driverPath); err == nil {
		link, _ := os.Readlink(driverPath)
		if link != "" {
			nic.Driver = filepath.Base(link)
		}
	}

	// 检查是否为 bond 设备
	if strings.HasPrefix(name, "bond") {
		nic.IsBond = true
		// 读取绑定模式
		modePath := fmt.Sprintf("/sys/class/net/%s/bonding/mode", name)
		data, err = os.ReadFile(modePath)
		if err == nil {
			nic.BondMode = strings.TrimSpace(string(data))
		}
		// 读取从网卡数量
		slavesPath := fmt.Sprintf("/sys/class/net/%s/bonding/slaves", name)
		data, err = os.ReadFile(slavesPath)
		if err == nil {
			slaves := strings.Fields(string(data))
			nic.BondSlaves = len(slaves)
		}
	}

	// 获取队列大小
	queuePath := fmt.Sprintf("/sys/class/net/%s/tx_queue_len", name)
	data, err = os.ReadFile(queuePath)
	if err == nil {
		queue, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if queue > 0 {
			nic.QueueSize = queue
		}
	}

	return nic
}

// getNumANodes 获取 NUMA 节点数
func (hc *HardwareChecker) getNumANodes() int {
	numaPath := "/sys/devices/system/node"
	entries, err := os.ReadDir(numaPath)
	if err != nil {
		return 1
	}

	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "node") {
			count++
		}
	}

	if count > 0 {
		return count
	}
	return 1
}

// RunRemote 通过 SSH 连接远程主机并收集硬件信息
func (hc *HardwareChecker) RunRemote(host string) *RemoteHardwareInfo {
	// 默认返回错误，因为需要 SSH 认证信息
	// 实际调用应该使用 RunRemoteWithAuth
	return &RemoteHardwareInfo{
		Host: host,
		Error: fmt.Errorf("需要使用 SSH 认证信息调用 RunRemoteWithAuth"),
	}
}

// RunRemoteWithAuth 通过 SSH 密码认证连接远程主机并收集硬件信息
func (hc *HardwareChecker) RunRemoteWithAuth(host, username, password string, port int) *RemoteHardwareInfo {
	remoteInfo := &RemoteHardwareInfo{
		Host: host,
	}

	// 收集硬件信息的脚本
	hardwareScript := `
echo "===CPU_INFO==="
cat /proc/cpuinfo 2>/dev/null | grep -E "model name|processor|physical id|cpu MHz" | head -50

echo "===MEMORY_INFO==="
free -b 2>/dev/null
dmidecode -t memory 2>/dev/null | grep -E "Type:|Speed:|Locator:" | head -20

echo "===DISK_INFO==="
lsblk -bd -o NAME,SIZE,ROTA,TYPE,MODEL 2>/dev/null || ls -la /sys/block/ 2>/dev/null

echo "===RAID_INFO==="
cat /proc/mdstat 2>/dev/null
lspci | grep -iE "raid|sas|storage" 2>/dev/null

echo "===NIC_INFO==="
ip -o link show 2>/dev/null
for dev in /sys/class/net/*/speed; do echo "$dev: $(cat $dev 2>/dev/null)"; done 2>/dev/null
for dev in /sys/class/net/*/mtu; do echo "$dev: $(cat $dev 2>/dev/null)"; done 2>/dev/null
`

	// 使用 SSH 密码认证执行命令
	output, err := common.RunSSHCommandWithAuth(host, username, password, port, hardwareScript)
	if err != nil {
		remoteInfo.Error = fmt.Errorf("SSH 连接失败：%v", err)
		return remoteInfo
	}

	// 解析输出
	remoteInfo.HardwareInfo = hc.parseRemoteHardwareOutput(string(output))
	
	return remoteInfo
}

// parseRemoteHardwareOutput 解析远程主机返回的硬件信息
func (hc *HardwareChecker) parseRemoteHardwareOutput(output string) *HardwareInfo {
	info := &HardwareInfo{
		CPUInfo:    &CPUInfo{},
		MemoryInfo: &MemoryInfo{},
	}

	lines := strings.Split(output, "\n")
	section := ""
	
	for _, line := range lines {
		if strings.Contains(line, "===CPU_INFO===") {
			section = "cpu"
			continue
		}
		if strings.Contains(line, "===MEMORY_INFO===") {
			section = "memory"
			continue
		}
		if strings.Contains(line, "===DISK_INFO===") {
			section = "disk"
			continue
		}
		if strings.Contains(line, "===RAID_INFO===") {
			section = "raid"
			continue
		}
		if strings.Contains(line, "===NIC_INFO===") {
			section = "nic"
			continue
		}

		switch section {
		case "cpu":
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					info.CPUInfo.Model = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(line, "processor") {
				info.CPUInfo.Cores++
			}
			if strings.HasPrefix(line, "cpu MHz") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					mhz, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
					info.CPUInfo.BaseFreq = int(mhz)
				}
			}
		case "memory":
			if strings.Contains(line, "Type:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					info.MemoryInfo.MemoryType = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	info.CPUInfo.NUMANodes = hc.getNumANodes()
	info.MemoryInfo.NUMANodes = info.CPUInfo.NUMANodes

	return info
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
