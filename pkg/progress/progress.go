// Package progress 提供进度条和预估时间显示功能
// 用于在长时间运行时显示测试进度
package progress

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressBar 进度条
type ProgressBar struct {
	// Total 总任务数
	Total int
	// Current 当前完成数
	Current int
	// Prefix 前缀文本
	Prefix string
	// Suffix 后缀文本
	Suffix string
	// Width 进度条宽度（字符数）
	Width int
	// StartTime 开始时间
	StartTime time.Time
	// mu 互斥锁
	mu sync.Mutex
	// UseANSI 是否使用 ANSI 转义码
	UseANSI bool
}

// NewProgressBar 创建新的进度条
func NewProgressBar(total int, prefix string) *ProgressBar {
	return &ProgressBar{
		Total:     total,
		Current:   0,
		Prefix:    prefix,
		Suffix:    "",
		Width:     30,
		StartTime: time.Now(),
		UseANSI:   true,
	}
}

// SetTotal 设置总任务数
func (pb *ProgressBar) SetTotal(total int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.Total = total
}

// Increment 增加当前进度
func (pb *ProgressBar) Increment() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.Current++
	pb.render()
}

// Add 增加指定数量的进度
func (pb *ProgressBar) Add(n int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.Current += n
	pb.render()
}

// SetCurrent 设置当前进度
func (pb *ProgressBar) SetCurrent(current int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.Current = current
	pb.render()
}

// SetSuffix 设置后缀文本
func (pb *ProgressBar) SetSuffix(suffix string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.Suffix = suffix
	pb.render()
}

// render 渲染进度条
func (pb *ProgressBar) render() {
	if pb.Total <= 0 {
		return
	}

	percent := float64(pb.Current) / float64(pb.Total)
	filled := int(math.Round(float64(pb.Width) * percent))

	// 构建进度条
	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.Width-filled)

	// 计算已用时间和预估时间
	elapsed := time.Since(pb.StartTime)
	var eta time.Duration
	if pb.Current > 0 {
		timePerItem := elapsed / time.Duration(pb.Current)
		remaining := pb.Total - pb.Current
		eta = time.Duration(remaining) * timePerItem
	}

	// 格式化时间
	elapsedStr := formatDuration(elapsed)
	etaStr := formatDuration(eta)

	// 构建输出
	var output strings.Builder
	output.WriteString(fmt.Sprintf("\r%s [", pb.Prefix))
	output.WriteString(bar)
	output.WriteString(fmt.Sprintf("] %3d/%d (%5.1f%%)", pb.Current, pb.Total, percent*100))

	if pb.Suffix != "" {
		output.WriteString(fmt.Sprintf(" - %s", pb.Suffix))
	}

	output.WriteString(fmt.Sprintf(" ETA: %s", etaStr))
	output.WriteString(fmt.Sprintf(" [%s elapsed]", elapsedStr))

	fmt.Fprint(os.Stdout, output.String())

	// 完成后换行
	if pb.Current >= pb.Total {
		fmt.Fprintln(os.Stdout)
	}
}

// Finish 完成进度条
func (pb *ProgressBar) Finish() {
	pb.mu.Lock()
	pb.Current = pb.Total
	pb.mu.Unlock()
	pb.render()
}

// Reset 重置进度条
func (pb *ProgressBar) Reset() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.Current = 0
	pb.StartTime = time.Now()
	pb.render()
}

// GetProgress 获取当前进度信息
func (pb *ProgressBar) GetProgress() (current, total int, percent float64, elapsed, eta time.Duration) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	current = pb.Current
	total = pb.Total
	if total > 0 {
		percent = float64(current) / float64(total) * 100
	}

	elapsed = time.Since(pb.StartTime)
	if current > 0 {
		timePerItem := elapsed / time.Duration(current)
		remaining := total - current
		eta = time.Duration(remaining) * timePerItem
	}

	return
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "--:--"
	}

	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// StepTracker 步骤跟踪器
type StepTracker struct {
	// Steps 步骤列表
	Steps []string
	// CurrentStep 当前步骤索引
	CurrentStep int
	// pb 进度条
	pb *ProgressBar
}

