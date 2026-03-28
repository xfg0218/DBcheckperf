// Package advisor 提供智能告警和优化建议功能
// 分析测试结果，检测性能问题并提供优化建议
package advisor

import (
	"fmt"
	"strings"

	"dbcheckperf/pkg/checker"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	// AlertInfo 信息
	AlertInfo AlertLevel = "INFO"
	// AlertWarning 警告
	AlertWarning AlertLevel = "WARNING"
	// AlertCritical 严重
	AlertCritical AlertLevel = "CRITICAL"
)

// Alert 告警信息
type Alert struct {
	// Level 告警级别
	Level AlertLevel `json:"level"`
	// Category 类别
	Category string `json:"category"`
	// Message 告警消息
	Message string `json:"message"`
	// CurrentValue 当前值
	CurrentValue string `json:"current_value"`
	// RecommendedValue 推荐值
	RecommendedValue string `json:"recommended_value"`
	// Suggestion 优化建议
	Suggestion string `json:"suggestion"`
}

// Advisor 智能顾问
type Advisor struct {
	// Verbose 是否显示详细输出
	Verbose bool
	// Thresholds 阈值配置
	Thresholds *ThresholdConfig
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	// Disk 磁盘阈值
	Disk *DiskThreshold `json:"disk"`
	// Memory 内存阈值
	Memory *MemoryThreshold `json:"memory"`
	// Network 网络阈值
	Network *NetworkThreshold `json:"network"`
	// Latency 延迟阈值
	Latency *LatencyThreshold `json:"latency"`
}

// DiskThreshold 磁盘阈值
type DiskThreshold struct {
	// MinWriteBandwidth 最小写入带宽 (MB/s)
	MinWriteBandwidth float64 `json:"min_write_bandwidth"`
	// MinReadBandwidth 最小读取带宽 (MB/s)
	MinReadBandwidth float64 `json:"min_read_bandwidth"`
	// MaxReadLatency 最大读延迟 (ms)
	MaxReadLatency float64 `json:"max_read_latency"`
	// MaxWriteLatency 最大写延迟 (ms)
	MaxWriteLatency float64 `json:"max_write_latency"`
}

// MemoryThreshold 内存阈值
type MemoryThreshold struct {
	// MinBandwidth 最小带宽 (MB/s)
	MinBandwidth float64 `json:"min_bandwidth"`
}

// NetworkThreshold 网络阈值
type NetworkThreshold struct {
	// MinBandwidth 最小带宽 (MB/s)
	MinBandwidth float64 `json:"min_bandwidth"`
	// MaxPacketLoss 最大丢包率 (%)
	MaxPacketLoss float64 `json:"max_packet_loss"`
}

// LatencyThreshold 延迟阈值
type LatencyThreshold struct {
	// MaxReadLatency 最大读延迟 (ms)
	MaxReadLatency float64 `json:"max_read_latency"`
	// MaxWriteLatency 最大写延迟 (ms)
	MaxWriteLatency float64 `json:"max_write_latency"`
}

// NewAdvisor 创建新的智能顾问
func NewAdvisor(verbose bool) *Advisor {
	return &Advisor{
		Verbose:    verbose,
		Thresholds: getDefaultThresholds(),
	}
}

// SetThresholds 设置自定义阈值
func (a *Advisor) SetThresholds(thresholds *ThresholdConfig) {
	a.Thresholds = thresholds
}

func getDefaultThresholds() *ThresholdConfig {
	return &ThresholdConfig{
		Disk: &DiskThreshold{
			MinWriteBandwidth: 100.0,  // 最低 100 MB/s
			MinReadBandwidth:  100.0,
			MaxReadLatency:    10.0,   // 最大 10ms
			MaxWriteLatency:   20.0,   // 最大 20ms
		},
		Memory: &MemoryThreshold{
			MinBandwidth: 10000.0,  // 最低 10 GB/s
		},
		Network: &NetworkThreshold{
			MinBandwidth:  100.0,    // 最低 100 MB/s
			MaxPacketLoss: 0.1,      // 最大 0.1% 丢包
		},
		Latency: &LatencyThreshold{
			MaxReadLatency:  5.0,    // 最大 5ms
			MaxWriteLatency: 10.0,   // 最大 10ms
		},
	}
}

