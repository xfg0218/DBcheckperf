// Package reporter 提供报告生成功能
// 支持表格、HTML、JSON 等多种格式的输出
package reporter

import (
	"fmt"
	"os"
	"strings"
	"time"

	"dbcheckperf/pkg/checker"
)

// HTMLReporter HTML 报告生成器
type HTMLReporter struct {
	// Title 报告标题
	Title string
	// OutputFile 输出文件路径
	OutputFile string
	// Verbose 是否显示详细输出
	Verbose bool
}

// NewHTMLReporter 创建新的 HTML 报告生成器
func NewHTMLReporter(outputFile string, verbose bool) *HTMLReporter {
	if outputFile == "" {
		outputFile = fmt.Sprintf("dbcheckperf_report_%s.html", time.Now().Format("20060102_150405"))
	}
	return &HTMLReporter{
		Title:      "dbcheckperf 性能测试报告",
		OutputFile: outputFile,
		Verbose:    verbose,
	}
}

// GenerateReport 生成完整的 HTML 报告
func (hr *HTMLReporter) GenerateReport(
	systemInfos []*checker.SystemInfo,
	diskResults []*checker.DiskResult,
	streamResults []*checker.StreamResult,
	networkResults []checker.NetworkResult,
	latencyResults []*checker.LatencyResult,
	hardwareInfos []*checker.HardwareInfo,
) error {
	var html strings.Builder

	// HTML 头部
	html.WriteString(hr.generateHeader())

	// 报告概览
	html.WriteString(hr.generateSummary(systemInfos, diskResults, streamResults, networkResults))

	// 系统信息
	if len(systemInfos) > 0 {
		html.WriteString(hr.generateSystemInfoSection(systemInfos))
	}

	// 硬件信息
	if len(hardwareInfos) > 0 {
		html.WriteString(hr.generateHardwareInfoSection(hardwareInfos))
	}

	// 磁盘 I/O 测试结果
	if len(diskResults) > 0 {
		html.WriteString(hr.generateDiskResultsSection(diskResults))
	}

	// 内存带宽测试结果
	if len(streamResults) > 0 {
		html.WriteString(hr.generateStreamResultsSection(streamResults))
	}

	// 网络测试结果
	if len(networkResults) > 0 {
		html.WriteString(hr.generateNetworkResultsSection(networkResults))
	}

	// 延迟测试结果
	if len(latencyResults) > 0 {
		html.WriteString(hr.generateLatencyResultsSection(latencyResults))
	}

	// HTML 尾部
	html.WriteString(hr.generateFooter())

	// 写入文件
	if err := os.WriteFile(hr.OutputFile, []byte(html.String()), 0644); err != nil {
		return fmt.Errorf("写入 HTML 报告失败：%v", err)
	}

	if hr.Verbose {
		fmt.Printf("HTML 报告已生成：%s\n", hr.OutputFile)
	}

	return nil
}

