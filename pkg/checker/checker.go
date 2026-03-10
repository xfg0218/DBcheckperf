// Package checker 提供系统性能检查功能
// 包含磁盘 I/O、网络、内存带宽和系统信息检测
package checker

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dbcheckperf/pkg/utils"
)

// DiskResult 磁盘测试结果
type DiskResult struct {
	// Host 主机名
	Host string
	// Dir 测试目录
	Dir string
	// WriteTime 写入时间（秒）
	WriteTime float64
	// ReadTime 读取时间（秒）
	ReadTime float64
	// WriteBytes 写入字节数
	WriteBytes uint64
	// ReadBytes 读取字节数
	ReadBytes uint64
	// WriteBandwidth 写入带宽（MB/s）
	WriteBandwidth float64
	// ReadBandwidth 读取带宽（MB/s）
	ReadBandwidth float64
	// RandWriteTime 随机写入时间（秒）
	RandWriteTime float64
	// RandReadTime 随机读取时间（秒）
	RandReadTime float64
	// RandWriteBytes 随机写入字节数
	RandWriteBytes uint64
	// RandReadBytes 随机读取字节数
	RandReadBytes uint64
	// RandWriteBandwidth 随机写入带宽（MB/s）
	RandWriteBandwidth float64
	// RandReadBandwidth 随机读取带宽（MB/s）
	RandReadBandwidth float64
}

// DiskChecker 磁盘性能检查器
type DiskChecker struct {
	// BlockSize 块大小（KB）
	BlockSize int
	// FileSize 文件大小
	FileSize uint64
	// Iterations 测试迭代次数
	Iterations int
	// Verbose 是否显示详细输出
	Verbose bool
	// TestRandom 是否测试随机读写
	TestRandom bool
	// RandBlockSize 随机读写块大小（KB）
	RandBlockSize int
}

// NewDiskChecker 创建新的磁盘检查器
func NewDiskChecker(blockSizeKB int, fileSize uint64, verbose bool) *DiskChecker {
	return &DiskChecker{
		BlockSize:     blockSizeKB,
		FileSize:      fileSize,
		Iterations:    1, // 仅测试一次
		Verbose:       verbose,
		TestRandom:    false,
		RandBlockSize: 4, // 随机读写默认 4KB
	}
}

// NewDiskCheckerWithRandom 创建支持随机读写的磁盘检查器
func NewDiskCheckerWithRandom(blockSizeKB int, fileSize uint64, verbose bool, testRandom bool, randBlockSizeKB int) *DiskChecker {
	return &DiskChecker{
		BlockSize:     blockSizeKB,
		FileSize:      fileSize,
		Iterations:    1,
		Verbose:       verbose,
		TestRandom:    testRandom,
		RandBlockSize: randBlockSizeKB,
	}
}

// Run 执行磁盘 I/O 测试
// 使用 dd 命令测试顺序读写性能，单次测试快速完成
func (dc *DiskChecker) Run(dir string) (*DiskResult, error) {
	// 计算测试文件大小
	testSize := dc.FileSize
	if testSize == 0 {
		// 默认使用 2 倍 RAM
		ram, err := utils.GetTotalRAM()
		if err != nil {
			ram = 2 * 1024 * 1024 * 1024 // 默认 2GB
		}
		testSize = ram
	}

	// 确保测试目录存在
	if err := utils.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("无法创建测试目录：%v", err)
	}

	result := &DiskResult{
		Host: getHostname(),
		Dir:  dir,
	}

	// 计算块大小和计数
	blockSize := dc.BlockSize * 1024 // 转换为字节
	count := int(testSize / uint64(blockSize))
	if count == 0 {
		count = 1
	}

	// 单次测试
	var writeTime float64
	var readTime float64
	var writeBytes, readBytes uint64
	var err error

	// 生成测试文件名
	testFile := fmt.Sprintf("%s/dd_test_%d", dir, time.Now().UnixNano())

	if dc.Verbose {
		fmt.Printf("磁盘测试：%s\n", testFile)
	}

	// 执行写入测试
	writeBytes, writeTime, err = dc.runWriteTest(testFile, blockSize, count)
	if err == nil {
		// 清空缓存，确保读取测试准确
		dc.dropCaches()

		// 执行读取测试
		readBytes, readTime, err = dc.runReadTest(testFile, blockSize)
	}

	// 设置顺序读写结果
	if err == nil || writeTime > 0 {
		result.WriteTime = writeTime
		result.WriteBytes = writeBytes
		result.WriteBandwidth = float64(writeBytes) / writeTime / (1024 * 1024)
	}

	if readTime > 0 {
		result.ReadTime = readTime
		result.ReadBytes = readBytes
		result.ReadBandwidth = float64(readBytes) / readTime / (1024 * 1024)
	}

	// 执行随机读写测试（如果启用）
	if dc.TestRandom {
		result.RandWriteBytes, result.RandWriteTime, result.RandReadBytes, result.RandReadTime, err = dc.runRandomTest(testFile)
		if err == nil && dc.Verbose {
			fmt.Printf("随机读写测试完成\n")
		}

		// 计算随机读写带宽
		if result.RandWriteTime > 0 {
			result.RandWriteBandwidth = float64(result.RandWriteBytes) / result.RandWriteTime / (1024 * 1024)
		}
		if result.RandReadTime > 0 {
			result.RandReadBandwidth = float64(result.RandReadBytes) / result.RandReadTime / (1024 * 1024)
		}
	}

	// 清理测试文件
	os.Remove(testFile)

	return result, nil
}

