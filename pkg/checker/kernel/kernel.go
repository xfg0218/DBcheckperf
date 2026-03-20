// Package kernel 提供内核参数收集功能
// 包含 VM 参数、IO 参数等与数据库性能相关的内核配置
package kernel

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"dbcheckperf/pkg/checker/common"
)

// KernelParams 内核参数
type KernelParams struct {
	// Host 主机名
	Host string
	// VM 参数
	VM VMParams
	// IO 参数
	IO IOParams
	// 网络参数
	Network NetworkParams
}

// VMParams 虚拟机/内存管理参数
type VMParams struct {
	// DirtyRatio 脏页比例上限 (%)
	DirtyRatio int
	// DirtyBackgroundRatio 后台写回比例 (%)
	DirtyBackgroundRatio int
	// Swappiness Swap 倾向性 (0-100)
	Swappiness int
	// OvercommitMemory 内存超卖策略
	OvercommitMemory int
	// OvercommitRatio 内存超卖比例 (%)
	OvercommitRatio int
	// DropCaches 是否允许清空缓存
	DropCaches int
	// MinFreeKbytes 最小空闲内存 (KB)
	MinFreeKbytes int
	// LowmemReserveRatio 低内存保留比例
	LowmemReserveRatio string
	// DirtyExpireCs 脏页过期时间 (厘秒)
	DirtyExpireCs int
	// DirtyWritebackCs 脏页写回间隔 (厘秒)
	DirtyWritebackCs int
	// VfsCachePressure VFS 缓存压力
	VfsCachePressure int
	// TransparentHugepage 透明大页
	TransparentHugepage string
	// NumaBalancing 自动 NUMA 平衡
	NumaBalancing int
}

// IOParams IO 相关参数
type IOParams struct {
	// ReadAheadKB 预读大小 (KB)
	ReadAheadKB int
	// MaxSectorsKB 最大 IO 大小 (KB)
	MaxSectorsKB int
	// MaxReadRequestKb 最大读请求大小 (KB)
	MaxReadRequestKb int
	// NrRequests 队列深度
	NrRequests int
	// Scheduler 调度算法
	Scheduler string
	// Iostats 是否启用 IO 统计
	Iostats int
}

// NetworkParams 网络相关参数
type NetworkParams struct {
	// Somaxconn 最大监听队列长度
	Somaxconn int
	// NetdevMaxBacklog 网络设备队列长度
	NetdevMaxBacklog int
	// WmemDefault 默认写缓冲区大小
	WmemDefault int
	// WmemMax 最大写缓冲区大小
	WmemMax int
	// RmemDefault 默认读缓冲区大小
	RmemDefault int
	// RmemMax 最大读缓冲区大小
	RmemMax int
	// TcpMaxSynBacklog TCP SYN 队列长度
	TcpMaxSynBacklog int
	// TcpMaxTwBuckets TCP TIME_WAIT 队列长度
	TcpMaxTwBuckets int
	// TcpFinTimeout TCP FIN 超时 (秒)
	TcpFinTimeout int
	// TcpKeepaliveTime TCP Keepalive 时间 (秒)
	TcpKeepaliveTime int
	// TcpTwReuse TCP TIME_WAIT 重用
	TcpTwReuse int
}

// KernelChecker 内核参数检查器
type KernelChecker struct {
	// Verbose 是否显示详细输出
	Verbose bool
	// Devices 监控的设备列表（用于设备相关参数）
	Devices []string
}

// NewKernelChecker 创建新的内核参数检查器
func NewKernelChecker(verbose bool, devices []string) *KernelChecker {
	return &KernelChecker{
		Verbose: verbose,
		Devices: devices,
	}
}

// Collect 收集内核参数
func (kc *KernelChecker) Collect() (*KernelParams, error) {
	params := &KernelParams{
		Host: common.GetHostname(),
	}

	// 收集 VM 参数
	params.VM = kc.collectVMParams()

	// 收集 IO 参数
	params.IO = kc.collectIOParams()

	// 收集网络参数
	params.Network = kc.collectNetworkParams()

	return params, nil
}