// AnalyzeResults 分析测试结果
func (a *Advisor) AnalyzeResults(
	diskResults []*checker.DiskResult,
	streamResults []*checker.StreamResult,
	networkResults []checker.NetworkResult,
	latencyResults []*checker.LatencyResult,
) *AnalysisReport {
	report := &AnalysisReport{
		Alerts:    []Alert{},
		Score:     100,
		Summary:   "",
	}

	// 分析磁盘结果
	if len(diskResults) > 0 {
		diskAlerts := a.analyzeDiskResults(diskResults)
		report.Alerts = append(report.Alerts, diskAlerts...)
	}

	// 分析内存结果
	if len(streamResults) > 0 {
		memoryAlerts := a.analyzeStreamResults(streamResults)
		report.Alerts = append(report.Alerts, memoryAlerts...)
	}

	// 分析网络结果
	if len(networkResults) > 0 {
		networkAlerts := a.analyzeNetworkResults(networkResults)
		report.Alerts = append(report.Alerts, networkAlerts...)
	}

	// 分析延迟结果
	if len(latencyResults) > 0 {
		latencyAlerts := a.analyzeLatencyResults(latencyResults)
		report.Alerts = append(report.Alerts, latencyAlerts...)
	}

	// 计算总分
	report.calculateScore()

	// 生成总结
	report.generateSummary()

	return report
}

// AnalysisReport 分析报告
type AnalysisReport struct {
	// Alerts 告警列表
	Alerts []Alert `json:"alerts"`
	// Score 性能评分 (0-100)
	Score int `json:"score"`
	// Summary 总结
	Summary string `json:"summary"`
	// HealthStatus 健康状态
	HealthStatus string `json:"health_status"`
}