// runWriteTest 执行单次写入测试
func (dc *DiskChecker) runWriteTest(testFile string, blockSize int, count int) (uint64, float64, error) {
	writeStart := time.Now()

	// 使用更严谨的参数：fsync 确保数据真正写入磁盘
	writeCmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", testFile),
		fmt.Sprintf("bs=%d", blockSize), fmt.Sprintf("count=%d", count),
		"oflag=direct", "conv=fsync")
	var writeErr bytes.Buffer
	writeCmd.Stderr = &writeErr

	if err := writeCmd.Run(); err != nil {
		// 尝试降级参数（不使用 oflag=direct）
		writeCmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", testFile),
			fmt.Sprintf("bs=%d", blockSize), fmt.Sprintf("count=%d", count), "conv=fsync")
		writeCmd.Stderr = &writeErr
		if err := writeCmd.Run(); err != nil {
			// 再次尝试最基本的参数
			writeCmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", testFile),
				fmt.Sprintf("bs=%d", blockSize), fmt.Sprintf("count=%d", count))
			writeCmd.Stderr = &writeErr
			if err := writeCmd.Run(); err != nil {
				return 0, 0, fmt.Errorf("写入测试失败：%v, %s", err, writeErr.String())
			}
		}
	}

	writeDuration := time.Since(writeStart).Seconds()
	writeBytes := dc.parseDDOutput(writeErr.String())

	// 验证写入的数据量
	expectedBytes := uint64(blockSize * count)
	if writeBytes == 0 {
		writeBytes = expectedBytes // 如果无法解析，使用预期值
	}

	// 验证写入完整性（允许 1% 误差）
	if float64(writeBytes) < float64(expectedBytes)*0.99 {
		return 0, 0, fmt.Errorf("写入数据不完整：预期 %d 字节，实际 %d 字节", expectedBytes, writeBytes)
	}

	return writeBytes, writeDuration, nil
}

// runReadTest 执行单次读取测试
func (dc *DiskChecker) runReadTest(testFile string, blockSize int) (uint64, float64, error) {
	// 先确保文件存在
	if _, err := os.Stat(testFile); err != nil {
		return 0, 0, fmt.Errorf("测试文件不存在：%s", testFile)
	}

	readStart := time.Now()

	// 使用 iflag=direct 绕过缓存，测试真实磁盘读取性能
	readCmd := exec.Command("dd", fmt.Sprintf("if=%s", testFile), "of=/dev/null",
		fmt.Sprintf("bs=%d", blockSize), "iflag=direct")
	var readErr bytes.Buffer
	readCmd.Stderr = &readErr

	if err := readCmd.Run(); err != nil {
		// 尝试降级参数
		readCmd = exec.Command("dd", fmt.Sprintf("if=%s", testFile), "of=/dev/null",
			fmt.Sprintf("bs=%d", blockSize))
		readCmd.Stderr = &readErr
		if err := readCmd.Run(); err != nil {
			return 0, 0, fmt.Errorf("读取测试失败：%v, %s", err, readErr.String())
		}
	}

	readDuration := time.Since(readStart).Seconds()
	readBytes := dc.parseDDOutput(readErr.String())

	if readBytes == 0 {
		// 如果无法解析，尝试从文件大小获取
		if info, statErr := os.Stat(testFile); statErr == nil {
			readBytes = uint64(info.Size())
		}
	}

	return readBytes, readDuration, nil
}

// dropCaches 清空系统缓存，确保读取测试准确
func (dc *DiskChecker) dropCaches() {
	// 尝试清空页面缓存、dentries 和 inodes
	// 需要 root 权限，失败时忽略
	cmd := exec.Command("sh", "-c", "sync; echo 3 > /proc/sys/vm/drop_caches 2>/dev/null || true")
	cmd.Run()

	// 额外调用 sync 确保数据同步
	exec.Command("sync").Run()

	// 短暂等待让缓存清空生效
	time.Sleep(100 * time.Millisecond)
}

// parseDDOutput 解析 dd 命令输出，提取字节数
func (dc *DiskChecker) parseDDOutput(output string) uint64 {
	// 匹配 "X bytes copied" 或 "X 字节已复制"
	re := regexp.MustCompile(`(\d+)\s*bytes`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		bytes, err := strconv.ParseUint(matches[1], 10, 64)
		if err == nil {
			return bytes
		}
	}

	// 尝试匹配中文输出
	re = regexp.MustCompile(`(\d+)\s*字节`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		bytes, err := strconv.ParseUint(matches[1], 10, 64)
		if err == nil {
			return bytes
		}
	}

	return 0
}

// runRandomTest 执行随机读写测试
// 使用 dd 命令配合 seek 参数进行随机读写测试
func (dc *DiskChecker) runRandomTest(testFile string) (uint64, float64, uint64, float64, error) {
	// 获取文件大小
	fileInfo, err := os.Stat(testFile)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("无法获取测试文件信息：%v", err)
	}
	fileSize := uint64(fileInfo.Size())

	// 随机读写块大小
	randBlockSize := dc.RandBlockSize * 1024 // 转换为字节
	randCount := 1000                        // 随机操作次数

	// 随机写入测试
	randWriteStart := time.Now()
	randWriteBytes, err := dc.runRandomWrite(testFile, randBlockSize, randCount, fileSize)
	randWriteDuration := time.Since(randWriteStart).Seconds()

	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("随机写入测试失败：%v", err)
	}

	// 清空缓存
	dc.dropCaches()

	// 随机读取测试
	randReadStart := time.Now()
	randReadBytes, err := dc.runRandomRead(testFile, randBlockSize, randCount, fileSize)
	randReadDuration := time.Since(randReadStart).Seconds()

	if err != nil {
		return randWriteBytes, randWriteDuration, 0, 0, fmt.Errorf("随机读取测试失败：%v", err)
	}

	return randWriteBytes, randWriteDuration, randReadBytes, randReadDuration, nil
}

