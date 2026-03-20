// Package config 提供配置管理功能
// 用于解析和管理 dbcheckperf 工具的运行配置参数
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ==================== TestType 常量测试 ====================

func TestTestTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		testType TestType
		expected string
	}{
		{"磁盘 I/O 测试", TestDisk, "d"},
		{"内存带宽测试", TestStream, "s"},
		{"网络串行测试", TestNetworkSerial, "n"},
		{"网络并行测试", TestNetworkParallel, "N"},
		{"网络全矩阵测试", TestNetworkMatrix, "M"},
		{"硬件信息收集", TestHardware, "H"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.testType) != tt.expected {
				t.Errorf("TestType 常量值 = %q; 期望 %q", tt.testType, tt.expected)
			}
		})
	}
}

// ==================== DefaultConfig 测试 ====================

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// 验证配置不为 nil
	if config == nil {
		t.Fatal("DefaultConfig() 返回 nil")
	}

	// 验证默认值
	if config.BlockSize != 32 {
		t.Errorf("BlockSize = %d; 期望 32", config.BlockSize)
	}

	if config.FileSize != "2xRAM" {
		t.Errorf("FileSize = %q; 期望 \"2xRAM\"", config.FileSize)
	}

	if config.Duration != 15*time.Second {
		t.Errorf("Duration = %v; 期望 15s", config.Duration)
	}

	if config.BufferSize != 8 {
		t.Errorf("BufferSize = %d; 期望 8", config.BufferSize)
	}

	// 验证默认测试类型
	expectedTestTypes := []TestType{TestDisk, TestStream, TestNetworkSerial}
	if len(config.TestTypes) != len(expectedTestTypes) {
		t.Errorf("TestTypes 长度 = %d; 期望 %d", len(config.TestTypes), len(expectedTestTypes))
	}

	for i, expected := range expectedTestTypes {
		if i < len(config.TestTypes) && config.TestTypes[i] != expected {
			t.Errorf("TestTypes[%d] = %q; 期望 %q", i, config.TestTypes[i], expected)
		}
	}

	// 验证 RandBlockSize 默认值为 0
	if config.RandBlockSize != 0 {
		t.Errorf("RandBlockSize = %d; 期望 0", config.RandBlockSize)
	}

	// 验证切片可以为 nil 或空（在 Go 中 nil 切片和空切片行为相同）
	// 注意：DefaultConfig 返回 nil 切片是可以接受的，因为它们在功能上等同于空切片
}

// TestDefaultConfig_Independence 测试多次调用返回独立实例
func TestDefaultConfig_Independence(t *testing.T) {
	config1 := DefaultConfig()
	config2 := DefaultConfig()

	// 修改 config1
	config1.BlockSize = 100
	config1.Hosts = append(config1.Hosts, "host1")

	// 验证 config2 不受影响
	if config1.BlockSize == config2.BlockSize {
		t.Error("DefaultConfig() 应该返回独立的实例")
	}

	if len(config1.Hosts) == len(config2.Hosts) {
		t.Error("修改 config1.Hosts 不应该影响 config2.Hosts")
	}
}

// ==================== Validate 测试 ====================

func TestValidate_HardwareOnly(t *testing.T) {
	// 仅硬件信息收集模式，不需要主机和测试目录
	config := &Config{
		TestTypes: []TestType{TestHardware},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("仅硬件模式 Validate() 不应该返回错误：%v", err)
	}
}

func TestValidate_HardwareWithOtherTypes(t *testing.T) {
	// 硬件模式与其他模式组合时，仍需验证主机和目录
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "硬件 + 磁盘测试，有主机和目录",
			config: &Config{
				TestTypes: []TestType{TestHardware, TestDisk},
				Hosts:     []string{"localhost"},
				TestDirs:  []string{tmpDir},
			},
			expectErr: false,
		},
		{
			name: "硬件 + 磁盘测试，无主机",
			config: &Config{
				TestTypes: []TestType{TestHardware, TestDisk},
				TestDirs:  []string{tmpDir},
			},
			expectErr: true,
		},
		{
			name: "硬件 + 磁盘测试，无目录",
			config: &Config{
				TestTypes: []TestType{TestHardware, TestDisk},
				Hosts:     []string{"localhost"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr && err == nil {
				t.Errorf("Validate() 期望错误但返回 nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Validate() 意外错误：%v", err)
			}
		})
	}
}

func TestValidate_NoHosts(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		Hosts:    []string{},
		HostFile: "",
		TestDirs: []string{tmpDir},
		TestTypes: []TestType{TestDisk},
	}

	err := config.Validate()
	if err != ErrNoHostsSpecified {
		t.Errorf("Validate() 应该返回 ErrNoHostsSpecified; 得到：%v", err)
	}
}

