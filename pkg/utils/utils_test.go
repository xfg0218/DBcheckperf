// Package utils 提供通用工具函数
// 包含字节转换、时间格式化、文件操作等辅助功能
package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// ==================== FormatBytes 测试 ====================

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"零字节", 0, "0 B"},
		{"1 字节", 1, "1 B"},
		{"1023 字节", 1023, "1023 B"},
		{"1 KB", 1024, "1.00 KB"},
		{"1.5 KB", 1536, "1.50 KB"},
		{"1 MB", 1048576, "1.00 MB"},
		{"2.5 MB", 2621440, "2.50 MB"},
		{"1 GB", 1073741824, "1.00 GB"},
		{"3.7 GB", 3975102464, "3.70 GB"},
		{"1 TB", 1099511627776, "1.00 TB"},
		{"2.5 TB", 2748779069440, "2.50 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %s; 期望 %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

// ==================== ParseFileSize 测试 ====================

func TestParseFileSize(t *testing.T) {
	tests := []struct {
		name        string
		sizeStr     string
		expected    uint64
		expectError bool
	}{
		{"1KB", "1KB", 1024, false},
		{"1MB", "1MB", 1048576, false},
		{"1GB", "1GB", 1073741824, false},
		{"1TB", "1TB", 1099511627776, false},
		{"512KB", "512KB", 524288, false},
		{"2.5MB", "2.5MB", 2621440, false},
		{"100 不带单位", "100", 100, false},
		{"带空格", " 1GB ", 1073741824, false},
		{"小写单位", "512mb", 536870912, false},
		{"混合大小写", "1Gb", 1073741824, false},
		{"无效格式", "abc", 0, true},
		{"负数", "-1GB", 0, true},
		{"空字符串", "", 0, true},
		{"无效单位", "100XB", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseFileSize(tt.sizeStr)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseFileSize(%q) 期望错误，但得到结果：%d", tt.sizeStr, result)
				}
			} else {
				if err != nil {
					t.Errorf("ParseFileSize(%q) 意外错误：%v", tt.sizeStr, err)
				}
				if result != tt.expected {
					t.Errorf("ParseFileSize(%q) = %d; 期望 %d", tt.sizeStr, result, tt.expected)
				}
			}
		})
	}
}

// TestParseFileSize_2xRAM 测试 2xRAM 特殊格式
func TestParseFileSize_2xRAM(t *testing.T) {
	result, err := ParseFileSize("2xRAM")
	// 注意：这个测试在 Linux 系统上会成功，在其他系统上可能失败
	if err != nil {
		t.Logf("ParseFileSize(\"2xRAM\") 在非 Linux 系统上可能失败：%v", err)
	} else {
		t.Logf("ParseFileSize(\"2xRAM\") = %d 字节", result)
	}
}

// ==================== FormatBandwidth 测试 ====================

func TestFormatBandwidth(t *testing.T) {
	tests := []struct {
		name     string
		mbPerSec float64
		expected string
	}{
		{"零带宽", 0, "0.00 MB/s"},
		{"整数带宽", 100, "100.00 MB/s"},
		{"小数带宽", 123.456, "123.46 MB/s"},
		{"高精度", 99.999, "100.00 MB/s"},
		{"大数值", 1000.5, "1000.50 MB/s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBandwidth(tt.mbPerSec)
			if result != tt.expected {
				t.Errorf("FormatBandwidth(%.3f) = %s; 期望 %s", tt.mbPerSec, result, tt.expected)
			}
		})
	}
}

// ==================== FormatDuration 测试 ====================

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		seconds  float64
		expected string
	}{
		{"零秒", 0, "0.00 s"},
		{"1 秒", 1, "1.00 s"},
		{"59 秒", 59, "59.00 s"},
		{"1 分钟", 60, "1.00 m"},
		{"1.5 分钟", 90, "1.50 m"},
		{"5 分钟", 300, "5.00 m"},
		{"59 分钟", 3540, "59.00 m"},
		{"1 小时", 3600, "1.00 h"},
		{"1.5 小时", 5400, "1.50 h"},
		{"2 小时", 7200, "2.00 h"},
		{"小数秒", 1.234, "1.23 s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.seconds)
			if result != tt.expected {
				t.Errorf("FormatDuration(%.3f) = %s; 期望 %s", tt.seconds, result, tt.expected)
			}
		})
	}
}

