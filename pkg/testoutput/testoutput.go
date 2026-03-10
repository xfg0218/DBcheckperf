// Package testoutput 提供测试输出示例和测试数据生成功能
// 用于演示和验证 dbcheckperf 工具的输出格式
package testoutput

import (
	"fmt"
	"strings"
	"time"

	"dbcheckperf/pkg/checker"
	"dbcheckperf/pkg/reporter"
	"dbcheckperf/pkg/utils"
)

// TestOutput 测试输出生成器
type TestOutput struct {
	reporter *reporter.Reporter
	verbose  bool
}

// NewTestOutput 创建新的测试输出生成器
func NewTestOutput(verbose bool) *TestOutput {
	return &TestOutput{
		reporter: reporter.NewReporter(true, true), // 始终启用详细模式以显示硬件详细信息
		verbose:  true,
	}
}

// RunDemo 运行演示模式，展示所有输出格式
func (t *TestOutput) RunDemo() {
	fmt.Println(strings.Repeat("═", 64))
	fmt.Println("       dbcheckperf 演示模式")
	fmt.Printf("       生成时间：%s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("═", 64))
	fmt.Println()

	t.printSystemInfoDemo()
	fmt.Println(strings.Repeat("-", 64))
	fmt.Println()

	t.printDiskTestDemo()
	fmt.Println(strings.Repeat("-", 64))
	fmt.Println()

	t.printStreamTestDemo()
	fmt.Println(strings.Repeat("-", 64))
	fmt.Println()

	t.printNetworkTestDemo("n")
	fmt.Println(strings.Repeat("-", 64))
	fmt.Println()

	t.printNetworkTestDemo("N")
	fmt.Println(strings.Repeat("-", 64))
	fmt.Println()

	t.printNetworkTestDemo("M")
	fmt.Println(strings.Repeat("-", 64))
	fmt.Println()

	t.printSummaryDemo()
}

// printSystemInfoDemo 打印系统信息演示
func (t *TestOutput) printSystemInfoDemo() {
	fmt.Println("【系统信息测试输出示例】")
	fmt.Println()

	infos := []*checker.SystemInfo{
		{
			Host:          "server1",
			CPUCore:       16,
			CPUModel:      "Intel Xeon E5-2680 v4",
			TotalRAM:      64 * 1024 * 1024 * 1024,
			DiskSize:      2 * 1024 * 1024 * 1024 * 1024,
			NICSpeed:      10000,
			IsVirtual:     false,
			VirtualType:   "Bare Metal",
			OSName:        "CentOS 7.9",
			KernelVersion: "3.10.0-1160.el7.x86_64",
			RAIDInfo: &checker.RAIDInfo{
				HasRAID:     true,
				RAIDModel:   "LSI MegaRAID SAS 3508",
				CacheSize:   2 * 1024 * 1024 * 1024,
				StripeSize:  256,
				RAIDLevel:   "RAID10",
			},
			NICBondInfo: &checker.NICBondInfo{
				HasBond:   true,
				BondMode:  "802.3ad Dynamic link aggregation",
				BondSlaves: 2,
				QueueSize: 1024,
			},
		},
		{
			Host:          "server2",
			CPUCore:       16,
			CPUModel:      "Intel Xeon Gold 6248R",
			TotalRAM:      64 * 1024 * 1024 * 1024,
			DiskSize:      2 * 1024 * 1024 * 1024 * 1024,
			NICSpeed:      10000,
			IsVirtual:     false,
			VirtualType:   "Bare Metal",
			OSName:        "Ubuntu 20.04.6 LTS",
			KernelVersion: "5.4.0-150-generic",
			RAIDInfo: &checker.RAIDInfo{
				HasRAID:     true,
				RAIDModel:   "Broadcom MegaRAID SAS 9460-16i",
				CacheSize:   4 * 1024 * 1024 * 1024,
				StripeSize:  512,
				RAIDLevel:   "RAID5",
			},
			NICBondInfo: &checker.NICBondInfo{
				HasBond:   false,
				BondMode:  "",
				BondSlaves: 0,
				QueueSize: 1000,
			},
		},
		{
			Host:          "server3",
			CPUCore:       8,
			CPUModel:      "AMD EPYC 7K62",
			TotalRAM:      32 * 1024 * 1024 * 1024,
			DiskSize:      1024 * 1024 * 1024 * 1024,
			NICSpeed:      1000,
			IsVirtual:     true,
			VirtualType:   "KVM",
			OSName:        "Rocky Linux 9.2",
			KernelVersion: "5.14.0-284.el9.x86_64",
			RAIDInfo: &checker.RAIDInfo{
				HasRAID:     false,
				RAIDModel:   "",
				CacheSize:   0,
				StripeSize:  64,
				RAIDLevel:   "",
			},
			NICBondInfo: &checker.NICBondInfo{
				HasBond:   true,
				BondMode:  "active-backup",
				BondSlaves: 2,
				QueueSize: 500,
			},
		},
	}

	t.reporter.PrintSystemInfo(infos)
}

// printDiskTestDemo 打印磁盘测试演示
func (t *TestOutput) printDiskTestDemo() {
	fmt.Println("【磁盘 I/O 测试输出示例】")
	fmt.Println()

	results := []*checker.DiskResult{
		{
			Host:           "server1",
			Dir:            "/data1",
			WriteTime:      45.23,
			ReadTime:       38.56,
			WriteBytes:     137438953472,
			ReadBytes:      137438953472,
			WriteBandwidth: 2920.45,
			ReadBandwidth:  3430.12,
			// 随机读写数据（4K 随机）
			RandWriteTime:      120.5,
			RandReadTime:       85.3,
			RandWriteBytes:     4294967296,
			RandReadBytes:      4294967296,
			RandWriteBandwidth: 35.64,
			RandReadBandwidth:  50.35,
		},
		{
			Host:           "server2",
			Dir:            "/data1",
			WriteTime:      46.78,
			ReadTime:       39.21,
			WriteBytes:     137438953472,
			ReadBytes:      137438953472,
			WriteBandwidth: 2823.67,
			ReadBandwidth:  3385.89,
			// 随机读写数据
			RandWriteTime:      125.2,
			RandReadTime:       88.7,
			RandWriteBytes:     4294967296,
			RandReadBytes:      4294967296,
			RandWriteBandwidth: 34.30,
			RandReadBandwidth:  48.42,
		},
		{
			Host:           "server3",
			Dir:            "/data1",
			WriteTime:      52.34,
			ReadTime:       44.12,
			WriteBytes:     137438953472,
			ReadBytes:      137438953472,
			WriteBandwidth: 2545.23,
			ReadBandwidth:  3012.45,
			// 随机读写数据（NVMe SSD）
			RandWriteTime:      95.8,
			RandReadTime:       72.4,
			RandWriteBytes:     4294967296,
			RandReadBytes:      4294967296,
			RandWriteBandwidth: 44.83,
			RandReadBandwidth:  59.32,
		},
	}

	writeAgg := &checker.AggregateResults{
		MinValue:    2545.23,
		MaxValue:    2920.45,
		AvgValue:    2763.12,
		MedianValue: 2823.67,
		MinHost:     "server3",
		MaxHost:     "server1",
		TotalValue:  8289.35,
		Count:       3,
	}

	readAgg := &checker.AggregateResults{
		MinValue:    3012.45,
		MaxValue:    3430.12,
		AvgValue:    3276.15,
		MedianValue: 3385.89,
		MinHost:     "server3",
		MaxHost:     "server1",
		TotalValue:  9828.46,
		Count:       3,
	}

	t.reporter.PrintDiskResults(results, writeAgg, readAgg)
}

// printStreamTestDemo 打印内存带宽测试演示
func (t *TestOutput) printStreamTestDemo() {
	fmt.Println("【内存带宽测试输出示例】")
	fmt.Println()

	results := []*checker.StreamResult{
		{
			Host:            "server1",
			CopyBandwidth:   45230.56,
			ScaleBandwidth:  44890.23,
			AddBandwidth:    46120.78,
			TriadBandwidth:  45780.34,
			TotalBandwidth:  182021.91,
		},
		{
			Host:            "server2",
			CopyBandwidth:   44980.12,
			ScaleBandwidth:  44560.89,
			AddBandwidth:    45890.45,
			TriadBandwidth:  45340.67,
			TotalBandwidth:  180772.13,
		},
		{
			Host:            "server3",
			CopyBandwidth:   23450.34,
			ScaleBandwidth:  23120.56,
			AddBandwidth:    23890.12,
			TriadBandwidth:  23560.78,
			TotalBandwidth:  94021.80,
		},
	}

	agg := &checker.AggregateResults{
		MinValue:    94021.80,
		MaxValue:    182021.91,
		AvgValue:    152271.95,
		MedianValue: 180772.13,
		MinHost:     "server3",
		MaxHost:     "server1",
		TotalValue:  456815.84,
		Count:       3,
	}

	t.reporter.PrintStreamResults(results, agg)
}

// printNetworkTestDemo 打印网络测试演示
func (t *TestOutput) printNetworkTestDemo(mode string) {
	modeNames := map[string]string{"n": "串行", "N": "并行", "M": "全矩阵"}
	fmt.Printf("【网络测试输出示例 (%s 模式)】\n", modeNames[mode])
	fmt.Println()

	var results []checker.NetworkResult

	switch mode {
	case "n":
		results = []checker.NetworkResult{
			{SourceHost: "localhost", DestHost: "server1", Bandwidth: 112.45, Duration: 15},
			{SourceHost: "localhost", DestHost: "server2", Bandwidth: 108.67, Duration: 15},
			{SourceHost: "localhost", DestHost: "server3", Bandwidth: 95.23, Duration: 15},
		}
	case "N":
		results = []checker.NetworkResult{
			{SourceHost: "localhost", DestHost: "server1", Bandwidth: 110.34, Duration: 15},
			{SourceHost: "localhost", DestHost: "server2", Bandwidth: 107.89, Duration: 15},
		}
	case "M":
		results = []checker.NetworkResult{
			{SourceHost: "server1", DestHost: "server2", Bandwidth: 115.67, Duration: 15},
			{SourceHost: "server1", DestHost: "server3", Bandwidth: 98.34, Duration: 15},
			{SourceHost: "server2", DestHost: "server1", Bandwidth: 114.89, Duration: 15},
			{SourceHost: "server2", DestHost: "server3", Bandwidth: 96.78, Duration: 15},
		}
	}

	agg := checker.AggregateNetworkResults(results)
	t.reporter.PrintNetworkResults(results, agg, mode)
}

// printSummaryDemo 打印汇总报告演示
func (t *TestOutput) printSummaryDemo() {
	fmt.Println("【完整测试报告示例】")
	fmt.Println()

	diskResults := []*checker.DiskResult{
		{
			Host:           "server1",
			Dir:            "/data1",
			WriteTime:      45.23,
			ReadTime:       38.56,
			WriteBytes:     137438953472,
			ReadBytes:      137438953472,
			WriteBandwidth: 2920.45,
			ReadBandwidth:  3430.12,
			// 随机读写数据
			RandWriteTime:      120.5,
			RandReadTime:       85.3,
			RandWriteBytes:     4294967296,
			RandReadBytes:      4294967296,
			RandWriteBandwidth: 35.64,
			RandReadBandwidth:  50.35,
		},
		{
			Host:           "server2",
			Dir:            "/data1",
			WriteTime:      46.78,
			ReadTime:       39.21,
			WriteBytes:     137438953472,
			ReadBytes:      137438953472,
			WriteBandwidth: 2823.67,
			ReadBandwidth:  3385.89,
			// 随机读写数据
			RandWriteTime:      125.2,
			RandReadTime:       88.7,
			RandWriteBytes:     4294967296,
			RandReadBytes:      4294967296,
			RandWriteBandwidth: 34.30,
			RandReadBandwidth:  48.42,
		},
	}

	streamResults := []*checker.StreamResult{
		{
			Host:            "server1",
			CopyBandwidth:   45230.56,
			ScaleBandwidth:  44890.23,
			AddBandwidth:    46120.78,
			TriadBandwidth:  45780.34,
			TotalBandwidth:  182021.91,
		},
		{
			Host:            "server2",
			CopyBandwidth:   44980.12,
			ScaleBandwidth:  44560.89,
			AddBandwidth:    45890.45,
			TriadBandwidth:  45340.67,
			TotalBandwidth:  180772.13,
		},
	}

	networkResults := []checker.NetworkResult{
		{SourceHost: "localhost", DestHost: "server1", Bandwidth: 112.45, Duration: 15},
		{SourceHost: "localhost", DestHost: "server2", Bandwidth: 108.67, Duration: 15},
	}

	t.reporter.PrintSummary(diskResults, streamResults, networkResults)
}

// PrintTestData 打印工具函数测试数据
func (t *TestOutput) PrintTestData() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    工 具 函 数 测 试 数 据                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("【字节格式化测试】")
	testBytes := []uint64{512, 1024, 1048576, 1073741824, 1099511627776}
	for _, b := range testBytes {
		fmt.Printf("  FormatBytes(%d) = %s\n", b, utils.FormatBytes(b))
	}
	fmt.Println()

	fmt.Println("【带宽格式化测试】")
	testBandwidths := []float64{0.5, 10.25, 100.75, 1000.5, 10000.25}
	for _, bw := range testBandwidths {
		fmt.Printf("  FormatBandwidth(%.2f) = %s\n", bw, utils.FormatBandwidth(bw))
	}
	fmt.Println()

	fmt.Println("【时间格式化测试】")
	testDurations := []float64{0.5, 30, 90, 3600, 7200}
	for _, d := range testDurations {
		fmt.Printf("  FormatDuration(%.1f) = %s\n", d, utils.FormatDuration(d))
	}
	fmt.Println()

	fmt.Println("【文件大小解析测试】")
	testFileSizes := []string{"512KB", "100MB", "5GB", "2xRAM"}
	for _, fs := range testFileSizes {
		size, err := utils.ParseFileSize(fs)
		if err != nil {
			fmt.Printf("  ParseFileSize(%s) = 错误：%v\n", fs, err)
		} else {
			fmt.Printf("  ParseFileSize(%s) = %d bytes (%s)\n", fs, size, utils.FormatBytes(size))
		}
	}
	fmt.Println()

	fmt.Println("【数学函数测试】")
	values := []float64{10.5, 20.3, 15.7, 8.2, 25.9}
	fmt.Printf("  值列表：%v\n", values)
	fmt.Printf("  MinSlice = %.2f\n", utils.MinSlice(values))
	fmt.Printf("  MaxSlice = %.2f\n", utils.MaxSlice(values))
	fmt.Printf("  Average = %.2f\n", utils.Average(values))
	fmt.Printf("  Median = %.2f\n", utils.Median(values))
	fmt.Println()
}

