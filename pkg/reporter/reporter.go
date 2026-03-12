// Package reporter 提供测试结果的报告和展示功能
// 包含表格格式化、结果汇总等功能
package reporter

import (
	"fmt"
	"os"
	"strings"

	"dbcheckperf/pkg/checker"
	"dbcheckperf/pkg/utils"
)

// Reporter 报告生成器
type Reporter struct {
	// DisplayPerHost 是否显示每台主机的详细结果
	DisplayPerHost bool
	// Verbose 是否显示详细输出
	Verbose bool
}

// NewReporter 创建新的报告生成器
func NewReporter(displayPerHost, verbose bool) *Reporter {
	return &Reporter{
		DisplayPerHost: displayPerHost,
		Verbose:        verbose,
	}
}

// PrintSystemInfo 打印系统信息表格
func (r *Reporter) PrintSystemInfo(infos []*checker.SystemInfo) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  系统信息")
	fmt.Println("====================")
	fmt.Println()

	// 打印表头
	header := fmt.Sprintf("%-20s %-8s %-25s %-12s %-10s %-15s %-20s %-15s",
		"主机名", "虚拟化", "CPU 型号", "CPU 核心数", "内存大小", "操作系统", "内核版本", "网卡速率")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 130))

	// 打印每行数据
	for _, info := range infos {
		virtType := "物理机"
		if info.IsVirtual {
			virtType = info.VirtualType
		}
		cpuModel := info.CPUModel
		if len(cpuModel) > 23 {
			cpuModel = cpuModel[:20] + "..."
		}
		row := fmt.Sprintf("%-20s %-8s %-25s %-12d %-12s %-15s %-20s %-15s",
			info.Host,
			virtType,
			cpuModel,
			info.CPUCore,
			utils.FormatBytes(info.TotalRAM),
			info.OSName,
			info.KernelVersion,
			fmt.Sprintf("%d Mbps", info.NICSpeed))
		fmt.Println(row)
	}

	fmt.Println()

	// 如果启用了详细模式，显示额外的硬件信息
	if r.Verbose {
		r.printHardwareDetails(infos)
	}
}

// printHardwareDetails 打印硬件详细信息（表格格式）
func (r *Reporter) printHardwareDetails(infos []*checker.SystemInfo) {
	fmt.Println("====================")
	fmt.Println("==  硬件详细信息")
	fmt.Println("====================")
	fmt.Println()

	// RAID 信息表格
	fmt.Println("--- RAID 信息 ---")
	fmt.Println()
	raidHeader := fmt.Sprintf("%-20s %-15s %-35s %-15s %-12s %-15s",
		"主机名", "状态", "RAID 型号", "缓存大小", "条带大小", "RAID 级别")
	fmt.Println(raidHeader)
	fmt.Println(strings.Repeat("-", 115))

	for _, info := range infos {
		if info.RAIDInfo != nil {
			status := "未检测到"
			model := "-"
			cache := "-"
			stripe := fmt.Sprintf("%d KB", info.RAIDInfo.StripeSize)
			level := "-"

			if info.RAIDInfo.HasRAID {
				status = "已检测"
				model = info.RAIDInfo.RAIDModel
				if len(model) > 33 {
					model = model[:30] + "..."
				}
				if info.RAIDInfo.CacheSize > 0 {
					cache = utils.FormatBytes(info.RAIDInfo.CacheSize)
				}
				if info.RAIDInfo.RAIDLevel != "" {
					level = info.RAIDInfo.RAIDLevel
				}
			}

			row := fmt.Sprintf("%-20s %-15s %-35s %-15s %-12s %-15s",
				info.Host, status, model, cache, stripe, level)
			fmt.Println(row)
		}
	}
	fmt.Println()

	// 网卡绑定信息表格
	fmt.Println("--- 网络绑定信息 ---")
	fmt.Println()
	bondHeader := fmt.Sprintf("%-20s %-12s %-35s %-12s %-12s",
		"主机名", "绑定状态", "绑定模式", "从网卡数", "队列大小")
	fmt.Println(bondHeader)
	fmt.Println(strings.Repeat("-", 95))

	for _, info := range infos {
		if info.NICBondInfo != nil {
			bondStatus := "未绑定"
			bondMode := "-"
			slaves := "-"
			queue := fmt.Sprintf("%d", info.NICBondInfo.QueueSize)

			if info.NICBondInfo.HasBond {
				bondStatus = "已绑定"
				bondMode = info.NICBondInfo.BondMode
				if len(bondMode) > 33 {
					bondMode = bondMode[:30] + "..."
				}
				slaves = fmt.Sprintf("%d", info.NICBondInfo.BondSlaves)
			}

			row := fmt.Sprintf("%-20s %-12s %-35s %-12s %-12s",
				info.Host, bondStatus, bondMode, slaves, queue)
			fmt.Println(row)
		}
	}
	fmt.Println()
}