// runRandomWrite 执行随机写入测试
func (dc *DiskChecker) runRandomWrite(testFile string, blockSize, count int, fileSize uint64) (uint64, error) {
	totalBytes := 0

	for i := 0; i < count; i++ {
		// 计算随机偏移量
		maxOffset := int64(fileSize) - int64(blockSize)
		if maxOffset <= 0 {
			maxOffset = 0
		}
		offset := int64(0)
		if maxOffset > 0 {
			offset = time.Now().UnixNano() % maxOffset
		}

		// 使用 dd 进行随机位置写入
		cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", testFile),
			fmt.Sprintf("bs=%d", blockSize), "count=1",
			fmt.Sprintf("seek=%d", offset/int64(blockSize)),
			"oflag=direct", "conv=notrunc,fsync")
		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf

		if err := cmd.Run(); err != nil {
			// 尝试降级参数
			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", testFile),
				fmt.Sprintf("bs=%d", blockSize), "count=1",
				fmt.Sprintf("seek=%d", offset/int64(blockSize)),
				"conv=notrunc,fsync")
			cmd.Stderr = &errBuf
			if err := cmd.Run(); err != nil {
				// 继续尝试，不中断测试
				continue
			}
		}
		totalBytes += blockSize
	}

	return uint64(totalBytes), nil
}

// runRandomRead 执行随机读取测试
func (dc *DiskChecker) runRandomRead(testFile string, blockSize, count int, fileSize uint64) (uint64, error) {
	totalBytes := 0

	for i := 0; i < count; i++ {
		// 计算随机偏移量
		maxOffset := int64(fileSize) - int64(blockSize)
		if maxOffset <= 0 {
			maxOffset = 0
		}
		offset := int64(0)
		if maxOffset > 0 {
			offset = time.Now().UnixNano() % maxOffset
		}

		// 使用 dd 进行随机位置读取
		cmd := exec.Command("dd", fmt.Sprintf("if=%s", testFile), "of=/dev/null",
			fmt.Sprintf("bs=%d", blockSize), "count=1",
			fmt.Sprintf("skip=%d", offset/int64(blockSize)),
			"iflag=direct")
		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf

		if err := cmd.Run(); err != nil {
			// 尝试降级参数
			cmd = exec.Command("dd", fmt.Sprintf("if=%s", testFile), "of=/dev/null",
				fmt.Sprintf("bs=%d", blockSize), "count=1",
				fmt.Sprintf("skip=%d", offset/int64(blockSize)))
			cmd.Stderr = &errBuf
			if err := cmd.Run(); err != nil {
				// 继续尝试，不中断测试
				continue
			}
		}
		totalBytes += blockSize
	}

	return uint64(totalBytes), nil
}

// getHostname 获取本地主机名
func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return name
}

// NetworkResult 网络测试结果
type NetworkResult struct {
	// SourceHost 源主机
	SourceHost string
	// DestHost 目标主机
	DestHost string
	// Bandwidth 带宽（MB/s）
	Bandwidth float64
	// Duration 测试持续时间（秒）
	Duration float64
}

// NetworkChecker 网络性能检查器
type NetworkChecker struct {
	// Duration 测试持续时间
	Duration time.Duration
	// BufferSize 缓冲区大小（KB）
	BufferSize int
	// UseNetperf 是否使用 netperf
	UseNetperf bool
	// Verbose 是否显示详细输出
	Verbose bool
	// Iterations 测试迭代次数
	Iterations int
}

// NewNetworkChecker 创建新的网络检查器
func NewNetworkChecker(duration time.Duration, bufferSizeKB int, useNetperf, verbose bool) *NetworkChecker {
	return &NetworkChecker{
		Duration:   duration,
		BufferSize: bufferSizeKB,
		UseNetperf: useNetperf,
		Verbose:    verbose,
		Iterations: 3, // 默认测试 3 次取平均值
	}
}

// RunParallel 执行并行网络测试
// 从当前主机同时向所有目标主机发送数据
func (nc *NetworkChecker) RunParallel(localHost string, remoteHosts []string) ([]NetworkResult, error) {
	var results []NetworkResult

	// 为每个远程主机启动测试
	done := make(chan NetworkResult, len(remoteHosts))
	errChan := make(chan error, len(remoteHosts))

	for _, remote := range remoteHosts {
		go func(dest string) {
			result, err := nc.testSingle(localHost, dest)
			if err != nil {
				errChan <- err
			} else {
				done <- result
			}
		}(remote)
	}

	// 收集结果
	for i := 0; i < len(remoteHosts); i++ {
		select {
		case result := <-done:
			results = append(results, result)
		case err := <-errChan:
			if nc.Verbose {
				fmt.Fprintf(os.Stderr, "网络测试警告：%v\n", err)
			}
		}
	}

	return results, nil
}

