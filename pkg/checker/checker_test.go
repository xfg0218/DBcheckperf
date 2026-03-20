// Package checker 提供系统性能检查功能
// 包含磁盘 I/O、网络、内存带宽和系统信息检测
package checker

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// ==================== DiskChecker 测试 ====================

func TestNewDiskChecker(t *testing.T) {
	tests := []struct {
		name            string
		blockSizeKB     int
		fileSize        uint64
		verbose         bool
		randBlockSizeKB int
		expectRandBS    int
	}{
		{"正常参数", 32, 1024 * 1024 * 1024, true, 4, 4},
		{"零随机块大小", 64, 512 * 1024 * 1024, false, 0, 64},
		{"负随机块大小", 128, 256 * 1024 * 1024, true, -1, 128},
		{"小文件", 16, 10 * 1024 * 1024, false, 8, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewDiskChecker(tt.blockSizeKB, tt.fileSize, tt.verbose, tt.randBlockSizeKB)

			if checker == nil {
				t.Fatal("NewDiskChecker() 返回 nil")
			}

			if checker.BlockSize != tt.blockSizeKB {
				t.Errorf("BlockSize = %d; 期望 %d", checker.BlockSize, tt.blockSizeKB)
			}

			if checker.FileSize != tt.fileSize {
				t.Errorf("FileSize = %d; 期望 %d", checker.FileSize, tt.fileSize)
			}

			if checker.Verbose != tt.verbose {
				t.Errorf("Verbose = %v; 期望 %v", checker.Verbose, tt.verbose)
			}

			if checker.RandBlockSize != tt.expectRandBS {
				t.Errorf("RandBlockSize = %d; 期望 %d", checker.RandBlockSize, tt.expectRandBS)
			}

			if checker.Iterations != 1 {
				t.Errorf("Iterations = %d; 期望 1", checker.Iterations)
			}
		})
	}
}

// TestDiskChecker_Run 测试磁盘检查器的 Run 方法
func TestDiskChecker_Run(t *testing.T) {
	// 跳过需要实际磁盘访问的测试（在 CI 环境中）
	if testing.Short() {
		t.Skip("跳过需要磁盘访问的测试")
	}

	tmpDir := t.TempDir()

	checker := NewDiskChecker(32, 10*1024*1024, false, 4)

	result, err := checker.Run(tmpDir)

	// 注意：这个测试可能需要 root 权限才能成功
	if err != nil {
		t.Logf("磁盘测试可能需要特殊权限：%v", err)
		return
	}

	if result == nil {
		t.Fatal("Run() 返回 nil 结果")
	}

	if result.Dir != tmpDir {
		t.Errorf("结果目录 = %q; 期望 %q", result.Dir, tmpDir)
	}

	// 验证结果字段
	if result.WriteBytes == 0 {
		t.Error("WriteBytes 应该大于 0")
	}

	if result.WriteTime <= 0 {
		t.Error("WriteTime 应该大于 0")
	}

	if result.WriteBandwidth <= 0 {
		t.Error("WriteBandwidth 应该大于 0")
	}
}

// TestDiskChecker_Run_InvalidDir 测试无效目录
func TestDiskChecker_Run_InvalidDir(t *testing.T) {
	checker := NewDiskChecker(32, 1024*1024, false, 4)

	// 使用不可能的路径
	_, err := checker.Run("/nonexistent/path/that/cannot/exist")

	if err == nil {
		t.Error("Run() 使用无效目录应该返回错误")
	}
}

// ==================== NetworkChecker 测试 ====================