// PrintDiskResults 打印磁盘测试结果
func (r *Reporter) PrintDiskResults(results []*checker.DiskResult, writeAgg, readAgg *checker.AggregateResults) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  磁盘 I/O 测试结果")
	fmt.Println("====================")
	fmt.Println()

	// 如果启用每主机显示，打印详细结果
	if r.DisplayPerHost {
		fmt.Println("--- 顺序写入性能 ---")
		r.printSeqDiskTable(results, true)
		fmt.Println()
		fmt.Println("--- 顺序读取性能 ---")
		r.printSeqDiskTable(results, false)
		fmt.Println()

		// 检查是否有随机读写数据
		hasRandom := false
		for _, result := range results {
			if result.RandWriteBandwidth > 0 || result.RandReadBandwidth > 0 {
				hasRandom = true
				break
			}
		}

		if hasRandom {
			fmt.Println("--- 随机写入性能 ---")
			r.printRandDiskTable(results, true)
			fmt.Println()
			fmt.Println("--- 随机读取性能 ---")
			r.printRandDiskTable(results, false)
			fmt.Println()
		}
	}

	// 打印顺序读写汇总结果
	fmt.Println("--- 顺序写入汇总 ---")
	r.printAggregate("顺序写入", writeAgg)
	fmt.Println()
	fmt.Println("--- 顺序读取汇总 ---")
	r.printAggregate("顺序读取", readAgg)
	fmt.Println()

	// 打印随机读写汇总（如果有）
	hasRandom := false
	for _, result := range results {
		if result.RandWriteBandwidth > 0 || result.RandReadBandwidth > 0 {
			hasRandom = true
			break
		}
	}

	if hasRandom {
		randWriteAgg := checker.AggregateDiskRandResults(results, true)
		randReadAgg := checker.AggregateDiskRandResults(results, false)
		fmt.Println("--- 随机写入汇总 ---")
		r.printAggregate("随机写入", randWriteAgg)
		fmt.Println()
		fmt.Println("--- 随机读取汇总 ---")
		r.printAggregate("随机读取", randReadAgg)
		fmt.Println()
	}
}

// printSeqDiskTable 打印顺序磁盘结果表格
func (r *Reporter) printSeqDiskTable(results []*checker.DiskResult, write bool) {
	// 打印表头
	header := fmt.Sprintf("%-20s %-15s %-15s %-15s",
		"主机名", "时间 (秒)", "数据量", "带宽 (MB/s)")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 65))

	// 打印每行数据
	for _, result := range results {
		var timeVal float64
		var bytesVal uint64
		var bandwidth float64

		if write {
			timeVal = result.WriteTime
			bytesVal = result.WriteBytes
			bandwidth = result.WriteBandwidth
		} else {
			timeVal = result.ReadTime
			bytesVal = result.ReadBytes
			bandwidth = result.ReadBandwidth
		}

		row := fmt.Sprintf("%-20s %-15.2f %-15s %-15.2f",
			result.Host,
			timeVal,
			utils.FormatBytes(bytesVal),
			bandwidth)
		fmt.Println(row)
	}
}