// RunBenchmark 运行基准参考值展示
func (t *TestOutput) RunBenchmark() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    硬 件 基 准 参 考 值                        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("【磁盘性能参考值】")
	fmt.Println("  HDD (7200 RPM):")
	fmt.Println("    顺序写入：80-160 MB/s")
	fmt.Println("    顺序读取：100-200 MB/s")
	fmt.Println("    4K 随机写入：0.5-2 MB/s")
	fmt.Println("    4K 随机读取：0.5-3 MB/s")
	fmt.Println()
	fmt.Println("  SSD (SATA):")
	fmt.Println("    顺序写入：200-550 MB/s")
	fmt.Println("    顺序读取：400-550 MB/s")
	fmt.Println("    4K 随机写入：30-150 MB/s")
	fmt.Println("    4K 随机读取：20-400 MB/s")
	fmt.Println()
	fmt.Println("  NVMe SSD:")
	fmt.Println("    顺序写入：1500-7000 MB/s")
	fmt.Println("    顺序读取：2000-7500 MB/s")
	fmt.Println("    4K 随机写入：100-600 MB/s")
	fmt.Println("    4K 随机读取：50-900 MB/s")
	fmt.Println()

	fmt.Println("【内存带宽参考值】")
	fmt.Println("  DDR4-2133 (双通道): ~34 GB/s")
	fmt.Println("  DDR4-2666 (双通道): ~42 GB/s")
	fmt.Println("  DDR4-3200 (双通道): ~51 GB/s")
	fmt.Println("  DDR5-4800 (双通道): ~76 GB/s")
	fmt.Println()

	fmt.Println("【网络带宽参考值】")
	fmt.Println("  1 Gbps: 理论最大值 125 MB/s，实际约 100-115 MB/s")
	fmt.Println("  10 Gbps: 理论最大值 1250 MB/s，实际约 1000-1200 MB/s")
	fmt.Println("  25 Gbps: 理论最大值 3125 MB/s，实际约 2500-3000 MB/s")
	fmt.Println("  100 Gbps: 理论最大值 12500 MB/s，实际约 10000-12000 MB/s")
	fmt.Println()

	fmt.Println("【CPU 核心数参考】")
	fmt.Println("  入门级：4-8 核心")
	fmt.Println("  中端：8-16 核心")
	fmt.Println("  高端：16-64 核心")
	fmt.Println("  服务器：32-128 核心")
	fmt.Println()
}

