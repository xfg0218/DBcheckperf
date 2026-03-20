// Package numa 提供 NUMA 相关信息收集功能
// 包含 NUMA 节点数、内存分布、CPU 亲和性等信息
package numa

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"dbcheckperf/pkg/checker/common"
)

// NUMAInfo NUMA 信息
type NUMAInfo struct {
	// Host 主机名
	Host string
	// NodeCount NUMA 节点数
	NodeCount int
	// CPUPerNode 每个节点的 CPU 核心数
	CPUPerNode []int
	// MemoryPerNode 每个节点的内存大小 (字节)
	MemoryPerNode []uint64
	// FreeMemoryPerNode 每个节点的空闲内存 (字节)
	FreeMemoryPerNode []uint64
	// TotalMemory 总内存 (字节)
	TotalMemory uint64
	// TotalFreeMemory 总空闲内存 (字节)
	TotalFreeMemory uint64
	// CPUDistance CPU 距离矩阵（简化版）
	CPUDistance [][]int
}

// NUMAChecker NUMA 检查器
type NUMAChecker struct {
	// Verbose 是否显示详细输出
	Verbose bool
}

// NewNUMAChecker 创建新的 NUMA 检查器
func NewNUMAChecker(verbose bool) *NUMAChecker {
	return &NUMAChecker{
		Verbose: verbose,
	}
}

// Collect 收集 NUMA 信息
func (nc *NUMAChecker) Collect() (*NUMAInfo, error) {
	info := &NUMAInfo{
		Host: common.GetHostname(),
	}

	// 获取 NUMA 节点数
	info.NodeCount = nc.getNodeCount()

	if info.NodeCount == 0 {
		// 不支持 NUMA 的系统
		info.NodeCount = 1
		info.CPUPerNode = []int{nc.getTotalCPUCore()}
		info.MemoryPerNode = []uint64{nc.getTotalRAM()}
		info.FreeMemoryPerNode = []uint64{nc.getFreeRAM()}
		info.TotalMemory = info.MemoryPerNode[0]
		info.TotalFreeMemory = info.FreeMemoryPerNode[0]
		return info, nil
	}

	// 获取每个节点的 CPU 核心数
	info.CPUPerNode = make([]int, info.NodeCount)
	for i := 0; i < info.NodeCount; i++ {
		info.CPUPerNode[i] = nc.getNodeCPUCore(i)
	}

	// 获取每个节点的内存大小
	info.MemoryPerNode = make([]uint64, info.NodeCount)
	info.FreeMemoryPerNode = make([]uint64, info.NodeCount)
	for i := 0; i < info.NodeCount; i++ {
		total, free := nc.getNodeMemory(i)
		info.MemoryPerNode[i] = total
		info.FreeMemoryPerNode[i] = free
		info.TotalMemory += total
		info.TotalFreeMemory += free
	}

	// 获取 CPU 距离矩阵
	info.CPUDistance = nc.getCPUDistance()

	return info, nil
}

// getNodeCount 获取 NUMA 节点数
func (nc *NUMAChecker) getNodeCount() int {
	// 尝试从 sysfs 读取
	numaPath := "/sys/devices/system/node"
	entries, err := os.ReadDir(numaPath)
	if err != nil {
		return 0 // 不支持 NUMA
	}

	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "node") {
			count++
		}
	}

	return count
}

// getTotalCPUCore 获取总 CPU 核心数
func (nc *NUMAChecker) getTotalCPUCore() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}

	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "processor") {
			count++
		}
	}

	return count
}

// getNodeCPUCore 获取指定 NUMA 节点的 CPU 核心数
func (nc *NUMAChecker) getNodeCPUCore(nodeID int) int {
	cpulistPath := fmt.Sprintf("/sys/devices/system/node/node%d/cpulist", nodeID)
	data, err := os.ReadFile(cpulistPath)
	if err != nil {
		return 0
	}

	// 解析 CPU 列表，如 "0-15,32-47"
	cpuList := strings.TrimSpace(string(data))
	if cpuList == "" {
		return 0
	}

	count := 0
	for _, part := range strings.Split(cpuList, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// 范围，如 "0-15"
			ranges := strings.Split(part, "-")
			if len(ranges) == 2 {
				start, _ := strconv.Atoi(ranges[0])
				end, _ := strconv.Atoi(ranges[1])
				count += end - start + 1
			}
		} else {
			// 单个 CPU
			if _, err := strconv.Atoi(part); err == nil {
				count++
			}
		}
	}

	return count
}

