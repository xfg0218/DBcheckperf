// Package checker 测试文件
// 测试主 API 和聚合函数
package checker

import (
	"fmt"
	"testing"
	"time"
)

// ==================== 公共 API 测试 ====================

// TestResolveToIP 测试 IP 地址解析
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

// ==================== 构造函数测试 ====================

func TestNewDiskChecker(t *testing.T) {
	tests := []struct {
		name            string
		blockSizeKB     int
		fileSize        uint64
		verbose         bool
		randBlockSizeKB int
		expectRandBS    int
	}{
		{"正常参数", 32, 1024*1024*1024, true, 4, 4},
		{"零随机块大小", 64, 512*1024*1024, false, 0, 64},
		{"负随机块大小", 128, 256*1024*1024, true, -1, 128},
		{"小文件", 16, 10*1024*1024, false, 8, 8},
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
		})
	}
}

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
		})
	}
}

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
		})
	}
}

func TestNewSystemChecker(t *testing.T) {
	checker := NewSystemChecker()

	if checker == nil {
		t.Fatal("NewSystemChecker() 返回 nil")
	}
}

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

// ==================== 辅助函数 ====================

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