// printRandDiskTable 打印随机磁盘结果表格
func (r *Reporter) printRandDiskTable(results []*checker.DiskResult, write bool) {
	// 打印表头
	header := fmt.Sprintf("%-20s %-15s %-15s %-15s",
		"主机名", "时间 (秒)", "数据量", "带宽 (MB/s)")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 65))

	// 打印每行数据
	for _, result := range results {
		var timeVal float64
		var bytesVal uint64
		var bandwidth float64

		if write {
			timeVal = result.RandWriteTime
			bytesVal = result.RandWriteBytes
			bandwidth = result.RandWriteBandwidth
		} else {
			timeVal = result.RandReadTime
			bytesVal = result.RandReadBytes
			bandwidth = result.RandReadBandwidth
		}

		row := fmt.Sprintf("%-20s %-15.2f %-15s %-15.2f",
			result.Host,
			timeVal,
			utils.FormatBytes(bytesVal),
			bandwidth)
		fmt.Println(row)
	}
}

// printAggregate 打印汇总统计
func (r *Reporter) printAggregate(testType string, agg *checker.AggregateResults) {
	if agg.Count == 0 {
		fmt.Printf("%s：无数据\n", testType)
		return
	}

	fmt.Printf("%s平均带宽：%s\n", testType, utils.FormatBandwidth(agg.AvgValue))
	fmt.Printf("%s中位带宽：%s\n", testType, utils.FormatBandwidth(agg.MedianValue))
	fmt.Printf("%s最小带宽：%s [%s]\n", testType, utils.FormatBandwidth(agg.MinValue), agg.MinHost)
	fmt.Printf("%s最大带宽：%s [%s]\n", testType, utils.FormatBandwidth(agg.MaxValue), agg.MaxHost)
}

// PrintStreamResults 打印内存带宽测试结果
func (r *Reporter) PrintStreamResults(results []*checker.StreamResult, agg *checker.AggregateResults) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  内存带宽测试结果")
	fmt.Println("====================")
	fmt.Println()

	// 如果启用每主机显示，打印详细结果
	if r.DisplayPerHost {
		// 打印表头
		header := fmt.Sprintf("%-20s %-15s %-15s %-15s %-15s %-15s",
			"主机名", "复制 (MB/s)", "缩放 (MB/s)", "加法 (MB/s)", "三合一 (MB/s)", "总计 (MB/s)")
		fmt.Println(header)
		fmt.Println(strings.Repeat("-", 95))

		// 打印每行数据
		for _, result := range results {
			row := fmt.Sprintf("%-20s %-15.2f %-15.2f %-15.2f %-15.2f %-15.2f",
				result.Host,
				result.CopyBandwidth,
				result.ScaleBandwidth,
				result.AddBandwidth,
				result.TriadBandwidth,
				result.TotalBandwidth)
			fmt.Println(row)
		}
		fmt.Println()
	}

	// 打印汇总结果
	fmt.Println("--- 汇总 ---")
	fmt.Printf("总带宽：%s\n", utils.FormatBandwidth(agg.AvgValue*float64(agg.Count)))
	fmt.Printf("平均带宽：%s\n", utils.FormatBandwidth(agg.AvgValue))
	fmt.Printf("最小带宽：%s [%s]\n", utils.FormatBandwidth(agg.MinValue), agg.MinHost)
	fmt.Printf("最大带宽：%s [%s]\n", utils.FormatBandwidth(agg.MaxValue), agg.MaxHost)
	fmt.Println()
}