// ==================== 文件/目录操作测试 ====================

func TestFileExists(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.txt")
	nonExistingFile := filepath.Join(tmpDir, "not_exists.txt")

	// 创建测试文件
	err := os.WriteFile(existingFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"存在的文件", existingFile, true},
		{"不存在的文件", nonExistingFile, false},
		{"空路径", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FileExists(tt.path)
			if result != tt.expected {
				t.Errorf("FileExists(%q) = %v; 期望 %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "exists")
	nonExistingDir := filepath.Join(tmpDir, "not_exists")

	// 创建测试目录
	err := os.Mkdir(existingDir, 0755)
	if err != nil {
		t.Fatalf("创建测试目录失败：%v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"存在的目录", existingDir, true},
		{"不存在的目录", nonExistingDir, false},
		{"空路径", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DirExists(tt.path)
			if result != tt.expected {
				t.Errorf("DirExists(%q) = %v; 期望 %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{"创建新目录", filepath.Join(tmpDir, "new_dir"), false},
		{"已存在的目录", tmpDir, false},
		{"嵌套目录", filepath.Join(tmpDir, "a", "b", "c"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureDir(tt.path)
			if tt.expectError {
				if err == nil {
					t.Errorf("EnsureDir(%q) 期望错误，但成功", tt.path)
				}
			} else {
				if err != nil {
					t.Errorf("EnsureDir(%q) 意外错误：%v", tt.path, err)
				}
				// 验证目录确实存在
				if !DirExists(tt.path) {
					t.Errorf("EnsureDir(%q) 后目录仍不存在", tt.path)
				}
			}
		})
	}
}

func TestNormalizeDirPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"正常路径", "/home/user", "/home/user"},
		{"带末尾斜杠", "/home/user/", "/home/user"},
		{"带空格", "  /home/user  ", "/home/user"},
		{"空字符串", "", ""},
		{"当前目录", ".", "."},
		{"父目录", "..", ".."},
		{"多重斜杠", "/home//user", "/home/user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeDirPath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeDirPath(%q) = %q; 期望 %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ==================== ReadHostFile 测试 ====================

func TestReadHostFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建测试主机文件
	hostFileContent := `# 这是注释
host1.example.com
host2.example.com:2222
user@host3.example.com
user2@host4.example.com:2223

# 另一个注释
host5.example.com
`
	hostFilePath := filepath.Join(tmpDir, "hosts.txt")
	err := os.WriteFile(hostFilePath, []byte(hostFileContent), 0644)
	if err != nil {
		t.Fatalf("创建测试主机文件失败：%v", err)
	}

	// 测试正常读取
	hosts, err := ReadHostFile(hostFilePath)
	if err != nil {
		t.Fatalf("ReadHostFile 意外失败：%v", err)
	}

	expectedHosts := []string{
		"host1.example.com",
		"host2.example.com:2222",
		"user@host3.example.com",
		"user2@host4.example.com:2223",
		"host5.example.com",
	}

	if len(hosts) != len(expectedHosts) {
		t.Errorf("ReadHostFile 返回 %d 个主机; 期望 %d", len(hosts), len(expectedHosts))
	}

	for i, expected := range expectedHosts {
		if i < len(hosts) && hosts[i] != expected {
			t.Errorf("主机 %d: %q; 期望 %q", i, hosts[i], expected)
		}
	}

	// 测试不存在的文件
	_, err = ReadHostFile(filepath.Join(tmpDir, "not_exists.txt"))
	if err == nil {
		t.Error("ReadHostFile 读取不存在的文件时应该返回错误")
	}
}

// ==================== 统计函数测试 ====================

func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"a 较小", 1.0, 2.0, 1.0},
		{"b 较小", 5.0, 3.0, 3.0},
		{"相等", 4.0, 4.0, 4.0},
		{"负数", -1.0, 1.0, -1.0},
		{"小数", 1.5, 1.7, 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Min(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Min(%.2f, %.2f) = %.2f; 期望 %.2f", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"a 较大", 5.0, 3.0, 5.0},
		{"b 较大", 1.0, 2.0, 2.0},
		{"相等", 4.0, 4.0, 4.0},
		{"负数", -1.0, 1.0, 1.0},
		{"小数", 1.7, 1.5, 1.7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Max(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Max(%.2f, %.2f) = %.2f; 期望 %.2f", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestAverage(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{"空切片", []float64{}, 0},
		{"单个值", []float64{5.0}, 5.0},
		{"两个值", []float64{3.0, 7.0}, 5.0},
		{"多个值", []float64{1.0, 2.0, 3.0, 4.0, 5.0}, 3.0},
		{"包含负数", []float64{-1.0, 0.0, 1.0}, 0.0},
		{"小数", []float64{1.5, 2.5, 3.5}, 2.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Average(tt.values)
			if result != tt.expected {
				t.Errorf("Average(%v) = %.2f; 期望 %.2f", tt.values, result, tt.expected)
			}
		})
	}
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{"空切片", []float64{}, 0},
		{"单个值", []float64{5.0}, 5.0},
		{"奇数个元素", []float64{3.0, 1.0, 2.0}, 2.0},
		{"偶数个元素", []float64{1.0, 2.0, 3.0, 4.0}, 2.5},
		{"未排序输入", []float64{5.0, 2.0, 8.0, 1.0, 9.0}, 5.0},
		{"包含负数", []float64{-1.0, 0.0, 1.0}, 0.0},
		{"重复值", []float64{1.0, 1.0, 1.0, 1.0}, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Median(tt.values)
			if result != tt.expected {
				t.Errorf("Median(%v) = %.2f; 期望 %.2f", tt.values, result, tt.expected)
			}
		})
	}
}

func TestMinSlice(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{"空切片", []float64{}, 0},
		{"单个值", []float64{5.0}, 5.0},
		{"多个值", []float64{3.0, 1.0, 4.0, 1.0, 5.0}, 1.0},
		{"包含负数", []float64{-1.0, 0.0, 1.0}, -1.0},
		{"全负数", []float64{-5.0, -2.0, -10.0}, -10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MinSlice(tt.values)
			if result != tt.expected {
				t.Errorf("MinSlice(%v) = %.2f; 期望 %.2f", tt.values, result, tt.expected)
			}
		})
	}
}

func TestMaxSlice(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{"空切片", []float64{}, 0},
		{"单个值", []float64{5.0}, 5.0},
		{"多个值", []float64{3.0, 1.0, 4.0, 1.0, 5.0}, 5.0},
		{"包含负数", []float64{-1.0, 0.0, 1.0}, 1.0},
		{"全负数", []float64{-5.0, -2.0, -10.0}, -2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaxSlice(tt.values)
			if result != tt.expected {
				t.Errorf("MaxSlice(%v) = %.2f; 期望 %.2f", tt.values, result, tt.expected)
			}
		})
	}
}

// ==================== 基准测试 ====================

func BenchmarkFormatBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatBytes(1048576)
	}
}

func BenchmarkParseFileSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseFileSize("1GB")
	}
}

func BenchmarkFormatBandwidth(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatBandwidth(100.5)
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatDuration(3600.5)
	}
}

func BenchmarkAverage(b *testing.B) {
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Average(values)
	}
}

func BenchmarkMedian(b *testing.B) {
	values := []float64{5.0, 2.0, 8.0, 1.0, 9.0, 3.0, 7.0, 4.0, 6.0, 10.0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Median(values)
	}
}

func BenchmarkMinSlice(b *testing.B) {
	values := []float64{5.0, 2.0, 8.0, 1.0, 9.0, 3.0, 7.0, 4.0, 6.0, 10.0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MinSlice(values)
	}
}

func BenchmarkMaxSlice(b *testing.B) {
	values := []float64{5.0, 2.0, 8.0, 1.0, 9.0, 3.0, 7.0, 4.0, 6.0, 10.0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MaxSlice(values)
	}
}