// RunQuickTest 运行快速测试
func (t *TestOutput) RunQuickTest() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    快 速 测 试 模 式                           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 快速系统信息
	fmt.Println("[1/4] 系统信息...")
	systemChecker := checker.NewSystemChecker()
	info, err := systemChecker.Run()
	if err != nil {
		fmt.Printf("  警告：无法获取完整系统信息：%v\n", err)
	} else {
		t.reporter.PrintSystemInfo([]*checker.SystemInfo{info})
	}

	// 快速内存测试
	fmt.Println("[2/4] 快速内存带宽估算...")
	streamChecker := checker.NewStreamChecker(false)
	streamResult, err := streamChecker.Run()
	if err != nil {
		fmt.Printf("  警告：内存测试失败：%v\n", err)
	} else {
		fmt.Printf("  总带宽：%s\n", utils.FormatBandwidth(streamResult.TotalBandwidth))
		fmt.Printf("  复制：%s | 缩放：%s | 加法：%s | 三合一：%s\n",
			utils.FormatBandwidth(streamResult.CopyBandwidth),
			utils.FormatBandwidth(streamResult.ScaleBandwidth),
			utils.FormatBandwidth(streamResult.AddBandwidth),
			utils.FormatBandwidth(streamResult.TriadBandwidth))
	}

	// 快速网络测试
	fmt.Println("[3/4] 快速网络测试...")
	fmt.Println("  (需要指定远程主机，跳过)")

	// 汇总
	fmt.Println("[4/4] 快速汇总...")
	if streamResult != nil {
		fmt.Printf("  内存带宽：%s\n", utils.FormatBandwidth(streamResult.TotalBandwidth))
	}
	fmt.Println()
	fmt.Println("快速测试完成。完整测试请使用正式模式。")
	fmt.Println()
}

