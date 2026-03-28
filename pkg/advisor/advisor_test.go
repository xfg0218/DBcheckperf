// Package advisor 提供智能告警和优化建议功能
// 本文件包含 advisor 包的单元测试
package advisor

import (
	"testing"

	"dbcheckperf/pkg/checker"
)

// ==================== Advisor 测试 ====================

func TestNewAdvisor(t *testing.T) {
	a := NewAdvisor(false)

	if a == nil {
		t.Fatal("NewAdvisor() 返回 nil")
	}

	if a.Thresholds == nil {
		t.Error("Thresholds 不应该为 nil")
	}

	if a.Thresholds.Disk.MinWriteBandwidth != 100.0 {
		t.Errorf("Disk.MinWriteBandwidth = %.1f; 期望 100.0", a.Thresholds.Disk.MinWriteBandwidth)
	}
}

func TestAdvisor_AnalyzeResults_Good(t *testing.T) {
	a := NewAdvisor(false)

	diskResults := []*checker.DiskResult{
		{
			Dir:            "/tmp",
			WriteBandwidth: 500.0,
			ReadBandwidth:  600.0,
		},
	}

	streamResults := []*checker.StreamResult{
		{TotalBandwidth: 20000.0},
	}

	report := a.AnalyzeResults(diskResults, streamResults, []checker.NetworkResult{}, []*checker.LatencyResult{})

	if report == nil {
		t.Fatal("AnalyzeResults() 返回 nil")
	}

	if report.Score < 90 {
		t.Errorf("性能评分 = %d; 期望 >= 90", report.Score)
	}

	if report.HealthStatus != "优秀" {
		t.Errorf("健康状态 = %q; 期望 \"优秀\"", report.HealthStatus)
	}

	t.Logf("分析报告：评分 %d, 状态 %s, 告警数 %d", report.Score, report.HealthStatus, len(report.Alerts))
}

func TestAdvisor_AnalyzeResults_BadDisk(t *testing.T) {
	a := NewAdvisor(false)

	// 模拟低性能磁盘
	diskResults := []*checker.DiskResult{
		{
			Dir:            "/data",
			WriteBandwidth: 50.0,   // 低于阈值
			ReadBandwidth:  60.0,   // 低于阈值
		},
	}

	report := a.AnalyzeResults(diskResults, []*checker.StreamResult{}, []checker.NetworkResult{}, []*checker.LatencyResult{})

	if report == nil {
		t.Fatal("AnalyzeResults() 返回 nil")
	}

	if len(report.Alerts) == 0 {
		t.Error("应该检测到磁盘性能问题")
	}

	// 验证告警级别
	hasCritical := false
	for _, alert := range report.Alerts {
		if alert.Level == AlertCritical {
			hasCritical = true
			break
		}
	}

	if !hasCritical {
		t.Log("警告：应该有严重告警")
	}

	t.Logf("检测到 %d 个告警，评分 %d", len(report.Alerts), report.Score)
}

func TestAdvisor_AnalyzeResults_BadNetwork(t *testing.T) {
	a := NewAdvisor(false)

	networkResults := []checker.NetworkResult{
		{
			SourceHost: "host1",
			DestHost:   "host2",
			Bandwidth:  50.0,     // 低于阈值
			PacketLoss: 2.5,      // 高于阈值
		},
	}

	report := a.AnalyzeResults([]*checker.DiskResult{}, []*checker.StreamResult{}, networkResults, []*checker.LatencyResult{})

	if report == nil {
		t.Fatal("AnalyzeResults() 返回 nil")
	}

	// 应该检测到网络问题
	hasNetworkAlert := false
	hasPacketLossAlert := false

	for _, alert := range report.Alerts {
		if alert.Category == "网络性能" {
			hasNetworkAlert = true
		}
		if alert.Category == "网络质量" {
			hasPacketLossAlert = true
		}
	}

	if !hasNetworkAlert {
		t.Error("应该检测到网络带宽问题")
	}

	if !hasPacketLossAlert {
		t.Error("应该检测到丢包率问题")
	}

	t.Logf("检测到 %d 个告警", len(report.Alerts))
}

