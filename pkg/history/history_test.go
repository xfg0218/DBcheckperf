// Package history 提供测试历史数据管理功能
// 本文件包含 history 包的单元测试
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"dbcheckperf/pkg/checker"
)

// ==================== TestRecord 测试 ====================

func TestTestRecord_Marshal(t *testing.T) {
	record := &TestRecord{
		ID:        "test_001",
		Timestamp: time.Now(),
		Hostname:  "localhost",
		TestType:  "ds",
		Tags:      []string{"baseline", "ssd"},
		Notes:     "基础测试",
		DiskResults: []DiskResultSummary{
			{
				Dir:            "/tmp",
				WriteBandwidth: 500.0,
				ReadBandwidth:  600.0,
			},
		},
	}

	// 序列化测试
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}

	if len(data) == 0 {
		t.Error("序列化数据为空")
	}

	t.Logf("序列化成功：%d 字节", len(data))
}

// ==================== HistoryManager 测试 ====================

func TestNewHistoryManager(t *testing.T) {
	hm := NewHistoryManager("", false)

	if hm == nil {
		t.Fatal("NewHistoryManager() 返回 nil")
	}

	if hm.DataDir == "" {
		t.Error("DataDir 不应该为空")
	}

	t.Logf("数据目录：%s", hm.DataDir)
}

func TestHistoryManager_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, false)

	err := hm.Initialize()
	if err != nil {
		t.Errorf("Initialize() 失败：%v", err)
	}

	// 验证目录已创建
	if _, err := os.Stat(hm.DataDir); os.IsNotExist(err) {
		t.Error("数据目录未创建")
	}
}

func TestHistoryManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, true)

	record := &TestRecord{
		ID:        "test_save_load",
		Timestamp: time.Now(),
		Hostname:  "localhost",
		TestType:  "ds",
		DiskResults: []DiskResultSummary{
			{
				Dir:            "/tmp",
				WriteBandwidth: 500.0,
				ReadBandwidth:  600.0,
			},
		},
	}

	// 保存
	err := hm.SaveRecord(record)
	if err != nil {
		t.Fatalf("SaveRecord() 失败：%v", err)
	}

	// 加载
	records, err := hm.ListRecords()
	if err != nil {
		t.Fatalf("ListRecords() 失败：%v", err)
	}

	if len(records) != 1 {
		t.Errorf("期望 1 条记录，实际 %d 条", len(records))
	}

	if records[0].ID != record.ID {
		t.Errorf("记录 ID 不匹配：%s != %s", records[0].ID, record.ID)
	}

	t.Logf("保存和加载成功")
}

func TestHistoryManager_ListRecords(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, false)

	// 空目录
	records, err := hm.ListRecords()
	if err != nil {
		t.Fatalf("ListRecords() 失败：%v", err)
	}

	if len(records) != 0 {
		t.Errorf("空目录应该返回 0 条记录，实际 %d 条", len(records))
	}

	// 添加几条记录
	for i := 0; i < 3; i++ {
		record := &TestRecord{
			ID:        fmt.Sprintf("test_%d", i),
			Timestamp: time.Now().Add(time.Duration(i) * time.Hour),
			Hostname:  "localhost",
			TestType:  "ds",
		}
		hm.SaveRecord(record)
	}

	records, err = hm.ListRecords()
	if err != nil {
		t.Fatalf("ListRecords() 失败：%v", err)
	}

	if len(records) != 3 {
		t.Errorf("期望 3 条记录，实际 %d 条", len(records))
	}

	// 验证按时间排序（最新的在前）
	if !records[0].Timestamp.After(records[1].Timestamp) {
		t.Error("记录应该按时间降序排列")
	}
}

func TestHistoryManager_DeleteRecord(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, false)

	record := &TestRecord{
		ID:        "test_delete",
		Timestamp: time.Now(),
		Hostname:  "localhost",
	}

	// 保存
	err := hm.SaveRecord(record)
	if err != nil {
		t.Fatalf("SaveRecord() 失败：%v", err)
	}

	// 获取文件名
	records, _ := hm.ListRecords()
	if len(records) == 0 {
		t.Fatal("没有记录可删除")
	}

	// 删除（使用正确的文件名格式）
	filename := fmt.Sprintf("%s_%s.json", record.Timestamp.Format("20060102_150405"), record.ID)
	err = hm.DeleteRecord(filename)
	if err != nil {
		t.Fatalf("DeleteRecord() 失败：%v", err)
	}

	// 验证已删除
	records, _ = hm.ListRecords()
	if len(records) != 0 {
		t.Errorf("删除后应该没有记录，实际 %d 条", len(records))
	}
}

func TestHistoryManager_GetRecentRecords(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, false)

	// 添加 5 条记录
	for i := 0; i < 5; i++ {
		record := &TestRecord{
			ID:        fmt.Sprintf("test_%d", i),
			Timestamp: time.Now(),
			Hostname:  "localhost",
		}
		hm.SaveRecord(record)
	}

	// 获取最近 3 条
	records, err := hm.GetRecentRecords(3)
	if err != nil {
		t.Fatalf("GetRecentRecords() 失败：%v", err)
	}

	if len(records) != 3 {
		t.Errorf("期望 3 条记录，实际 %d 条", len(records))
	}
}