// testSingle 测试单个主机对的网络性能
// 多次测试取平均值以提高准确性
func (nc *NetworkChecker) testSingle(localHost, remoteHost string) (NetworkResult, error) {
	result := NetworkResult{
		SourceHost: localHost,
		DestHost:   remoteHost,
		Duration:   nc.Duration.Seconds(),
	}

	var bandwidths []float64

	for i := 0; i < nc.Iterations; i++ {
		if nc.Verbose {
			fmt.Printf("网络测试迭代 %d/%d: %s -> %s\n", i+1, nc.Iterations, localHost, remoteHost)
		}

		var bw float64
		var err error

		if nc.UseNetperf {
			bw, err = nc.testWithNetperf(remoteHost)
		} else {
			bw, err = nc.testWithTCP(remoteHost)
		}

		if err != nil {
			if nc.Verbose {
				fmt.Fprintf(os.Stderr, "网络测试警告：%v\n", err)
			}
			continue
		}

		bandwidths = append(bandwidths, bw)
	}

	if len(bandwidths) > 0 {
		result.Bandwidth = utils.Average(bandwidths)
	} else {
		result.Bandwidth = 0
	}

	return result, nil
}

// testWithNetperf 使用 netperf 进行测试
func (nc *NetworkChecker) testWithNetperf(remoteHost string) (float64, error) {
	cmd := exec.Command("netperf", "-H", remoteHost, "-t", "TCP_STREAM",
		"-l", fmt.Sprintf("%.0f", nc.Duration.Seconds()))
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return nc.parseNetperfOutput(string(output)), nil
}

// testWithTCP 使用 TCP 传输进行测试（改进版）
func (nc *NetworkChecker) testWithTCP(remoteHost string) (float64, error) {
	// 尝试多种方法进行网络测试
	methods := []func(string) (float64, error){
		nc.testWithIperf,
		nc.testWithCurl,
		nc.testWithTCPStream,
	}

	for _, method := range methods {
		bw, err := method(remoteHost)
		if err == nil && bw > 0 {
			return bw, nil
		}
	}

	// 所有方法都失败，使用 ping 估算
	return nc.estimateNetworkSpeed(remoteHost), nil
}

// testWithIperf 使用 iperf3 进行测试（优先选择）
func (nc *NetworkChecker) testWithIperf(remoteHost string) (float64, error) {
	// 检查 iperf3 是否可用
	if _, err := exec.LookPath("iperf3"); err != nil {
		return 0, fmt.Errorf("iperf3 不可用")
	}

	// 运行 iperf3 客户端测试
	cmd := exec.Command("iperf3", "-c", remoteHost, "-t", fmt.Sprintf("%.0f", nc.Duration.Seconds()), "-J")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// 解析 JSON 输出（简化解析）
	outputStr := string(output)
	// 查找 bits_per_second 字段
	re := regexp.MustCompile(`"bits_per_second":\s*(\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(outputStr)
	if len(matches) >= 2 {
		bps, _ := strconv.ParseFloat(matches[1], 64)
		return bps / 8 / (1024 * 1024), nil // 转换为 MB/s
	}

	return 0, fmt.Errorf("无法解析 iperf3 输出")
}

// testWithCurl 使用 curl 进行 HTTP 下载测试
func (nc *NetworkChecker) testWithCurl(remoteHost string) (float64, error) {
	if _, err := exec.LookPath("curl"); err != nil {
		return 0, fmt.Errorf("curl 不可用")
	}

	// 尝试从远程主机下载测试文件
	testURL := fmt.Sprintf("http://%s/test", remoteHost)
	start := time.Now()

	cmd := exec.Command("curl", "-o", "/dev/null", "-w", "%{speed_download}", "--connect-timeout", "5", "--max-time", fmt.Sprintf("%.0f", nc.Duration.Seconds()), testURL)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	duration := time.Since(start).Seconds()
	if duration > 0 {
		// curl 输出的是字节/秒
		speed, _ := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
		return speed / (1024 * 1024), nil // 转换为 MB/s
	}

	return 0, fmt.Errorf("curl 测试时间为零")
}

// testWithTCPStream 使用 TCP 流进行测试（备用方法）
func (nc *NetworkChecker) testWithTCPStream(remoteHost string) (float64, error) {
	// 创建一个 TCP 连接到指定端口（默认使用 9999）
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:9999", remoteHost), 5*time.Second)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// 设置测试参数
	testDuration := nc.Duration
	if testDuration <= 0 {
		testDuration = 5 * time.Second
	}

	// 创建测试数据缓冲区
	bufferSize := nc.BufferSize * 1024
	if bufferSize <= 0 {
		bufferSize = 64 * 1024 // 默认 64KB
	}
	buffer := make([]byte, bufferSize)

	// 填充测试数据
	for i := range buffer {
		buffer[i] = byte(i % 256)
	}

	// 开始传输测试
	start := time.Now()
	var totalBytes int64
	deadline := start.Add(testDuration)

	for time.Now().Before(deadline) {
		n, err := conn.Write(buffer)
		if err != nil {
			break
		}
		totalBytes += int64(n)
	}

	duration := time.Since(start).Seconds()
	if duration > 0 && totalBytes > 0 {
		return float64(totalBytes) / duration / (1024 * 1024), nil
	}

	return 0, fmt.Errorf("TCP 流测试失败")
}

// parseNetperfOutput 解析 netperf 输出
func (nc *NetworkChecker) parseNetperfOutput(output string) float64 {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			// netperf 输出格式：LOCAL_HOST  REMOTE_HOST  PROTOCOL  BANDWIDTH  ...
			bandwidth, err := strconv.ParseFloat(fields[4], 64)
			if err == nil {
				return bandwidth / 8 // 转换为 MB/s
			}
		}
	}
	return 0
}

// estimateNetworkSpeed 使用 ping 估算网络速度（备用方法）
func (nc *NetworkChecker) estimateNetworkSpeed(remoteHost string) float64 {
	// 使用 ping 来估算网络质量
	cmd := exec.Command("ping", "-c", "1", "-W", "2", remoteHost)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}

	// 解析 ping 时间
	re := regexp.MustCompile(`time[=<](\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		pingTime, _ := strconv.ParseFloat(matches[1], 64)
		// 基于 ping 时间估算带宽（粗略估算）
		if pingTime < 1 {
			return 1000 // <1ms，可能是本地网络
		} else if pingTime < 10 {
			return 500 // <10ms，局域网
		} else if pingTime < 100 {
			return 100 // <100ms，城域网
		} else {
			return 50 // >100ms，广域网
		}
	}

	return 0
}

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
		Host: getHostname(),
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
		Host: getHostname(),
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			switch strings.ToLower(fields[0]) {
			case "copy":
				result.CopyBandwidth, _ = strconv.ParseFloat(fields[1], 64)
			case "scale":
				result.ScaleBandwidth, _ = strconv.ParseFloat(fields[1], 64)
			case "add":
				result.AddBandwidth, _ = strconv.ParseFloat(fields[1], 64)
			case "triad":
				result.TriadBandwidth, _ = strconv.ParseFloat(fields[1], 64)
			}
		}
	}

	result.TotalBandwidth = result.CopyBandwidth + result.ScaleBandwidth +
		result.AddBandwidth + result.TriadBandwidth

	return result, nil
}