// NewStepTracker 创建新的步骤跟踪器
func NewStepTracker(steps []string) *StepTracker {
	st := &StepTracker{
		Steps:       steps,
		CurrentStep: 0,
	}

	if len(steps) > 0 {
		st.pb = NewProgressBar(len(steps), "进度")
		st.updatePrefix()
	}

	return st
}

// Start 开始跟踪
func (st *StepTracker) Start() {
	if st.pb != nil {
		st.pb.StartTime = time.Now()
		st.updatePrefix()
	}
}

// Next 进入下一步
func (st *StepTracker) Next() {
	if st.CurrentStep < len(st.Steps)-1 {
		st.CurrentStep++
		if st.pb != nil {
			st.pb.Increment()
			st.updatePrefix()
		}
	}
}

// Complete 完成所有步骤
func (st *StepTracker) Complete() {
	if st.pb != nil {
		st.pb.Finish()
	}
}

// GetCurrentStep 获取当前步骤
func (st *StepTracker) GetCurrentStep() string {
	if st.CurrentStep < len(st.Steps) {
		return st.Steps[st.CurrentStep]
	}
	return ""
}

// GetRemainingSteps 获取剩余步骤数
func (st *StepTracker) GetRemainingSteps() int {
	return len(st.Steps) - st.CurrentStep - 1
}

// GetEstimatedTotalTime 获取预估总时间
func (st *StepTracker) GetEstimatedTotalTime() time.Duration {
	if st.pb == nil {
		return 0
	}
	_, _, _, elapsed, eta := st.pb.GetProgress()
	return elapsed + eta
}

func (st *StepTracker) updatePrefix() {
	if st.pb != nil && st.CurrentStep < len(st.Steps) {
		st.pb.Prefix = fmt.Sprintf("[%d/%d] %s:", st.CurrentStep+1, len(st.Steps), st.Steps[st.CurrentStep])
	}
}

// TaskEstimator 任务时间预估器
type TaskEstimator struct {
	// taskTimes 任务耗时记录
	taskTimes map[string][]time.Duration
	// mu 互斥锁
	mu sync.Mutex
}

// NewTaskEstimator 创建新的任务预估器
func NewTaskEstimator() *TaskEstimator {
	return &TaskEstimator{
		taskTimes: make(map[string][]time.Duration),
	}
}

// RecordTask 记录任务耗时
func (te *TaskEstimator) RecordTask(taskName string, duration time.Duration) {
	te.mu.Lock()
	defer te.mu.Unlock()

	te.taskTimes[taskName] = append(te.taskTimes[taskName], duration)

	// 只保留最近 10 次记录
	if len(te.taskTimes[taskName]) > 10 {
		te.taskTimes[taskName] = te.taskTimes[taskName][1:]
	}
}

// EstimateTask 预估任务耗时
func (te *TaskEstimator) EstimateTask(taskName string) time.Duration {
	te.mu.Lock()
	defer te.mu.Unlock()

	times, ok := te.taskTimes[taskName]
	if !ok || len(times) == 0 {
		return 0
	}

	// 计算平均值
	var total time.Duration
	for _, t := range times {
		total += t
	}
	return total / time.Duration(len(times))
}

// EstimateTasks 预估多个任务总耗时
func (te *TaskEstimator) EstimateTasks(taskNames []string) time.Duration {
	var total time.Duration
	for _, name := range taskNames {
		total += te.EstimateTask(name)
	}
	return total
}

// GetAverageTime 获取平均耗时
func (te *TaskEstimator) GetAverageTime(taskName string) time.Duration {
	return te.EstimateTask(taskName)
}

// Clear 清除记录
func (te *TaskEstimator) Clear() {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.taskTimes = make(map[string][]time.Duration)
}

// SimpleProgress 简单进度显示（不使用 ANSI）
type SimpleProgress struct {
	Total   int
	Current int
}

// Update 更新进度
func (sp *SimpleProgress) Update(current int) {
	sp.Current = current
	percent := float64(current) / float64(sp.Total) * 100
	fmt.Printf("\r进度：[%3d/%d] %5.1f%%", current, sp.Total, percent)
	if current >= sp.Total {
		fmt.Println(" 完成!")
	}
}