// PrintTestModes 打印测试模式说明
func (t *TestOutput) PrintTestModes() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    测 试 模 式 说 明                           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("dbcheckperf 提供以下测试输出模式:")
	fmt.Println()
	fmt.Println("  -test-demo      演示模式")
	fmt.Println("                  展示所有类型的测试输出格式，包括系统信息、")
	fmt.Println("                  磁盘 I/O、内存带宽和网络测试的完整输出。")
	fmt.Println()
	fmt.Println("  -test-data      数据模式")
	fmt.Println("                  展示工具函数的测试数据，包括字节格式化、")
	fmt.Println("                  带宽格式化、时间格式化和数学函数等。")
	fmt.Println()
	fmt.Println("  -test-benchmark 基准模式")
	fmt.Println("                  展示硬件性能基准参考值，帮助判断测试结果")
	fmt.Println("                  是否在正常范围内。")
	fmt.Println()
	fmt.Println("  -test-quick     快速模式")
	fmt.Println("                  运行简化的本地测试，快速获取系统信息和")
	fmt.Println("                  内存带宽估算。")
	fmt.Println()
	fmt.Println("使用示例:")
	fmt.Println("  ./dbcheckperf -test-demo      # 演示所有输出格式")
	fmt.Println("  ./dbcheckperf -test-data      # 查看工具函数测试数据")
	fmt.Println("  ./dbcheckperf -test-benchmark # 查看硬件基准参考值")
	fmt.Println("  ./dbcheckperf -test-quick     # 运行快速本地测试")
	fmt.Println()
}
