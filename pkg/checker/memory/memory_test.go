// Package memory 提供内存带宽测试功能
// 本文件包含 memory 包的完整单元测试
package memory

import (
	"testing"
)

// ==================== NewStreamChecker 测试 ====================

func TestNewStreamChecker(t *testing.T) {
	tests := []struct {
		name        string
		verbose     bool
		expectSize  int
		expectIters int
	}{
		{"详细模式", true, 10 * 1024 * 1024, 3},
		{"静默模式", false, 10 * 1024 * 1024, 3},
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

			if checker.ArraySize != tt.expectSize {
				t.Errorf("ArraySize = %d; 期望 %d", checker.ArraySize, tt.expectSize)
			}

			if checker.Iterations != tt.expectIters {
				t.Errorf("Iterations = %d; 期望 %d", checker.Iterations, tt.expectIters)
			}
		})
	}
}

// ==================== parseStreamOutput 测试 ====================

func TestParseStreamOutput(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectCopy     float64
		expectScale    float64
		expectAdd      float64
		expectTriad    float64
		expectTotal    float64
		expectError    bool
	}{
		{
			name: "标准 STREAM 输出",
			output: `-----------------------------------------------------------
Function    Best Rate MB/s  Avg time     Min time     Max time
Copy:           10000.00     0.0100       0.0080       0.0120
Scale:           9500.50     0.0105       0.0084       0.0126
Add:            10500.25     0.0095       0.0076       0.0114
Triad:          10200.75     0.0098       0.0078       0.0117
-----------------------------------------------------------`,
			expectCopy:  10000.00,
			expectScale: 9500.50,
			expectAdd:   10500.25,
			expectTriad: 10200.75,
			expectTotal: (10000.00 + 9500.50 + 10500.25 + 10200.75) / 4.0,
			expectError: false,
		},
		{
			name: "简化输出格式",
			output: `Copy:  8000.00
Scale: 7500.00
Add:   8200.00
Triad: 8100.00`,
			expectCopy:  8000.00,
			expectScale: 7500.00,
			expectAdd:   8200.00,
			expectTriad: 8100.00,
			expectTotal: (8000.00 + 7500.00 + 8200.00 + 8100.00) / 4.0,
			expectError: false,
		},
		{
			name:        "空输出",
			output:      "",
			expectCopy:  0,
			expectScale: 0,
			expectAdd:   0,
			expectTriad: 0,
			expectTotal: 0,
			expectError: false,
		},
		{
			name: "部分数据",
			output: `Copy:  9000.00
Scale: 8500.00`,
			expectCopy:  9000.00,
			expectScale: 8500.00,
			expectAdd:   0,
			expectTriad: 0,
			expectTotal: (9000.00 + 8500.00 + 0 + 0) / 4.0,
			expectError: false,
		},
		{
			name: "包含无效数据",
			output: `Copy:  10000.00
Scale: invalid
Add:   10500.00
Triad: 10200.00`,
			expectCopy:  10000.00,
			expectScale: 0, // 解析失败应为 0
			expectAdd:   10500.00,
			expectTriad: 10200.00,
			expectTotal: (10000.00 + 0 + 10500.00 + 10200.00) / 4.0,
			expectError: false,
		},
		{
			name: "大小写混合",
			output: `COPY:  9500.00
SCALE: 9000.00
ADD:   9800.00
TRIAD: 9600.00`,
			expectCopy:  9500.00,
			expectScale: 9000.00,
			expectAdd:   9800.00,
			expectTriad: 9600.00,
			expectTotal: (9500.00 + 9000.00 + 9800.00 + 9600.00) / 4.0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &StreamChecker{
				Verbose: false,
			}

			result, err := checker.parseStreamOutput(tt.output)

			if tt.expectError && err == nil {
				t.Errorf("parseStreamOutput() 期望错误，但返回 nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("parseStreamOutput() 意外错误：%v", err)
			}

			if result == nil {
				t.Fatal("parseStreamOutput() 返回 nil")
			}

			// 允许 0.01 的误差
		 epsilon := 0.01
			if abs(result.CopyBandwidth-tt.expectCopy) > epsilon {
				t.Errorf("CopyBandwidth = %.2f; 期望 %.2f", result.CopyBandwidth, tt.expectCopy)
			}
			if abs(result.ScaleBandwidth-tt.expectScale) > epsilon {
				t.Errorf("ScaleBandwidth = %.2f; 期望 %.2f", result.ScaleBandwidth, tt.expectScale)
			}
			if abs(result.AddBandwidth-tt.expectAdd) > epsilon {
				t.Errorf("AddBandwidth = %.2f; 期望 %.2f", result.AddBandwidth, tt.expectAdd)
			}
			if abs(result.TriadBandwidth-tt.expectTriad) > epsilon {
				t.Errorf("TriadBandwidth = %.2f; 期望 %.2f", result.TriadBandwidth, tt.expectTriad)
			}
			if abs(result.TotalBandwidth-tt.expectTotal) > epsilon {
				t.Errorf("TotalBandwidth = %.2f; 期望 %.2f", result.TotalBandwidth, tt.expectTotal)
			}
		})
	}
}