// SystemInfo 系统信息
type SystemInfo struct {
	// Host 主机名
	Host string
	// CPUCore CPU 核心数
	CPUCore int
	// CPUModel CPU 型号
	CPUModel string
	// TotalRAM 总内存（字节）
	TotalRAM uint64
	// DiskSize 磁盘大小（字节）
	DiskSize uint64
	// NICSpeed 网卡速率（Mbps）
	NICSpeed int
	// IsVirtual 是否为虚拟机
	IsVirtual bool
	// VirtualType 虚拟化类型
	VirtualType string
	// OSName 操作系统名称
	OSName string
	// KernelVersion 内核版本
	KernelVersion string
	// RAIDInfo RAID 信息
	RAIDInfo *RAIDInfo
	// NICBondInfo 网卡绑定信息
	NICBondInfo *NICBondInfo
}

// RAIDInfo RAID 信息
type RAIDInfo struct {
	// HasRAID 是否有 RAID 卡
	HasRAID bool
	// RAIDModel RAID 卡型号
	RAIDModel string
	// CacheSize RAID 卡缓存大小（字节）
	CacheSize uint64
	// StripeSize 条带大小（KB）
	StripeSize int
	// RAIDLevel RAID 级别
	RAIDLevel string
}

// NICBondInfo 网卡绑定信息
type NICBondInfo struct {
	// HasBond 是否做了绑定
	HasBond bool
	// BondMode 绑定模式
	BondMode string
	// BondSlaves 绑定从网卡数量
	BondSlaves int
	// QueueSize 队列大小
	QueueSize int
}

// SystemChecker 系统信息检查器
type SystemChecker struct{}

// NewSystemChecker 创建新的系统信息检查器
func NewSystemChecker() *SystemChecker {
	return &SystemChecker{}
}

// Run 收集系统信息
func (sc *SystemChecker) Run() (*SystemInfo, error) {
	info := &SystemInfo{
		Host: getHostname(),
	}

	// 获取 CPU 核心数
	info.CPUCore = sc.getCPUCore()

	// 获取 CPU 型号
	info.CPUModel = sc.getCPUModel()

	// 获取总内存
	ram, err := utils.GetTotalRAM()
	if err != nil {
		ram = 0
	}
	info.TotalRAM = ram

	// 获取磁盘大小
	info.DiskSize = sc.getDiskSize()

	// 获取网卡速率
	info.NICSpeed = sc.getNICSpeed()

	// 检测是否为虚拟机
	info.IsVirtual, info.VirtualType = sc.detectVirtualization()

	// 获取操作系统信息
	info.OSName = sc.getOSName()

	// 获取内核版本
	info.KernelVersion = sc.getKernelVersion()

	// 获取 RAID 信息
	info.RAIDInfo = sc.getRAIDInfo()

	// 获取网卡绑定信息
	info.NICBondInfo = sc.getNICBondInfo()

	return info, nil
}

// getCPUCore 获取 CPU 核心数（兼容 ARM 和 AMD）
func (sc *SystemChecker) getCPUCore() int {
	// 尝试从 /proc/cpuinfo 读取
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		count := 0
		for _, line := range strings.Split(string(data), "\n") {
			// x86/x86_64 (Intel/AMD)
			if strings.HasPrefix(line, "processor") {
				count++
			}
			// ARM/ARM64
			if strings.HasPrefix(line, "Processor") || strings.HasPrefix(line, "processor") {
				if strings.Contains(line, ":") {
					count++
				}
			}
		}
		if count > 0 {
			return count
		}
	}

	// 尝试从 /sys/devices/system/cpu 读取（ARM 和 AMD 通用）
	cpuDir := "/sys/devices/system/cpu"
	entries, err := os.ReadDir(cpuDir)
	if err == nil {
		count := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "cpu") {
				// 检查是否为在线 CPU
				onlinePath := fmt.Sprintf("%s/%s/online", cpuDir, entry.Name())
				if data, err := os.ReadFile(onlinePath); err == nil {
					if strings.TrimSpace(string(data)) == "1" {
						count++
					}
				} else {
					// 如果没有 online 文件，默认在线
					count++
				}
			}
		}
		if count > 0 {
			return count
		}
	}

	// 回退方法
	return 1
}