// generateHeader 生成 HTML 头部
func (hr *HTMLReporter) generateHeader() string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + hr.Title + `</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: #fff;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            overflow: hidden;
        }
        header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 40px;
            text-align: center;
        }
        header h1 {
            font-size: 2.5em;
            margin-bottom: 10px;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.2);
        }
        header p {
            font-size: 1.1em;
            opacity: 0.9;
        }
        .timestamp {
            background: rgba(255,255,255,0.2);
            padding: 8px 16px;
            border-radius: 20px;
            display: inline-block;
            margin-top: 15px;
            font-size: 0.9em;
        }
        section {
            padding: 30px 40px;
            border-bottom: 1px solid #eee;
        }
        section:last-child { border-bottom: none; }
        h2 {
            color: #667eea;
            font-size: 1.8em;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 3px solid #667eea;
        }
        h3 {
            color: #764ba2;
            font-size: 1.3em;
            margin: 20px 0 15px 0;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
            font-size: 0.95em;
        }
        th, td {
            padding: 12px 15px;
            text-align: left;
            border: 1px solid #ddd;
        }
        th {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            font-weight: 600;
            text-transform: uppercase;
            font-size: 0.85em;
            letter-spacing: 0.5px;
        }
        tr:nth-child(even) { background-color: #f8f9fa; }
        tr:hover { background-color: #e9ecef; }
        .metric-card {
            background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
            border-radius: 8px;
            padding: 20px;
            margin: 15px 0;
            border-left: 4px solid #667eea;
        }
        .metric-card h4 {
            color: #667eea;
            margin-bottom: 10px;
        }
        .metric-value {
            font-size: 2em;
            font-weight: bold;
            color: #764ba2;
        }
        .metric-unit {
            font-size: 0.5em;
            color: #666;
        }
        .status-good { color: #28a745; font-weight: bold; }
        .status-warning { color: #ffc107; font-weight: bold; }
        .status-bad { color: #dc3545; font-weight: bold; }
        .badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 0.85em;
            font-weight: 600;
        }
        .badge-primary { background: #667eea; color: white; }
        .badge-success { background: #28a745; color: white; }
        .badge-info { background: #17a2b8; color: white; }
        .grid-2 {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 20px;
        }
        .grid-3 {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 20px;
        }
        @media (max-width: 768px) {
            .grid-2, .grid-3 { grid-template-columns: 1fr; }
            body { padding: 10px; }
            section { padding: 20px; }
        }
        footer {
            background: #f8f9fa;
            padding: 20px 40px;
            text-align: center;
            color: #666;
            font-size: 0.9em;
        }
        .progress-bar {
            background: #e9ecef;
            border-radius: 10px;
            height: 20px;
            overflow: hidden;
            margin: 10px 0;
        }
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #667eea 0%, #764ba2 100%);
            transition: width 0.3s ease;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>` + hr.Title + `</h1>
            <p>服务器性能基准测试</p>
            <div class="timestamp">
                📅 生成时间：` + time.Now().Format("2006-01-02 15:04:05") + `
            </div>
        </header>
`
}

// generateSummary 生成报告概览
func (hr *HTMLReporter) generateSummary(
	systemInfos []*checker.SystemInfo,
	diskResults []*checker.DiskResult,
	streamResults []*checker.StreamResult,
	networkResults []checker.NetworkResult,
) string {
	var sb strings.Builder

	sb.WriteString(`<section id="summary">`)
	sb.WriteString(`<h2>📊 报告概览</h2>`)
	sb.WriteString(`<div class="grid-3">`)

	// 磁盘 I/O 概览
	if len(diskResults) > 0 {
		writeAvg, readAvg := hr.calculateAvgDiskIO(diskResults)
		sb.WriteString(`<div class="metric-card">`)
		sb.WriteString(`<h4>💾 磁盘 I/O</h4>`)
		sb.WriteString(fmt.Sprintf(`<div class="metric-value">%.1f<span class="metric-unit"> MB/s</span></div>`, writeAvg))
		sb.WriteString(fmt.Sprintf(`<p>平均写入：%.1f MB/s | 平均读取：%.1f MB/s</p>`, writeAvg, readAvg))
		sb.WriteString(`</div>`)
	}

	// 内存带宽概览
	if len(streamResults) > 0 {
		avgBandwidth := hr.calculateAvgMemory(streamResults)
		sb.WriteString(`<div class="metric-card">`)
		sb.WriteString(`<h4>🧠 内存带宽</h4>`)
		sb.WriteString(fmt.Sprintf(`<div class="metric-value">%.1f<span class="metric-unit"> MB/s</span></div>`, avgBandwidth))
		sb.WriteString(fmt.Sprintf(`<p>平均总带宽：%.1f MB/s</p>`, avgBandwidth))
		sb.WriteString(`</div>`)
	}

	// 网络性能概览
	if len(networkResults) > 0 {
		avgNetwork := hr.calculateAvgNetwork(networkResults)
		sb.WriteString(`<div class="metric-card">`)
		sb.WriteString(`<h4>🌐 网络性能</h4>`)
		sb.WriteString(fmt.Sprintf(`<div class="metric-value">%.1f<span class="metric-unit"> MB/s</span></div>`, avgNetwork))
		sb.WriteString(fmt.Sprintf(`<p>平均带宽：%.1f MB/s</p>`, avgNetwork))
		sb.WriteString(`</div>`)
	}

	sb.WriteString(`</div>`)

	// 测试主机统计
	sb.WriteString(`<div class="metric-card">`)
	sb.WriteString(`<h4>🖥️ 测试主机统计</h4>`)
	sb.WriteString(fmt.Sprintf(`<p>系统信息：%d 台 | 磁盘测试：%d 次 | 内存测试：%d 次 | 网络测试：%d 次</p>`,
		len(systemInfos), len(diskResults), len(streamResults), len(networkResults)))
	sb.WriteString(`</div>`)

	sb.WriteString(`</section>`)

	return sb.String()
}