// collectVMParams 收集 VM 参数
func (kc *KernelChecker) collectVMParams() VMParams {
	vm := VMParams{}

	vm.DirtyRatio = kc.readIntProcSys("vm/dirty_ratio")
	vm.DirtyBackgroundRatio = kc.readIntProcSys("vm/dirty_background_ratio")
	vm.Swappiness = kc.readIntProcSys("vm/swappiness")
	vm.OvercommitMemory = kc.readIntProcSys("vm/overcommit_memory")
	vm.OvercommitRatio = kc.readIntProcSys("vm/overcommit_ratio")
	vm.DropCaches = kc.readIntProcSys("vm/drop_caches")
	vm.MinFreeKbytes = kc.readIntProcSys("vm/min_free_kbytes")
	vm.LowmemReserveRatio = kc.readStringProcSys("vm/lowmem_reserve_ratio")
	vm.DirtyExpireCs = kc.readIntProcSys("vm/dirty_expire_centisecs")
	vm.DirtyWritebackCs = kc.readIntProcSys("vm/dirty_writeback_centisecs")
	vm.VfsCachePressure = kc.readIntProcSys("vm/vfs_cache_pressure")
	vm.TransparentHugepage = kc.readStringProcSys("kernel/mm/transparent_hugepage/enabled")
	vm.NumaBalancing = kc.readIntProcSys("kernel/numa_balancing")

	return vm
}

// collectIOParams 收集 IO 参数
func (kc *KernelChecker) collectIOParams() IOParams {
	io := IOParams{}

	// 读取第一个设备的 IO 参数
	device := ""
	if len(kc.Devices) > 0 {
		device = kc.Devices[0]
	} else {
		device = kc.getDefaultBlockDevice()
	}

	if device != "" {
		io.ReadAheadKB = kc.readIntSysBlock(device, "queue/read_ahead_kb")
		io.MaxSectorsKB = kc.readIntSysBlock(device, "queue/max_sectors_kb")
		io.MaxReadRequestKb = kc.readIntSysBlock(device, "queue/max_read_request_kb")
		io.NrRequests = kc.readIntSysBlock(device, "queue/nr_requests")
		io.Scheduler = kc.readSchedulerSysBlock(device)
		io.Iostats = kc.readIntSysBlock(device, "queue/iostats")
	}

	return io
}

// collectNetworkParams 收集网络参数
func (kc *KernelChecker) collectNetworkParams() NetworkParams {
	net := NetworkParams{}

	net.Somaxconn = kc.readIntProcSys("net/core/somaxconn")
	net.NetdevMaxBacklog = kc.readIntProcSys("net/core/netdev_max_backlog")
	net.WmemDefault = kc.readIntProcSys("net/core/wmem_default")
	net.WmemMax = kc.readIntProcSys("net/core/wmem_max")
	net.RmemDefault = kc.readIntProcSys("net/core/rmem_default")
	net.RmemMax = kc.readIntProcSys("net/core/rmem_max")
	net.TcpMaxSynBacklog = kc.readIntProcSys("net/ipv4/tcp_max_syn_backlog")
	net.TcpMaxTwBuckets = kc.readIntProcSys("net/ipv4/tcp_max_tw_buckets")
	net.TcpFinTimeout = kc.readIntProcSys("net/ipv4/tcp_fin_timeout")
	net.TcpKeepaliveTime = kc.readIntProcSys("net/ipv4/tcp_keepalive_time")
	net.TcpTwReuse = kc.readIntProcSys("net/ipv4/tcp_tw_reuse")

	return net
}

// readIntProcSys 读取 /proc/sys/ 下的整数参数
func (kc *KernelChecker) readIntProcSys(path string) int {
	fullPath := fmt.Sprintf("/proc/sys/%s", path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return 0
	}
	val, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return val
}