func TestHistoryManager_SearchRecords(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, false)

	// 添加带标签的记录
	record1 := &TestRecord{
		ID:        "test_1",
		Timestamp: time.Now(),
		Hostname:  "localhost",
		Tags:      []string{"baseline", "ssd"},
	}
	record2 := &TestRecord{
		ID:        "test_2",
		Timestamp: time.Now(),
		Hostname:  "localhost",
		Tags:      []string{"production", "hdd"},
	}
	record3 := &TestRecord{
		ID:        "test_3",
		Timestamp: time.Now(),
		Hostname:  "localhost",
		Tags:      []string{"baseline", "hdd"},
	}

	hm.SaveRecord(record1)
	hm.SaveRecord(record2)
	hm.SaveRecord(record3)

	// 搜索标签 "baseline"
	matched, err := hm.SearchRecords([]string{"baseline"})
	if err != nil {
		t.Fatalf("SearchRecords() 失败：%v", err)
	}

	if len(matched) != 2 {
		t.Errorf("期望匹配 2 条记录，实际 %d 条", len(matched))
	}

	// 搜索标签 "baseline" 和 "ssd"
	matched, err = hm.SearchRecords([]string{"baseline", "ssd"})
	if err != nil {
		t.Fatalf("SearchRecords() 失败：%v", err)
	}

	if len(matched) != 1 {
		t.Errorf("期望匹配 1 条记录，实际 %d 条", len(matched))
	}
}

// ==================== ComparisonResult 测试 ====================

func TestCompareRecords(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, false)

	oldRecord := &TestRecord{
		ID:        "old",
		Timestamp: time.Now().Add(-24 * time.Hour),
		DiskResults: []DiskResultSummary{
			{WriteBandwidth: 500.0, ReadBandwidth: 600.0},
		},
		StreamResults: []StreamResultSummary{
			{TotalBandwidth: 20000.0},
		},
	}

	newRecord := &TestRecord{
		ID:        "new",
		Timestamp: time.Now(),
		DiskResults: []DiskResultSummary{
			{WriteBandwidth: 550.0, ReadBandwidth: 660.0},
		},
		StreamResults: []StreamResultSummary{
			{TotalBandwidth: 22000.0},
		},
	}

	result := hm.CompareRecords(oldRecord, newRecord)

	if result == nil {
		t.Fatal("CompareRecords() 返回 nil")
	}

	// 验证磁盘对比
	if result.DiskComparison == nil {
		t.Fatal("DiskComparison 不应该为 nil")
	}

	if result.DiskComparison.WriteChange != 50.0 {
		t.Errorf("WriteChange = %.1f; 期望 50.0", result.DiskComparison.WriteChange)
	}

	if result.DiskComparison.WritePercent != 10.0 {
		t.Errorf("WritePercent = %.1f%%; 期望 10.0%%", result.DiskComparison.WritePercent)
	}

	// 验证内存对比
	if result.MemoryComparison == nil {
		t.Fatal("MemoryComparison 不应该为 nil")
	}

	if result.MemoryComparison.BandwidthPercent != 10.0 {
		t.Errorf("BandwidthPercent = %.1f%%; 期望 10.0%%", result.MemoryComparison.BandwidthPercent)
	}

	t.Logf("对比成功：写入提升 %.1f%%, 内存提升 %.1f%%",
		result.DiskComparison.WritePercent,
		result.MemoryComparison.BandwidthPercent)
}

func TestCompareRecords_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, false)

	oldRecord := &TestRecord{
		ID:        "old",
		Timestamp: time.Now().Add(-24 * time.Hour),
	}

	newRecord := &TestRecord{
		ID:        "new",
		Timestamp: time.Now(),
	}

	result := hm.CompareRecords(oldRecord, newRecord)

	if result == nil {
		t.Fatal("CompareRecords() 返回 nil")
	}

	// 空记录对比不应该 panic
	t.Log("空记录对比成功")
}

// ==================== 辅助函数测试 ====================

func TestAverageDiskResults(t *testing.T) {
	results := []DiskResultSummary{
		{WriteBandwidth: 400.0, ReadBandwidth: 500.0},
		{WriteBandwidth: 600.0, ReadBandwidth: 700.0},
	}

	avg := averageDiskResults(results)

	if avg.write != 500.0 {
		t.Errorf("平均写入 = %.1f; 期望 500.0", avg.write)
	}

	if avg.read != 600.0 {
		t.Errorf("平均读取 = %.1f; 期望 600.0", avg.read)
	}
}

func TestAverageDiskResults_Empty(t *testing.T) {
	results := []DiskResultSummary{}
	avg := averageDiskResults(results)

	if avg.write != 0 {
		t.Errorf("空结果平均写入 = %.1f; 期望 0", avg.write)
	}
}

// ==================== 集成测试 ====================

func TestHistoryManager_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	hm := NewHistoryManager(tmpDir, true)

	// 创建测试记录
	record := hm.CreateRecordFromResults(
		"localhost",
		"ds",
		[]*checker.DiskResult{
			{
				Dir:            "/tmp",
				WriteBandwidth: 500.0,
				ReadBandwidth:  600.0,
			},
		},
		[]*checker.StreamResult{
			{TotalBandwidth: 20000.0},
		},
		[]checker.NetworkResult{},
		[]*checker.LatencyResult{},
		[]string{"integration-test"},
		"集成测试记录",
	)

	// 保存
	err := hm.SaveRecord(record)
	if err != nil {
		t.Fatalf("保存失败：%v", err)
	}

	// 获取最近的记录
	recent, err := hm.GetRecentRecords(1)
	if err != nil {
		t.Fatalf("获取最近记录失败：%v", err)
	}

	if len(recent) != 1 {
		t.Errorf("期望 1 条记录，实际 %d 条", len(recent))
	}

	// 搜索
	matched, err := hm.SearchRecords([]string{"integration-test"})
	if err != nil {
		t.Fatalf("搜索失败：%v", err)
	}

	if len(matched) != 1 {
		t.Errorf("期望匹配 1 条记录，实际 %d 条", len(matched))
	}

	t.Log("集成测试成功")
}