// getNodeMemory 获取指定 NUMA 节点的内存信息
func (nc *NUMAChecker) getNodeMemory(nodeID int) (total, free uint64) {
	meminfoPath := fmt.Sprintf("/sys/devices/system/node/node%d/meminfo", nodeID)
	data, err := os.ReadFile(meminfoPath)
	if err != nil {
		return 0, 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				total = kb * 1024 // 转换为字节
			}
		}
		if strings.Contains(line, "MemFree:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				free = kb * 1024 // 转换为字节
			}
		}
	}

	return total, free
}

// getTotalRAM 获取总内存
func (nc *NUMAChecker) getTotalRAM() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}

	return 0
}

// getFreeRAM 获取空闲内存
func (nc *NUMAChecker) getFreeRAM() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemFree:") || strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}

	return 0
}

// getCPUDistance 获取 CPU 距离矩阵
func (nc *NUMAChecker) getCPUDistance() [][]int {
	distancePath := "/sys/devices/system/node/node0/distance"
	data, err := os.ReadFile(distancePath)
	if err != nil {
		return nil
	}

	var matrix [][]int
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		// 跳过标题行
		if fields[0] == "node" {
			continue
		}

		row := make([]int, len(fields))
		for i, f := range fields {
			val, _ := strconv.Atoi(f)
			row[i] = val
		}
		matrix = append(matrix, row)
	}

	return matrix
}

// GetNUMAInfo 获取 NUMA 信息（快捷函数）
func GetNUMAInfo() (*NUMAInfo, error) {
	checker := NewNUMAChecker(false)
	return checker.Collect()
}

// IsNUMASystem 检查系统是否为 NUMA 架构
func IsNUMASystem() bool {
	info, err := GetNUMAInfo()
	if err != nil {
		return false
	}
	return info.NodeCount > 1
}