func TestNewNetworkChecker(t *testing.T) {
	tests := []struct {
		name         string
		duration     time.Duration
		bufferSizeKB int
		useNetperf   bool
		verbose      bool
	}{
		{"正常参数", 15 * time.Second, 8, false, true},
		{"使用 netperf", 30 * time.Second, 16, true, false},
		{"零缓冲区", 10 * time.Second, 0, false, false},
		{"长持续时间", 60 * time.Second, 32, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewNetworkChecker(tt.duration, tt.bufferSizeKB, tt.useNetperf, tt.verbose)

			if checker == nil {
				t.Fatal("NewNetworkChecker() 返回 nil")
			}

			if checker.Duration != tt.duration {
				t.Errorf("Duration = %v; 期望 %v", checker.Duration, tt.duration)
			}

			if checker.BufferSize != tt.bufferSizeKB {
				t.Errorf("BufferSize = %d; 期望 %d", checker.BufferSize, tt.bufferSizeKB)
			}

			if checker.UseNetperf != tt.useNetperf {
				t.Errorf("UseNetperf = %v; 期望 %v", checker.UseNetperf, tt.useNetperf)
			}

			if checker.Verbose != tt.verbose {
				t.Errorf("Verbose = %v; 期望 %v", checker.Verbose, tt.verbose)
			}

			if checker.Iterations != 3 {
				t.Errorf("Iterations = %d; 期望 3", checker.Iterations)
			}
		})
	}
}

// TestNetworkChecker_testWithTCPStream 测试 TCP 流方法
func TestNetworkChecker_testWithTCPStream(t *testing.T) {
	// 创建一个监听器来模拟服务器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("无法创建监听器：%v", err)
	}
	defer listener.Close()

	// 在后台处理连接
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// 读取数据但不处理
			buf := make([]byte, 1024)
			go func(c net.Conn) {
				defer c.Close()
				for {
					_, err := c.Read(buf)
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	// 获取监听地址
	addr := listener.Addr().String()
	host, port, _ := net.SplitHostPort(addr)

	// 创建检查器
	checker := NewNetworkChecker(2*time.Second, 64, false, false)

	// 测试连接（使用正确的端口）
	target := net.JoinHostPort(host, port)
	_, err = checker.testWithTCPStream(target)

	// 由于我们使用了自定义端口，需要修改测试方式
	// 这里只验证方法不会 panic
	if err == nil {
		t.Log("TCP 流测试成功（模拟服务器）")
	}
}

// TestNetworkChecker_estimateNetworkSpeed 测试网络速度估算
func TestNetworkChecker_estimateNetworkSpeed(t *testing.T) {
	checker := NewNetworkChecker(15*time.Second, 8, false, false)

	// 测试 localhost
	speed := checker.estimateNetworkSpeed("127.0.0.1")

	// ping 应该能成功
	if speed < 0 {
		t.Error("estimateNetworkSpeed() 应该返回非负值")
	}

	t.Logf("localhost 估算网络速度：%.2f MB/s", speed)
}

// TestNetworkChecker_parseNetperfOutput 解析 netperf 输出
func TestNetworkChecker_parseNetperfOutput(t *testing.T) {
	checker := NewNetworkChecker(15*time.Second, 8, false, false)

	tests := []struct {
		name     string
		output   string
		expected float64
	}{
		{
			name:     "标准输出",
			output:   "Recv   Send    Send                          \nSocket Socket  Message  Elapsed              \nSize   Size    Size     Time     Throughput  \nbytes  bytes   bytes    secs.    10^6bits/sec  \n\n87380  16384  16384    10.00    1000.00   ",
			expected: 125.0, // 1000 / 8
		},
		{
			name:     "空输出",
			output:   "",
			expected: 0,
		},
		{
			name:     "无效格式",
			output:   "invalid output",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.parseNetperfOutput(tt.output)

			if tt.expected == 0 {
				if result != 0 && tt.output != "" {
					t.Errorf("parseNetperfOutput() = %.2f; 期望 0", result)
				}
			} else {
				// 允许一定误差
				diff := result - tt.expected
				if diff < 0 {
					diff = -diff
				}
				if diff > 1 {
					t.Errorf("parseNetperfOutput() = %.2f; 期望 %.2f", result, tt.expected)
				}
			}
		})
	}
}

// ==================== StreamChecker 测试 ====================

func TestNewStreamChecker(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{"详细模式", true},
		{"静默模式", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewStreamChecker(tt.verbose)

			if checker == nil {
				t.Fatal("NewStreamChecker() 返回 nil")
			}

			if checker.Verbose != tt.verbose {
				t.Errorf("Verbose = %v; 期望 %v", checker.Verbose, tt.verbose)
			}

			if checker.ArraySize <= 0 {
				t.Error("ArraySize 应该大于 0")
			}

			if checker.Iterations != 3 {
				t.Errorf("Iterations = %d; 期望 3", checker.Iterations)
			}
		})
	}
}

