// Package latency 提供磁盘 I/O 延迟和 IOPS 测试功能
// 使用 dd 和 fio 命令测量读写延迟和 IOPS
package latency

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dbcheckperf/pkg/checker/common"
	"dbcheckperf/pkg/utils"
)

// LatencyResult 延迟测试结果
type LatencyResult struct {
	// Host 主机名
	Host string
	// Dir 测试目录
	Dir string
	// ReadLatencyAvg 平均读延迟 (ms)
	ReadLatencyAvg float64
	// ReadLatencyMax 最大读延迟 (ms)
	ReadLatencyMax float64
	// ReadLatencyMin 最小读延迟 (ms)
	ReadLatencyMin float64
	// WriteLatencyAvg 平均写延迟 (ms)
	WriteLatencyAvg float64
	// WriteLatencyMax 最大写延迟 (ms)
	WriteLatencyMax float64
	// WriteLatencyMin 最小写延迟 (ms)
	WriteLatencyMin float64
	// ReadIOPS 读 IOPS (次/秒)
	ReadIOPS float64
	// WriteIOPS 写 IOPS (次/秒)
	WriteIOPS float64
	// BlockSize 测试块大小 (字节)
	BlockSize int64
	// IOSize IO 大小 (字节)
	IOSize int64
}

// LatencyChecker 延迟检查器
type LatencyChecker struct {
	// BlockSize 块大小 (KB)
	BlockSize int
	// IOSize IO 大小 (KB)，用于 IOPS 测试
	IOSize int
	// Iterations 测试迭代次数
	Iterations int
	// Verbose 是否显示详细输出
	Verbose bool
	// UseFio 是否使用 fio (更准确)
	UseFio bool
}

// NewLatencyChecker 创建新的延迟检查器
func NewLatencyChecker(blockSizeKB, ioSizeKB int, verbose, useFio bool) *LatencyChecker {
	if blockSizeKB <= 0 {
		blockSizeKB = 4 // 默认 4KB，模拟随机小 IO
	}
	if ioSizeKB <= 0 {
		ioSizeKB = blockSizeKB
	}
	return &LatencyChecker{
		BlockSize:  blockSizeKB,
		IOSize:     ioSizeKB,
		Iterations: 10, // 默认测试 10 次取平均值
		Verbose:    verbose,
		UseFio:     useFio,
	}
}

// Run 执行延迟和 IOPS 测试
func (lc *LatencyChecker) Run(dir string) (*LatencyResult, error) {
	// 尝试使用 fio 进行更准确的测试
	if lc.UseFio {
		if fioResult, err := lc.runFioTest(dir); err == nil {
			return fioResult, nil
		}
		if lc.Verbose {
			fmt.Fprintf(os.Stderr, "fio 不可用，使用 dd 方法进行延迟测试\n")
		}
	}

	// 使用 dd 方法进行延迟测试
	return lc.runDDTest(dir)
}

// runDDTest 使用 dd 命令进行延迟测试
func (lc *LatencyChecker) runDDTest(dir string) (*LatencyResult, error) {
	result := &LatencyResult{
		Host:      common.GetHostname(),
		Dir:       dir,
		BlockSize: int64(lc.BlockSize) * 1024,
		IOSize:    int64(lc.IOSize) * 1024,
	}

	// 生成测试文件
	testFile := path.Join(dir, fmt.Sprintf("latency_test_%d", time.Now().UnixNano()))
	defer os.Remove(testFile)

	// 先创建一个足够大的测试文件
	totalSize := uint64(result.IOSize) * uint64(lc.Iterations*10)
	if err := utils.CreateTestFile(testFile, totalSize); err != nil {
		return nil, fmt.Errorf("创建测试文件失败：%v", err)
	}

	// 测试读延迟
	readLatencies, err := lc.measureReadLatency(testFile)
	if err == nil && len(readLatencies) > 0 {
		result.ReadLatencyAvg, result.ReadLatencyMax, result.ReadLatencyMin = calcLatencyStats(readLatencies)
	}

	// 清空缓存
	dropCaches()

	// 测试写延迟
	writeLatencies, err := lc.measureWriteLatency(testFile)
	if err == nil && len(writeLatencies) > 0 {
		result.WriteLatencyAvg, result.WriteLatencyMax, result.WriteLatencyMin = calcLatencyStats(writeLatencies)
	}

	// 计算 IOPS
	result.ReadIOPS = calcIOPS(result.ReadLatencyAvg, result.IOSize)
	result.WriteIOPS = calcIOPS(result.WriteLatencyAvg, result.IOSize)

	return result, nil
}

