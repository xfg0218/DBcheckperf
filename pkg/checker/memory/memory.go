// Package memory 提供内存带宽测试功能
// 使用 STREAM 基准测试方法
package memory

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dbcheckperf/pkg/checker/common"
	"dbcheckperf/pkg/utils"
)

// StreamResult 内存带宽测试结果
type StreamResult struct {
	// Host 主机名
	Host string
	// CopyBandwidth 复制带宽（MB/s）
	CopyBandwidth float64
	// ScaleBandwidth 缩放带宽（MB/s）
	ScaleBandwidth float64
	// AddBandwidth 加法带宽（MB/s）
	AddBandwidth float64
	// TriadBandwidth 三合一带宽（MB/s）
	TriadBandwidth float64
	// TotalBandwidth 总带宽（MB/s）
	TotalBandwidth float64
}

// StreamChecker 内存带宽检查器
type StreamChecker struct {
	// Verbose 是否显示详细输出
	Verbose bool
	// ArraySize 测试数组大小（元素个数）
	ArraySize int
	// Iterations 测试迭代次数
	Iterations int
}

// NewStreamChecker 创建新的内存带宽检查器
func NewStreamChecker(verbose bool) *StreamChecker {
	return &StreamChecker{
		Verbose:    verbose,
		ArraySize:  10 * 1024 * 1024, // 默认 10M 个元素（约 80MB）
		Iterations: 3,                // 默认测试 3 次取平均值
	}
}

// Run 执行内存带宽测试
// 使用 STREAM 基准测试方法，实际执行内存操作测试
func (sc *StreamChecker) Run() (*StreamResult, error) {
	// 尝试运行 STREAM 程序
	streamPath, err := exec.LookPath("stream")
	if err == nil {
		cmd := exec.Command(streamPath)
		output, err := cmd.Output()
		if err == nil {
			return sc.parseStreamOutput(string(output))
		}
	}

	// 如果 STREAM 不可用，使用 Go 实现的内存带宽测试
	return sc.runMemoryBandwidthTest()
}

// runMemoryBandwidthTest 运行内存带宽测试（Go 实现）
func (sc *StreamChecker) runMemoryBandwidthTest() (*StreamResult, error) {
	result := &StreamResult{
		Host: common.GetHostname(),
	}

	if sc.Verbose {
		fmt.Printf("内存带宽测试：数组大小 %d 元素，迭代 %d 次\n", sc.ArraySize, sc.Iterations)
	}

	// 分配测试数组
	a := make([]float64, sc.ArraySize)
	b := make([]float64, sc.ArraySize)
	c := make([]float64, sc.ArraySize)

	// 初始化数据
	for i := 0; i < sc.ArraySize; i++ {
		a[i] = 1.0
		b[i] = 2.0
		c[i] = 0.0
	}

	// 1. Copy 测试：a[i] = b[i]
	var copyBandwidths []float64
	for iter := 0; iter < sc.Iterations; iter++ {
		start := time.Now()
		for i := 0; i < sc.ArraySize; i++ {
			a[i] = b[i]
		}
		duration := time.Since(start).Seconds()
		// 数据传输量：读取 b + 写入 a = 2 * N * 8 bytes
		bytesTransferred := float64(sc.ArraySize) * 8 * 2
		bandwidth := bytesTransferred / duration / (1024 * 1024) // MB/s
		copyBandwidths = append(copyBandwidths, bandwidth)
	}
	result.CopyBandwidth = utils.Average(copyBandwidths)

	// 2. Scale 测试：a[i] = b[i] * 3
	var scaleBandwidths []float64
	for iter := 0; iter < sc.Iterations; iter++ {
		start := time.Now()
		for i := 0; i < sc.ArraySize; i++ {
			a[i] = b[i] * 3.0
		}
		duration := time.Since(start).Seconds()
		bytesTransferred := float64(sc.ArraySize) * 8 * 2
		bandwidth := bytesTransferred / duration / (1024 * 1024)
		scaleBandwidths = append(scaleBandwidths, bandwidth)
	}
	result.ScaleBandwidth = utils.Average(scaleBandwidths)

	// 3. Add 测试：a[i] = b[i] + c[i]
	var addBandwidths []float64
	for iter := 0; iter < sc.Iterations; iter++ {
		start := time.Now()
		for i := 0; i < sc.ArraySize; i++ {
			a[i] = b[i] + c[i]
		}
		duration := time.Since(start).Seconds()
		// 数据传输量：读取 b + 读取 c + 写入 a = 3 * N * 8 bytes
		bytesTransferred := float64(sc.ArraySize) * 8 * 3
		bandwidth := bytesTransferred / duration / (1024 * 1024)
		addBandwidths = append(addBandwidths, bandwidth)
	}
	result.AddBandwidth = utils.Average(addBandwidths)

	// 4. Triad 测试：a[i] = b[i] + c[i] * 3
	var triadBandwidths []float64
	for iter := 0; iter < sc.Iterations; iter++ {
		start := time.Now()
		for i := 0; i < sc.ArraySize; i++ {
			a[i] = b[i] + c[i]*3.0
		}
		duration := time.Since(start).Seconds()
		bytesTransferred := float64(sc.ArraySize) * 8 * 3
		bandwidth := bytesTransferred / duration / (1024 * 1024)
		triadBandwidths = append(triadBandwidths, bandwidth)
	}
	result.TriadBandwidth = utils.Average(triadBandwidths)

	// 计算总带宽（四项平均）
	result.TotalBandwidth = (result.CopyBandwidth + result.ScaleBandwidth +
		result.AddBandwidth + result.TriadBandwidth) / 4.0

	if sc.Verbose {
		fmt.Printf("内存带宽测试结果：\n")
		fmt.Printf("  Copy:  %.2f MB/s\n", result.CopyBandwidth)
		fmt.Printf("  Scale: %.2f MB/s\n", result.ScaleBandwidth)
		fmt.Printf("  Add:   %.2f MB/s\n", result.AddBandwidth)
		fmt.Printf("  Triad: %.2f MB/s\n", result.TriadBandwidth)
		fmt.Printf("  Total: %.2f MB/s\n", result.TotalBandwidth)
	}

	return result, nil
}

// parseStreamOutput 解析 STREAM 输出
func (sc *StreamChecker) parseStreamOutput(output string) (*StreamResult, error) {
	result := &StreamResult{
		Host: common.GetHostname(),
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := strings.ToLower(strings.TrimSuffix(fields[0], ":"))
			value, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				if sc.Verbose {
					fmt.Printf("警告：解析 STREAM 输出失败：%v\n", err)
				}
				continue
			}
			switch key {
			case "copy":
				result.CopyBandwidth = value
			case "scale":
				result.ScaleBandwidth = value
			case "add":
				result.AddBandwidth = value
			case "triad":
				result.TriadBandwidth = value
			}
		}
	}

	// 计算总带宽（四项平均值）
	if result.CopyBandwidth > 0 || result.ScaleBandwidth > 0 ||
		result.AddBandwidth > 0 || result.TriadBandwidth > 0 {
		result.TotalBandwidth = (result.CopyBandwidth + result.ScaleBandwidth +
			result.AddBandwidth + result.TriadBandwidth) / 4.0
	}

	return result, nil
}
