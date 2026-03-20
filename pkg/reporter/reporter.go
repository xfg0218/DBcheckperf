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
	// BlockSize 顺序读写块大小（KB）
	BlockSize int
	// RandBlockSize 随机读写块大小（KB），0 表示使用 BlockSize
	RandBlockSize int
}

// NewReporter 创建新的报告生成器
func NewReporter(displayPerHost, verbose bool) *Reporter {
	return &Reporter{
		DisplayPerHost: displayPerHost,
		Verbose:        verbose,
		BlockSize:      32, // 默认 32KB
		RandBlockSize:  0,  // 默认 0，表示使用 BlockSize
	}
}

// SetBlockSize 设置块大小
func (r *Reporter) SetBlockSize(sizeKB int) {
	r.BlockSize = sizeKB
}

// SetRandBlockSize 设置随机读写块大小
func (r *Reporter) SetRandBlockSize(sizeKB int) {
	r.RandBlockSize = sizeKB
}

// getRandBlockSize 获取实际使用的随机块大小
func (r *Reporter) getRandBlockSize() int {
	if r.RandBlockSize > 0 {
		return r.RandBlockSize
	}
	return r.BlockSize
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
		"主机名", "时间 (秒)", "数据量", "速度 (MB/s)")
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
		"主机名", "时间 (秒)", "数据量", "速度 (MB/s)")
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
			"IP 地址", "复制 (MB/s)", "缩放 (MB/s)", "加法 (MB/s)", "三合一 (MB/s)", "总计 (MB/s)")
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
		randWriteAgg := checker.AggregateDiskRandResults(diskResults, true)
		randReadAgg := checker.AggregateDiskRandResults(diskResults, false)
		fmt.Printf("║  顺序写入带宽：%-42s ║\n", fmt.Sprintf("平均 %s, 最小 %s [%s]",
			utils.FormatBandwidth(writeAgg.AvgValue),
			utils.FormatBandwidth(writeAgg.MinValue),
			writeAgg.MinHost))
		fmt.Printf("║  顺序读取带宽：%-42s ║\n", fmt.Sprintf("平均 %s, 最小 %s [%s]",
			utils.FormatBandwidth(readAgg.AvgValue),
			utils.FormatBandwidth(readAgg.MinValue),
			readAgg.MinHost))
		fmt.Printf("║  随机写入带宽：%-42s ║\n", fmt.Sprintf("平均 %s, 最小 %s [%s]",
			utils.FormatBandwidth(randWriteAgg.AvgValue),
			utils.FormatBandwidth(randWriteAgg.MinValue),
			randWriteAgg.MinHost))
		fmt.Printf("║  随机读取带宽：%-42s ║\n", fmt.Sprintf("平均 %s, 最小 %s [%s]",
			utils.FormatBandwidth(randReadAgg.AvgValue),
			utils.FormatBandwidth(randReadAgg.MinValue),
			randReadAgg.MinHost))
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
			"顺序写入",
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
			"顺序读取",
			utils.FormatBandwidth(readAgg.AvgValue),
			utils.FormatBandwidth(readAgg.MinValue),
			utils.FormatBandwidth(readAgg.MaxValue),
			fmt.Sprintf("%d 次测试", readAgg.Count))
		fmt.Println(row)
	}

	// 随机写入
	if len(diskResults) > 0 {
		randWriteAgg := checker.AggregateDiskRandResults(diskResults, true)
		randLabel := fmt.Sprintf("随机写入")
		row := fmt.Sprintf("%-20s %-18s %-18s %-18s %-18s",
			randLabel,
			utils.FormatBandwidth(randWriteAgg.AvgValue),
			utils.FormatBandwidth(randWriteAgg.MinValue),
			utils.FormatBandwidth(randWriteAgg.MaxValue),
			fmt.Sprintf("%d 次测试", randWriteAgg.Count))
		fmt.Println(row)
	}

	// 随机读取
	if len(diskResults) > 0 {
		randReadAgg := checker.AggregateDiskRandResults(diskResults, false)
		randLabel := fmt.Sprintf("随机读取")
		row := fmt.Sprintf("%-20s %-18s %-18s %-18s %-18s",
			randLabel,
			utils.FormatBandwidth(randReadAgg.AvgValue),
			utils.FormatBandwidth(randReadAgg.MinValue),
			utils.FormatBandwidth(randReadAgg.MaxValue),
			fmt.Sprintf("%d 次测试", randReadAgg.Count))
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

// PrintHardwareResults 打印硬件信息表格（竖行展示，支持多主机）
func (r *Reporter) PrintHardwareResults(infos []*checker.RemoteHardwareInfo) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  硬件信息报告")
	fmt.Println("====================")
	fmt.Println()

	// 对每台主机显示硬件信息
	for i, remoteInfo := range infos {
		// 显示分隔线（第一台主机前不显示）
		if i > 0 {
			fmt.Println(strings.Repeat("-", 60))
			fmt.Println()
		}

		// 显示错误信息（如果有）
		if remoteInfo.Error != nil {
			fmt.Printf("主机：%s\n", remoteInfo.Host)
			fmt.Printf("错误：%v\n", remoteInfo.Error)
			fmt.Println()
			continue
		}

		info := remoteInfo.HardwareInfo
		if info == nil {
			fmt.Printf("主机：%s\n", remoteInfo.Host)
			fmt.Println("错误：无硬件信息")
			fmt.Println()
			continue
		}

		// 显示主机名（多主机模式下）
		if len(infos) > 1 {
			fmt.Printf("主机：%s\n", remoteInfo.Host)
			fmt.Println()
		}

		// CPU 信息（竖行展示）
		fmt.Println("--- CPU 信息 ---")
		fmt.Println()
		cpuModel := info.CPUInfo.Model
		baseFreqStr := "-"
		if info.CPUInfo.BaseFreq > 0 {
			baseFreqStr = fmt.Sprintf("%d MHz", info.CPUInfo.BaseFreq)
		}
		turboFreqStr := "-"
		if info.CPUInfo.TurboFreq > 0 {
			turboFreqStr = fmt.Sprintf("%d MHz", info.CPUInfo.TurboFreq)
		}
		fmt.Printf("  CPU 型号：    %s\n", cpuModel)
		fmt.Printf("  CPU 核心数：   %d\n", info.CPUInfo.Cores)
		fmt.Printf("  CPU 插槽数：   %d\n", info.CPUInfo.Sockets)
		fmt.Printf("  基准频率：   %s\n", baseFreqStr)
		fmt.Printf("  睿频：       %s\n", turboFreqStr)
		fmt.Println()

		// 内存信息（竖行展示）
		fmt.Println("--- 内存信息 ---")
		fmt.Println()
		memType := info.MemoryInfo.MemoryType
		if memType == "" || memType == "Unknown" {
			memType = "-"
		}
		memSpeedStr := "-"
		if info.MemoryInfo.MemorySpeed > 0 {
			memSpeedStr = fmt.Sprintf("%d MHz", info.MemoryInfo.MemorySpeed)
		}
		fmt.Printf("  总内存：     %s\n", utils.FormatBytes(info.MemoryInfo.TotalMemory))
		fmt.Printf("  内存类型：   %s\n", memType)
		fmt.Printf("  内存速度：   %s\n", memSpeedStr)
		fmt.Printf("  插槽数：     %d\n", info.MemoryInfo.MemorySlots)
		fmt.Println()

		// 磁盘信息（竖行展示）
		fmt.Println("--- 磁盘信息 ---")
		fmt.Println()
		if len(info.DiskInfos) == 0 {
			fmt.Println("  未检测到物理磁盘")
		} else {
			for j, disk := range info.DiskInfos {
				if j > 0 {
					fmt.Println()
				}
				vendor := disk.Vendor
				if vendor == "" {
					vendor = "-"
				}
				rotational := "否"
				if disk.Rotational {
					rotational = "是"
				}
				fmt.Printf("  设备名：   %s\n", disk.Name)
				fmt.Printf("  型号：     %s\n", disk.Model)
				fmt.Printf("  厂家：     %s\n", vendor)
				fmt.Printf("  类型：     %s\n", disk.Type)
				fmt.Printf("  大小：     %s\n", utils.FormatBytes(disk.Size))
				fmt.Printf("  旋转磁盘： %s\n", rotational)
			}
		}
		fmt.Println()

		// RAID 信息（竖行展示）
		fmt.Println("--- RAID 信息 ---")
		fmt.Println()
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

		fmt.Printf("  状态：       %s\n", raidStatus)
		fmt.Printf("  RAID 型号：   %s\n", raidModel)
		fmt.Printf("  缓存大小：   %s\n", raidCache)
		fmt.Printf("  条带大小：   %s\n", raidStripe)
		fmt.Printf("  RAID 级别：   %s\n", raidLevel)
		fmt.Printf("  电池备份：   %s\n", batteryBackup)
		fmt.Println()

		// 网卡信息（竖行展示）
		fmt.Println("--- 网卡信息 ---")
		fmt.Println()
		if len(info.NICInfos) == 0 {
			fmt.Println("  未检测到网卡")
		} else {
			for j, nic := range info.NICInfos {
				if j > 0 {
					fmt.Println()
				}
				speedStr := fmt.Sprintf("%d Mbps", nic.Speed)
				if nic.Speed <= 0 {
					speedStr = "未知"
				}
				bondStatus := "否"
				bondMode := "-"
				if nic.IsBond {
					bondStatus = "是"
					bondMode = nic.BondMode
				}
				fmt.Printf("  设备名：   %s\n", nic.Name)
				fmt.Printf("  速率：     %s\n", speedStr)
				fmt.Printf("  MTU:       %d\n", nic.MTU)
				fmt.Printf("  队列大小： %d\n", nic.QueueSize)
				fmt.Printf("  绑定：     %s\n", bondStatus)
				fmt.Printf("  绑定模式： %s\n", bondMode)
				fmt.Printf("  驱动：     %s\n", nic.Driver)
				fmt.Printf("  MAC 地址：  %s\n", nic.MACAddress)
			}
		}
		fmt.Println()
	}
}