// readStringProcSys 读取 /proc/sys/ 下的字符串参数
func (kc *KernelChecker) readStringProcSys(path string) string {
	fullPath := fmt.Sprintf("/proc/sys/%s", path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// readIntSysBlock 读取 /sys/block/ 下的整数参数
func (kc *KernelChecker) readIntSysBlock(device, param string) int {
	fullPath := fmt.Sprintf("/sys/block/%s/%s", device, param)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return 0
	}
	val, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return val
}

// readSchedulerSysBlock 读取调度算法
func (kc *KernelChecker) readSchedulerSysBlock(device string) string {
	fullPath := fmt.Sprintf("/sys/block/%s/queue/scheduler", device)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "unknown"
	}

	scheduler := strings.TrimSpace(string(data))
	// 解析当前调度算法（方括号内的值）
	start := strings.Index(scheduler, "[")
	end := strings.Index(scheduler, "]")
	if start >= 0 && end > start {
		return scheduler[start+1 : end]
	}

	// 如果没有方括号，返回第一个
	fields := strings.Fields(scheduler)
	if len(fields) > 0 {
		return fields[0]
	}

	return "unknown"
}

// getDefaultBlockDevice 获取默认的块设备
func (kc *KernelChecker) getDefaultBlockDevice() string {
	// 尝试读取 /proc/mounts 获取根设备
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "/" {
			device := fields[0]
			// 提取设备名，如 /dev/sda2 -> sda
			if strings.HasPrefix(device, "/dev/") {
				devName := strings.TrimPrefix(device, "/dev/")
				// 处理分区，如 sda2 -> sda
				for i, c := range devName {
					if c >= '0' && c <= '9' {
						return devName[:i]
					}
				}
				return devName
			}
		}
	}

	// 尝试列出 /sys/block 获取第一个非回环设备
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "vd") {
			return name
		}
	}

	return ""
}

// GetKernelParams 获取内核参数（快捷函数）
func GetKernelParams() (*KernelParams, error) {
	checker := NewKernelChecker(false, nil)
	return checker.Collect()
}

// GetVMParams 获取 VM 参数
func GetVMParams() (*VMParams, error) {
	params, err := GetKernelParams()
	if err != nil {
		return nil, err
	}
	return &params.VM, nil
}

// GetIOParams 获取 IO 参数
func GetIOParams(device string) (*IOParams, error) {
	checker := NewKernelChecker(false, []string{device})
	params, err := checker.Collect()
	if err != nil {
		return nil, err
	}
	return &params.IO, nil
}

// SetVMParam 设置 VM 参数
func SetVMParam(param string, value int) error {
	path := fmt.Sprintf("/proc/sys/vm/%s", param)
	val := fmt.Sprintf("%d", value)
	return os.WriteFile(path, []byte(val), 0644)
}

// SetParam 设置任意内核参数
func SetParam(path string, value string) error {
	fullPath := fmt.Sprintf("/proc/sys/%s", path)
	return os.WriteFile(fullPath, []byte(value), 0644)
}

// CollectRemote 在远程主机收集内核参数
func CollectRemote(host string) (*KernelParams, error) {
	params := &KernelParams{
		Host: host,
	}

	// 收集 VM 参数
	params.VM = collectRemoteVMParams(host)

	// 收集 IO 参数
	params.IO = collectRemoteIOParams(host)

	// 收集网络参数
	params.Network = collectRemoteNetworkParams(host)

	return params, nil
}