// generateSystemInfoSection 生成系统信息部分
func (hr *HTMLReporter) generateSystemInfoSection(systemInfos []*checker.SystemInfo) string {
	var sb strings.Builder

	sb.WriteString(`<section id="system-info">`)
	sb.WriteString(`<h2>🖥️ 系统信息</h2>`)
	sb.WriteString(`<table>`)
	sb.WriteString(`<thead>`)
	sb.WriteString(`<tr>`)
	sb.WriteString(`<th>主机名</th><th>虚拟化</th><th>CPU 型号</th><th>核心数</th>`)
	sb.WriteString(`<th>内存大小</th><th>磁盘大小</th><th>服务器厂家</th><th>操作系统</th><th>内核版本</th><th>网卡速率</th>`)
	sb.WriteString(`</tr>`)
	sb.WriteString(`</thead>`)
	sb.WriteString(`<tbody>`)

	for _, info := range systemInfos {
		sb.WriteString(`<tr>`)
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, info.Host))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, getVirtualType(info)))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, info.CPUModel))
		sb.WriteString(fmt.Sprintf(`<td>%d</td>`, info.CPUCore))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, formatBytes(info.TotalRAM)))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, formatBytes(info.DiskSize)))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, "未知")) // 服务器厂家（需要额外传递）
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, info.OSName))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, info.KernelVersion))
		sb.WriteString(fmt.Sprintf(`<td>%d Mbps</td>`, info.NICSpeed))
		sb.WriteString(`</tr>`)
	}

	sb.WriteString(`</tbody>`)
	sb.WriteString(`</table>`)
	sb.WriteString(`</section>`)

	return sb.String()
}