// PrintLatencyResults 打印延迟测试结果
func (r *Reporter) PrintLatencyResults(results []*checker.LatencyResult) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  磁盘延迟和 IOPS 测试结果")
	fmt.Println("====================")
	fmt.Println()

	// 打印表头
	header := fmt.Sprintf("%-20s %-10s %-12s %-12s %-12s %-12s %-12s %-12s %-12s",
		"主机名", "块大小", "读延迟 (ms)", "写延迟 (ms)", "读 IOPS", "写 IOPS",
		"读延迟最大", "写延迟最大", "测试目录")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 120))

	// 打印每行数据
	for _, result := range results {
		blockSize := fmt.Sprintf("%dKB", result.BlockSize/1024)
		row := fmt.Sprintf("%-20s %-10s %-12.3f %-12.3f %-12.0f %-12.0f %-12.3f %-12.3f %-12s",
			result.Host,
			blockSize,
			result.ReadLatencyAvg,
			result.WriteLatencyAvg,
			result.ReadIOPS,
			result.WriteIOPS,
			result.ReadLatencyMax,
			result.WriteLatencyMax,
			result.Dir)
		fmt.Println(row)
	}

	fmt.Println()
}

// PrintIOStatsResults 打印 IO 统计结果
func (r *Reporter) PrintIOStatsResults(stats []*checker.IOStats) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  IO 统计信息")
	fmt.Println("====================")
	fmt.Println()

	// 打印表头
	header := fmt.Sprintf("%-20s %-10s %-10s %-10s %-10s %-10s %-10s %-10s %-10s %-8s",
		"设备", "读/s", "写/s", "读 KB/s", "写 KB/s", "等待 (ms)", "利用率", "队列长", "服务 (ms)", "饱和")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 110))

	// 打印每行数据
	for _, stat := range stats {
		utilization := fmt.Sprintf("%.1f%%", stat.Utilization)
		saturation := ""
		if stat.Utilization > 80 {
			saturation = "⚠️"
		}
		row := fmt.Sprintf("%-20s %-10.1f %-10.1f %-10.1f %-10.1f %-10.2f %-10s %-10.2f %-10.2f %-8s",
			stat.Device,
			stat.ReadPerSec,
			stat.WritePerSec,
			stat.ReadKBPerSec,
			stat.WriteKBPerSec,
			stat.Await,
			utilization,
			stat.AvgQueueLength,
			stat.Svctm,
			saturation)
		fmt.Println(row)
	}

	fmt.Println()
}

