// Package system 提供系统信息收集功能
// 包含 CPU、内存、磁盘、网卡、虚拟化、操作系统等信息
package system

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"dbcheckperf/pkg/checker/common"
	"dbcheckperf/pkg/utils"
)

// SystemInfo 系统信息
type SystemInfo struct {
	// Host 主机名
	Host string
	// CPUCore CPU 核心数
	CPUCore int
	// CPUModel CPU 型号
	CPUModel string
	// TotalRAM 总内存（字节）
	TotalRAM uint64
	// DiskSize 磁盘大小（字节）
	DiskSize uint64
	// NICSpeed 网卡速率（Mbps）
	NICSpeed int
	// IsVirtual 是否为虚拟机
	IsVirtual bool
	// VirtualType 虚拟化类型
	VirtualType string
	// OSName 操作系统名称
	OSName string
	// KernelVersion 内核版本
	KernelVersion string
	// RAIDInfo RAID 信息
	RAIDInfo *RAIDInfo
	// NICBondInfo 网卡绑定信息
	NICBondInfo *NICBondInfo
}

// RAIDInfo RAID 信息
type RAIDInfo struct {
	// HasRAID 是否有 RAID 卡
	HasRAID bool
	// RAIDModel RAID 卡型号
	RAIDModel string
	// CacheSize RAID 卡缓存大小（字节）
	CacheSize uint64
	// StripeSize 条带大小（KB）
	StripeSize int
	// RAIDLevel RAID 级别
	RAIDLevel string
}

// NICBondInfo 网卡绑定信息
type NICBondInfo struct {
	// HasBond 是否做了绑定
	HasBond bool
	// BondMode 绑定模式
	BondMode string
	// BondSlaves 绑定从网卡数量
	BondSlaves int
	// QueueSize 队列大小
	QueueSize int
}

// SystemChecker 系统信息检查器
type SystemChecker struct{}

// NewSystemChecker 创建新的系统信息检查器
func NewSystemChecker() *SystemChecker {
	return &SystemChecker{}
}

// Run 收集系统信息
func (sc *SystemChecker) Run() (*SystemInfo, error) {
	info := &SystemInfo{
		Host: common.GetHostname(),
	}

	// 获取 CPU 核心数
	info.CPUCore = sc.getCPUCore()

	// 获取 CPU 型号
	info.CPUModel = sc.getCPUModel()

	// 获取总内存
	ram, err := utils.GetTotalRAM()
	if err != nil {
		ram = 0
	}
	info.TotalRAM = ram

	// 获取磁盘大小
	info.DiskSize = sc.getDiskSize()

	// 获取网卡速率
	info.NICSpeed = sc.getNICSpeed()

	// 检测是否为虚拟机
	info.IsVirtual, info.VirtualType = sc.detectVirtualization()

	// 获取操作系统信息
	info.OSName = sc.getOSName()

	// 获取内核版本
	info.KernelVersion = sc.getKernelVersion()

	// 获取 RAID 信息
	info.RAIDInfo = sc.getRAIDInfo()

	// 获取网卡绑定信息
	info.NICBondInfo = sc.getNICBondInfo()

	return info, nil
}

// getCPUCore 获取 CPU 核心数（兼容 ARM 和 AMD）
func (sc *SystemChecker) getCPUCore() int {
	// 尝试从 /proc/cpuinfo 读取
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		count := 0
		for _, line := range strings.Split(string(data), "\n") {
			// x86/x86_64 (Intel/AMD)
			if strings.HasPrefix(line, "processor") {
				count++
			}
			// ARM/ARM64
			if strings.HasPrefix(line, "Processor") || strings.HasPrefix(line, "processor") {
				if strings.Contains(line, ":") {
					count++
				}
			}
		}
		if count > 0 {
			return count
		}
	}

	// 尝试从 /sys/devices/system/cpu 读取（ARM 和 AMD 通用）
	cpuDir := "/sys/devices/system/cpu"
	entries, err := os.ReadDir(cpuDir)
	if err == nil {
		count := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "cpu") {
				// 检查是否为在线 CPU
				onlinePath := fmt.Sprintf("%s/%s/online", cpuDir, entry.Name())
				if data, err := os.ReadFile(onlinePath); err == nil {
					if strings.TrimSpace(string(data)) == "1" {
						count++
					}
				} else {
					// 如果没有 online 文件，默认在线
					count++
				}
			}
		}
		if count > 0 {
			return count
		}
	}

	// 回退方法
	return 1
}