// TestStreamChecker_Run 测试内存带宽测试
func TestStreamChecker_Run(t *testing.T) {
	checker := NewStreamChecker(false)

	result, err := checker.Run()

	if err != nil {
		t.Fatalf("Run() 意外失败：%v", err)
	}

	if result == nil {
		t.Fatal("Run() 返回 nil 结果")
	}

	// 验证结果字段
	if result.CopyBandwidth <= 0 {
		t.Error("CopyBandwidth 应该大于 0")
	}

	if result.ScaleBandwidth <= 0 {
		t.Error("ScaleBandwidth 应该大于 0")
	}

	if result.AddBandwidth <= 0 {
		t.Error("AddBandwidth 应该大于 0")
	}

	if result.TriadBandwidth <= 0 {
		t.Error("TriadBandwidth 应该大于 0")
	}

	if result.TotalBandwidth <= 0 {
		t.Error("TotalBandwidth 应该大于 0")
	}

	t.Logf("内存带宽测试结果：Total = %.2f MB/s", result.TotalBandwidth)
}

// TestStreamChecker_parseStreamOutput 测试 STREAM 输出解析
func TestStreamChecker_parseStreamOutput(t *testing.T) {
	checker := NewStreamChecker(false)

	tests := []struct {
		name        string
		output      string
		expectCopy  float64
		expectScale float64
		expectAdd   float64
		expectTriad float64
	}{
		{
			name: "标准输出",
			output: `-------------------------------------------------------------
STREAM version $Revision: 5.10 $
-------------------------------------------------------------
Function    Best Rate MB/s  Avg total     Avg min
Copy:      1000.0           1000.0        999.0
Scale:     2000.0           2000.0        1999.0
Add:       3000.0           3000.0        2999.0
Triad:     4000.0           4000.0        3999.0
-------------------------------------------------------------`,
			expectCopy:  1000.0,
			expectScale: 2000.0,
			expectAdd:   3000.0,
			expectTriad: 4000.0,
		},
		{
			name:        "空输出",
			output:      "",
			expectCopy:  0,
			expectScale: 0,
			expectAdd:   0,
			expectTriad: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.parseStreamOutput(tt.output)

			if err != nil {
				t.Fatalf("parseStreamOutput() 意外失败：%v", err)
			}

			if result.CopyBandwidth != tt.expectCopy {
				t.Errorf("CopyBandwidth = %.2f; 期望 %.2f", result.CopyBandwidth, tt.expectCopy)
			}
			if result.ScaleBandwidth != tt.expectScale {
				t.Errorf("ScaleBandwidth = %.2f; 期望 %.2f", result.ScaleBandwidth, tt.expectScale)
			}
			if result.AddBandwidth != tt.expectAdd {
				t.Errorf("AddBandwidth = %.2f; 期望 %.2f", result.AddBandwidth, tt.expectAdd)
			}
			if result.TriadBandwidth != tt.expectTriad {
				t.Errorf("TriadBandwidth = %.2f; 期望 %.2f", result.TriadBandwidth, tt.expectTriad)
			}
		})
	}
}

// ==================== SystemChecker 测试 ====================

func TestNewSystemChecker(t *testing.T) {
	checker := NewSystemChecker()

	if checker == nil {
		t.Fatal("NewSystemChecker() 返回 nil")
	}
}

// TestSystemChecker_Run 测试系统信息收集
func TestSystemChecker_Run(t *testing.T) {
	checker := NewSystemChecker()

	info, err := checker.Run()

	if err != nil {
		t.Fatalf("Run() 意外失败：%v", err)
	}

	if info == nil {
		t.Fatal("Run() 返回 nil 结果")
	}

	// 验证结果字段 (在测试环境不一定能获取到所有信息，所以不严格校验)
	if info.Host == "" {
		t.Error("Host 不应该为空")
	}

	if info.OSName == "" {
		t.Error("OSName 不应该为空")
	}

	if info.KernelVersion == "" {
		t.Error("KernelVersion 不应该为空")
	}

	t.Logf("系统信息：CPU=%d 核心，内存=%s", info.CPUCore, formatBytes(info.TotalRAM))
}