// generateHardwareInfoSection 生成硬件信息部分
func (hr *HTMLReporter) generateHardwareInfoSection(hardwareInfos []*checker.HardwareInfo) string {
	var sb strings.Builder

	sb.WriteString(`<section id="hardware-info">`)
	sb.WriteString(`<h2>🔧 硬件详细信息</h2>`)

	for i, hw := range hardwareInfos {
		sb.WriteString(fmt.Sprintf(`<h3>主机 %d: %s</h3>`, i+1, hw.Host))

		// OS 信息 - 新增
		sb.WriteString(`<div class="metric-card">`)
		sb.WriteString(`<h4>OS 信息</h4>`)
		osType := hw.OSType
		if osType == "" {
			osType = "未知"
		}
		serverVendor := hw.ServerVendor
		if serverVendor == "" {
			serverVendor = "未知"
		}
		serverModel := hw.ServerModel
		if serverModel == "" {
			serverModel = "未知"
		}
		architecture := hw.Architecture
		if architecture == "" {
			architecture = "未知"
		}
		sb.WriteString(fmt.Sprintf(`<p>OS 类型：%s | 服务器厂家：%s | 服务器型号：%s | 架构：%s</p>`,
			osType, serverVendor, serverModel, architecture))
		sb.WriteString(`</div>`)

		// CPU 信息
		if hw.CPUInfo != nil {
			sb.WriteString(`<div class="metric-card">`)
			sb.WriteString(`<h4>CPU 信息</h4>`)
			sb.WriteString(fmt.Sprintf(`<p>型号：%s | 核心数：%d | 插槽数：%d</p>`, hw.CPUInfo.Model, hw.CPUInfo.Cores, hw.CPUInfo.Sockets))
			sb.WriteString(fmt.Sprintf(`<p>基准频率：%d MHz | 睿频：%d MHz | NUMA 节点数：%d</p>`,
				hw.CPUInfo.BaseFreq, hw.CPUInfo.TurboFreq, hw.CPUInfo.NUMANodes))
			sb.WriteString(`</div>`)
		}

		// 磁盘信息
		if len(hw.DiskInfos) > 0 {
			sb.WriteString(`<h4>磁盘信息</h4>`)
			sb.WriteString(`<table>`)
			sb.WriteString(`<thead><tr><th>设备</th><th>型号</th><th>类型</th><th>容量</th><th>块大小</th><th>调度算法</th></tr></thead>`)
			sb.WriteString(`<tbody>`)
			for _, disk := range hw.DiskInfos {
				sb.WriteString(`<tr>`)
				sb.WriteString(fmt.Sprintf(`<td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%d 字节</td><td>%s</td>`,
					disk.Name, disk.Model, disk.Type, formatBytes(disk.Size), disk.BlockSize, disk.Scheduler))
				sb.WriteString(`</tr>`)
			}
			sb.WriteString(`</tbody>`)
			sb.WriteString(`</table>`)
		}

		// 网卡信息
		if len(hw.NICInfos) > 0 {
			sb.WriteString(`<h4>网卡信息</h4>`)
			sb.WriteString(`<table>`)
			sb.WriteString(`<thead><tr><th>设备</th><th>速率</th><th>MTU</th><th>绑定状态</th><th>驱动</th></tr></thead>`)
			sb.WriteString(`<tbody>`)
			for _, nic := range hw.NICInfos {
				sb.WriteString(`<tr>`)
				sb.WriteString(fmt.Sprintf(`<td>%s</td><td>%d Mbps</td><td>%d</td><td>%v</td><td>%s</td>`,
					nic.Name, nic.Speed, nic.MTU, nic.IsBond, nic.Driver))
				sb.WriteString(`</tr>`)
			}
			sb.WriteString(`</tbody>`)
			sb.WriteString(`</table>`)
		}
	}

	sb.WriteString(`</section>`)

	return sb.String()
}

// generateDiskResultsSection 生成磁盘测试结果部分
func (hr *HTMLReporter) generateDiskResultsSection(diskResults []*checker.DiskResult) string {
	var sb strings.Builder

	sb.WriteString(`<section id="disk-results">`)
	sb.WriteString(`<h2>💾 磁盘 I/O 测试结果</h2>`)
	sb.WriteString(`<table>`)
	sb.WriteString(`<thead>`)
	sb.WriteString(`<tr>`)
	sb.WriteString(`<th>主机</th><th>测试目录</th><th>写入带宽</th><th>读取带宽</th>`)
	sb.WriteString(`<th>随机写入</th><th>随机读取</th><th>块大小</th><th>测试时间</th>`)
	sb.WriteString(`</tr>`)
	sb.WriteString(`</thead>`)
	sb.WriteString(`<tbody>`)

	for _, result := range diskResults {
		sb.WriteString(`<tr>`)
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, result.Host))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, result.Dir))
		sb.WriteString(fmt.Sprintf(`<td class="status-good">%.2f MB/s</td>`, result.WriteBandwidth))
		sb.WriteString(fmt.Sprintf(`<td class="status-good">%.2f MB/s</td>`, result.ReadBandwidth))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, result.RandWriteBandwidth))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, result.RandReadBandwidth))
		sb.WriteString(fmt.Sprintf(`<td>%d KB</td>`, result.BlockSize/1024))
		sb.WriteString(fmt.Sprintf(`<td>%.2f s</td>`, result.WriteTime))
		sb.WriteString(`</tr>`)
	}

	sb.WriteString(`</tbody>`)
	sb.WriteString(`</table>`)
	sb.WriteString(`</section>`)

	return sb.String()
}