func TestValidate_WithHostFile(t *testing.T) {
	tmpDir := t.TempDir()
	hostFile := filepath.Join(tmpDir, "hosts.txt")

	// 创建测试主机文件
	err := os.WriteFile(hostFile, []byte("localhost"), 0644)
	if err != nil {
		t.Fatalf("创建测试主机文件失败：%v", err)
	}

	config := &Config{
		Hosts:    []string{},
		HostFile: hostFile,
		TestDirs: []string{tmpDir},
		TestTypes: []TestType{TestDisk},
	}

	err = config.Validate()
	if err != nil {
		t.Errorf("使用 HostFile 时 Validate() 不应该返回错误：%v", err)
	}
}

func TestValidate_NoTestDirs(t *testing.T) {
	config := &Config{
		Hosts:     []string{"localhost"},
		TestDirs:  []string{},
		TestTypes: []TestType{TestDisk},
	}

	err := config.Validate()
	if err != ErrNoTestDirsSpecified {
		t.Errorf("Validate() 应该返回 ErrNoTestDirsSpecified; 得到：%v", err)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		Hosts:     []string{"localhost", "host2"},
		TestDirs:  []string{tmpDir, "/tmp"},
		TestTypes: []TestType{TestDisk, TestStream},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("有效配置 Validate() 不应该返回错误：%v", err)
	}
}

// TestValidate_MultipleHosts 测试多主机配置
func TestValidate_MultipleHosts(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		hosts     []string
		expectErr bool
	}{
		{"单个主机", []string{"localhost"}, false},
		{"多个主机", []string{"localhost", "host2", "host3"}, false},
		{"带端口的主机", []string{"localhost:22", "host2:2222"}, false},
		{"带用户的主机", []string{"user@localhost", "admin@host2"}, false},
		{"空主机列表", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Hosts:     tt.hosts,
				TestDirs:  []string{tmpDir},
				TestTypes: []TestType{TestDisk},
			}

			err := config.Validate()
			if tt.expectErr && err == nil {
				t.Errorf("Validate() 期望错误但返回 nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Validate() 意外错误：%v", err)
			}
		})
	}
}

// TestValidate_MultipleTestDirs 测试多测试目录配置
func TestValidate_MultipleTestDirs(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	tests := []struct {
		name      string
		testDirs  []string
		expectErr bool
	}{
		{"单个目录", []string{tmpDir1}, false},
		{"多个目录", []string{tmpDir1, tmpDir2, "/tmp"}, false},
		{"空目录列表", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Hosts:     []string{"localhost"},
				TestDirs:  tt.testDirs,
				TestTypes: []TestType{TestDisk},
			}

			err := config.Validate()
			if tt.expectErr && err == nil {
				t.Errorf("Validate() 期望错误但返回 nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Validate() 意外错误：%v", err)
			}
		})
	}
}

// ==================== 错误变量测试 ====================

func TestErrorVariables(t *testing.T) {
	// 验证错误变量已正确定义
	if ErrNoHostsSpecified == nil {
		t.Error("ErrNoHostsSpecified 应该被定义")
	}

	if ErrNoTestDirsSpecified == nil {
		t.Error("ErrNoTestDirsSpecified 应该被定义")
	}

	// 验证错误类型
	if ErrNoHostsSpecified != os.ErrInvalid {
		t.Error("ErrNoHostsSpecified 应该基于 os.ErrInvalid")
	}

	if ErrNoTestDirsSpecified != os.ErrInvalid {
		t.Error("ErrNoTestDirsSpecified 应该基于 os.ErrInvalid")
	}
}

// ==================== hasTestType 辅助函数测试 ====================

func TestHasTestType(t *testing.T) {
	tests := []struct {
		name     string
		types    []TestType
		target   TestType
		expected bool
	}{
		{"空列表", []TestType{}, TestDisk, false},
		{"单个匹配", []TestType{TestDisk}, TestDisk, true},
		{"单个不匹配", []TestType{TestDisk}, TestStream, false},
		{"多个匹配 - 第一个", []TestType{TestDisk, TestStream, TestNetworkSerial}, TestDisk, true},
		{"多个匹配 - 中间", []TestType{TestDisk, TestStream, TestNetworkSerial}, TestStream, true},
		{"多个匹配 - 最后", []TestType{TestDisk, TestStream, TestNetworkSerial}, TestNetworkSerial, true},
		{"不匹配", []TestType{TestDisk, TestStream}, TestHardware, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasTestType(tt.types, tt.target)
			if result != tt.expected {
				t.Errorf("hasTestType(%v, %q) = %v; 期望 %v",
					tt.types, tt.target, result, tt.expected)
			}
		})
	}
}