// GetCPUAffinity 获取 CPU 亲和性信息
func GetCPUAffinity(pid int) ([]int, error) {
	var cmd *exec.Cmd
	if pid <= 0 {
		// 当前进程
		cmd = exec.Command("taskset", "-cp", "self")
	} else {
		cmd = exec.Command("taskset", "-cp", fmt.Sprintf("%d", pid))
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// 解析输出，如 "current affinity list: 0-15,32-47"
	re := regexp.MustCompile(`affinity list:\s*(.+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return nil, fmt.Errorf("无法解析 CPU 亲和性")
	}

	cpuList := strings.TrimSpace(matches[1])
	cpus := parseCPUList(cpuList)
	return cpus, nil
}

// parseCPUList 解析 CPU 列表字符串
func parseCPUList(cpuList string) []int {
	var cpus []int
	for _, part := range strings.Split(cpuList, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			ranges := strings.Split(part, "-")
			if len(ranges) == 2 {
				start, _ := strconv.Atoi(ranges[0])
				end, _ := strconv.Atoi(ranges[1])
				for i := start; i <= end; i++ {
					cpus = append(cpus, i)
				}
			}
		} else {
			if cpu, err := strconv.Atoi(part); err == nil {
				cpus = append(cpus, cpu)
			}
		}
	}
	return cpus
}

// SetCPUAffinity 设置进程 CPU 亲和性
func SetCPUAffinity(pid int, cpus []int) error {
	if len(cpus) == 0 {
		return fmt.Errorf("CPU 列表为空")
	}

	cpuList := formatCPUList(cpus)
	var cmd *exec.Cmd
	if pid <= 0 {
		cmd = exec.Command("taskset", "-cp", cpuList, "self")
	} else {
		cmd = exec.Command("taskset", "-cp", cpuList, fmt.Sprintf("%d", pid))
	}

	return cmd.Run()
}

// formatCPUList 格式化 CPU 列表
func formatCPUList(cpus []int) string {
	if len(cpus) == 0 {
		return ""
	}

	// 简单实现，直接连接
	parts := make([]string, len(cpus))
	for i, cpu := range cpus {
		parts[i] = fmt.Sprintf("%d", cpu)
	}
	return strings.Join(parts, ",")
}

// GetInterruptAffinity 获取中断亲和性信息
func GetInterruptAffinity(irqNum int) ([]int, error) {
	// 读取中断亲和性
	affinityPath := fmt.Sprintf("/proc/irq/%d/smp_affinity_list", irqNum)
	data, err := os.ReadFile(affinityPath)
	if err != nil {
		// 尝试使用 smp_affinity（位掩码格式）
		return nil, err
	}

	cpuList := strings.TrimSpace(string(data))
	return parseCPUList(cpuList), nil
}

// SetInterruptAffinity 设置中断亲和性
func SetInterruptAffinity(irqNum int, cpus []int) error {
	affinityPath := fmt.Sprintf("/proc/irq/%d/smp_affinity_list", irqNum)
	cpuList := formatCPUList(cpus)
	return os.WriteFile(affinityPath, []byte(cpuList), 0644)
}

// CollectRemote 在远程主机收集 NUMA 信息
func CollectRemote(host string) (*NUMAInfo, error) {
	// 在远程主机执行 numactl 命令
	cmd := fmt.Sprintf("numactl --hardware 2>/dev/null || cat /sys/devices/system/node/online 2>/dev/null || echo '0'")
	output, err := common.RunSSHCommand(host, cmd)
	if err != nil {
		return nil, err
	}

	return parseRemoteNUMAOutput(output)
}

// parseRemoteNUMAOutput 解析远程 NUMA 输出
func parseRemoteNUMAOutput(output string) (*NUMAInfo, error) {
	info := &NUMAInfo{}

	// 尝试解析 numactl --hardware 输出
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "available:") {
			// 解析节点数
			re := regexp.MustCompile(`(\d+)\s+nodes`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				info.NodeCount, _ = strconv.Atoi(matches[1])
			}
		}
	}

	if info.NodeCount == 0 {
		// 回退到简单解析
		info.NodeCount = 1
	}

	return info, nil
}

// PrintNUMAInfo 打印 NUMA 信息（用于调试）
func PrintNUMAInfo(info *NUMAInfo) {
	fmt.Printf("NUMA 信息:\n")
	fmt.Printf("  节点数：%d\n", info.NodeCount)
	fmt.Printf("  总内存：%s\n", formatBytes(info.TotalMemory))

	for i := 0; i < info.NodeCount; i++ {
		fmt.Printf("  节点 %d:\n", i)
		fmt.Printf("    CPU 核心数：%d\n", info.CPUPerNode[i])
		fmt.Printf("    内存：%s\n", formatBytes(info.MemoryPerNode[i]))
		fmt.Printf("    空闲内存：%s\n", formatBytes(info.FreeMemoryPerNode[i]))
	}

	if len(info.CPUDistance) > 0 {
		fmt.Printf("  CPU 距离矩阵:\n")
		for i, row := range info.CPUDistance {
			fmt.Printf("    节点 %d: %v\n", i, row)
		}
	}
}

// formatBytes 格式化字节数
func formatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// GetNUMADeviceAffinity 获取设备（如网卡、磁盘）的 NUMA 亲和性
func GetNUMADeviceAffinity(deviceType, deviceName string) (int, error) {
	// 尝试从 sysfs 读取设备的 NUMA 节点
	devicePath := fmt.Sprintf("/sys/class/%s/%s/device/numa_node", deviceType, deviceName)
	data, err := os.ReadFile(devicePath)
	if err != nil {
		return -1, err
	}

	nodeID, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return nodeID, nil
}

// GetNetworkDeviceNUMA 获取网卡的 NUMA 节点
func GetNetworkDeviceNUMA(iface string) (int, error) {
	return GetNUMADeviceAffinity("net", iface)
}

// GetBlockDeviceNUMA 获取块设备的 NUMA 节点
func GetBlockDeviceNUMA(device string) (int, error) {
	return GetNUMADeviceAffinity("block", device)
}