// getCPUModel 获取 CPU 型号（兼容 ARM 和 AMD）
func (sc *SystemChecker) getCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			// x86/x86_64 (Intel/AMD)
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
			// ARM/ARM64 - 不同内核版本可能格式不同
			if strings.HasPrefix(line, "model name") ||
				strings.HasPrefix(line, "CPU part") ||
				strings.HasPrefix(line, "Features") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					model := strings.TrimSpace(parts[1])
					if model != "" && model != "Unknown" {
						// ARM CPU 可能没有 model name，使用 Features 或其他字段
						return sc.parseARMModel(model)
					}
				}
			}
		}
	}

	// 尝试使用 lscpu 命令（更可靠）
	cmd := exec.Command("lscpu")
	output, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "Model name:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// ARM 特定：尝试从 /sys/devices/system/cpu/cpu0/cpuidle 读取
	return "Unknown"
}

// parseARMModel 解析 ARM CPU 型号
func (sc *SystemChecker) parseARMModel(model string) string {
	// ARM 实现 ID 映射
	armImplementations := map[string]string{
		"0xd03": "Cortex-A53",
		"0xd07": "Cortex-A57",
		"0xd08": "Cortex-A72",
		"0xd09": "Cortex-A73",
		"0xd0a": "Cortex-A75",
		"0xd0b": "Cortex-A76",
		"0xd0c": "Neoverse-N1",
		"0xd0d": "Cortex-A77",
		"0xd40": "Neoverse-V1",
		"0xd41": "Cortex-A78",
		"0xd42": "Cortex-A78AE",
		"0xd43": "Cortex-A65AE",
		"0xd44": "Cortex-X1",
		"0xd46": "Cortex-A510",
		"0xd47": "Cortex-A710",
		"0xd48": "Cortex-X2",
		"0xd49": "Cortex-A715",
		"0xd4e": "Cortex-X3",
		"0xd4f": "Cortex-A520",
		"0xd80": "Cortex-A520",
		"0xd81": "Cortex-A720",
		"0xd82": "Cortex-X4",
		"0xd84": "Cortex-X925",
		"0xd8e": "Cortex-A725",
	}

	// 检查是否包含实现 ID
	for id, name := range armImplementations {
		if strings.Contains(strings.ToLower(model), id) {
			return "ARM " + name
		}
	}

	// 检查是否包含厂商信息
	if strings.Contains(strings.ToLower(model), "qualcomm") ||
		strings.Contains(strings.ToLower(model), "kryo") {
		return "Qualcomm Snapdragon"
	}
	if strings.Contains(strings.ToLower(model), "apple") {
		return "Apple Silicon"
	}
	if strings.Contains(strings.ToLower(model), "nvidia") ||
		strings.Contains(strings.ToLower(model), "carmel") {
		return "NVIDIA"
	}
	if strings.Contains(strings.ToLower(model), "ampere") {
		return "Ampere Altra"
	}
	if strings.Contains(strings.ToLower(model), "fujitsu") {
		return "Fujitsu A64FX"
	}

	return model
}

// detectVirtualization 检测是否为虚拟机及虚拟化类型
func (sc *SystemChecker) detectVirtualization() (bool, string) {
	// 方法 1: 检查 /sys/class/dmi/product/name
	data, err := os.ReadFile("/sys/class/dmi/id/product_name")
	if err == nil {
		content := strings.ToLower(strings.TrimSpace(string(data)))
		if strings.Contains(content, "kvm") || strings.Contains(content, "virtual") {
			return true, "KVM"
		}
		if strings.Contains(content, "vmware") {
			return true, "VMware"
		}
		if strings.Contains(content, "xen") {
			return true, "Xen"
		}
		if strings.Contains(content, "hyperv") {
			return true, "Hyper-V"
		}
	}

	// 方法 2: 使用 systemd-detect-virt
	cmd := exec.Command("systemd-detect-virt")
	output, err := cmd.Output()
	if err == nil {
		virtType := strings.TrimSpace(string(output))
		if virtType != "none" {
			return true, virtType
		}
	}

	// 方法 3: 检查 /proc/cpuinfo 中的虚拟化标志
	data, err = os.ReadFile("/proc/cpuinfo")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "flags") {
				if strings.Contains(line, "vmx") || strings.Contains(line, "svm") {
					// 有虚拟化标志但不一定是虚拟机
					continue
				}
			}
		}
	}

	return false, "Bare Metal"
}

// getOSName 获取操作系统名称
func (sc *SystemChecker) getOSName() string {
	// 尝试读取 /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				name := strings.TrimPrefix(line, "PRETTY_NAME=")
				return strings.Trim(name, "\"")
			}
		}
	}

	// 尝试读取 /etc/redhat-release
	data, err = os.ReadFile("/etc/redhat-release")
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// 使用 uname 命令
	cmd := exec.Command("uname", "-o")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return "Linux"
}

// getKernelVersion 获取内核版本
func (sc *SystemChecker) getKernelVersion() string {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	// 尝试读取 /proc/version
	data, err := os.ReadFile("/proc/version")
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			return fields[2]
		}
	}

	return "Unknown"
}

