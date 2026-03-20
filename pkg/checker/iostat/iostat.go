// Package iostat 提供磁盘 IO 统计功能
// 通过解析 /proc/diskstats 文件获取实时 IO 指标
package iostat

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dbcheckperf/pkg/checker/common"
)

// IOStats IO 统计信息
type IOStats struct {
	// Host 主机名
	Host string
	// Device 设备名称
	Device string
	// ReadPerSec 每秒读次数 (r/s)
	ReadPerSec float64
	// WritePerSec 每秒写次数 (w/s)
	WritePerSec float64
	// ReadKBPerSec 每秒读 KB 数 (rkB/s)
	ReadKBPerSec float64
	// WriteKBPerSec 每秒写 KB 数 (wkB/s)
	WriteKBPerSec float64
	// Await 平均等待时间 (ms)
	Await float64
	// ReadAwait 读平均等待时间 (ms)
	ReadAwait float64
	// WriteAwait 写平均等待时间 (ms)
	WriteAwait float64
	// Utilization 磁盘利用率 (%util)
	Utilization float64
	// AvgQueueLength 平均队列长度 (avgqu-sz)
	AvgQueueLength float64
	// Svctm 平均服务时间 (ms)
	Svctm float64
	// ReadMerged 合并的读次数
	ReadMerged float64
	// WriteMerged 合并的写次数
	WriteMerged float64
}

// DiskStats 磁盘统计原始数据
type DiskStats struct {
	// 读操作次数
	Reads uint64
	// 读操作耗时 (ms)
	ReadTicks uint64
	// 写操作次数
	Writes uint64
	// 写操作耗时 (ms)
	WriteTicks uint64
	// IO 在队列中的总时间 (ms)
	IoTicks uint64
	// IO 在队列中花费的加权时间 (ms)
	WeightedIoTicks uint64
	// 读扇区数
	SectorsRead uint64
	// 写扇区数
	SectorsWrite uint64
}

// IOStatChecker IO 统计检查器
type IOStatChecker struct {
	// Verbose 是否显示详细输出
	Verbose bool
	// Interval 采样间隔 (秒)
	Interval time.Duration
	// Devices 监控的设备列表，为空则监控所有
	Devices []string
}

// NewIOStatChecker 创建新的 IO 统计检查器
func NewIOStatChecker(verbose bool, interval time.Duration, devices []string) *IOStatChecker {
	if interval <= 0 {
		interval = time.Second // 默认 1 秒间隔
	}
	return &IOStatChecker{
		Verbose:  verbose,
		Interval: interval,
		Devices:  devices,
	}
}

// Collect 收集 IO 统计信息
func (ic *IOStatChecker) Collect() ([]IOStats, error) {
	// 获取两次采样数据计算差值
	stats1, err := ic.readProcDiskstats()
	if err != nil {
		return nil, err
	}

	// 等待采样间隔
	time.Sleep(ic.Interval)

	stats2, err := ic.readProcDiskstats()
	if err != nil {
		return nil, err
	}

	// 计算差值并生成 IO 统计
	return ic.calculateStats(stats1, stats2), nil
}

// readProcDiskstats 读取 /proc/diskstats
func (ic *IOStatChecker) readProcDiskstats() (map[string]*DiskStats, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := make(map[string]*DiskStats)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		device := fields[2]

		// 如果指定了设备列表，只监控指定设备
		if len(ic.Devices) > 0 && !contains(ic.Devices, device) {
			continue
		}

		// 跳过分区，只监控磁盘设备
		if !ic.isWholeDisk(device) {
			continue
		}

		ds := &DiskStats{}
		ds.Reads, _ = strconv.ParseUint(fields[3], 10, 64)
		ds.ReadTicks, _ = strconv.ParseUint(fields[6], 10, 64)
		ds.Writes, _ = strconv.ParseUint(fields[7], 10, 64)
		ds.WriteTicks, _ = strconv.ParseUint(fields[10], 10, 64)
		ds.IoTicks, _ = strconv.ParseUint(fields[12], 10, 64)
		ds.WeightedIoTicks, _ = strconv.ParseUint(fields[13], 10, 64)
		ds.SectorsRead, _ = strconv.ParseUint(fields[5], 10, 64)
		ds.SectorsWrite, _ = strconv.ParseUint(fields[9], 10, 64)

		stats[device] = ds
	}

	return stats, scanner.Err()
}