// getCPUModel 获取 CPU 型号（兼容 ARM 和 AMD）
func (sc *SystemChecker) getCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			// x86/x86_64 (Intel/AMD)
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
			// ARM/ARM64
			if strings.HasPrefix(line, "model name") ||
				strings.HasPrefix(line, "CPU part") ||
				strings.HasPrefix(line, "Features") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					model := strings.TrimSpace(parts[1])
					if model != "" && model != "Unknown" {
						return sc.parseARMModel(model)
					}
				}
			}
		}
	}

	// 尝试使用 lscpu 命令
	cmd := exec.Command("lscpu")
	output, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "Model name:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	return "Unknown"
}

// parseARMModel 解析 ARM CPU 型号
func (sc *SystemChecker) parseARMModel(model string) string {
	model = strings.TrimSpace(model)
	if idx := strings.Index(model, " r"); idx != -1 {
		model = strings.TrimSpace(model[:idx])
	}

	// ARM 实现 ID 映射
	armImplementations := map[string]string{
		"0xd03": "Cortex-A53",
		"0xd07": "Cortex-A57",
		"0xd08": "Cortex-A72",
		"0xd09": "Cortex-A73",
		"0xd0a": "Cortex-A75",
		"0xd0b": "Cortex-A76",
		"0xd0c": "Neoverse-N1",
		"0xd0d": "Cortex-A77",
		"0xd40": "Neoverse-V1",
		"0xd41": "Cortex-A78",
		"0xd42": "Cortex-A78AE",
		"0xd43": "Cortex-A65AE",
		"0xd44": "Cortex-X1",
		"0xd46": "Cortex-A510",
		"0xd47": "Cortex-A710",
		"0xd48": "Cortex-X2",
		"0xd49": "Cortex-A715",
		"0xd4e": "Cortex-X3",
		"0xd4f": "Cortex-A520",
		"0xd80": "Cortex-A520",
		"0xd81": "Cortex-A720",
		"0xd82": "Cortex-X4",
		"0xd84": "Cortex-X925",
		"0xd8e": "Cortex-A725",
	}

	// 检查是否包含实现 ID
	for id, name := range armImplementations {
		if strings.Contains(strings.ToLower(model), id) {
			return "ARM " + name
		}
	}

	// 检查是否包含厂商信息
	if strings.Contains(strings.ToLower(model), "qualcomm") ||
		strings.Contains(strings.ToLower(model), "kryo") {
		return "Qualcomm Snapdragon"
	}
	if strings.Contains(strings.ToLower(model), "apple") {
		return "Apple Silicon"
	}
	if strings.Contains(strings.ToLower(model), "nvidia") ||
		strings.Contains(strings.ToLower(model), "carmel") {
		return "NVIDIA"
	}
	if strings.Contains(strings.ToLower(model), "ampere") {
		return "Ampere Altra"
	}
	if strings.Contains(strings.ToLower(model), "fujitsu") {
		return "Fujitsu A64FX"
	}

	return model
}

// detectVirtualization 检测是否为虚拟机及虚拟化类型
func (sc *SystemChecker) detectVirtualization() (bool, string) {
	// 方法 1: 检查 /sys/class/dmi/product/name
	data, err := os.ReadFile("/sys/class/dmi/id/product_name")
	if err == nil {
		content := strings.ToLower(strings.TrimSpace(string(data)))
		if strings.Contains(content, "kvm") || strings.Contains(content, "virtual") {
			return true, "KVM"
		}
		if strings.Contains(content, "vmware") {
			return true, "VMware"
		}
		if strings.Contains(content, "xen") {
			return true, "Xen"
		}
		if strings.Contains(content, "hyperv") {
			return true, "Hyper-V"
		}
	}

	// 方法 2: 使用 systemd-detect-virt
	cmd := exec.Command("systemd-detect-virt")
	output, err := cmd.Output()
	if err == nil {
		virtType := strings.TrimSpace(string(output))
		if virtType != "none" {
			return true, virtType
		}
	}

	return false, "Bare Metal"
}

// getOSName 获取操作系统名称
func (sc *SystemChecker) getOSName() string {
	// 尝试读取 /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				name := strings.TrimPrefix(line, "PRETTY_NAME=")
				return strings.Trim(name, "\"")
			}
		}
	}

	// 尝试读取 /etc/redhat-release
	data, err = os.ReadFile("/etc/redhat-release")
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// 使用 uname 命令
	cmd := exec.Command("uname", "-o")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return "Linux"
}

// getKernelVersion 获取内核版本
func (sc *SystemChecker) getKernelVersion() string {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	// 尝试读取 /proc/version
	data, err := os.ReadFile("/proc/version")
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			return fields[2]
		}
	}

	return "Unknown"
}