// PrintNUMAInfo 打印 NUMA 信息
func (r *Reporter) PrintNUMAInfo(info *checker.NUMAInfo) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  NUMA 信息")
	fmt.Println("====================")
	fmt.Println()

	fmt.Printf("主机名：%s\n", info.Host)
	fmt.Printf("NUMA 节点数：%d\n", info.NodeCount)
	fmt.Printf("总内存：%s\n", utils.FormatBytes(info.TotalMemory))
	fmt.Println()

	for i := 0; i < info.NodeCount; i++ {
		fmt.Printf("节点 %d:\n", i)
		fmt.Printf("  CPU 核心数：%d\n", info.CPUPerNode[i])
		fmt.Printf("  内存：%s\n", utils.FormatBytes(info.MemoryPerNode[i]))
		fmt.Printf("  空闲内存：%s\n", utils.FormatBytes(info.FreeMemoryPerNode[i]))
		fmt.Println()
	}

	if len(info.CPUDistance) > 0 {
		fmt.Println("CPU 距离矩阵:")
		for i, row := range info.CPUDistance {
			fmt.Printf("  节点 %d: %v\n", i, row)
		}
		fmt.Println()
	}
}

// PrintKernelParams 打印内核参数
func (r *Reporter) PrintKernelParams(params *checker.KernelParams) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  内核参数")
	fmt.Println("====================")
	fmt.Println()

	fmt.Printf("主机名：%s\n\n", params.Host)

	fmt.Println("VM 参数:")
	fmt.Printf("  dirty_ratio:              %d%%\n", params.VM.DirtyRatio)
	fmt.Printf("  dirty_background_ratio:   %d%%\n", params.VM.DirtyBackgroundRatio)
	fmt.Printf("  swappiness:               %d\n", params.VM.Swappiness)
	fmt.Printf("  overcommit_memory:        %d\n", params.VM.OvercommitMemory)
	fmt.Printf("  min_free_kbytes:          %d KB\n", params.VM.MinFreeKbytes)
	fmt.Printf("  transparent_hugepage:     %s\n", params.VM.TransparentHugepage)
	fmt.Printf("  numa_balancing:           %d\n", params.VM.NumaBalancing)
	fmt.Println()

	fmt.Println("IO 参数:")
	fmt.Printf("  read_ahead_kb:            %d KB\n", params.IO.ReadAheadKB)
	fmt.Printf("  max_sectors_kb:           %d KB\n", params.IO.MaxSectorsKB)
	fmt.Printf("  nr_requests:              %d\n", params.IO.NrRequests)
	fmt.Printf("  scheduler:                %s\n", params.IO.Scheduler)
	fmt.Println()

	fmt.Println("网络参数:")
	fmt.Printf("  somaxconn:                %d\n", params.Network.Somaxconn)
	fmt.Printf("  netdev_max_backlog:       %d\n", params.Network.NetdevMaxBacklog)
	fmt.Printf("  tcp_max_syn_backlog:      %d\n", params.Network.TcpMaxSynBacklog)
	fmt.Printf("  tcp_max_tw_buckets:         %d\n", params.Network.TcpMaxTwBuckets)
	fmt.Printf("  tcp_tw_reuse:             %d\n", params.Network.TcpTwReuse)
	fmt.Printf("  tcp_keepalive_time:       %d 秒\n", params.Network.TcpKeepaliveTime)
	fmt.Println()

	// 检查并显示警告
	warnings := r.checkKernelParams(params)
	if len(warnings) > 0 {
		fmt.Println("⚠️  警告:")
		for _, w := range warnings {
			fmt.Printf("  - %s\n", w)
		}
		fmt.Println()
	}
}

