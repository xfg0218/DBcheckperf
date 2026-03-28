// Package history 提供测试历史数据管理功能
// 支持保存、加载和对比历史测试结果
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"dbcheckperf/pkg/checker"
)

// TestRecord 测试记录
type TestRecord struct {
	// ID 记录 ID
	ID string `json:"id"`
	// Timestamp 测试时间戳
	Timestamp time.Time `json:"timestamp"`
	// Hostname 主机名
	Hostname string `json:"hostname"`
	// TestType 测试类型
	TestType string `json:"test_type"`
	// DiskResults 磁盘测试结果
	DiskResults []DiskResultSummary `json:"disk_results,omitempty"`
	// StreamResults 内存测试结果
	StreamResults []StreamResultSummary `json:"stream_results,omitempty"`
	// NetworkResults 网络测试结果
	NetworkResults []NetworkResultSummary `json:"network_results,omitempty"`
	// LatencyResults 延迟测试结果
	LatencyResults []LatencyResultSummary `json:"latency_results,omitempty"`
	// Tags 标签（用于分类和搜索）
	Tags []string `json:"tags,omitempty"`
	// Notes 备注
	Notes string `json:"notes,omitempty"`
}

// DiskResultSummary 磁盘结果摘要
type DiskResultSummary struct {
	Dir              string  `json:"dir"`
	WriteBandwidth   float64 `json:"write_bandwidth"`
	ReadBandwidth    float64 `json:"read_bandwidth"`
	RandWriteBandwidth float64 `json:"rand_write_bandwidth"`
	RandReadBandwidth  float64 `json:"rand_read_bandwidth"`
}

// StreamResultSummary 内存结果摘要
type StreamResultSummary struct {
	TotalBandwidth float64 `json:"total_bandwidth"`
}

// NetworkResultSummary 网络结果摘要
type NetworkResultSummary struct {
	SourceHost string  `json:"source_host"`
	DestHost   string  `json:"dest_host"`
	Bandwidth  float64 `json:"bandwidth"`
}

// LatencyResultSummary 延迟结果摘要
type LatencyResultSummary struct {
	Dir             string  `json:"dir"`
	ReadLatencyAvg  float64 `json:"read_latency_avg"`
	WriteLatencyAvg float64 `json:"write_latency_avg"`
	ReadIOPS        float64 `json:"read_iops"`
	WriteIOPS       float64 `json:"write_iops"`
}

// HistoryManager 历史数据管理器
type HistoryManager struct {
	// DataDir 数据目录
	DataDir string
	// Verbose 是否显示详细输出
	Verbose bool
}

// NewHistoryManager 创建新的历史数据管理器
func NewHistoryManager(dataDir string, verbose bool) *HistoryManager {
	if dataDir == "" {
		// 默认使用用户主目录下的 .dbcheckperf/history
		home, _ := os.UserHomeDir()
		if home != "" {
			dataDir = filepath.Join(home, ".dbcheckperf", "history")
		} else {
			dataDir = "./.dbcheckperf_history"
		}
	}
	return &HistoryManager{
		DataDir: dataDir,
		Verbose: verbose,
	}
}

// Initialize 初始化数据目录
func (hm *HistoryManager) Initialize() error {
	if err := os.MkdirAll(hm.DataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败：%v", err)
	}
	return nil
}

// SaveRecord 保存测试记录
func (hm *HistoryManager) SaveRecord(record *TestRecord) error {
	if err := hm.Initialize(); err != nil {
		return err
	}

	// 生成文件名
	filename := fmt.Sprintf("%s_%s.json", record.Timestamp.Format("20060102_150405"), record.ID)
	filepath := filepath.Join(hm.DataDir, filename)

	// 序列化
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败：%v", err)
	}

	// 写入文件
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败：%v", err)
	}

	if hm.Verbose {
		fmt.Printf("测试记录已保存：%s\n", filepath)
	}

	return nil
}

// LoadRecord 加载测试记录
func (hm *HistoryManager) LoadRecord(filename string) (*TestRecord, error) {
	filepath := filepath.Join(hm.DataDir, filename)

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败：%v", err)
	}

	var record TestRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("反序列化失败：%v", err)
	}

	return &record, nil
}

// ListRecords 列出所有测试记录
func (hm *HistoryManager) ListRecords() ([]TestRecord, error) {
	if err := hm.Initialize(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(hm.DataDir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败：%v", err)
	}

	var records []TestRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		record, err := hm.LoadRecord(entry.Name())
		if err != nil {
			if hm.Verbose {
				fmt.Printf("警告：加载 %s 失败：%v\n", entry.Name(), err)
			}
			continue
		}

		records = append(records, *record)
	}

	// 按时间戳排序（最新的在前）
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.After(records[j].Timestamp)
	})

	return records, nil
}