// getRAIDInfo 获取 RAID 信息
func (sc *SystemChecker) getRAIDInfo() *RAIDInfo {
	info := &RAIDInfo{
		StripeSize: 64, // 默认条带大小
	}

	// 方法 1: 使用 lspci 检测 RAID 卡
	cmd := exec.Command("lspci")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "raid") || strings.Contains(lineLower, "sas") || strings.Contains(lineLower, "storage controller") {
				info.HasRAID = true
				if idx := strings.Index(line, ":"); idx != -1 {
					info.RAIDModel = strings.TrimSpace(line[idx+1:])
				}
				break
			}
		}
	}

	// 方法 2: 检查 /proc/scsi/scsi
	if !info.HasRAID {
		data, err := os.ReadFile("/proc/scsi/scsi")
		if err == nil {
			if strings.Contains(strings.ToLower(string(data)), "raid") {
				info.HasRAID = true
			}
		}
	}

	// 方法 3: 检测软件 RAID (mdraid)
	data, err := os.ReadFile("/proc/mdstat")
	if err == nil && strings.Contains(string(data), "active") {
		info.HasRAID = true
		info.RAIDModel = "Linux Software RAID (mdraid)"
		info.StripeSize = 512
	}

	// 方法 4: 从块设备获取条带大小
	if info.StripeSize == 64 {
		entries, err := os.ReadDir("/sys/block")
		if err == nil {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "sd") || strings.HasPrefix(entry.Name(), "nvme") {
					logicalBlockPath := fmt.Sprintf("/sys/block/%s/queue/logical_block_size", entry.Name())
					data, err := os.ReadFile(logicalBlockPath)
					if err == nil {
						blockSize, _ := strconv.Atoi(strings.TrimSpace(string(data)))
						if blockSize > 0 {
							info.StripeSize = blockSize / 1024 // 转换为 KB
						}
					}
					break
				}
			}
		}
	}

	return info
}

// getNICBondInfo 获取网卡绑定信息
func (sc *SystemChecker) getNICBondInfo() *NICBondInfo {
	info := &NICBondInfo{
		QueueSize: 1000, // 默认队列大小
	}

	// 检查网络绑定接口
	bondPath := "/sys/class/net"
	entries, err := os.ReadDir(bondPath)
	if err != nil {
		return info
	}

	bondCount := 0
	maxSlaves := 0
	bondMode := ""

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "bond") {
			bondCount++
			info.HasBond = true

			// 读取绑定模式
			modePath := fmt.Sprintf("%s/%s/bonding/mode", bondPath, entry.Name())
			data, err := os.ReadFile(modePath)
			if err == nil {
				modeStr := strings.TrimSpace(string(data))
				if bondMode == "" {
					bondMode = modeStr
				}
			}

			// 读取从网卡数量
			slavesPath := fmt.Sprintf("%s/%s/bonding/slaves", bondPath, entry.Name())
			data, err = os.ReadFile(slavesPath)
			if err == nil {
				slaves := strings.Fields(string(data))
				if len(slaves) > maxSlaves {
					maxSlaves = len(slaves)
				}
			}
		}
	}

	if info.HasBond {
		info.BondMode = bondMode
		info.BondSlaves = maxSlaves
	}

	// 获取队列大小（从主网卡）
	primaryNIC := ""
	entries, err = os.ReadDir(bondPath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasPrefix(entry.Name(), "bond") &&
				!strings.HasPrefix(entry.Name(), "lo") &&
				!strings.HasPrefix(entry.Name(), "docker") {
				primaryNIC = entry.Name()
				break
			}
		}
	}

	// 读取队列长度
	if primaryNIC != "" {
		sizePath := fmt.Sprintf("%s/%s/tx_queue_len", bondPath, primaryNIC)
		data, err := os.ReadFile(sizePath)
		if err == nil {
			size, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if size > 0 {
				info.QueueSize = size
			}
		}
	}

	return info
}

// getDiskSize 获取磁盘总大小
func (sc *SystemChecker) getDiskSize() uint64 {
	// 使用 df 命令获取根分区大小
	cmd := exec.Command("df", "-B1", "/")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 2 {
			size, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				return size
			}
		}
	}

	return 0
}

// getNICSpeed 获取网卡速率
func (sc *SystemChecker) getNICSpeed() int {
	// 尝试从 /sys/class/net 读取
	netPath := "/sys/class/net"
	entries, err := os.ReadDir(netPath)
	if err != nil {
		return 1000 // 默认 1Gbps
	}

	maxSpeed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			speedPath := fmt.Sprintf("%s/%s/speed", netPath, entry.Name())
			data, err := os.ReadFile(speedPath)
			if err == nil {
				speed, err := strconv.Atoi(strings.TrimSpace(string(data)))
				if err == nil && speed > maxSpeed {
					maxSpeed = speed
				}
			}
		}
	}

	if maxSpeed > 0 {
		return maxSpeed
	}

	return 1000 // 默认 1Gbps
}