// calculateStats 计算 IO 统计信息
func (ic *IOStatChecker) calculateStats(stats1, stats2 map[string]*DiskStats) []IOStats {
	var results []IOStats

	interval := ic.Interval.Seconds()
	if interval <= 0 {
		interval = 1.0
	}

	for device, s2 := range stats2 {
		s1, ok := stats1[device]
		if !ok {
			continue
		}

		stats := IOStats{
			Host:   common.GetHostname(),
			Device: device,
		}

		// 计算差值
		readDelta := float64(s2.Reads - s1.Reads)
		writeDelta := float64(s2.Writes - s1.Writes)
		readSectorsDelta := float64(s2.SectorsRead - s1.SectorsRead)
		writeSectorsDelta := float64(s2.SectorsWrite - s1.SectorsWrite)
		readTicksDelta := float64(s2.ReadTicks - s1.ReadTicks)
		writeTicksDelta := float64(s2.WriteTicks - s1.WriteTicks)
		ioTicksDelta := float64(s2.IoTicks - s1.IoTicks)
		weightedIoTicksDelta := float64(s2.WeightedIoTicks - s1.WeightedIoTicks)

		// 计算速率
		stats.ReadPerSec = readDelta / interval
		stats.WritePerSec = writeDelta / interval
		stats.ReadKBPerSec = (readSectorsDelta * 512) / interval / 1024 // 假设扇区大小为 512 字节
		stats.WriteKBPerSec = (writeSectorsDelta * 512) / interval / 1024

		// 计算延迟
		if readDelta > 0 {
			stats.ReadAwait = readTicksDelta / readDelta
		}
		if writeDelta > 0 {
			stats.WriteAwait = writeTicksDelta / writeDelta
		}
		if readDelta+writeDelta > 0 {
			stats.Await = (readTicksDelta + writeTicksDelta) / (readDelta + writeDelta)
		}

		// 计算利用率
		stats.Utilization = (ioTicksDelta / interval / 1000) * 100 // 转换为百分比
		if stats.Utilization > 100 {
			stats.Utilization = 100
		}

		// 计算平均队列长度
		stats.AvgQueueLength = weightedIoTicksDelta / interval / 1000

		// 计算服务时间
		totalIO := readDelta + writeDelta
		if totalIO > 0 {
			stats.Svctm = (readTicksDelta + writeTicksDelta) / totalIO
		}

		results = append(results, stats)
	}

	return results
}

// isWholeDisk 判断是否为整盘设备
func (ic *IOStatChecker) isWholeDisk(device string) bool {
	// NVMe 设备
	if strings.HasPrefix(device, "nvme") {
		// nvme0n1 是整盘，nvme0n1p1 是分区
		return !strings.Contains(device, "p")
	}

	// SCSI/SATA 设备
	if strings.HasPrefix(device, "sd") {
		// sda 是整盘，sda1 是分区
		for _, c := range device {
			if c >= '0' && c <= '9' {
				return false
			}
		}
		return true
	}

	// VIRTIO 设备
	if strings.HasPrefix(device, "vd") {
		// vda 是整盘，vda1 是分区
		for _, c := range device {
			if c >= '0' && c <= '9' {
				return false
			}
		}
		return true
	}

	// 其他设备，假设是整盘
	return true
}

// CollectWithIostat 使用 iostat 命令收集 IO 统计
func (ic *IOStatChecker) CollectWithIostat() ([]IOStats, error) {
	// 检查 iostat 是否可用
	if _, err := exec.LookPath("iostat"); err != nil {
		return nil, fmt.Errorf("iostat 不可用")
	}

	// 运行 iostat 命令
	args := []string{
		"-x", // 扩展统计
		"-k", // KB 为单位
		fmt.Sprintf("%.0f", ic.Interval.Seconds()),
		"2", // 采样 2 次
	}

	if len(ic.Devices) > 0 {
		args = append(args, ic.Devices...)
	}

	cmd := exec.Command("iostat", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return ic.parseIostatOutput(string(output))
}

// parseIostatOutput 解析 iostat 输出
func (ic *IOStatChecker) parseIostatOutput(output string) ([]IOStats, error) {
	var results []IOStats
	lines := strings.Split(output, "\n")

	var stats IOStats

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 跳过标题行
		if strings.HasPrefix(line, "Linux") ||
			strings.HasPrefix(line, "Device:") ||
			strings.HasPrefix(line, "avg-cpu:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 20 {
			continue
		}

		// 解析设备行
		device := fields[0]
		if device == "" || device == "Device" {
			continue
		}

		stats = IOStats{
			Host:   common.GetHostname(),
			Device: device,
		}

		// 解析扩展统计字段
		// Device: rrqm/s wrqm/s r/s w/s rkB/s wkB/s avgrq-sz avgqu-sz await r_await w_await svctm %util
		stats.ReadPerSec, _ = strconv.ParseFloat(fields[3], 64)
		stats.WritePerSec, _ = strconv.ParseFloat(fields[4], 64)
		stats.ReadKBPerSec, _ = strconv.ParseFloat(fields[5], 64)
		stats.WriteKBPerSec, _ = strconv.ParseFloat(fields[6], 64)
		stats.AvgQueueLength, _ = strconv.ParseFloat(fields[8], 64)
		stats.Await, _ = strconv.ParseFloat(fields[9], 64)
		stats.ReadAwait, _ = strconv.ParseFloat(fields[10], 64)
		stats.WriteAwait, _ = strconv.ParseFloat(fields[11], 64)
		stats.Svctm, _ = strconv.ParseFloat(fields[12], 64)
		stats.Utilization, _ = strconv.ParseFloat(fields[13], 64)

		results = append(results, stats)
	}

	return results, nil
}