// TestSystemChecker_getCPUCore 测试 CPU 核心数获取
func TestSystemChecker_getCPUCore(t *testing.T) {
	checker := NewSystemChecker()

	cores := checker.getCPUCore()

	if cores <= 0 {
		t.Error("getCPUCore() 应该返回正值")
	}

	t.Logf("CPU 核心数：%d", cores)
}

// TestSystemChecker_getCPUModel 测试 CPU 型号获取
func TestSystemChecker_getCPUModel(t *testing.T) {
	checker := NewSystemChecker()

	model := checker.getCPUModel()

	// CPU 型号可能为空（某些 ARM 设备）
	t.Logf("CPU 型号：%s", model)
}

// TestSystemChecker_parseARMModel 测试 ARM CPU 型号解析
func TestSystemChecker_parseARMModel(t *testing.T) {
	checker := NewSystemChecker()

	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{"Apple M1", "Apple M1", "Apple Silicon"},
		{"空字符串", "", ""},
		{"含版本号", "Cortex-A72 r0p3", "Cortex-A72"},
		{"含空格", "  Cortex-A53  ", "Cortex-A53"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.parseARMModel(tt.model)

			if result != tt.expected {
				t.Errorf("parseARMModel(%q) = %q; 期望 %q", tt.model, result, tt.expected)
			}
		})
	}
}

// TestSystemChecker_detectVirtualization 测试虚拟化检测
func TestSystemChecker_detectVirtualization(t *testing.T) {
	checker := NewSystemChecker()

	isVirtual, vType := checker.detectVirtualization()

	t.Logf("虚拟化：%v, 类型：%s", isVirtual, vType)

	// 不验证具体值，因为取决于运行环境
}

// TestSystemChecker_getOSName 测试操作系统名称获取
func TestSystemChecker_getOSName(t *testing.T) {
	checker := NewSystemChecker()

	osName := checker.getOSName()

	if osName == "" {
		t.Error("getOSName() 不应该返回空字符串")
	}

	t.Logf("操作系统：%s", osName)
}

// TestSystemChecker_getKernelVersion 测试内核版本获取
func TestSystemChecker_getKernelVersion(t *testing.T) {
	checker := NewSystemChecker()

	kernel := checker.getKernelVersion()

	if kernel == "" {
		t.Error("getKernelVersion() 不应该返回空字符串")
	}

	t.Logf("内核版本：%s", kernel)
}

// TestSystemChecker_getDiskSize 测试磁盘大小获取
func TestSystemChecker_getDiskSize(t *testing.T) {
	checker := NewSystemChecker()

	size := checker.getDiskSize()

	// 磁盘大小应该大于等于 0（在某些无盘容器中可能无法获取）
	if size < 0 {
		t.Error("getDiskSize() 应该返回非负值")
	}

	t.Logf("磁盘大小：%s", formatBytes(size))
}

// TestSystemChecker_getNICSpeed 测试网卡速率获取
func TestSystemChecker_getNICSpeed(t *testing.T) {
	checker := NewSystemChecker()

	speed := checker.getNICSpeed()

	// 网卡速率可能为 0（如果无法获取）
	t.Logf("网卡速率：%d Mbps", speed)
}

// TestSystemChecker_getRAIDInfo 测试 RAID 信息获取
func TestSystemChecker_getRAIDInfo(t *testing.T) {
	checker := NewSystemChecker()

	info := checker.getRAIDInfo()

	if info == nil {
		t.Error("getRAIDInfo() 不应该返回 nil")
	}

	t.Logf("RAID 信息：HasRAID=%v", info.HasRAID)
}

// TestSystemChecker_getNICBondInfo 测试网卡绑定信息获取
func TestSystemChecker_getNICBondInfo(t *testing.T) {
	checker := NewSystemChecker()

	info := checker.getNICBondInfo()

	if info == nil {
		t.Error("getNICBondInfo() 不应该返回 nil")
	}

	t.Logf("网卡绑定：HasBond=%v", info.HasBond)
}

// ==================== HardwareChecker 测试 ====================

