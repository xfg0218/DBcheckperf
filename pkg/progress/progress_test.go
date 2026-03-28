// Package progress 提供进度条和预估时间显示功能
// 本文件包含 progress 包的单元测试
package progress

import (
	"testing"
	"time"
)

// ==================== ProgressBar 测试 ====================

func TestNewProgressBar(t *testing.T) {
	pb := NewProgressBar(100, "测试")

	if pb == nil {
		t.Fatal("NewProgressBar() 返回 nil")
	}

	if pb.Total != 100 {
		t.Errorf("Total = %d; 期望 100", pb.Total)
	}

	if pb.Current != 0 {
		t.Errorf("Current = %d; 期望 0", pb.Current)
	}

	if pb.Prefix != "测试" {
		t.Errorf("Prefix = %q; 期望 \"测试\"", pb.Prefix)
	}
}

func TestProgressBar_Basic(t *testing.T) {
	pb := NewProgressBar(10, "测试")

	// 测试 Increment
	for i := 0; i < 10; i++ {
		pb.Increment()
	}
	if pb.Current != 10 {
		t.Errorf("Increment 10 次后 Current = %d; 期望 10", pb.Current)
	}

	// 测试 Reset
	pb.Reset()
	if pb.Current != 0 {
		t.Errorf("Reset() 后 Current = %d; 期望 0", pb.Current)
	}
}

func TestProgressBar_Add(t *testing.T) {
	pb := NewProgressBar(100, "测试")

	pb.Add(25)
	if pb.Current != 25 {
		t.Errorf("Add(25) 后 Current = %d; 期望 25", pb.Current)
	}

	pb.Add(35)
	if pb.Current != 60 {
		t.Errorf("Add(35) 后 Current = %d; 期望 60", pb.Current)
	}
}

func TestProgressBar_GetProgress(t *testing.T) {
	pb := NewProgressBar(100, "测试")

	// 初始状态
	current, total, percent, _, _ := pb.GetProgress()
	if current != 0 || total != 100 || percent != 0 {
		t.Errorf("初始进度错误：%d/%d (%.1f%%)", current, total, percent)
	}

	// 50% 进度
	pb.SetCurrent(50)
	current, total, percent, elapsed, eta := pb.GetProgress()

	if current != 50 || total != 100 {
		t.Errorf("进度错误：%d/%d", current, total)
	}

	if percent < 49.0 || percent > 51.0 {
		t.Errorf("百分比错误：%.1f%%", percent)
	}

	// 验证时间
	if elapsed < 0 {
		t.Error("已用时间应该 >= 0")
	}

	// ETA 应该 >= 0
	if eta < 0 {
		t.Error("ETA 应该 >= 0")
	}

	t.Logf("进度：%d/%d (%.1f%%), 已用：%v, ETA: %v", current, total, percent, elapsed, eta)
}

// ==================== StepTracker 测试 ====================

func TestNewStepTracker(t *testing.T) {
	steps := []string{"步骤 1", "步骤 2", "步骤 3"}
	st := NewStepTracker(steps)

	if st == nil {
		t.Fatal("NewStepTracker() 返回 nil")
	}

	if len(st.Steps) != 3 {
		t.Errorf("Steps 长度 = %d; 期望 3", len(st.Steps))
	}
}

func TestStepTracker_Basic(t *testing.T) {
	steps := []string{"步骤 1", "步骤 2", "步骤 3"}
	st := NewStepTracker(steps)
	st.Start()

	// 初始步骤
	if st.GetCurrentStep() != "步骤 1" {
		t.Errorf("当前步骤 = %q; 期望 \"步骤 1\"", st.GetCurrentStep())
	}

	// 下一步
	st.Next()
	if st.GetCurrentStep() != "步骤 2" {
		t.Errorf("Next() 后当前步骤 = %q; 期望 \"步骤 2\"", st.GetCurrentStep())
	}

	// 最后一步
	st.Next()
	if st.GetCurrentStep() != "步骤 3" {
		t.Errorf("Next() 后当前步骤 = %q; 期望 \"步骤 3\"", st.GetCurrentStep())
	}

	// 完成
	st.Complete()
}

func TestStepTracker_GetRemainingSteps(t *testing.T) {
	steps := []string{"步骤 1", "步骤 2", "步骤 3", "步骤 4"}
	st := NewStepTracker(steps)
	st.Start()

	remaining := st.GetRemainingSteps()
	if remaining != 3 {
		t.Errorf("初始剩余步骤 = %d; 期望 3", remaining)
	}

	st.Next()
	remaining = st.GetRemainingSteps()
	if remaining != 2 {
		t.Errorf("Next() 后剩余步骤 = %d; 期望 2", remaining)
	}
}

// ==================== TaskEstimator 测试 ====================

func TestTaskEstimator_Basic(t *testing.T) {
	te := NewTaskEstimator()

	// 记录几次任务耗时
	te.RecordTask("磁盘测试", 10*time.Second)
	te.RecordTask("磁盘测试", 12*time.Second)
	te.RecordTask("磁盘测试", 11*time.Second)

	// 预估应该接近平均值
	estimated := te.EstimateTask("磁盘测试")
	expected := 11 * time.Second

	// 允许 1 秒误差
	diff := estimated - expected
	if diff < -time.Second || diff > time.Second {
		t.Errorf("预估时间 = %v; 期望约 %v", estimated, expected)
	}
}

func TestTaskEstimator_UnknownTask(t *testing.T) {
	te := NewTaskEstimator()

	estimated := te.EstimateTask("不存在的任务")
	if estimated != 0 {
		t.Errorf("未知任务预估时间 = %v; 期望 0", estimated)
	}
}

func TestTaskEstimator_Clear(t *testing.T) {
	te := NewTaskEstimator()

	te.RecordTask("任务", 5*time.Second)
	te.Clear()

	estimated := te.EstimateTask("任务")
	if estimated != 0 {
		t.Errorf("Clear() 后预估时间 = %v; 期望 0", estimated)
	}
}

// ==================== formatDuration 测试 ====================

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"零时间", 0, "0:00"},
		{"30 秒", 30 * time.Second, "0:30"},
		{"1 分钟", 1 * time.Minute, "1:00"},
		{"1 分 30 秒", 90 * time.Second, "1:30"},
		{"1 小时", 1 * time.Hour, "1:00:00"},
		{"2 小时 15 分 30 秒", 2*time.Hour + 15*time.Minute + 30*time.Second, "2:15:30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q; 期望 %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration_Negative(t *testing.T) {
	result := formatDuration(-1 * time.Second)
	if result != "--:--" {
		t.Errorf("formatDuration(-1s) = %q; 期望 \"--:--\"", result)
	}
}

// ==================== 基准测试 ====================

func BenchmarkProgressBar_Increment(b *testing.B) {
	pb := NewProgressBar(b.N, "基准测试")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pb.Increment()
	}
}

func BenchmarkTaskEstimator_Record(b *testing.B) {
	te := NewTaskEstimator()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		te.RecordTask("任务", 10*time.Second)
	}
}