// checkKernelParams 检查内核参数是否合理
func (r *Reporter) checkKernelParams(params *checker.KernelParams) []string {
	var warnings []string

	if params.VM.DirtyRatio > 40 {
		warnings = append(warnings, fmt.Sprintf("dirty_ratio=%d 过高，可能导致 IO 突发", params.VM.DirtyRatio))
	}

	if params.VM.Swappiness > 60 {
		warnings = append(warnings, fmt.Sprintf("swappiness=%d 过高，可能导致频繁 swap", params.VM.Swappiness))
	}

	if strings.Contains(params.VM.TransparentHugepage, "[always]") {
		warnings = append(warnings, "transparent_hugepage=always 可能影响数据库性能，建议设置为 madvise 或 never")
	}

	if params.VM.MinFreeKbytes < 102400 {
		warnings = append(warnings, fmt.Sprintf("min_free_kbytes=%d 过低，建议设置为 102400 以上", params.VM.MinFreeKbytes))
	}

	return warnings
}

// PrintNetworkQuality 打印网络质量结果
func (r *Reporter) PrintNetworkQuality(results []*checker.NetworkResult) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  网络质量测试")
	fmt.Println("====================")
	fmt.Println()

	// 打印表头
	header := fmt.Sprintf("%-20s %-20s %-12s %-12s %-12s %-10s %-10s %-8s",
		"源主机", "目标主机", "延迟 (ms)", "延迟最大", "丢包率", "MTU", "重传率", "质量")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 110))

	// 打印每行数据
	for _, result := range results {
		quality := "良好"
		if result.PacketLoss > 1 {
			quality = "差"
		} else if result.LatencyAvg > 10 {
			quality = "一般"
		}

		lossStr := fmt.Sprintf("%.2f%%", result.PacketLoss)
		if result.PacketLoss == 0 {
			lossStr = "0%"
		}

		row := fmt.Sprintf("%-20s %-20s %-12.2f %-12.2f %-12s %-10d %-10.2f %-8s",
			result.SourceHost,
			result.DestHost,
			result.LatencyAvg,
			result.LatencyMax,
			lossStr,
			result.MTU,
			result.TCPRetransmit,
			quality)
		fmt.Println(row)
	}

	fmt.Println()
}