// getRAIDInfo 获取 RAID 信息
func (sc *SystemChecker) getRAIDInfo() *RAIDInfo {
	info := &RAIDInfo{}

	// 方法 1: 使用 lspci 检测 RAID 卡
	cmd := exec.Command("lspci")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "raid") || strings.Contains(lineLower, "sas") || strings.Contains(lineLower, "storage controller") {
				info.HasRAID = true
				// 提取 RAID 卡型号
				if idx := strings.Index(line, ":"); idx != -1 {
					info.RAIDModel = strings.TrimSpace(line[idx+1:])
				}
				break
			}
		}
	}

	// 方法 2: 检查 /proc/scsi/scsi
	if !info.HasRAID {
		data, err := os.ReadFile("/proc/scsi/scsi")
		if err == nil {
			if strings.Contains(strings.ToLower(string(data)), "raid") {
				info.HasRAID = true
			}
		}
	}

	// 方法 3: 使用 megacli (LSI/Broadcom RAID 卡)
	if info.HasRAID {
		cmd = exec.Command("megacli", "adpallinfo", "-aALL")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Product Name") {
					info.RAIDModel = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				}
				if strings.Contains(line, "Driver Version") {
					// 跳过
				}
				if strings.Contains(line, "Strip Size") {
					parts := strings.Fields(line)
					if len(parts) >= 4 {
						sizeStr := parts[3]
						size, _ := strconv.Atoi(sizeStr)
						info.StripeSize = size
					}
				}
			}
		}
	}

	// 方法 4: 使用 storcli (LSI/Broadcom RAID 卡)
	if info.StripeSize == 0 {
		cmd = exec.Command("storcli", "/c0", "show", "all")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Strip Size") {
					parts := strings.Fields(line)
					if len(parts) >= 4 {
						sizeStr := parts[3]
						size, _ := strconv.Atoi(strings.TrimSuffix(sizeStr, "KB"))
						info.StripeSize = size
					}
				}
			}
		}
	}

	// 方法 5: 从 /sys/block 读取队列信息（如果没有 RAID 卡）
	if !info.HasRAID {
		// 尝试检测软件 RAID (mdraid)
		data, err := os.ReadFile("/proc/mdstat")
		if err == nil && strings.Contains(string(data), "active") {
			info.HasRAID = true
			info.RAIDModel = "Linux Software RAID (mdraid)"
			info.StripeSize = 512 // 默认软件 RAID 条带大小
		}
	}

	// 如果没有检测到 RAID，尝试从块设备获取条带大小
	if info.StripeSize == 0 {
		// 尝试从 /sys/block/*/queue 读取
		entries, err := os.ReadDir("/sys/block")
		if err == nil {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "sd") || strings.HasPrefix(entry.Name(), "nvme") {
					logicalBlockPath := fmt.Sprintf("/sys/block/%s/queue/logical_block_size", entry.Name())
					data, err := os.ReadFile(logicalBlockPath)
					if err == nil {
						blockSize, _ := strconv.Atoi(strings.TrimSpace(string(data)))
						if blockSize > 0 {
							info.StripeSize = blockSize / 1024 // 转换为 KB
						}
					}
					break
				}
			}
		}
	}

	// 默认值
	if info.StripeSize == 0 {
		info.StripeSize = 64 // 默认 64KB
	}

	return info
}

// getNICBondInfo 获取网卡绑定信息
func (sc *SystemChecker) getNICBondInfo() *NICBondInfo {
	info := &NICBondInfo{}

	// 检查网络绑定接口
	bondPath := "/sys/class/net"
	entries, err := os.ReadDir(bondPath)
	if err != nil {
		return info
	}

	bondCount := 0
	maxSlaves := 0
	bondMode := ""

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "bond") {
			bondCount++
			info.HasBond = true

			// 读取绑定模式
			modePath := fmt.Sprintf("%s/%s/bonding/mode", bondPath, entry.Name())
			data, err := os.ReadFile(modePath)
			if err == nil {
				modeStr := strings.TrimSpace(string(data))
				if bondMode == "" {
					bondMode = modeStr
				}
			}

			// 读取从网卡数量
			slavesPath := fmt.Sprintf("%s/%s/bonding/slaves", bondPath, entry.Name())
			data, err = os.ReadFile(slavesPath)
			if err == nil {
				slaves := strings.Fields(string(data))
				if len(slaves) > maxSlaves {
					maxSlaves = len(slaves)
				}
			}
		}
	}

	if info.HasBond {
		info.BondMode = bondMode
		info.BondSlaves = maxSlaves
	}

	// 获取队列大小（从主网卡）
	// 优先查找非 bond 网卡
	primaryNIC := ""
	entries, err = os.ReadDir(bondPath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasPrefix(entry.Name(), "bond") &&
				!strings.HasPrefix(entry.Name(), "lo") &&
				!strings.HasPrefix(entry.Name(), "docker") {
				primaryNIC = entry.Name()
				break
			}
		}
	}

	// 如果没有找到，使用第一个非 bond 网卡
	if primaryNIC == "" {
		entries, err = os.ReadDir(bondPath)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && !strings.HasPrefix(entry.Name(), "bond") {
					primaryNIC = entry.Name()
					break
				}
			}
		}
	}

	// 读取队列长度
	if primaryNIC != "" {
		sizePath := fmt.Sprintf("%s/%s/tx_queue_len", bondPath, primaryNIC)
		data, err := os.ReadFile(sizePath)
		if err == nil {
			size, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if size > 0 {
				info.QueueSize = size
			}
		}

		// 如果 tx_queue_len 为 0，尝试从 ring 参数获取
		if info.QueueSize == 0 {
			cmd := exec.Command("ethtool", "-g", primaryNIC)
			output, err := cmd.Output()
			if err == nil {
				lines := strings.Split(string(output), "\n")
				for _, line := range lines {
					if strings.Contains(line, "TX:") {
						// 下一行可能是预分配值
						continue
					}
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if strings.Contains(line, "Pre-set maximum") || strings.Contains(line, "Current hardware") {
							continue
						}
					}
				}
				// 解析 TX 队列大小
				for i, line := range lines {
					if strings.TrimSpace(line) == "TX:" && i+1 < len(lines) {
						nextLine := lines[i+1]
						fields := strings.Fields(nextLine)
						if len(fields) >= 2 {
							size, _ := strconv.Atoi(fields[1])
							if size > 0 {
								info.QueueSize = size
							}
						}
					}
				}
			}
		}
	}

	// 默认队列大小
	if info.QueueSize == 0 {
		info.QueueSize = 1000 // 默认 1000
	}

	return info
}