// collectRemoteVMParams 在远程主机收集 VM 参数
func collectRemoteVMParams(host string) VMParams {
	vm := VMParams{}

	vm.DirtyRatio = readRemoteIntProcSys(host, "vm/dirty_ratio")
	vm.DirtyBackgroundRatio = readRemoteIntProcSys(host, "vm/dirty_background_ratio")
	vm.Swappiness = readRemoteIntProcSys(host, "vm/swappiness")
	vm.OvercommitMemory = readRemoteIntProcSys(host, "vm/overcommit_memory")
	vm.OvercommitRatio = readRemoteIntProcSys(host, "vm/overcommit_ratio")
	vm.MinFreeKbytes = readRemoteIntProcSys(host, "vm/min_free_kbytes")
	vm.DirtyExpireCs = readRemoteIntProcSys(host, "vm/dirty_expire_centisecs")
	vm.DirtyWritebackCs = readRemoteIntProcSys(host, "vm/dirty_writeback_centisecs")
	vm.VfsCachePressure = readRemoteIntProcSys(host, "vm/vfs_cache_pressure")
	vm.TransparentHugepage = readRemoteStringProcSys(host, "kernel/mm/transparent_hugepage/enabled")
	vm.NumaBalancing = readRemoteIntProcSys(host, "kernel/numa_balancing")

	return vm
}

// collectRemoteIOParams 在远程主机收集 IO 参数
func collectRemoteIOParams(host string) IOParams {
	io := IOParams{}

	// 获取默认设备
	device := getDefaultRemoteBlockDevice(host)
	if device == "" {
		return io
	}

	io.ReadAheadKB = readRemoteIntSysBlock(host, device, "queue/read_ahead_kb")
	io.MaxSectorsKB = readRemoteIntSysBlock(host, device, "queue/max_sectors_kb")
	io.NrRequests = readRemoteIntSysBlock(host, device, "queue/nr_requests")
	io.Scheduler = readRemoteSchedulerSysBlock(host, device)

	return io
}

// collectRemoteNetworkParams 在远程主机收集网络参数
func collectRemoteNetworkParams(host string) NetworkParams {
	net := NetworkParams{}

	net.Somaxconn = readRemoteIntProcSys(host, "net/core/somaxconn")
	net.NetdevMaxBacklog = readRemoteIntProcSys(host, "net/core/netdev_max_backlog")
	net.WmemDefault = readRemoteIntProcSys(host, "net/core/wmem_default")
	net.WmemMax = readRemoteIntProcSys(host, "net/core/wmem_max")
	net.RmemDefault = readRemoteIntProcSys(host, "net/core/rmem_default")
	net.RmemMax = readRemoteIntProcSys(host, "net/core/rmem_max")
	net.TcpMaxSynBacklog = readRemoteIntProcSys(host, "net/ipv4/tcp_max_syn_backlog")
	net.TcpMaxTwBuckets = readRemoteIntProcSys(host, "net/ipv4/tcp_max_tw_buckets")
	net.TcpFinTimeout = readRemoteIntProcSys(host, "net/ipv4/tcp_fin_timeout")
	net.TcpKeepaliveTime = readRemoteIntProcSys(host, "net/ipv4/tcp_keepalive_time")
	net.TcpTwReuse = readRemoteIntProcSys(host, "net/ipv4/tcp_tw_reuse")

	return net
}

// readRemoteIntProcSys 在远程主机读取整数参数
func readRemoteIntProcSys(host, path string) int {
	cmd := fmt.Sprintf("cat /proc/sys/%s 2>/dev/null", path)
	output, err := common.RunSSHCommand(host, cmd)
	if err != nil {
		return 0
	}
	val, _ := strconv.Atoi(strings.TrimSpace(output))
	return val
}

// readRemoteStringProcSys 在远程主机读取字符串参数
func readRemoteStringProcSys(host, path string) string {
	cmd := fmt.Sprintf("cat /proc/sys/%s 2>/dev/null", path)
	output, err := common.RunSSHCommand(host, cmd)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(output)
}

// readRemoteIntSysBlock 在远程主机读取 /sys/block/参数
func readRemoteIntSysBlock(host, device, param string) int {
	cmd := fmt.Sprintf("cat /sys/block/%s/%s 2>/dev/null", device, param)
	output, err := common.RunSSHCommand(host, cmd)
	if err != nil {
		return 0
	}
	val, _ := strconv.Atoi(strings.TrimSpace(output))
	return val
}