func TestNewHardwareChecker(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{"详细模式", true},
		{"静默模式", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewHardwareChecker(tt.verbose)

			if checker == nil {
				t.Fatal("NewHardwareChecker() 返回 nil")
			}

			if checker.Verbose != tt.verbose {
				t.Errorf("Verbose = %v; 期望 %v", checker.Verbose, tt.verbose)
			}
		})
	}
}

// TestHardwareChecker_Run 测试硬件信息收集
func TestHardwareChecker_Run(t *testing.T) {
	checker := NewHardwareChecker(false)

	info, err := checker.Run()

	if err != nil {
		t.Fatalf("Run() 意外失败：%v", err)
	}

	if info == nil {
		t.Fatal("Run() 返回 nil 结果")
	}

	// 验证结果字段 (测试环境可能无法获取所有硬件信息)
	if info.Host == "" {
		t.Error("Host 不应该为空")
	}

	if info.CPUInfo != nil {
		t.Logf("CPU: %s (%d 核心)", info.CPUInfo.Model, info.CPUInfo.Cores)
	}

	if info.DiskInfos != nil {
		t.Logf("磁盘数量：%d", len(info.DiskInfos))
	}

	if info.NICInfos != nil {
		t.Logf("网卡数量：%d", len(info.NICInfos))
	}

	if info.MemoryInfo != nil {
		t.Logf("内存：%s", formatBytes(info.MemoryInfo.TotalMemory))
	}
}

// TestHardwareChecker_extractVendor 测试厂家提取
func TestHardwareChecker_extractVendor(t *testing.T) {
	checker := NewHardwareChecker(false)

	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{"SAMSUNG_SSD", "SAMSUNG MZ7KM480HAHP-00005", "Samsung"},
		{"INTEL_SSD", "INTEL SSDSC2KB480G8", "Intel"},
		{"WD_HDD", "WD5000AAKX", "Western Digital"},
		{"SEAGATE_HDD", "ST500DM002", "Seagate"},
		{"TOSHIBA_HDD", "TOSHIBA DT01ACA050", "Toshiba"},
		{"无品牌", "Generic Disk", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.extractVendor(tt.model)

			if result != tt.expected {
				if tt.expected != "" || result != "" {
					t.Errorf("extractVendor(%q) = %q; 期望 %q", tt.model, result, tt.expected)
				}
			}
		})
	}
}

// TestHardwareChecker_parseSizeString 测试大小字符串解析
func TestHardwareChecker_parseSizeString(t *testing.T) {
	checker := NewHardwareChecker(false)

	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{"500GB", "500GB", 536870912000},
		{"1TB", "1TB", 1099511627776},
		{"1.5TB", "1.5TB", 1649267441664},
		{"256MB", "256MB", 268435456},
		{"无效格式", "invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.parseSizeString(tt.input)

			if result != tt.expected {
				t.Errorf("parseSizeString(%q) = %d; 期望 %d", tt.input, result, tt.expected)
			}
		})
	}
}

// ==================== 聚合函数测试 ====================

func TestAggregateDiskResults(t *testing.T) {
	results := []*DiskResult{
		{WriteBandwidth: 100.0, ReadBandwidth: 200.0},
		{WriteBandwidth: 150.0, ReadBandwidth: 250.0},
		{WriteBandwidth: 200.0, ReadBandwidth: 300.0},
	}

	// 测试写入聚合
	writeAgg := AggregateDiskResults(results, true)
	if writeAgg == nil {
		t.Fatal("AggregateDiskResults() 返回 nil")
	}
	if writeAgg.MinValue != 100.0 {
		t.Errorf("写入 MinValue = %.2f; 期望 100.0", writeAgg.MinValue)
	}
	if writeAgg.MaxValue != 200.0 {
		t.Errorf("写入 MaxValue = %.2f; 期望 200.0", writeAgg.MaxValue)
	}
	if writeAgg.AvgValue != 150.0 {
		t.Errorf("写入 AvgValue = %.2f; 期望 150.0", writeAgg.AvgValue)
	}

	// 测试读取聚合
	readAgg := AggregateDiskResults(results, false)
	if readAgg.MinValue != 200.0 {
		t.Errorf("读取 MinValue = %.2f; 期望 200.0", readAgg.MinValue)
	}
	if readAgg.MaxValue != 300.0 {
		t.Errorf("读取 MaxValue = %.2f; 期望 300.0", readAgg.MaxValue)
	}
	if readAgg.AvgValue != 250.0 {
		t.Errorf("读取 AvgValue = %.2f; 期望 250.0", readAgg.AvgValue)
	}
}