// generateStreamResultsSection 生成内存带宽测试结果部分
func (hr *HTMLReporter) generateStreamResultsSection(streamResults []*checker.StreamResult) string {
	var sb strings.Builder

	sb.WriteString(`<section id="stream-results">`)
	sb.WriteString(`<h2>🧠 内存带宽测试结果</h2>`)
	sb.WriteString(`<table>`)
	sb.WriteString(`<thead>`)
	sb.WriteString(`<tr>`)
	sb.WriteString(`<th>主机</th><th>Copy</th><th>Scale</th><th>Add</th><th>Triad</th><th>总带宽</th>`)
	sb.WriteString(`</tr>`)
	sb.WriteString(`</thead>`)
	sb.WriteString(`<tbody>`)

	// 计算汇总统计
	var totalCopy, totalScale, totalAdd, totalTriad, totalSum float64

	for _, result := range streamResults {
		sb.WriteString(`<tr>`)
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, result.Host))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, result.CopyBandwidth))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, result.ScaleBandwidth))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, result.AddBandwidth))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, result.TriadBandwidth))
		sb.WriteString(fmt.Sprintf(`<td class="status-good">%.2f MB/s</td>`, result.TotalBandwidth))
		sb.WriteString(`</tr>`)

		totalCopy += result.CopyBandwidth
		totalScale += result.ScaleBandwidth
		totalAdd += result.AddBandwidth
		totalTriad += result.TriadBandwidth
		totalSum += result.TotalBandwidth
	}

	// 添加汇总行
	n := float64(len(streamResults))
	if n > 0 {
		sb.WriteString(`<tfoot>`)
		sb.WriteString(`<tr style="font-weight: bold; background: #e9ecef;">`)
		sb.WriteString(`<td>平均</td>`)
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, totalCopy/n))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, totalScale/n))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, totalAdd/n))
		sb.WriteString(fmt.Sprintf(`<td>%.2f MB/s</td>`, totalTriad/n))
		sb.WriteString(fmt.Sprintf(`<td class="status-good">%.2f MB/s</td>`, totalSum/n))
		sb.WriteString(`</tr>`)
		sb.WriteString(`</tfoot>`)
	}

	sb.WriteString(`</tbody>`)
	sb.WriteString(`</table>`)
	sb.WriteString(`</section>`)

	return sb.String()
}

// generateNetworkResultsSection 生成网络测试结果部分
func (hr *HTMLReporter) generateNetworkResultsSection(networkResults []checker.NetworkResult) string {
	var sb strings.Builder

	sb.WriteString(`<section id="network-results">`)
	sb.WriteString(`<h2>🌐 网络测试结果</h2>`)
	sb.WriteString(`<table>`)
	sb.WriteString(`<thead>`)
	sb.WriteString(`<tr>`)
	sb.WriteString(`<th>源主机</th><th>目标主机</th><th>带宽</th><th>延迟</th><th>丢包率</th>`)
	sb.WriteString(`</tr>`)
	sb.WriteString(`</thead>`)
	sb.WriteString(`<tbody>`)

	for _, result := range networkResults {
		sb.WriteString(`<tr>`)
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, result.SourceHost))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, result.DestHost))
		sb.WriteString(fmt.Sprintf(`<td class="status-good">%.2f MB/s</td>`, result.Bandwidth))
		sb.WriteString(fmt.Sprintf(`<td>%.2f ms</td>`, result.LatencyAvg))
		sb.WriteString(fmt.Sprintf(`<td>%.2f%%</td>`, result.PacketLoss))
		sb.WriteString(`</tr>`)
	}

	sb.WriteString(`</tbody>`)
	sb.WriteString(`</table>`)
	sb.WriteString(`</section>`)

	return sb.String()
}