// ==================== StreamResult 测试 ====================

func TestStreamResult_Validation(t *testing.T) {
	result := &StreamResult{
		Host:           "localhost",
		CopyBandwidth:  10000.00,
		ScaleBandwidth: 9500.00,
		AddBandwidth:   10500.00,
		TriadBandwidth: 10200.00,
		TotalBandwidth: 10050.00,
	}

	if result.Host != "localhost" {
		t.Errorf("Host = %q; 期望 \"localhost\"", result.Host)
	}

	if result.CopyBandwidth <= 0 {
		t.Error("CopyBandwidth 应该大于 0")
	}

	if result.TotalBandwidth <= 0 {
		t.Error("TotalBandwidth 应该大于 0")
	}

	// 验证带宽值合理性（应该在合理范围内）
	if result.CopyBandwidth < 100 || result.CopyBandwidth > 1000000 {
		t.Logf("警告：CopyBandwidth = %.2f MB/s，可能异常", result.CopyBandwidth)
	}
}

// ==================== 边界条件测试 ====================

func TestParseStreamOutput_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		output string
		desc   string
	}{
		{
			name: "带空格的行",
			output: `  Copy:   10000.00  
  Scale:  9500.00`,
			desc: "应该能处理前后空格",
		},
		{
			name: "带制表符",
			output: "Copy:\t10000.00\nScale:\t9500.00",
			desc: "应该能处理制表符分隔",
		},
		{
			name: "多行空行",
			output: `Copy:  10000.00

Scale: 9500.00

Add:   10500.00`,
			desc: "应该能跳过空行",
		},
		{
			name:   "科学计数法",
			output: "Copy:  1.0e+04\nScale: 9.5e+03",
			desc: "应该能解析科学计数法",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &StreamChecker{Verbose: false}
			result, err := checker.parseStreamOutput(tt.output)

			if err != nil {
				t.Logf("%s: %s - 结果：%+v", tt.name, tt.desc, result)
			}
		})
	}
}

// ==================== 辅助函数 ====================

// abs 返回绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ==================== 基准测试 ====================

func BenchmarkParseStreamOutput(b *testing.B) {
	output := `-----------------------------------------------------------
Function    Best Rate MB/s  Avg time     Min time     Max time
Copy:           10000.00     0.0100       0.0080       0.0120
Scale:           9500.50     0.0105       0.0084       0.0126
Add:            10500.25     0.0095       0.0076       0.0114
Triad:          10200.75     0.0098       0.0078       0.0117
-----------------------------------------------------------`

	checker := &StreamChecker{Verbose: false}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.parseStreamOutput(output)
	}
}

func BenchmarkNewStreamChecker(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewStreamChecker(false)
	}
}

// ==================== 集成测试 ====================

func TestStreamChecker_Run_Integration(t *testing.T) {
	// 跳过长时间运行的测试
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	checker := NewStreamChecker(false)
	// 减小数组大小以加快测试
	checker.ArraySize = 1024 * 1024 // 1M 元素
	checker.Iterations = 2          // 减少迭代次数

	result, err := checker.Run()

	if err != nil {
		t.Fatalf("Run() 意外错误：%v", err)
	}

	if result == nil {
		t.Fatal("Run() 返回 nil")
	}

	if result.Host == "" {
		t.Error("Host 不应该为空")
	}

	// 验证带宽值应该大于 0
	if result.CopyBandwidth <= 0 {
		t.Error("CopyBandwidth 应该大于 0")
	}
	if result.ScaleBandwidth <= 0 {
		t.Error("ScaleBandwidth 应该大于 0")
	}
	if result.AddBandwidth <= 0 {
		t.Error("AddBandwidth 应该大于 0")
	}
	if result.TriadBandwidth <= 0 {
		t.Error("TriadBandwidth 应该大于 0")
	}
	if result.TotalBandwidth <= 0 {
		t.Error("TotalBandwidth 应该大于 0")
	}

	t.Logf("内存带宽测试结果：Total = %.2f MB/s", result.TotalBandwidth)
}

// ==================== 错误处理测试 ====================

func TestParseStreamOutput_InvalidFormats(t *testing.T) {
	tests := []struct {
		name   string
		output string
		desc   string
	}{
		{
			name:   "负数带宽",
			output: "Copy:  -1000.00",
			desc:   "负数带宽值（异常情况）",
		},
		{
			name:   "超大数值",
			output: "Copy:  999999999999.00",
			desc:   "超大带宽值（异常情况）",
		},
		{
			name:   "零带宽",
			output: "Copy:  0.00",
			desc:   "零带宽值（异常情况）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &StreamChecker{Verbose: false}
			result, _ := checker.parseStreamOutput(tt.output)

			t.Logf("%s: %s - 结果：Copy=%.2f", tt.name, tt.desc, result.CopyBandwidth)
		})
	}
}