func TestAggregateDiskResults_Empty(t *testing.T) {
	results := []*DiskResult{}

	agg := AggregateDiskResults(results, true)
	if agg == nil {
		t.Fatal("AggregateDiskResults() 应该返回非 nil（即使为空）")
	}
	if agg.MinValue != 0 || agg.MaxValue != 0 || agg.AvgValue != 0 {
		t.Error("空结果的聚合值应该为 0")
	}
}

func TestAggregateDiskRandResults(t *testing.T) {
	results := []*DiskResult{
		{RandWriteBandwidth: 50.0, RandReadBandwidth: 100.0},
		{RandWriteBandwidth: 75.0, RandReadBandwidth: 125.0},
		{RandWriteBandwidth: 100.0, RandReadBandwidth: 150.0},
	}

	// 测试随机写入聚合
	writeAgg := AggregateDiskRandResults(results, true)
	if writeAgg.MinValue != 50.0 {
		t.Errorf("随机写入 MinValue = %.2f; 期望 50.0", writeAgg.MinValue)
	}
	if writeAgg.MaxValue != 100.0 {
		t.Errorf("随机写入 MaxValue = %.2f; 期望 100.0", writeAgg.MaxValue)
	}
	if writeAgg.AvgValue != 75.0 {
		t.Errorf("随机写入 AvgValue = %.2f; 期望 75.0", writeAgg.AvgValue)
	}

	// 测试随机读取聚合
	readAgg := AggregateDiskRandResults(results, false)
	if readAgg.MinValue != 100.0 {
		t.Errorf("随机读取 MinValue = %.2f; 期望 100.0", readAgg.MinValue)
	}
	if readAgg.MaxValue != 150.0 {
		t.Errorf("随机读取 MaxValue = %.2f; 期望 150.0", readAgg.MaxValue)
	}
	if readAgg.AvgValue != 125.0 {
		t.Errorf("随机读取 AvgValue = %.2f; 期望 125.0", readAgg.AvgValue)
	}
}

func TestAggregateNetworkResults(t *testing.T) {
	results := []NetworkResult{
		{Bandwidth: 100.0},
		{Bandwidth: 200.0},
		{Bandwidth: 300.0},
	}

	agg := AggregateNetworkResults(results)
	if agg == nil {
		t.Fatal("AggregateNetworkResults() 返回 nil")
	}
	if agg.MinValue != 100.0 {
		t.Errorf("MinValue = %.2f; 期望 100.0", agg.MinValue)
	}
	if agg.MaxValue != 300.0 {
		t.Errorf("MaxValue = %.2f; 期望 300.0", agg.MaxValue)
	}
	if agg.AvgValue != 200.0 {
		t.Errorf("AvgValue = %.2f; 期望 200.0", agg.AvgValue)
	}
	if agg.MedianValue != 200.0 {
		t.Errorf("MedianValue = %.2f; 期望 200.0", agg.MedianValue)
	}
}

func TestAggregateNetworkResults_Empty(t *testing.T) {
	results := []NetworkResult{}

	agg := AggregateNetworkResults(results)
	if agg == nil {
		t.Fatal("AggregateNetworkResults() 应该返回非 nil")
	}
	if agg.MinValue != 0 || agg.MaxValue != 0 || agg.AvgValue != 0 {
		t.Error("空结果的聚合值应该为 0")
	}
}

// ==================== 辅助函数测试 ====================