// PrintNetworkResults 打印网络测试结果
func (r *Reporter) PrintNetworkResults(results []checker.NetworkResult, agg *checker.AggregateResults, mode string) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  网络性能测试结果")
	fmt.Println("====================")
	fmt.Println()

	// 显示测试模式
	modeDesc := map[string]string{
		"n": "串行模式",
		"N": "并行模式",
		"M": "全矩阵模式",
	}
	fmt.Printf("测试模式：%s\n", modeDesc[mode])
	fmt.Println()

	// 如果启用每主机显示，打印详细结果
	if r.DisplayPerHost || mode == "n" {
		// 打印表头
		header := fmt.Sprintf("%-20s -> %-20s %-15s",
			"源主机", "目标主机", "带宽 (MB/s)")
		fmt.Println(header)
		fmt.Println(strings.Repeat("-", 60))

		// 打印每行数据
		for _, result := range results {
			row := fmt.Sprintf("%-20s -> %-20s %-15.2f",
				result.SourceHost,
				result.DestHost,
				result.Bandwidth)
			fmt.Println(row)
		}
		fmt.Println()
	}

	// 打印汇总结果
	fmt.Println("--- 汇总 ---")
	fmt.Printf("总带宽：%s\n", utils.FormatBandwidth(agg.TotalValue))
	fmt.Printf("平均带宽：%s\n", utils.FormatBandwidth(agg.AvgValue))
	fmt.Printf("中位带宽：%s\n", utils.FormatBandwidth(agg.MedianValue))
	fmt.Printf("最小带宽：%s\n", utils.FormatBandwidth(agg.MinValue))
	fmt.Printf("最大带宽：%s\n", utils.FormatBandwidth(agg.MaxValue))

	// 检查是否低于阈值
	if agg.AvgValue < 100 {
		fmt.Println()
		fmt.Println("⚠️  警告：平均带宽低于 100 MB/s，建议使用 -r n 选项进行串行测试以获取每台主机的结果")
	}
	fmt.Println()
}

// PrintSummary 打印最终汇总报告（表格格式）
func (r *Reporter) PrintSummary(diskResults []*checker.DiskResult, streamResults []*checker.StreamResult, networkResults []checker.NetworkResult) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      测 试 汇 总 报 告                        ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")

	// 磁盘汇总
	if len(diskResults) > 0 {
		writeAgg := checker.AggregateDiskResults(diskResults, true)
		readAgg := checker.AggregateDiskResults(diskResults, false)
		fmt.Printf("║  磁盘写入带宽：%-45s ║\n", fmt.Sprintf("平均 %s, 最小 %s [%s]",
			utils.FormatBandwidth(writeAgg.AvgValue),
			utils.FormatBandwidth(writeAgg.MinValue),
			writeAgg.MinHost))
		fmt.Printf("║  磁盘读取带宽：%-45s ║\n", fmt.Sprintf("平均 %s, 最小 %s [%s]",
			utils.FormatBandwidth(readAgg.AvgValue),
			utils.FormatBandwidth(readAgg.MinValue),
			readAgg.MinHost))
	}

	// 内存汇总
	if len(streamResults) > 0 {
		var totalBandwidth float64
		var minBandwidth float64 = -1
		var minHost string
		for _, r := range streamResults {
			totalBandwidth += r.TotalBandwidth
			if minBandwidth < 0 || r.TotalBandwidth < minBandwidth {
				minBandwidth = r.TotalBandwidth
				minHost = r.Host
			}
		}
		avgBandwidth := totalBandwidth / float64(len(streamResults))
		fmt.Printf("║  内存带宽：%-52s ║\n", fmt.Sprintf("平均 %s, 最小 %s [%s]",
			utils.FormatBandwidth(avgBandwidth),
			utils.FormatBandwidth(minBandwidth),
			minHost))
	}

	// 网络汇总
	if len(networkResults) > 0 {
		var totalBandwidth float64
		var minBandwidth float64 = -1
		for _, r := range networkResults {
			totalBandwidth += r.Bandwidth
			if minBandwidth < 0 || r.Bandwidth < minBandwidth {
				minBandwidth = r.Bandwidth
			}
		}
		avgBandwidth := totalBandwidth / float64(len(networkResults))
		fmt.Printf("║  网络带宽：%-52s ║\n", fmt.Sprintf("平均 %s, 最小 %s",
			utils.FormatBandwidth(avgBandwidth),
			utils.FormatBandwidth(minBandwidth)))
	}

	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 打印综合测试结果表格
	r.printTestSummaryTable(diskResults, streamResults, networkResults)
}