// measureReadLatency 测量读延迟
func (lc *LatencyChecker) measureReadLatency(testFile string) ([]float64, error) {
	var latencies []float64

	for i := 0; i < lc.Iterations; i++ {
		// 使用 dd 读取单个块，测量时间
		cmd := exec.Command("dd", fmt.Sprintf("if=%s", testFile),
			fmt.Sprintf("bs=%d", lc.BlockSize*1024),
			"count=1", "iflag=direct", "of=/dev/null")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		if err == nil {
			latencyMs := float64(duration.Nanoseconds()) / 1e6 // 转换为 ms
			latencies = append(latencies, latencyMs)
			if lc.Verbose {
				fmt.Printf("读延迟测试 %d/%d: %.3f ms\n", i+1, lc.Iterations, latencyMs)
			}
		}
	}

	if len(latencies) == 0 {
		return nil, fmt.Errorf("读延迟测试失败")
	}

	return latencies, nil
}

// measureWriteLatency 测量写延迟
func (lc *LatencyChecker) measureWriteLatency(testFile string) ([]float64, error) {
	var latencies []float64

	// 创建临时写入文件
	tempFile := testFile + ".write"
	defer os.Remove(tempFile)

	for i := 0; i < lc.Iterations; i++ {
		// 使用 dd 写入单个块，测量时间
		cmd := exec.Command("dd", "if=/dev/zero",
			fmt.Sprintf("of=%s", tempFile),
			fmt.Sprintf("bs=%d", lc.BlockSize*1024),
			"count=1", "oflag=direct", "conv=fsync")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		if err == nil {
			latencyMs := float64(duration.Nanoseconds()) / 1e6 // 转换为 ms
			latencies = append(latencies, latencyMs)
			if lc.Verbose {
				fmt.Printf("写延迟测试 %d/%d: %.3f ms\n", i+1, lc.Iterations, latencyMs)
			}
		}
	}

	if len(latencies) == 0 {
		return nil, fmt.Errorf("写延迟测试失败")
	}

	return latencies, nil
}

// runFioTest 使用 fio 进行更准确的延迟和 IOPS 测试
func (lc *LatencyChecker) runFioTest(dir string) (*LatencyResult, error) {
	// 检查 fio 是否可用
	if _, err := exec.LookPath("fio"); err != nil {
		return nil, fmt.Errorf("fio 不可用")
	}

	result := &LatencyResult{
		Host:      common.GetHostname(),
		Dir:       dir,
		BlockSize: int64(lc.BlockSize) * 1024,
		IOSize:    int64(lc.IOSize) * 1024,
	}

	// 运行 fio 读测试
	readOutput, err := lc.runFioReadWriteTest(dir, "read")
	if err != nil {
		return nil, fmt.Errorf("fio 读测试失败：%v", err)
	}

	// 解析读结果
	readStats := parseFioOutput(readOutput)
	result.ReadLatencyAvg = readStats["latency_avg"]
	result.ReadLatencyMax = readStats["latency_max"]
	result.ReadLatencyMin = readStats["latency_min"]
	result.ReadIOPS = readStats["iops"]

	// 运行 fio 写测试
	writeOutput, err := lc.runFioReadWriteTest(dir, "write")
	if err != nil {
		return nil, fmt.Errorf("fio 写测试失败：%v", err)
	}

	// 解析写结果
	writeStats := parseFioOutput(writeOutput)
	result.WriteLatencyAvg = writeStats["latency_avg"]
	result.WriteLatencyMax = writeStats["latency_max"]
	result.WriteLatencyMin = writeStats["latency_min"]
	result.WriteIOPS = writeStats["iops"]

	return result, nil
}