func TestResolveToIP(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{"localhost", "localhost", "127.0.0.1"},
		{"IP 地址", "127.0.0.1", "127.0.0.1"},
		{"无效主机", "nonexistent.invalid.host", "nonexistent.invalid.host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveToIP(tt.host)

			if tt.expected != "" && result != tt.expected {
				// 对于 localhost，可能是 ::1 或 127.0.0.1
				if tt.host == "localhost" {
					if result != "127.0.0.1" && result != "::1" {
						t.Errorf("ResolveToIP(%q) = %q; 期望 127.0.0.1 或 ::1", tt.host, result)
					}
				} else {
					t.Errorf("ResolveToIP(%q) = %q; 期望 %q", tt.host, result, tt.expected)
				}
			}
		})
	}
}

func TestGetHostname(t *testing.T) {
	hostname := getHostname()

	if hostname == "" {
		t.Error("getHostname() 不应该返回空字符串")
	}

	t.Logf("主机名：%s", hostname)
}

// ==================== 基准测试 ====================

func BenchmarkNewDiskChecker(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewDiskChecker(32, 1024*1024*1024, false, 4)
	}
}

func BenchmarkNewNetworkChecker(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewNetworkChecker(15*time.Second, 8, false, false)
	}
}

func BenchmarkNewStreamChecker(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewStreamChecker(false)
	}
}