// printTestSummaryTable 打印综合测试结果表格
func (r *Reporter) printTestSummaryTable(diskResults []*checker.DiskResult, streamResults []*checker.StreamResult, networkResults []checker.NetworkResult) {
	fmt.Println("====================")
	fmt.Println("==  综合测试结果表")
	fmt.Println("====================")
	fmt.Println()

	// 表头
	header := fmt.Sprintf("%-20s %-18s %-18s %-18s %-18s",
		"测试项目", "平均值", "最小值", "最大值", "备注")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 95))

	// 磁盘写入
	if len(diskResults) > 0 {
		writeAgg := checker.AggregateDiskResults(diskResults, true)
		remark := fmt.Sprintf("%d 次测试", writeAgg.Count)
		row := fmt.Sprintf("%-20s %-18s %-18s %-18s %-18s",
			"磁盘写入",
			utils.FormatBandwidth(writeAgg.AvgValue),
			utils.FormatBandwidth(writeAgg.MinValue),
			utils.FormatBandwidth(writeAgg.MaxValue),
			remark)
		fmt.Println(row)
	}

	// 磁盘读取
	if len(diskResults) > 0 {
		readAgg := checker.AggregateDiskResults(diskResults, false)
		row := fmt.Sprintf("%-20s %-18s %-18s %-18s %-18s",
			"磁盘读取",
			utils.FormatBandwidth(readAgg.AvgValue),
			utils.FormatBandwidth(readAgg.MinValue),
			utils.FormatBandwidth(readAgg.MaxValue),
			fmt.Sprintf("%d 次测试", readAgg.Count))
		fmt.Println(row)
	}

	// 内存带宽
	if len(streamResults) > 0 {
		var totalBandwidth, minBandwidth, maxBandwidth float64
		minBandwidth = -1
		for _, r := range streamResults {
			totalBandwidth += r.TotalBandwidth
			if minBandwidth < 0 || r.TotalBandwidth < minBandwidth {
				minBandwidth = r.TotalBandwidth
			}
			if r.TotalBandwidth > maxBandwidth {
				maxBandwidth = r.TotalBandwidth
			}
		}
		avgBandwidth := totalBandwidth / float64(len(streamResults))
		row := fmt.Sprintf("%-20s %-18s %-18s %-18s %-18s",
			"内存带宽",
			utils.FormatBandwidth(avgBandwidth),
			utils.FormatBandwidth(minBandwidth),
			utils.FormatBandwidth(maxBandwidth),
			fmt.Sprintf("%d 次测试", len(streamResults)))
		fmt.Println(row)
	}

	// 网络带宽
	if len(networkResults) > 0 {
		var totalBandwidth, minBandwidth, maxBandwidth float64
		minBandwidth = -1
		for _, r := range networkResults {
			totalBandwidth += r.Bandwidth
			if minBandwidth < 0 || r.Bandwidth < minBandwidth {
				minBandwidth = r.Bandwidth
			}
			if r.Bandwidth > maxBandwidth {
				maxBandwidth = r.Bandwidth
			}
		}
		avgBandwidth := totalBandwidth / float64(len(networkResults))
		row := fmt.Sprintf("%-20s %-18s %-18s %-18s %-18s",
			"网络带宽",
			utils.FormatBandwidth(avgBandwidth),
			utils.FormatBandwidth(minBandwidth),
			utils.FormatBandwidth(maxBandwidth),
			fmt.Sprintf("%d 次测试", len(networkResults)))
		fmt.Println(row)
	}

	fmt.Println()
}

// PrintError 打印错误信息
func (r *Reporter) PrintError(err error) {
	fmt.Fprintf(os.Stderr, "错误：%v\n", err)
}

// PrintProgress 打印进度信息
func (r *Reporter) PrintProgress(message string) {
	if r.Verbose {
		fmt.Printf("[进度] %s\n", message)
	}
}

// PrintVerbose 打印详细输出
func (r *Reporter) PrintVerbose(format string, args ...interface{}) {
	if r.Verbose {
		fmt.Printf(format, args...)
	}
}