// getDiskSize 获取磁盘总大小
func (sc *SystemChecker) getDiskSize() uint64 {
	// 使用 df 命令获取根分区大小
	cmd := exec.Command("df", "-B1", "/")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 2 {
			size, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				return size
			}
		}
	}

	return 0
}

// getNICSpeed 获取网卡速率
func (sc *SystemChecker) getNICSpeed() int {
	// 尝试从 /sys/class/net 读取
	netPath := "/sys/class/net"
	entries, err := os.ReadDir(netPath)
	if err != nil {
		return 1000 // 默认 1Gbps
	}

	maxSpeed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			speedPath := fmt.Sprintf("%s/%s/speed", netPath, entry.Name())
			data, err := os.ReadFile(speedPath)
			if err == nil {
				speed, err := strconv.Atoi(strings.TrimSpace(string(data)))
				if err == nil && speed > maxSpeed {
					maxSpeed = speed
				}
			}
		}
	}

	if maxSpeed > 0 {
		return maxSpeed
	}

	// 尝试使用 ethtool
	cmd := exec.Command("ethtool", "eth0")
	output, err := cmd.Output()
	if err == nil {
		re := regexp.MustCompile(`Speed:\s*(\d+)`)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) >= 2 {
			speed, _ := strconv.Atoi(matches[1])
			return speed
		}
	}

	return 1000 // 默认 1Gbps
}

// AggregateResults 聚合多个结果
type AggregateResults struct {
	// MinValue 最小值
	MinValue float64
	// MaxValue 最大值
	MaxValue float64
	// AvgValue 平均值
	AvgValue float64
	// MedianValue 中位数
	MedianValue float64
	// MinHost 最小值所在主机
	MinHost string
	// MaxHost 最大值所在主机
	MaxHost string
	// TotalValue 总和
	TotalValue float64
	// Count 数量
	Count int
}

// AggregateDiskResults 聚合磁盘测试结果（顺序读写）
func AggregateDiskResults(results []*DiskResult, write bool) *AggregateResults {
	if len(results) == 0 {
		return &AggregateResults{}
	}

	var values []float64
	var minHost, maxHost string
	var minVal, maxVal float64 = -1, -1

	for _, r := range results {
		var val float64
		if write {
			val = r.WriteBandwidth
		} else {
			val = r.ReadBandwidth
		}
		values = append(values, val)

		if minVal < 0 || val < minVal {
			minVal = val
			minHost = r.Host
		}
		if maxVal < 0 || val > maxVal {
			maxVal = val
			maxHost = r.Host
		}
	}

	agg := &AggregateResults{
		MinValue:    minVal,
		MaxValue:    maxVal,
		MinHost:     minHost,
		MaxHost:     maxHost,
		AvgValue:    utils.Average(values),
		MedianValue: utils.Median(values),
		TotalValue:  minVal + maxVal, // 简化处理
		Count:       len(results),
	}

	return agg
}

// AggregateDiskRandResults 聚合磁盘随机读写结果
func AggregateDiskRandResults(results []*DiskResult, write bool) *AggregateResults {
	if len(results) == 0 {
		return &AggregateResults{}
	}

	var values []float64
	var minHost, maxHost string
	var minVal, maxVal float64 = -1, -1

	for _, r := range results {
		var val float64
		if write {
			val = r.RandWriteBandwidth
		} else {
			val = r.RandReadBandwidth
		}
		// 跳过没有随机读写数据的结果
		if val == 0 {
			continue
		}
		values = append(values, val)

		if minVal < 0 || val < minVal {
			minVal = val
			minHost = r.Host
		}
		if maxVal < 0 || val > maxVal {
			maxVal = val
			maxHost = r.Host
		}
	}

	if len(values) == 0 {
		return &AggregateResults{}
	}

	agg := &AggregateResults{
		MinValue:    minVal,
		MaxValue:    maxVal,
		MinHost:     minHost,
		MaxHost:     maxHost,
		AvgValue:    utils.Average(values),
		MedianValue: utils.Median(values),
		TotalValue:  minVal + maxVal,
		Count:       len(values),
	}

	return agg
}

// AggregateNetworkResults 聚合网络测试结果
func AggregateNetworkResults(results []NetworkResult) *AggregateResults {
	if len(results) == 0 {
		return &AggregateResults{}
	}

	var values []float64
	for _, r := range results {
		values = append(values, r.Bandwidth)
	}

	return &AggregateResults{
		MinValue:    utils.MinSlice(values),
		MaxValue:    utils.MaxSlice(values),
		AvgValue:    utils.Average(values),
		MedianValue: utils.Median(values),
		TotalValue:  sumSlice(values),
		Count:       len(results),
	}
}

// sumSlice 计算切片总和
func sumSlice(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum
}