// CollectRemote 在远程主机收集 IO 统计
func (ic *IOStatChecker) CollectRemote(host string) ([]IOStats, error) {
	// 尝试使用 iostat
	if stats, err := ic.collectRemoteWithIostat(host); err == nil {
		return stats, nil
	}

	// 回退到解析 /proc/diskstats
	return ic.collectRemoteProcDiskstats(host)
}

// collectRemoteWithIostat 在远程主机使用 iostat
func (ic *IOStatChecker) collectRemoteWithIostat(host string) ([]IOStats, error) {
	cmd := fmt.Sprintf("iostat -xk %.0f 2", ic.Interval.Seconds())
	output, err := common.RunSSHCommand(host, cmd)
	if err != nil {
		return nil, err
	}

	return ic.parseIostatOutput(output)
}

// collectRemoteProcDiskstats 在远程主机解析 /proc/diskstats
func (ic *IOStatChecker) collectRemoteProcDiskstats(host string) ([]IOStats, error) {
	// 第一次采样
	cmd1 := "cat /proc/diskstats"
	output1, err := common.RunSSHCommand(host, cmd1)
	if err != nil {
		return nil, err
	}

	stats1 := ic.parseProcDiskstatsOutput(output1)

	// 等待采样间隔
	time.Sleep(ic.Interval)

	// 第二次采样
	output2, err := common.RunSSHCommand(host, cmd1)
	if err != nil {
		return nil, err
	}

	stats2 := ic.parseProcDiskstatsOutput(output2)

	return ic.calculateStats(stats1, stats2), nil
}

// parseProcDiskstatsOutput 解析 /proc/diskstats 输出
func (ic *IOStatChecker) parseProcDiskstatsOutput(output string) map[string]*DiskStats {
	stats := make(map[string]*DiskStats)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		device := fields[2]

		if len(ic.Devices) > 0 && !contains(ic.Devices, device) {
			continue
		}

		if !ic.isWholeDisk(device) {
			continue
		}

		ds := &DiskStats{}
		ds.Reads, _ = strconv.ParseUint(fields[3], 10, 64)
		ds.ReadTicks, _ = strconv.ParseUint(fields[6], 10, 64)
		ds.Writes, _ = strconv.ParseUint(fields[7], 10, 64)
		ds.WriteTicks, _ = strconv.ParseUint(fields[10], 10, 64)
		ds.IoTicks, _ = strconv.ParseUint(fields[12], 10, 64)
		ds.WeightedIoTicks, _ = strconv.ParseUint(fields[13], 10, 64)
		ds.SectorsRead, _ = strconv.ParseUint(fields[5], 10, 64)
		ds.SectorsWrite, _ = strconv.ParseUint(fields[9], 10, 64)

		stats[device] = ds
	}

	return stats
}

// contains 检查切片是否包含指定值
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// GetSnapshot 获取单次 IO 统计快照
func GetSnapshot() ([]IOStats, error) {
	checker := NewIOStatChecker(false, time.Second, nil)
	return checker.Collect()
}

// GetSnapshotForDevice 获取指定设备的 IO 统计快照
func GetSnapshotForDevice(device string) (*IOStats, error) {
	checker := NewIOStatChecker(false, time.Second, []string{device})
	stats, err := checker.Collect()
	if err != nil {
		return nil, err
	}

	if len(stats) > 0 {
		return &stats[0], nil
	}

	return nil, fmt.Errorf("未找到设备 %s 的统计信息", device)
}