// ==================== Config 结构体字段测试 ====================

func TestConfig_Fields(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		Hosts:            []string{"host1", "host2"},
		HostFile:         "/path/to/hosts.txt",
		TestDirs:         []string{tmpDir, "/tmp"},
		TempDir:          "/tmp/dbcheckperf",
		TestTypes:        []TestType{TestDisk, TestStream},
		BlockSize:        64,
		FileSize:         "1GB",
		Duration:         30 * time.Second,
		DisplayPerHost:   true,
		Verbose:          true,
		VeryVerbose:      true,
		UseNetperf:       true,
		BufferSize:       16,
		RandBlockSize:    4,
	}

	// 验证所有字段
	if len(config.Hosts) != 2 {
		t.Errorf("Hosts 长度 = %d; 期望 2", len(config.Hosts))
	}

	if config.HostFile != "/path/to/hosts.txt" {
		t.Errorf("HostFile = %q; 期望 \"/path/to/hosts.txt\"", config.HostFile)
	}

	if len(config.TestDirs) != 2 {
		t.Errorf("TestDirs 长度 = %d; 期望 2", len(config.TestDirs))
	}

	if config.TempDir != "/tmp/dbcheckperf" {
		t.Errorf("TempDir = %q; 期望 \"/tmp/dbcheckperf\"", config.TempDir)
	}

	if len(config.TestTypes) != 2 {
		t.Errorf("TestTypes 长度 = %d; 期望 2", len(config.TestTypes))
	}

	if config.BlockSize != 64 {
		t.Errorf("BlockSize = %d; 期望 64", config.BlockSize)
	}

	if config.FileSize != "1GB" {
		t.Errorf("FileSize = %q; 期望 \"1GB\"", config.FileSize)
	}

	if config.Duration != 30*time.Second {
		t.Errorf("Duration = %v; 期望 30s", config.Duration)
	}

	if !config.DisplayPerHost {
		t.Error("DisplayPerHost 应该为 true")
	}

	if !config.Verbose {
		t.Error("Verbose 应该为 true")
	}

	if !config.VeryVerbose {
		t.Error("VeryVerbose 应该为 true")
	}

	if !config.UseNetperf {
		t.Error("UseNetperf 应该为 true")
	}

	if config.BufferSize != 16 {
		t.Errorf("BufferSize = %d; 期望 16", config.BufferSize)
	}

	if config.RandBlockSize != 4 {
		t.Errorf("RandBlockSize = %d; 期望 4", config.RandBlockSize)
	}
}

// ==================== 边界条件测试 ====================

func TestValidate_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		config    *Config
		expectErr bool
		desc      string
	}{
		{
			name: "仅 HostFile 无 Hosts",
			config: &Config{
				HostFile:  "/path/to/hosts.txt",
				TestDirs:  []string{tmpDir},
				TestTypes: []TestType{TestDisk},
			},
			expectErr: false,
			desc:      "HostFile 可以替代 Hosts",
		},
		{
			name: "空 TestTypes",
			config: &Config{
				Hosts:     []string{"localhost"},
				TestDirs:  []string{tmpDir},
				TestTypes: []TestType{},
			},
			expectErr: false,
			desc:      "空 TestTypes 应该通过验证（虽然可能无意义）",
		},
		{
			name: "nil TestTypes",
			config: &Config{
				Hosts:     []string{"localhost"},
				TestDirs:  []string{tmpDir},
				TestTypes: nil,
			},
			expectErr: false,
			desc:      "nil TestTypes 应该通过验证",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr && err == nil {
				t.Errorf("Validate() 期望错误：%s", tt.desc)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Validate() 意外错误：%v (%s)", err, tt.desc)
			}
		})
	}
}

// ==================== 基准测试 ====================

func BenchmarkDefaultConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DefaultConfig()
	}
}

func BenchmarkValidate_ValidConfig(b *testing.B) {
	config := &Config{
		Hosts:     []string{"localhost", "host2", "host3"},
		TestDirs:  []string{"/tmp", "/var/tmp"},
		TestTypes: []TestType{TestDisk, TestStream, TestNetworkSerial},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Validate()
	}
}

func BenchmarkValidate_HardwareOnly(b *testing.B) {
	config := &Config{
		TestTypes: []TestType{TestHardware},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Validate()
	}
}

func BenchmarkHasTestType(b *testing.B) {
	types := []TestType{TestDisk, TestStream, TestNetworkSerial, TestNetworkParallel, TestNetworkMatrix, TestHardware}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasTestType(types, TestStream)
	}
}