// generateLatencyResultsSection 生成延迟测试结果部分
func (hr *HTMLReporter) generateLatencyResultsSection(latencyResults []*checker.LatencyResult) string {
	var sb strings.Builder

	sb.WriteString(`<section id="latency-results">`)
	sb.WriteString(`<h2>⏱️ 磁盘延迟测试结果</h2>`)
	sb.WriteString(`<table>`)
	sb.WriteString(`<thead>`)
	sb.WriteString(`<tr>`)
	sb.WriteString(`<th>主机</th><th>测试目录</th>`)
	sb.WriteString(`<th>读延迟 (平均)</th><th>读延迟 (最大)</th>`)
	sb.WriteString(`<th>写延迟 (平均)</th><th>写延迟 (最大)</th>`)
	sb.WriteString(`<th>读 IOPS</th><th>写 IOPS</th>`)
	sb.WriteString(`</tr>`)
	sb.WriteString(`</thead>`)
	sb.WriteString(`<tbody>`)

	for _, result := range latencyResults {
		sb.WriteString(`<tr>`)
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, result.Host))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, result.Dir))
		sb.WriteString(fmt.Sprintf(`<td>%.2f ms</td>`, result.ReadLatencyAvg))
		sb.WriteString(fmt.Sprintf(`<td>%.2f ms</td>`, result.ReadLatencyMax))
		sb.WriteString(fmt.Sprintf(`<td>%.2f ms</td>`, result.WriteLatencyAvg))
		sb.WriteString(fmt.Sprintf(`<td>%.2f ms</td>`, result.WriteLatencyMax))
		sb.WriteString(fmt.Sprintf(`<td>%.0f</td>`, result.ReadIOPS))
		sb.WriteString(fmt.Sprintf(`<td>%.0f</td>`, result.WriteIOPS))
		sb.WriteString(`</tr>`)
	}

	sb.WriteString(`</tbody>`)
	sb.WriteString(`</table>`)
	sb.WriteString(`</section>`)

	return sb.String()
}

// generateFooter 生成 HTML 尾部
func (hr *HTMLReporter) generateFooter() string {
	return `
        <footer>
            <p>dbcheckperf - 数据库性能检查工具</p>
            <p>Generated by dbcheckperf HTML Reporter</p>
        </footer>
    </div>
</body>
</html>
`
}

// 辅助函数

func (hr *HTMLReporter) calculateAvgDiskIO(results []*checker.DiskResult) (writeAvg, readAvg float64) {
	if len(results) == 0 {
		return 0, 0
	}
	for _, r := range results {
		writeAvg += r.WriteBandwidth
		readAvg += r.ReadBandwidth
	}
	return writeAvg / float64(len(results)), readAvg / float64(len(results))
}

func (hr *HTMLReporter) calculateAvgMemory(results []*checker.StreamResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var total float64
	for _, r := range results {
		total += r.TotalBandwidth
	}
	return total / float64(len(results))
}

func (hr *HTMLReporter) calculateAvgNetwork(results []checker.NetworkResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var total float64
	for _, r := range results {
		total += r.Bandwidth
	}
	return total / float64(len(results))
}

// getVirtualType 获取虚拟化类型显示字符串
func getVirtualType(info *checker.SystemInfo) string {
	if !info.IsVirtual {
		return "物理机"
	}
	if info.VirtualType == "" {
		return "虚拟机"
	}
	return info.VirtualType
}

// formatBytes 格式化字节数显示
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