// PrintHardwareResults 打印硬件信息表格
func (r *Reporter) PrintHardwareResults(info *checker.HardwareInfo) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                    硬 件 信 息 报 告                                       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// CPU 信息表格
	fmt.Println("┌────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│  📌 CPU 信息                                                                               │")
	fmt.Println("├───────────────┬──────────────────────┬──────────┬──────────┬────────────┬─────────────────┤")
	fmt.Println("│     主机名     │       CPU 型号        │  核心数   │  插槽数   │   基准频率   │      睿频        │")
	fmt.Println("├───────────────┼──────────────────────┼──────────┼──────────┼────────────┼─────────────────┤")

	cpuModel := info.CPUInfo.Model
	if len(cpuModel) > 20 {
		cpuModel = cpuModel[:17] + "..."
	}
	baseFreqStr := "-"
	if info.CPUInfo.BaseFreq > 0 {
		baseFreqStr = fmt.Sprintf("%d MHz", info.CPUInfo.BaseFreq)
	}
	turboFreqStr := "-"
	if info.CPUInfo.TurboFreq > 0 {
		turboFreqStr = fmt.Sprintf("%d MHz", info.CPUInfo.TurboFreq)
	}
	cpuRow := fmt.Sprintf("│ %-13s │ %-20s │ %-8d │ %-8d │ %-10s │ %-15s │",
		info.Host,
		cpuModel,
		info.CPUInfo.Cores,
		info.CPUInfo.Sockets,
		baseFreqStr,
		turboFreqStr)
	fmt.Println(cpuRow)
	fmt.Println("└───────────────┴──────────────────────┴──────────┴──────────┴────────────┴─────────────────┘")
	fmt.Println()

	// 内存信息表格
	fmt.Println("┌────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│  📌 内存信息                                                                               │")
	fmt.Println("├───────────────┬────────────────┬────────────────┬────────────────┬────────────────────────┤")
	fmt.Println("│     主机名     │    总内存       │    内存类型     │    内存速度     │       插槽数            │")
	fmt.Println("├───────────────┼────────────────┼────────────────┼────────────────┼────────────────────────┤")

	memType := info.MemoryInfo.MemoryType
	if memType == "" || memType == "Unknown" {
		memType = "-"
	}
	memSpeedStr := "-"
	if info.MemoryInfo.MemorySpeed > 0 {
		memSpeedStr = fmt.Sprintf("%d MHz", info.MemoryInfo.MemorySpeed)
	}
	memRow := fmt.Sprintf("│ %-13s │ %-14s │ %-14s │ %-14s │ %-22d │",
		info.Host,
		utils.FormatBytes(info.MemoryInfo.TotalMemory),
		memType,
		memSpeedStr,
		info.MemoryInfo.MemorySlots)
	fmt.Println(memRow)
	fmt.Println("└───────────────┴────────────────┴────────────────┴────────────────┴────────────────────────┘")
	fmt.Println()

	// 磁盘信息表格
	fmt.Println("┌────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│  📌 磁盘信息                                                                               │")
	fmt.Println("├────────────┬─────────────────────────────┬──────────────┬──────────┬───────────┬──────────┤")
	fmt.Println("│   设备名    │          型号               │     厂家      │   类型    │    大小    │  旋转   │")
	fmt.Println("├────────────┼─────────────────────────────┼──────────────┼──────────┼───────────┼──────────┤")

	if len(info.DiskInfos) == 0 {
		fmt.Println("│                                        未检测到物理磁盘                                  │")
	} else {
		for _, disk := range info.DiskInfos {
			model := disk.Model
			if len(model) > 27 {
				model = model[:24] + "..."
			}
			vendor := disk.Vendor
			if vendor == "" {
				vendor = "-"
			}
			rotational := "否"
			if disk.Rotational {
				rotational = "是"
			}
			diskRow := fmt.Sprintf("│ %-10s │ %-27s │ %-12s │ %-8s │ %-9s │ %-8s │",
				disk.Name,
				model,
				vendor,
				disk.Type,
				utils.FormatBytes(disk.Size),
				rotational)
			fmt.Println(diskRow)
		}
	}
	fmt.Println("└────────────┴─────────────────────────────┴──────────────┴──────────┴───────────┴──────────┘")
	fmt.Println()

	// RAID 信息表格
	fmt.Println("┌────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│  📌 RAID 信息                                                                              │")
	fmt.Println("├──────────────┬──────────────────────────┬──────────────┬──────────────┬───────────┬───────┤")
	fmt.Println("│     状态      │       RAID 卡型号         │   缓存大小    │   条带大小    │ RAID 级别  │ 电池  │")
	fmt.Println("├──────────────┼──────────────────────────┼──────────────┼──────────────┼───────────┼───────┤")

	raidStatus := "未检测到"
	raidModel := "-"
	raidCache := "-"
	raidStripe := fmt.Sprintf("%d KB", info.RAIDInfo.StripeSize)
	raidLevel := "-"
	batteryBackup := "-"

	if info.RAIDInfo.HasRAID {
		raidStatus = "已检测"
		if info.RAIDInfo.RAIDModel != "" {
			raidModel = info.RAIDInfo.RAIDModel
			if len(raidModel) > 24 {
				raidModel = raidModel[:21] + "..."
			}
		}
		if info.RAIDInfo.CacheSize > 0 {
			raidCache = utils.FormatBytes(info.RAIDInfo.CacheSize)
		}
		if info.RAIDInfo.RAIDLevel != "" {
			raidLevel = info.RAIDInfo.RAIDLevel
		}
		if info.RAIDInfo.BatteryBackup {
			batteryBackup = "支持"
		} else {
			batteryBackup = "不支持"
		}
	}

	raidRow := fmt.Sprintf("│ %-12s │ %-24s │ %-12s │ %-12s │ %-9s │ %-5s │",
		raidStatus,
		raidModel,
		raidCache,
		raidStripe,
		raidLevel,
		batteryBackup)
	fmt.Println(raidRow)
	fmt.Println("└──────────────┴──────────────────────────┴──────────────┴──────────────┴───────────┴───────┘")
	fmt.Println()

	// 网卡信息表格
	fmt.Println("┌────────────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│  📌 网卡信息                                                                                       │")
	fmt.Println("├────────────┬──────────────┬─────────┬─────────┬─────────┬────────────┬────────────┬───────────────┤")
	fmt.Println("│   设备名    │     速率      │   MTU   │  队列   │  绑定   │  绑定模式   │    驱动     │   MAC 地址     │")
	fmt.Println("├────────────┼──────────────┼─────────┼─────────┼─────────┼────────────┼────────────┼───────────────┤")

	if len(info.NICInfos) == 0 {
		fmt.Println("│                                            未检测到网卡                                              │")
	} else {
		for _, nic := range info.NICInfos {
			speedStr := fmt.Sprintf("%d Mbps", nic.Speed)
			if nic.Speed <= 0 {
				speedStr = "未知"
			}
			bondStatus := "否"
			bondMode := "-"
			if nic.IsBond {
				bondStatus = "是"
				bondMode = nic.BondMode
				if len(bondMode) > 10 {
					bondMode = bondMode[:7] + "..."
				}
			}
			driver := nic.Driver
			if len(driver) > 10 {
				driver = driver[:7] + "..."
			}
			macAddr := nic.MACAddress
			if len(macAddr) > 13 {
				macAddr = macAddr[:10] + "..."
			}
			nicRow := fmt.Sprintf("│ %-10s │ %-12s │ %-7d │ %-7d │ %-7s │ %-10s │ %-10s │ %-13s │",
				nic.Name,
				speedStr,
				nic.MTU,
				nic.QueueSize,
				bondStatus,
				bondMode,
				driver,
				macAddr)
			fmt.Println(nicRow)
		}
	}
	fmt.Println("└────────────┴──────────────┴─────────┴─────────┴─────────┴────────────┴────────────┴───────────────┘")
	fmt.Println()
}