// runFioReadWriteTest 运行 fio 读写测试
func (lc *LatencyChecker) runFioReadWriteTest(dir, rwType string) (string, error) {
	testFile := path.Join(dir, fmt.Sprintf("fio_test_%s_%d", rwType, time.Now().UnixNano()))
	defer os.Remove(testFile)

	// fio 命令参数
	args := []string{
		"--name=latency_test",
		fmt.Sprintf("--filename=%s", testFile),
		fmt.Sprintf("--bs=%d", lc.BlockSize*1024),
		fmt.Sprintf("--size=%d", lc.IOSize*1024*lc.Iterations),
		"--direct=1",
		"--ioengine=libaio",
		"--iodepth=1", // 队列深度为 1，测量单次 IO 延迟
		fmt.Sprintf("--rw=%s", rwType),
		"--runtime=10",
		"--time_based",
		"--output-format=json",
	}

	cmd := exec.Command("fio", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// parseFioOutput 解析 fio JSON 输出
func parseFioOutput(output string) map[string]float64 {
	stats := make(map[string]float64)

	// 简化解析，查找关键值
	// 查找 lat (nsec) 或 lat (usec) 或 lat (msec)
	latencyUnit := "msec"
	if strings.Contains(output, `"lat (nsec)"`) {
		latencyUnit = "nsec"
	} else if strings.Contains(output, `"lat (usec)"`) {
		latencyUnit = "usec"
	}

	// 解析平均延迟
	re := regexp.MustCompile(`"avg"\s*:\s*(\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		avg, _ := strconv.ParseFloat(matches[1], 64)
		if latencyUnit == "nsec" {
			avg /= 1e6 // 转换为 ms
		} else if latencyUnit == "usec" {
			avg /= 1000
		}
		stats["latency_avg"] = avg
	}

	// 解析最大延迟
	re = regexp.MustCompile(`"max"\s*:\s*(\d+(?:\.\d+)?)`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		max, _ := strconv.ParseFloat(matches[1], 64)
		if latencyUnit == "nsec" {
			max /= 1e6
		} else if latencyUnit == "usec" {
			max /= 1000
		}
		stats["latency_max"] = max
	}

	// 解析最小延迟
	re = regexp.MustCompile(`"min"\s*:\s*(\d+(?:\.\d+)?)`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		min, _ := strconv.ParseFloat(matches[1], 64)
		if latencyUnit == "nsec" {
			min /= 1e6
		} else if latencyUnit == "usec" {
			min /= 1000
		}
		stats["latency_min"] = min
	}

	// 解析 IOPS
	re = regexp.MustCompile(`"iops"\s*:\s*(\d+(?:\.\d+)?)`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		iops, _ := strconv.ParseFloat(matches[1], 64)
		stats["iops"] = iops
	}

	return stats
}

// calcLatencyStats 计算延迟统计值
func calcLatencyStats(latencies []float64) (avg, max, min float64) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}

	sum := 0.0
	max = latencies[0]
	min = latencies[0]

	for _, lat := range latencies {
		sum += lat
		if lat > max {
			max = lat
		}
		if lat < min {
			min = lat
		}
	}

	avg = sum / float64(len(latencies))
	return avg, max, min
}

// calcIOPS 根据延迟计算 IOPS
func calcIOPS(avgLatencyMs float64, ioSize int64) float64 {
	if avgLatencyMs <= 0 {
		return 0
	}
	// IOPS = 1000ms / 平均延迟 (ms)
	return 1000.0 / avgLatencyMs
}

// dropCaches 清空系统缓存
func dropCaches() {
	// 需要 root 权限，忽略错误
	cmd := exec.Command("sh", "-c", "echo 3 > /proc/sys/vm/drop_caches")
	cmd.Run()
}

// RunRemote 在远程主机执行延迟测试
func (lc *LatencyChecker) RunRemote(host, dir string) (*LatencyResult, error) {
	// 尝试使用 fio
	if lc.UseFio {
		if fioResult, err := lc.runRemoteFioTest(host, dir); err == nil {
			return fioResult, nil
		}
	}

	// 使用 dd 方法
	return lc.runRemoteDDTest(host, dir)
}

// runRemoteDDTest 在远程主机使用 dd 测试
func (lc *LatencyChecker) runRemoteDDTest(host, dir string) (*LatencyResult, error) {
	result := &LatencyResult{
		Host:      host,
		Dir:       dir,
		BlockSize: int64(lc.BlockSize) * 1024,
		IOSize:    int64(lc.IOSize) * 1024,
	}

	testFile := path.Join(dir, fmt.Sprintf("latency_test_%d", time.Now().UnixNano()))

	// 创建测试文件
	createCmd := fmt.Sprintf("dd if=/dev/zero of=%s bs=%d count=%d oflag=direct conv=fsync >/dev/null 2>&1",
		testFile, lc.BlockSize*1024, lc.Iterations*10)
	common.RunSSHCommand(host, createCmd)
	defer common.RunSSHCommand(host, fmt.Sprintf("rm -f %s", testFile))

	// 测试读延迟
	var readLatencies []float64
	for i := 0; i < lc.Iterations; i++ {
		cmd := fmt.Sprintf("dd if=%s bs=%d count=1 iflag=direct of=/dev/null 2>&1 | grep -oP '[\\d.]+\\s*(ms|s)' | tail -1",
			testFile, lc.BlockSize*1024)
		output, err := common.RunSSHCommand(host, cmd)
		if err == nil {
			latency := parseLatencyFromOutput(output)
			if latency > 0 {
				readLatencies = append(readLatencies, latency)
			}
		}
	}

	if len(readLatencies) > 0 {
		result.ReadLatencyAvg, result.ReadLatencyMax, result.ReadLatencyMin = calcLatencyStats(readLatencies)
		result.ReadIOPS = calcIOPS(result.ReadLatencyAvg, result.IOSize)
	}

	// 清空远程缓存
	dropRemoteCaches(host)

	// 测试写延迟
	var writeLatencies []float64
	tempFile := testFile + ".write"
	for i := 0; i < lc.Iterations; i++ {
		cmd := fmt.Sprintf("dd if=/dev/zero of=%s bs=%d count=1 oflag=direct conv=fsync 2>&1 | grep -oP '[\\d.]+\\s*(ms|s)' | tail -1",
			tempFile, lc.BlockSize*1024)
		output, err := common.RunSSHCommand(host, cmd)
		common.RunSSHCommand(host, fmt.Sprintf("rm -f %s", tempFile))
		if err == nil {
			latency := parseLatencyFromOutput(output)
			if latency > 0 {
				writeLatencies = append(writeLatencies, latency)
			}
		}
	}

	if len(writeLatencies) > 0 {
		result.WriteLatencyAvg, result.WriteLatencyMax, result.WriteLatencyMin = calcLatencyStats(writeLatencies)
		result.WriteIOPS = calcIOPS(result.WriteLatencyAvg, result.IOSize)
	}

	return result, nil
}

// runRemoteFioTest 在远程主机使用 fio 测试
func (lc *LatencyChecker) runRemoteFioTest(host, dir string) (*LatencyResult, error) {
	testFile := path.Join(dir, fmt.Sprintf("fio_test_%d", time.Now().UnixNano()))

	// 读测试
	readCmd := fmt.Sprintf("fio --name=latency_test --filename=%s --bs=%d --size=%d --direct=1 --ioengine=libaio --iodepth=1 --rw=read --runtime=10 --time_based --output-format=json 2>/dev/null",
		testFile, lc.BlockSize*1024, lc.IOSize*1024*lc.Iterations)
	readOutput, err := common.RunSSHCommand(host, readCmd)
	if err != nil {
		return nil, err
	}
	defer common.RunSSHCommand(host, fmt.Sprintf("rm -f %s", testFile))

	readStats := parseFioOutput(readOutput)

	// 写测试
	writeCmd := fmt.Sprintf("fio --name=latency_test --filename=%s --bs=%d --size=%d --direct=1 --ioengine=libaio --iodepth=1 --rw=write --runtime=10 --time_based --output-format=json 2>/dev/null",
		testFile, lc.BlockSize*1024, lc.IOSize*1024*lc.Iterations)
	writeOutput, err := common.RunSSHCommand(host, writeCmd)
	if err != nil {
		return nil, err
	}

	writeStats := parseFioOutput(writeOutput)

	result := &LatencyResult{
		Host:            host,
		Dir:             dir,
		BlockSize:       int64(lc.BlockSize) * 1024,
		IOSize:          int64(lc.IOSize) * 1024,
		ReadLatencyAvg:  readStats["latency_avg"],
		ReadLatencyMax:  readStats["latency_max"],
		ReadLatencyMin:  readStats["latency_min"],
		ReadIOPS:        readStats["iops"],
		WriteLatencyAvg: writeStats["latency_avg"],
		WriteLatencyMax: writeStats["latency_max"],
		WriteLatencyMin: writeStats["latency_min"],
		WriteIOPS:       writeStats["iops"],
	}

	return result, nil
}

// parseLatencyFromOutput 从 dd 输出解析延迟
func parseLatencyFromOutput(output string) float64 {
	// 解析 dd 输出中的时间信息
	re := regexp.MustCompile(`([\d.]+)\s*(ms|s)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 3 {
		value, _ := strconv.ParseFloat(matches[1], 64)
		unit := matches[2]
		if unit == "s" {
			value *= 1000 // 转换为 ms
		}
		return value
	}

	// 尝试解析复制的字节和时间
	re = regexp.MustCompile(`(\d+)\s*bytes.*in\s*([\d.]+)\s*(ms|s)`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 4 {
		timeVal, _ := strconv.ParseFloat(matches[2], 64)
		unit := matches[3]
		if unit == "s" {
			timeVal *= 1000
		}
		return timeVal
	}

	return 0
}

// dropRemoteCaches 清空远程主机缓存
func dropRemoteCaches(host string) {
	common.RunSSHCommand(host, "echo 3 > /proc/sys/vm/drop_caches")
}