// readRemoteSchedulerSysBlock 在远程主机读取调度算法
func readRemoteSchedulerSysBlock(host, device string) string {
	cmd := fmt.Sprintf("cat /sys/block/%s/queue/scheduler 2>/dev/null", device)
	output, err := common.RunSSHCommand(host, cmd)
	if err != nil {
		return "unknown"
	}

	scheduler := strings.TrimSpace(output)
	start := strings.Index(scheduler, "[")
	end := strings.Index(scheduler, "]")
	if start >= 0 && end > start {
		return scheduler[start+1 : end]
	}

	fields := strings.Fields(scheduler)
	if len(fields) > 0 {
		return fields[0]
	}

	return "unknown"
}

// getDefaultRemoteBlockDevice 获取远程主机默认块设备
func getDefaultRemoteBlockDevice(host string) string {
	cmd := "df / | tail -1 | awk '{print $1}'"
	output, err := common.RunSSHCommand(host, cmd)
	if err != nil {
		return ""
	}

	device := strings.TrimSpace(output)
	if strings.HasPrefix(device, "/dev/") {
		devName := strings.TrimPrefix(device, "/dev/")
		// 处理分区
		for i, c := range devName {
			if c >= '0' && c <= '9' {
				return devName[:i]
			}
		}
		return devName
	}

	return ""
}

// PrintKernelParams 打印内核参数（用于调试）
func PrintKernelParams(params *KernelParams) {
	fmt.Printf("内核参数 (%s):\n", params.Host)
	fmt.Printf("\nVM 参数:\n")
	fmt.Printf("  dirty_ratio: %d\n", params.VM.DirtyRatio)
	fmt.Printf("  dirty_background_ratio: %d\n", params.VM.DirtyBackgroundRatio)
	fmt.Printf("  swappiness: %d\n", params.VM.Swappiness)
	fmt.Printf("  overcommit_memory: %d\n", params.VM.OvercommitMemory)
	fmt.Printf("  min_free_kbytes: %d\n", params.VM.MinFreeKbytes)
	fmt.Printf("  transparent_hugepage: %s\n", params.VM.TransparentHugepage)
	fmt.Printf("  numa_balancing: %d\n", params.VM.NumaBalancing)

	fmt.Printf("\nIO 参数:\n")
	fmt.Printf("  read_ahead_kb: %d\n", params.IO.ReadAheadKB)
	fmt.Printf("  max_sectors_kb: %d\n", params.IO.MaxSectorsKB)
	fmt.Printf("  nr_requests: %d\n", params.IO.NrRequests)
	fmt.Printf("  scheduler: %s\n", params.IO.Scheduler)

	fmt.Printf("\n网络参数:\n")
	fmt.Printf("  somaxconn: %d\n", params.Network.Somaxconn)
	fmt.Printf("  netdev_max_backlog: %d\n", params.Network.NetdevMaxBacklog)
	fmt.Printf("  tcp_max_syn_backlog: %d\n", params.Network.TcpMaxSynBacklog)
	fmt.Printf("  tcp_max_tw_buckets: %d\n", params.Network.TcpMaxTwBuckets)
	fmt.Printf("  tcp_tw_reuse: %d\n", params.Network.TcpTwReuse)
}

// CheckVMParams 检查 VM 参数是否合理
func CheckVMParams(vm *VMParams) []string {
	var warnings []string

	// 检查 dirty_ratio
	if vm.DirtyRatio > 40 {
		warnings = append(warnings, fmt.Sprintf("dirty_ratio=%d 过高，可能导致 IO 突发", vm.DirtyRatio))
	}

	// 检查 swappiness
	if vm.Swappiness > 60 {
		warnings = append(warnings, fmt.Sprintf("swappiness=%d 过高，可能导致频繁 swap", vm.Swappiness))
	}

	// 检查 transparent_hugepage
	if strings.Contains(vm.TransparentHugepage, "[always]") {
		warnings = append(warnings, "transparent_hugepage=always 可能影响数据库性能，建议设置为 madvise 或 never")
	}

	return warnings
}