// CreateRecordFromResults 从测试结果创建记录
func (hm *HistoryManager) CreateRecordFromResults(
	hostname string,
	testType string,
	diskResults []*checker.DiskResult,
	streamResults []*checker.StreamResult,
	networkResults []checker.NetworkResult,
	latencyResults []*checker.LatencyResult,
	tags []string,
	notes string,
) *TestRecord {
	record := &TestRecord{
		ID:        fmt.Sprintf("run_%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Hostname:  hostname,
		TestType:  testType,
		Tags:      tags,
		Notes:     notes,
	}

	// 转换磁盘结果
	for _, r := range diskResults {
		record.DiskResults = append(record.DiskResults, DiskResultSummary{
			Dir:                r.Dir,
			WriteBandwidth:     r.WriteBandwidth,
			ReadBandwidth:      r.ReadBandwidth,
			RandWriteBandwidth: r.RandWriteBandwidth,
			RandReadBandwidth:  r.RandReadBandwidth,
		})
	}

	// 转换内存结果
	for _, r := range streamResults {
		record.StreamResults = append(record.StreamResults, StreamResultSummary{
			TotalBandwidth: r.TotalBandwidth,
		})
	}

	// 转换网络结果
	for _, r := range networkResults {
		record.NetworkResults = append(record.NetworkResults, NetworkResultSummary{
			SourceHost: r.SourceHost,
			DestHost:   r.DestHost,
			Bandwidth:  r.Bandwidth,
		})
	}

	// 转换延迟结果
	for _, r := range latencyResults {
		record.LatencyResults = append(record.LatencyResults, LatencyResultSummary{
			Dir:             r.Dir,
			ReadLatencyAvg:  r.ReadLatencyAvg,
			WriteLatencyAvg: r.WriteLatencyAvg,
			ReadIOPS:        r.ReadIOPS,
			WriteIOPS:       r.WriteIOPS,
		})
	}

	return record
}

// CompareRecords 对比两个测试记录
func (hm *HistoryManager) CompareRecords(oldRecord, newRecord *TestRecord) *ComparisonResult {
	result := &ComparisonResult{
		OldRecord: oldRecord,
		NewRecord: newRecord,
		OldTime:   oldRecord.Timestamp,
		NewTime:   newRecord.Timestamp,
	}

	// 对比磁盘结果
	if len(oldRecord.DiskResults) > 0 && len(newRecord.DiskResults) > 0 {
		result.DiskComparison = compareDiskResults(oldRecord.DiskResults, newRecord.DiskResults)
	}

	// 对比内存结果
	if len(oldRecord.StreamResults) > 0 && len(newRecord.StreamResults) > 0 {
		result.MemoryComparison = compareMemoryResults(oldRecord.StreamResults, newRecord.StreamResults)
	}

	// 对比网络结果
	if len(oldRecord.NetworkResults) > 0 && len(newRecord.NetworkResults) > 0 {
		result.NetworkComparison = compareNetworkResults(oldRecord.NetworkResults, newRecord.NetworkResults)
	}

	// 对比延迟结果
	if len(oldRecord.LatencyResults) > 0 && len(newRecord.LatencyResults) > 0 {
		result.LatencyComparison = compareLatencyResults(oldRecord.LatencyResults, newRecord.LatencyResults)
	}

	return result
}

// ComparisonResult 对比结果
type ComparisonResult struct {
	OldRecord         *TestRecord
	NewRecord         *TestRecord
	OldTime           time.Time
	NewTime           time.Time
	DiskComparison    *DiskComparison    `json:"disk,omitempty"`
	MemoryComparison  *MemoryComparison  `json:"memory,omitempty"`
	NetworkComparison *NetworkComparison `json:"network,omitempty"`
	LatencyComparison *LatencyComparison `json:"latency,omitempty"`
}

// DiskComparison 磁盘对比
type DiskComparison struct {
	WriteChange    float64 `json:"write_change"`
	ReadChange     float64 `json:"read_change"`
	RandWriteChange float64 `json:"rand_write_change"`
	RandReadChange  float64 `json:"rand_read_change"`
	WritePercent   float64 `json:"write_percent"`
	ReadPercent    float64 `json:"read_percent"`
}

// MemoryComparison 内存对比
type MemoryComparison struct {
	BandwidthChange float64 `json:"bandwidth_change"`
	BandwidthPercent float64 `json:"bandwidth_percent"`
}

// NetworkComparison 网络对比
type NetworkComparison struct {
	BandwidthChange float64 `json:"bandwidth_change"`
	BandwidthPercent float64 `json:"bandwidth_percent"`
}

// LatencyComparison 延迟对比
type LatencyComparison struct {
	ReadLatencyChange   float64 `json:"read_latency_change"`
	WriteLatencyChange  float64 `json:"write_latency_change"`
	ReadIOPSChange      float64 `json:"read_iops_change"`
	WriteIOPSChange     float64 `json:"write_iops_change"`
}

func compareDiskResults(oldResults, newResults []DiskResultSummary) *DiskComparison {
	oldAvg := averageDiskResults(oldResults)
	newAvg := averageDiskResults(newResults)

	comp := &DiskComparison{}

	if oldAvg.write > 0 {
		comp.WriteChange = newAvg.write - oldAvg.write
		comp.WritePercent = (comp.WriteChange / oldAvg.write) * 100
	}

	if oldAvg.read > 0 {
		comp.ReadChange = newAvg.read - oldAvg.read
		comp.ReadPercent = (comp.ReadChange / oldAvg.read) * 100
	}

	comp.RandWriteChange = newAvg.randWrite - oldAvg.randWrite
	comp.RandReadChange = newAvg.randRead - oldAvg.randRead

	return comp
}

type diskAverages struct {
	write     float64
	read      float64
	randWrite float64
	randRead  float64
}

func averageDiskResults(results []DiskResultSummary) diskAverages {
	if len(results) == 0 {
		return diskAverages{}
	}

	var avg diskAverages
	for _, r := range results {
		avg.write += r.WriteBandwidth
		avg.read += r.ReadBandwidth
		avg.randWrite += r.RandWriteBandwidth
		avg.randRead += r.RandReadBandwidth
	}

	n := float64(len(results))
	avg.write /= n
	avg.read /= n
	avg.randWrite /= n
	avg.randRead /= n

	return avg
}

func compareMemoryResults(oldResults, newResults []StreamResultSummary) *MemoryComparison {
	oldAvg := averageStreamResults(oldResults)
	newAvg := averageStreamResults(newResults)

	comp := &MemoryComparison{}

	if oldAvg > 0 {
		comp.BandwidthChange = newAvg - oldAvg
		comp.BandwidthPercent = (comp.BandwidthChange / oldAvg) * 100
	}

	return comp
}

func averageStreamResults(results []StreamResultSummary) float64 {
	if len(results) == 0 {
		return 0
	}

	var total float64
	for _, r := range results {
		total += r.TotalBandwidth
	}
	return total / float64(len(results))
}

func compareNetworkResults(oldResults, newResults []NetworkResultSummary) *NetworkComparison {
	oldAvg := averageNetworkResults(oldResults)
	newAvg := averageNetworkResults(newResults)

	comp := &NetworkComparison{}

	if oldAvg > 0 {
		comp.BandwidthChange = newAvg - oldAvg
		comp.BandwidthPercent = (comp.BandwidthChange / oldAvg) * 100
	}

	return comp
}

func averageNetworkResults(results []NetworkResultSummary) float64 {
	if len(results) == 0 {
		return 0
	}

	var total float64
	for _, r := range results {
		total += r.Bandwidth
	}
	return total / float64(len(results))
}

func compareLatencyResults(oldResults, newResults []LatencyResultSummary) *LatencyComparison {
	oldAvg := averageLatencyResults(oldResults)
	newAvg := averageLatencyResults(newResults)

	comp := &LatencyComparison{
		ReadLatencyChange:  newAvg.readLatency - oldAvg.readLatency,
		WriteLatencyChange: newAvg.writeLatency - oldAvg.writeLatency,
		ReadIOPSChange:     newAvg.readIOPS - oldAvg.readIOPS,
		WriteIOPSChange:    newAvg.writeIOPS - oldAvg.writeIOPS,
	}

	return comp
}

type latencyAverages struct {
	readLatency  float64
	writeLatency float64
	readIOPS     float64
	writeIOPS    float64
}

func averageLatencyResults(results []LatencyResultSummary) latencyAverages {
	if len(results) == 0 {
		return latencyAverages{}
	}

	var avg latencyAverages
	for _, r := range results {
		avg.readLatency += r.ReadLatencyAvg
		avg.writeLatency += r.WriteLatencyAvg
		avg.readIOPS += r.ReadIOPS
		avg.writeIOPS += r.WriteIOPS
	}

	n := float64(len(results))
	avg.readLatency /= n
	avg.writeLatency /= n
	avg.readIOPS /= n
	avg.writeIOPS /= n

	return avg
}

// DeleteRecord 删除测试记录
func (hm *HistoryManager) DeleteRecord(filename string) error {
	filepath := filepath.Join(hm.DataDir, filename)
	if err := os.Remove(filepath); err != nil {
		return fmt.Errorf("删除文件失败：%v", err)
	}
	return nil
}

// GetRecentRecords 获取最近的测试记录
func (hm *HistoryManager) GetRecentRecords(limit int) ([]TestRecord, error) {
	records, err := hm.ListRecords()
	if err != nil {
		return nil, err
	}

	if len(records) <= limit {
		return records, nil
	}

	return records[:limit], nil
}

// SearchRecords 搜索测试记录（按标签）
func (hm *HistoryManager) SearchRecords(tags []string) ([]TestRecord, error) {
	allRecords, err := hm.ListRecords()
	if err != nil {
		return nil, err
	}

	var matched []TestRecord
	for _, record := range allRecords {
		if hasTags(record.Tags, tags) {
			matched = append(matched, record)
		}
	}

	return matched, nil
}

func hasTags(recordTags, searchTags []string) bool {
	for _, searchTag := range searchTags {
		found := false
		for _, recordTag := range recordTags {
			if recordTag == searchTag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(searchTags) > 0
}