func TestAdvisor_AnalyzeResults_BadLatency(t *testing.T) {
	a := NewAdvisor(false)

	latencyResults := []*checker.LatencyResult{
		{
			Dir:             "/data",
			ReadLatencyAvg:  25.0,   // 高于阈值
			WriteLatencyAvg: 50.0,   // 高于阈值
		},
	}

	report := a.AnalyzeResults([]*checker.DiskResult{}, []*checker.StreamResult{}, []checker.NetworkResult{}, latencyResults)

	if report == nil {
		t.Fatal("AnalyzeResults() 返回 nil")
	}

	hasLatencyAlert := false
	for _, alert := range report.Alerts {
		if alert.Category == "磁盘延迟" {
			hasLatencyAlert = true
		}
	}

	if !hasLatencyAlert {
		t.Error("应该检测到延迟问题")
	}

	t.Logf("检测到 %d 个告警", len(report.Alerts))
}

// ==================== AnalysisReport 测试 ====================

func TestAnalysisReport_CalculateScore(t *testing.T) {
	report := &AnalysisReport{
		Alerts: []Alert{
			{Level: AlertCritical},
			{Level: AlertWarning},
			{Level: AlertInfo},
		},
	}

	report.calculateScore()

	// 100 - 20 - 10 - 5 = 65
	if report.Score != 65 {
		t.Errorf("评分 = %d; 期望 65", report.Score)
	}

	if report.HealthStatus != "一般" {
		t.Errorf("健康状态 = %q; 期望 \"一般\"", report.HealthStatus)
	}
}

func TestAnalysisReport_CalculateScore_AllCritical(t *testing.T) {
	report := &AnalysisReport{
		Alerts: []Alert{
			{Level: AlertCritical},
			{Level: AlertCritical},
			{Level: AlertCritical},
			{Level: AlertCritical},
			{Level: AlertCritical},
			{Level: AlertCritical},
		},
	}

	report.calculateScore()

	// 100 - 6*20 = -20 -> 0
	if report.Score != 0 {
		t.Errorf("评分 = %d; 期望 0", report.Score)
	}

	if report.HealthStatus != "需优化" {
		t.Errorf("健康状态 = %q; 期望 \"需优化\"", report.HealthStatus)
	}
}

func TestAnalysisReport_GenerateSummary(t *testing.T) {
	tests := []struct {
		name            string
		alerts          []Alert
		expectedSummary string
	}{
		{
			name: "无告警",
			alerts: []Alert{},
			expectedSummary: "性能表现优秀，未检测到问题",
		},
		{
			name: "有警告",
			alerts: []Alert{
				{Level: AlertWarning},
				{Level: AlertWarning},
			},
			expectedSummary: "检测到 2 个警告，性能总体良好",
		},
		{
			name: "有严重问题",
			alerts: []Alert{
				{Level: AlertCritical},
				{Level: AlertWarning},
			},
			expectedSummary: "检测到 1 个严重问题和 1 个警告，建议立即优化",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &AnalysisReport{
				Alerts: tt.alerts,
			}
			report.generateSummary()

			if report.Summary != tt.expectedSummary {
				t.Errorf("Summary = %q; 期望 %q", report.Summary, tt.expectedSummary)
			}
		})
	}
}

// ==================== GetOptimizationSuggestions 测试 ====================

func TestGetOptimizationSuggestions(t *testing.T) {
	a := NewAdvisor(false)

	report := &AnalysisReport{
		Alerts: []Alert{
			{
				Category: "磁盘性能",
				Suggestion: "建议更换 SSD",
			},
			{
				Category: "磁盘性能",
				Suggestion: "建议检查 RAID 配置",
			},
			{
				Category: "内存性能",
				Suggestion: "建议升级内存频率",
			},
		},
	}

	suggestions := a.GetOptimizationSuggestions(report)

	// 应该去重，只返回 2 条建议
	if len(suggestions) != 2 {
		t.Errorf("期望 2 条建议，实际 %d 条", len(suggestions))
	}

	t.Logf("优化建议：%v", suggestions)
}