// PrintDiskInfoResults 打印磁盘详细信息
func (r *Reporter) PrintDiskInfoResults(results []*checker.DiskResult) {
	fmt.Println()
	fmt.Println("====================")
	fmt.Println("==  磁盘详细信息")
	fmt.Println("====================")
	fmt.Println()

	// 打印表头
	header := fmt.Sprintf("%-20s %-12s %-8s %-20s %-15s %-15s %-10s %-10s",
		"主机名", "设备", "类型", "型号", "容量", "文件系统", "可用空间", "inode")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 120))

	// 打印每行数据
	for _, result := range results {
		diskType := result.DiskType
		if diskType == "" {
			diskType = "unknown"
		}

		model := result.DiskModel
		if len(model) > 18 {
			model = model[:15] + "..."
		}
		if model == "" {
			model = "unknown"
		}

		fs := result.FileSystem
		if fs == "" {
			fs = "unknown"
		}

		freeStr := utils.FormatBytes(result.FreeSpace)
		inodeStr := fmt.Sprintf("%.1f%%", result.InodeUsage)
		if result.InodeUsage == 0 {
			inodeStr = "-"
		}

		row := fmt.Sprintf("%-20s %-12s %-8s %-20s %-15s %-15s %-10s %-10s",
			result.Host,
			result.Dir,
			diskType,
			model,
			utils.FormatBytes(result.DiskCapacity),
			fs,
			freeStr,
			inodeStr)
		fmt.Println(row)
	}

	fmt.Println()
}