func (r *AnalysisReport) calculateScore() {
	score := 100

	for _, alert := range r.Alerts {
		switch alert.Level {
		case AlertCritical:
			score -= 20
		case AlertWarning:
			score -= 10
		case AlertInfo:
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}

	r.Score = score

	// 确定健康状态
	if score >= 90 {
		r.HealthStatus = "优秀"
	} else if score >= 70 {
		r.HealthStatus = "良好"
	} else if score >= 50 {
		r.HealthStatus = "一般"
	} else {
		r.HealthStatus = "需优化"
	}
}

func (r *AnalysisReport) generateSummary() {
	criticalCount := 0
	warningCount := 0

	for _, alert := range r.Alerts {
		if alert.Level == AlertCritical {
			criticalCount++
		} else if alert.Level == AlertWarning {
			warningCount++
		}
	}

	if criticalCount > 0 {
		r.Summary = fmt.Sprintf("检测到 %d 个严重问题和 %d 个警告，建议立即优化", criticalCount, warningCount)
	} else if warningCount > 0 {
		r.Summary = fmt.Sprintf("检测到 %d 个警告，性能总体良好", warningCount)
	} else {
		r.Summary = "性能表现优秀，未检测到问题"
	}
}

func (a *Advisor) analyzeDiskResults(results []*checker.DiskResult) []Alert {
	var alerts []Alert

	for _, result := range results {
		// 检查写入带宽
		if result.WriteBandwidth < a.Thresholds.Disk.MinWriteBandwidth {
			alert := Alert{
				Level:            AlertWarning,
				Category:         "磁盘性能",
				Message:          fmt.Sprintf("目录 %s 的写入带宽过低", result.Dir),
				CurrentValue:     fmt.Sprintf("%.1f MB/s", result.WriteBandwidth),
				RecommendedValue: fmt.Sprintf("> %.1f MB/s", a.Thresholds.Disk.MinWriteBandwidth),
				Suggestion:       "建议检查磁盘类型（HDD/SSD）、RAID 配置或更换高性能存储",
			}

			// 如果远低于阈值，升级为严重
			if result.WriteBandwidth < a.Thresholds.Disk.MinWriteBandwidth*0.3 {
				alert.Level = AlertCritical
				alert.Suggestion = "严重性能瓶颈，建议立即更换 SSD 或检查存储配置"
			}

			alerts = append(alerts, alert)
		}

		// 检查读取带宽
		if result.ReadBandwidth < a.Thresholds.Disk.MinReadBandwidth {
			alerts = append(alerts, Alert{
				Level:            AlertWarning,
				Category:         "磁盘性能",
				Message:          fmt.Sprintf("目录 %s 的读取带宽过低", result.Dir),
				CurrentValue:     fmt.Sprintf("%.1f MB/s", result.ReadBandwidth),
				RecommendedValue: fmt.Sprintf("> %.1f MB/s", a.Thresholds.Disk.MinReadBandwidth),
				Suggestion:       "建议检查磁盘健康状态和文件系统配置",
			})
		}

		// 检查随机读写性能
		if result.RandWriteBandwidth < 5.0 {
			alerts = append(alerts, Alert{
				Level:            AlertInfo,
				Category:         "磁盘性能",
				Message:          fmt.Sprintf("目录 %s 的随机写入性能较低", result.Dir),
				CurrentValue:     fmt.Sprintf("%.1f MB/s", result.RandWriteBandwidth),
				RecommendedValue: "> 5.0 MB/s",
				Suggestion:       "随机 I/O 性能影响数据库事务处理，建议使用 SSD",
			})
		}
	}

	return alerts
}

func (a *Advisor) analyzeStreamResults(results []*checker.StreamResult) []Alert {
	var alerts []Alert

	for _, result := range results {
		if result.TotalBandwidth < a.Thresholds.Memory.MinBandwidth {
			alerts = append(alerts, Alert{
				Level:            AlertWarning,
				Category:         "内存性能",
				Message:          "内存带宽低于预期",
				CurrentValue:     fmt.Sprintf("%.1f MB/s", result.TotalBandwidth),
				RecommendedValue: fmt.Sprintf("> %.1f MB/s", a.Thresholds.Memory.MinBandwidth),
				Suggestion:       "检查内存通道配置（建议双通道/四通道）或升级内存频率",
			})
		}
	}

	return alerts
}

func (a *Advisor) analyzeNetworkResults(results []checker.NetworkResult) []Alert {
	var alerts []Alert

	for _, result := range results {
		// 检查带宽
		if result.Bandwidth < a.Thresholds.Network.MinBandwidth {
			alerts = append(alerts, Alert{
				Level:            AlertWarning,
				Category:         "网络性能",
				Message:          fmt.Sprintf("%s -> %s 的网络带宽过低", result.SourceHost, result.DestHost),
				CurrentValue:     fmt.Sprintf("%.1f MB/s", result.Bandwidth),
				RecommendedValue: fmt.Sprintf("> %.1f MB/s", a.Thresholds.Network.MinBandwidth),
				Suggestion:       "检查网络配置、交换机端口速率或升级网络基础设施",
			})
		}

		// 检查丢包率
		if result.PacketLoss > a.Thresholds.Network.MaxPacketLoss {
			level := AlertWarning
			if result.PacketLoss > 1.0 {
				level = AlertCritical
			}

			alerts = append(alerts, Alert{
				Level:            level,
				Category:         "网络质量",
				Message:          fmt.Sprintf("%s -> %s 的丢包率过高", result.SourceHost, result.DestHost),
				CurrentValue:     fmt.Sprintf("%.2f%%", result.PacketLoss),
				RecommendedValue: fmt.Sprintf("< %.2f%%", a.Thresholds.Network.MaxPacketLoss),
				Suggestion:       "检查网络连接、网线质量、交换机端口或联系网络管理员",
			})
		}
	}

	return alerts
}

func (a *Advisor) analyzeLatencyResults(results []*checker.LatencyResult) []Alert {
	var alerts []Alert

	for _, result := range results {
		// 检查读延迟
		if result.ReadLatencyAvg > a.Thresholds.Latency.MaxReadLatency {
			level := AlertWarning
			if result.ReadLatencyAvg > a.Thresholds.Latency.MaxReadLatency*5 {
				level = AlertCritical
			}

			alerts = append(alerts, Alert{
				Level:            level,
				Category:         "磁盘延迟",
				Message:          fmt.Sprintf("目录 %s 的读延迟过高", result.Dir),
				CurrentValue:     fmt.Sprintf("%.2f ms", result.ReadLatencyAvg),
				RecommendedValue: fmt.Sprintf("< %.1f ms", a.Thresholds.Latency.MaxReadLatency),
				Suggestion:       "建议使用 SSD 或 NVMe 存储，检查磁盘队列深度",
			})
		}

		// 检查写延迟
		if result.WriteLatencyAvg > a.Thresholds.Latency.MaxWriteLatency {
			level := AlertWarning
			if result.WriteLatencyAvg > a.Thresholds.Latency.MaxWriteLatency*5 {
				level = AlertCritical
			}

			alerts = append(alerts, Alert{
				Level:            level,
				Category:         "磁盘延迟",
				Message:          fmt.Sprintf("目录 %s 的写延迟过高", result.Dir),
				CurrentValue:     fmt.Sprintf("%.2f ms", result.WriteLatencyAvg),
				RecommendedValue: fmt.Sprintf("< %.1f ms", a.Thresholds.Latency.MaxWriteLatency),
				Suggestion:       "建议使用带缓存的 RAID 卡或升级存储系统",
			})
		}

		// 检查 IOPS
		if result.ReadIOPS < 100 {
			alerts = append(alerts, Alert{
				Level:            AlertInfo,
				Category:         "磁盘 IOPS",
				Message:          fmt.Sprintf("目录 %s 的读 IOPS 较低", result.Dir),
				CurrentValue:     fmt.Sprintf("%.0f IOPS", result.ReadIOPS),
				RecommendedValue: "> 1000 IOPS (SSD)",
				Suggestion:       "低 IOPS 会影响数据库并发性能，建议使用 SSD 或 NVMe",
			})
		}
	}

	return alerts
}

// GetOptimizationSuggestions 获取优化建议列表
func (a *Advisor) GetOptimizationSuggestions(report *AnalysisReport) []string {
	var suggestions []string
	seenCategories := make(map[string]bool)

	for _, alert := range report.Alerts {
		if !seenCategories[alert.Category] {
			suggestions = append(suggestions, alert.Suggestion)
			seenCategories[alert.Category] = true
		}
	}

	return suggestions
}

// FormatReport 格式化报告为文本
func (a *Advisor) FormatReport(report *AnalysisReport) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("═══════════════════════════════════════════════════════════\n")
	sb.WriteString("                    性能分析报告\n")
	sb.WriteString("═══════════════════════════════════════════════════════════\n\n")

	// 总体评分
	sb.WriteString(fmt.Sprintf("性能评分：%d/100 ", report.Score))
	switch report.HealthStatus {
	case "优秀":
		sb.WriteString("🟢")
	case "良好":
		sb.WriteString("🟡")
	case "一般":
		sb.WriteString("🟠")
	case "需优化":
		sb.WriteString("🔴")
	}
	sb.WriteString(fmt.Sprintf(" [%s]\n\n", report.HealthStatus))

	// 总结
	sb.WriteString(fmt.Sprintf("摘要：%s\n\n", report.Summary))

	// 告警列表
	if len(report.Alerts) > 0 {
		sb.WriteString("───────────────────────────────────────────────────────\n")
		sb.WriteString("检测到的问题:\n")
		sb.WriteString("───────────────────────────────────────────────────────\n")

		for i, alert := range report.Alerts {
			icon := "⚠️"
			if alert.Level == AlertCritical {
				icon = "🔴"
			} else if alert.Level == AlertInfo {
				icon = "ℹ️"
			}

			sb.WriteString(fmt.Sprintf("\n%d. %s [%s] %s\n", i+1, icon, alert.Level, alert.Message))
			sb.WriteString(fmt.Sprintf("   当前值：%s\n", alert.CurrentValue))
			sb.WriteString(fmt.Sprintf("   推荐值：%s\n", alert.RecommendedValue))
			sb.WriteString(fmt.Sprintf("   建议：%s\n", alert.Suggestion))
		}
	}

	// 优化建议
	suggestions := a.GetOptimizationSuggestions(report)
	if len(suggestions) > 0 {
		sb.WriteString("\n───────────────────────────────────────────────────────\n")
		sb.WriteString("优化建议:\n")
		sb.WriteString("───────────────────────────────────────────────────────\n")

		for i, suggestion := range suggestions {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, suggestion))
		}
	}

	sb.WriteString("\n═══════════════════════════════════════════════════════════\n")

	return sb.String()
}