// ==================== FormatReport 测试 ====================

func TestFormatReport(t *testing.T) {
	a := NewAdvisor(false)

	report := &AnalysisReport{
		Score:        75,
		HealthStatus: "良好",
		Summary:      "检测到 1 个警告，性能总体良好",
		Alerts: []Alert{
			{
				Level:            AlertWarning,
				Category:         "磁盘性能",
				Message:          "目录 /data 的写入带宽过低",
				CurrentValue:     "50.0 MB/s",
				RecommendedValue: "> 100.0 MB/s",
				Suggestion:       "建议检查磁盘类型或更换 SSD",
			},
		},
	}

	formatted := a.FormatReport(report)

	if formatted == "" {
		t.Error("格式化报告不应该为空")
	}

	// 验证包含关键信息
	if !contains(formatted, "性能分析报告") {
		t.Error("报告应该包含标题")
	}

	if !contains(formatted, "75/100") {
		t.Error("报告应该包含评分")
	}

	if !contains(formatted, "写入带宽") {
		t.Error("报告应该包含告警类别")
	}

	t.Logf("\n%s", formatted)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ==================== SetThresholds 测试 ====================

func TestSetThresholds(t *testing.T) {
	a := NewAdvisor(false)

	customThresholds := &ThresholdConfig{
		Disk: &DiskThreshold{
			MinWriteBandwidth: 200.0,  // 自定义阈值
		},
	}

	a.SetThresholds(customThresholds)

	if a.Thresholds.Disk.MinWriteBandwidth != 200.0 {
		t.Errorf("MinWriteBandwidth = %.1f; 期望 200.0", a.Thresholds.Disk.MinWriteBandwidth)
	}
}

// ==================== AlertLevel 测试 ====================

func TestAlertLevel_Values(t *testing.T) {
	levels := []AlertLevel{
		AlertInfo,
		AlertWarning,
		AlertCritical,
	}

	expected := []string{"INFO", "WARNING", "CRITICAL"}

	for i, level := range levels {
		if string(level) != expected[i] {
			t.Errorf("AlertLevel[%d] = %q; 期望 %q", i, level, expected[i])
		}
	}
}

// ==================== 集成测试 ====================

func TestAdvisor_Integration(t *testing.T) {
	a := NewAdvisor(false)

	// 模拟混合场景
	diskResults := []*checker.DiskResult{
		{
			Dir:              "/data",
			WriteBandwidth:   450.0,
			ReadBandwidth:    500.0,
			RandWriteBandwidth: 8.0,
		},
	}

	streamResults := []*checker.StreamResult{
		{TotalBandwidth: 18000.0},  // 略低于阈值
	}

	networkResults := []checker.NetworkResult{
		{
			SourceHost: "host1",
			DestHost:   "host2",
			Bandwidth:  950.0,
			PacketLoss: 0.05,  // 正常
		},
	}

	report := a.AnalyzeResults(diskResults, streamResults, networkResults, []*checker.LatencyResult{})

	if report == nil {
		t.Fatal("AnalyzeResults() 返回 nil")
	}

	// 验证报告完整性
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("评分超出范围：%d", report.Score)
	}

	if report.HealthStatus == "" {
		t.Error("健康状态不应该为空")
	}

	if report.Summary == "" {
		t.Error("总结不应该为空")
	}

	// 格式化报告
	formatted := a.FormatReport(report)
	if formatted == "" {
		t.Error("格式化报告不应该为空")
	}

	t.Logf("集成测试完成：评分 %d, 状态 %s, 告警 %d 个", report.Score, report.HealthStatus, len(report.Alerts))
}