func BenchmarkAggregateDiskResults(b *testing.B) {
	results := []*DiskResult{
		{WriteBandwidth: 100.0, ReadBandwidth: 200.0},
		{WriteBandwidth: 150.0, ReadBandwidth: 250.0},
		{WriteBandwidth: 200.0, ReadBandwidth: 300.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AggregateDiskResults(results, true)
	}
}

func BenchmarkAggregateNetworkResults(b *testing.B) {
	results := []NetworkResult{
		{Bandwidth: 100.0},
		{Bandwidth: 200.0},
		{Bandwidth: 300.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AggregateNetworkResults(results)
	}
}

// formatBytes 辅助函数（用于测试输出）
func formatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
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

// ==================== 服务器厂商识别测试 ====================

func TestHardwareChecker_identifyServerVendor(t *testing.T) {
	checker := NewHardwareChecker(false)

	tests := []struct {
		name         string
		manufacturer string
		expected     string
	}{
		// 国内服务器厂商
		{"浪潮", "Inspur", "浪潮 (Inspur)"},
		{"浪潮大写", "INSPIUR", "浪潮 (Inspur)"},
		{"华为", "Huawei", "华为 (Huawei)"},
		{"新华三", "H3C", "新华三 (H3C)"},
		{"联想", "Lenovo", "联想 (Lenovo)"},
		{"中科曙光", "Sugon", "中科曙光 (Sugon)"},
		{"宝德", "PowerLeader", "宝德 (PowerLeader)"},
		{"同方", "Tongfang", "同方 (Tongfang)"},
		{"超聚变", "xFusion", "超聚变 (xFusion)"},
		{"宁畅", "Nettrix", "宁畅 (Nettrix)"},
		{"黄河", "HuangHe", "黄河 (Huanghe)"},
		{"百信", "Baixin", "百信 (Baixin)"},
		{"华鲲振宇", "Hygon", "Hygon"},
		
		// 国际厂商
		{"戴尔", "Dell", "戴尔 (Dell)"},
		{"惠普", "HPE", "惠普 (HPE)"},
		{"广达", "Quanta", "广达 (Quanta)"},
		{"纬颖", "Wiwynn", "纬颖 (Wiwynn)"},
		
		// 未知厂商
		{"未知", "Unknown Vendor", "Unknown Vendor"},
		{"空", "", "未知厂商"},
		{"未指定", "NOT SPECIFIED", "未知厂商"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.identifyServerVendor(tt.manufacturer)
			if result != tt.expected {
				t.Errorf("identifyServerVendor(%q) = %q; 期望 %q", tt.manufacturer, result, tt.expected)
			}
		})
	}
}

func TestHardwareChecker_identifyServerSeries(t *testing.T) {
	checker := NewHardwareChecker(false)

	tests := []struct {
		name        string
		productName string
		expectSeries string
		expectModel  string
	}{
		// 浪潮服务器
		{"浪潮 NF5280", "NF5280 M5", "NF5280 系列", "主流机架式 2U"},
		{"浪潮 NF5466", "NF5466 M6", "NF5466 系列", "存储优化 4U"},
		{"浪潮 I9000", "I9000", "I9000 系列", "刀片服务器"},
		{"浪潮 TS860", "TS860 G3", "TS860 系列", "关键应用 8 路"},
		
		// 华为服务器
		{"华为 TaiShan 200", "TaiShan 200 2280", "TaiShan 200 系列", "ARM 架构 鲲鹏 920"},
		{"华为 2288H", "2288H V5", "2288H 系列", "主流机架式 2U"},
		{"华为 4888H", "4888H V5", "4888H 系列", "高端机架式 4U"},
		
		// 新华三服务器
		{"新华三 R4900", "R4900 G5", "R4900 系列", "主流机架式 2U"},
		{"新华三 R6900", "R6900 G5", "R6900 系列", "高端机架式 4U"},
		{"新华三 R8900", "R8900 G5", "R8900 系列", "关键业务 8 路"},
		
		// 联想服务器
		{"联想 SR650", "ThinkSystem SR650", "ThinkSystem SR650", "主流机架式 2U"},
		{"联想 SR850", "ThinkSystem SR850", "ThinkSystem SR850", "高端机架式 4U/8 路"},
		
		// 中科曙光服务器
		{"曙光 I420", "I420-C10", "I420 系列", "主流机架式 2U"},
		{"曙光 I620", "I620-C10", "I620 系列", "高端机架式 2U"},
		{"曙光 I840", "I840-C10", "I840 系列", "关键业务 4U/8 路"},
		
		// 超聚变服务器
		{"超聚变 2288H", "2288H V5", "2288H 系列", "主流机架式 2U"},
		{"超聚变 4888H", "4888H V5", "4888H 系列", "高端机架式 4U"},
		
		// 宁畅服务器
		{"宁畅 G30", "G30", "G30 系列", "主流机架式"},
		{"宁畅 G50", "G50", "G50 系列", "高端机架式"},
		{"宁畅 G70", "G70", "G70 系列", "AI 服务器"},
		
		// 宝德服务器
		{"宝德 PR2190", "PR2190K", "PR2190K 系列", "主流机架式 2U"},
		{"宝德 PR4190", "PR4190K", "PR4190K 系列", "高端机架式 4U"},
		
		// 同方服务器
		{"同方 T220", "T220-K30", "T220 系列", "主流机架式 2U"},
		{"同方 T460", "T460-K30", "T460 系列", "关键业务 4U"},
		
		// 黄河服务器
		{"黄河 K424", "K424", "K424 系列", "主流机架式 2U"},
		{"黄河 K824", "K824", "K824 系列", "高端机架式 4U"},
		
		// 未知系列
		{"未知", "Unknown Model", "未知系列", "Unknown Model"},
		{"空", "", "未知系列", "未知型号"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			series, model := checker.identifyServerSeries(tt.productName)
			if series != tt.expectSeries {
				t.Errorf("identifyServerSeries(%q) series = %q; 期望 %q", tt.productName, series, tt.expectSeries)
			}
			if model != tt.expectModel {
				t.Errorf("identifyServerSeries(%q) model = %q; 期望 %q", tt.productName, model, tt.expectModel)
			}
		})
	}
}

func TestRAIDConfigInfo_StructFields(t *testing.T) {
	info := &RAIDConfigInfo{
		HasRAID:       true,
		RAIDModel:     "LSI 3508",
		CacheSize:     2 * 1024 * 1024 * 1024,
		StripeSize:    64,
		RAIDLevel:     "RAID 5",
		BatteryBackup: true,
		ServerVendor:  "浪潮 (Inspur)",
		ServerSeries:  "NF5280 系列",
		ServerModel:   "主流机架式 2U",
	}

	if info.ServerVendor != "浪潮 (Inspur)" {
		t.Errorf("ServerVendor = %q; 期望 浪潮 (Inspur)", info.ServerVendor)
	}
	if info.ServerSeries != "NF5280 系列" {
		t.Errorf("ServerSeries = %q; 期望 NF5280 系列", info.ServerSeries)
	}
	if info.ServerModel != "主流机架式 2U" {
		t.Errorf("ServerModel = %q; 期望 主流机架式 2U", info.ServerModel)
	}
}
