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
	// Host 远程主机名（用于远程测试）
	Host string
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

// RunRemote 通过 SSH 在远程主机执行磁盘 I/O 测试
// 参数 host: 远程主机名或 IP 地址
// 参数 dir: 远程主机上的测试目录
// 返回：包含远程主机磁盘测试结果的 DiskResult 结构体
func (dc *DiskChecker) RunRemote(host string, dir string) (*DiskResult, error) {
	result := &DiskResult{
		Host: host,
		Dir:  dir,
	}

	// 计算测试文件大小
	testSize := dc.FileSize
	if testSize == 0 {
		// 默认使用 2 倍 RAM，远程主机需要通过 SSH 获取
		ramStr, err := dc.runSSHCommand(host, "free -b | grep Mem | awk '{print $2}'")
		if err != nil {
			ramStr = "2147483648" // 默认 2GB
		}
		ram, _ := strconv.ParseUint(strings.TrimSpace(ramStr), 10, 64)
		if ram == 0 {
			ram = 2 * 1024 * 1024 * 1024
		}
		testSize = ram
	}

	// 计算块大小和计数
	blockSize := dc.BlockSize * 1024 // 转换为字节
	count := int(testSize / uint64(blockSize))
	if count == 0 {
		count = 1
	}

	// 生成测试文件名
	testFile := fmt.Sprintf("%s/dd_test_%d", dir, time.Now().UnixNano())

	if dc.Verbose {
		fmt.Printf("远程磁盘测试：%s@%s\n", testFile, host)
	}

	// 执行写入测试
	writeCmd := fmt.Sprintf("dd if=/dev/zero of=%s bs=%d count=%d oflag=direct conv=fsync 2>&1",
		testFile, blockSize, count)
	writeOutput, writeTime, err := dc.runSSHCommandWithTime(host, writeCmd)
	var writeBytes uint64
	if err == nil {
		writeBytes = dc.parseDDOutput(writeOutput)
		if writeBytes == 0 {
			writeBytes = uint64(blockSize * count)
		}
	}

	// 清空远程缓存
	dc.dropRemoteCaches(host)

	// 执行读取测试
	readCmd := fmt.Sprintf("dd if=%s of=/dev/null bs=%d iflag=direct 2>&1", testFile, blockSize)
	readOutput, readTime, err := dc.runSSHCommandWithTime(host, readCmd)
	var readBytes uint64
	if err == nil {
		readBytes = dc.parseDDOutput(readOutput)
		if readBytes == 0 {
			readBytes = writeBytes
		}
	}

	// 设置顺序读写结果
	if writeTime > 0 {
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
		randWriteBytes, randWriteTime, randReadBytes, randReadTime, err := dc.runRemoteRandomTest(host, testFile)
		if err == nil && dc.Verbose {
			fmt.Printf("远程随机读写测试完成\n")
		}

		// 计算随机读写带宽
		if randWriteTime > 0 {
			result.RandWriteTime = randWriteTime
			result.RandWriteBytes = randWriteBytes
			result.RandWriteBandwidth = float64(randWriteBytes) / randWriteTime / (1024 * 1024)
		}
		if randReadTime > 0 {
			result.RandReadTime = randReadTime
			result.RandReadBytes = randReadBytes
			result.RandReadBandwidth = float64(randReadBytes) / randReadTime / (1024 * 1024)
		}
	}

	// 清理远程测试文件
	dc.runSSHCommand(host, fmt.Sprintf("rm -f %s", testFile))

	return result, nil
}

// runSSHCommand 通过 SSH 执行远程命令并返回输出
func (dc *DiskChecker) runSSHCommand(host string, command string) (string, error) {
	cmd := exec.Command("ssh",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		host, command)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("SSH 命令执行失败：%v", err)
	}
	return string(output), nil
}

// runSSHCommandWithTime 通过 SSH 执行远程命令并返回输出和执行时间
func (dc *DiskChecker) runSSHCommandWithTime(host string, command string) (string, float64, error) {
	start := time.Now()
	output, err := dc.runSSHCommand(host, command)
	duration := time.Since(start).Seconds()
	return output, duration, err
}

// dropRemoteCaches 清空远程主机系统缓存
func (dc *DiskChecker) dropRemoteCaches(host string) {
	// 尝试清空页面缓存、dentries 和 inodes
	dc.runSSHCommand(host, "sync; echo 3 > /proc/sys/vm/drop_caches 2>/dev/null || true")
	// 短暂等待让缓存清空生效
	time.Sleep(100 * time.Millisecond)
}

// runRemoteRandomTest 执行远程随机读写测试
func (dc *DiskChecker) runRemoteRandomTest(host string, testFile string) (uint64, float64, uint64, float64, error) {
	// 获取文件大小
	sizeOutput, err := dc.runSSHCommand(host, fmt.Sprintf("stat -c %%s %s 2>/dev/null || stat -f %%z %s 2>/dev/null", testFile, testFile))
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("无法获取远程测试文件大小：%v", err)
	}
	fileSize, _ := strconv.ParseUint(strings.TrimSpace(sizeOutput), 10, 64)

	// 随机读写块大小
	randBlockSize := dc.RandBlockSize * 1024 // 转换为字节
	randCount := 1000                        // 随机操作次数

	// 随机写入测试
	randWriteStart := time.Now()
	randWriteBytes, err := dc.runRemoteRandomWrite(host, testFile, randBlockSize, randCount, fileSize)
	randWriteDuration := time.Since(randWriteStart).Seconds()

	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("远程随机写入测试失败：%v", err)
	}

	// 清空远程缓存
	dc.dropRemoteCaches(host)

	// 随机读取测试
	randReadStart := time.Now()
	randReadBytes, err := dc.runRemoteRandomRead(host, testFile, randBlockSize, randCount, fileSize)
	randReadDuration := time.Since(randReadStart).Seconds()

	if err != nil {
		return randWriteBytes, randWriteDuration, 0, 0, fmt.Errorf("远程随机读取测试失败：%v", err)
	}

	return randWriteBytes, randWriteDuration, randReadBytes, randReadDuration, nil
}

// runRemoteRandomWrite 执行远程随机写入测试
func (dc *DiskChecker) runRemoteRandomWrite(host string, testFile string, blockSize, count int, fileSize uint64) (uint64, error) {
	totalBytes := uint64(0)

	for i := 0; i < count; i++ {
		// 计算随机偏移量（以块为单位）
		maxBlock := int64(fileSize) / int64(blockSize)
		offset := int64(0)
		if maxBlock > 1 {
			offset = time.Now().UnixNano() % maxBlock
		}

		// 使用 dd 进行随机位置写入
		cmd := fmt.Sprintf("dd if=/dev/zero of=%s bs=%d count=1 seek=%d oflag=direct conv=notrunc,fsync 2>/dev/null",
			testFile, blockSize, offset)
		_, err := dc.runSSHCommand(host, cmd)
		if err == nil {
			totalBytes += uint64(blockSize)
		}
	}

	return totalBytes, nil
}

// runRemoteRandomRead 执行远程随机读取测试
func (dc *DiskChecker) runRemoteRandomRead(host string, testFile string, blockSize, count int, fileSize uint64) (uint64, error) {
	totalBytes := uint64(0)

	for i := 0; i < count; i++ {
		// 计算随机偏移量（以块为单位）
		maxBlock := int64(fileSize) / int64(blockSize)
		offset := int64(0)
		if maxBlock > 1 {
			offset = time.Now().UnixNano() % maxBlock
		}

		// 使用 dd 进行随机位置读取
		cmd := fmt.Sprintf("dd if=%s of=/dev/null bs=%d count=1 skip=%d iflag=direct 2>/dev/null",
			testFile, blockSize, offset)
		_, err := dc.runSSHCommand(host, cmd)
		if err == nil {
			totalBytes += uint64(blockSize)
		}
	}

	return totalBytes, nil
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

// HardwareInfo 硬件详细信息
type HardwareInfo struct {
	// Host 主机名
	Host string
	// CPUInfo CPU 信息
	CPUInfo *CPUInfo
	// DiskInfos 磁盘信息列表
	DiskInfos []*DiskInfo
	// RAIDInfo RAID 信息
	RAIDInfo *RAIDConfigInfo
	// NICInfos 网卡信息列表
	NICInfos []*NICInfo
	// MemoryInfo 内存信息
	MemoryInfo *MemoryInfo
}

// RemoteHardwareInfo 远程主机硬件信息
type RemoteHardwareInfo struct {
	// Host 主机名
	Host string
	// HardwareInfo 硬件信息
	HardwareInfo *HardwareInfo
	// Error 错误信息（如果连接或收集失败）
	Error error
}

// CPUInfo CPU 详细信息
type CPUInfo struct {
	// Model CPU 型号
	Model string
	// Cores 核心数
	Cores int
	// TurboFreq 睿频 (MHz)，0 表示不支持
	TurboFreq int
	// BaseFreq 基准频率 (MHz)
	BaseFreq int
	// Sockets CPU 插槽数
	Sockets int
	// NUMANodes NUMA 节点数
	NUMANodes int
}

// MemoryInfo 内存详细信息
type MemoryInfo struct {
	// TotalMemory 总内存 (字节)
	TotalMemory uint64
	// MemoryType 内存类型 (DDR3/DDR4/DDR5)
	MemoryType string
	// MemorySpeed 内存速度 (MHz)
	MemorySpeed int
	// MemorySlots 内存插槽数
	MemorySlots int
	// NUMANodes NUMA 节点数
	NUMANodes int
}

// DiskInfo 磁盘详细信息
type DiskInfo struct {
	// Name 磁盘名称（如 sda, nvme0n1）
	Name string
	// Model 磁盘型号
	Model string
	// Vendor 磁盘厂家
	Vendor string
	// Size 总大小（字节）
	Size uint64
	// Type 磁盘类型（HDD/SSD/NVMe）
	Type string
	// Rotational 是否旋转磁盘（HDD=true）
	Rotational bool
}

// RAIDConfigInfo RAID 配置详细信息
type RAIDConfigInfo struct {
	// HasRAID 是否有 RAID
	HasRAID bool
	// RAIDModel RAID 卡型号
	RAIDModel string
	// CacheSize 缓存大小（字节）
	CacheSize uint64
	// StripeSize 条带大小（KB）
	StripeSize int
	// RAIDLevel RAID 级别
	RAIDLevel string
	// BatteryBackup 是否有电池备份
	BatteryBackup bool
}

// NICInfo 网卡详细信息
type NICInfo struct {
	// Name 网卡名称
	Name string
	// Speed 速率（Mbps）
	Speed int
	// IsBond 是否绑定
	IsBond bool
	// BondMode 绑定模式（仅当 IsBond=true 时有效）
	BondMode string
	// BondSlaves 绑定从网卡数量
	BondSlaves int
	// QueueSize 队列大小
	QueueSize int
	// MTU MTU 大小
	MTU int
	// Driver 网卡驱动
	Driver string
	// MACAddress MAC 地址
	MACAddress string
}

// SystemChecker 系统信息检查器
type SystemChecker struct{}

// NewSystemChecker 创建新的系统信息检查器
func NewSystemChecker() *SystemChecker {
	return &SystemChecker{}
}

// HardwareChecker 硬件信息检查器
type HardwareChecker struct {
	// Verbose 是否显示详细输出
	Verbose bool
	// OSInfo 操作系统信息
	OSInfo *utils.OSInfo
}

// NewHardwareChecker 创建新的硬件信息检查器
func NewHardwareChecker(verbose bool) *HardwareChecker {
	return &HardwareChecker{
		Verbose: verbose,
		OSInfo:  utils.GetOSInfo(),
	}
}

// Run 执行硬件信息收集
func (hc *HardwareChecker) Run() (*HardwareInfo, error) {
	info := &HardwareInfo{
		Host: getHostname(),
	}

	// 收集 CPU 信息
	info.CPUInfo = hc.collectCPUInfo()

	// 收集磁盘信息
	info.DiskInfos = hc.collectDiskInfo()

	// 收集 RAID 信息
	info.RAIDInfo = hc.collectRAIDInfo()

	// 收集网卡信息
	info.NICInfos = hc.collectNICInfo()

	// 收集内存信息
	info.MemoryInfo = hc.collectMemoryInfo()

	return info, nil
}

// RunRemote 通过 SSH 连接远程主机并收集硬件信息
// 参数 host: 远程主机名或 IP 地址
// 返回：包含远程主机硬件信息的 RemoteHardwareInfo 结构体
// 注意：此方法不依赖远程主机上的 dbcheckperf 命令，直接通过 SSH 执行底层硬件收集命令
func (hc *HardwareChecker) RunRemote(host string) *RemoteHardwareInfo {
	result := &RemoteHardwareInfo{
		Host: host,
	}

	// 测试 SSH 连接是否可用（使用标准 SSH 选项）
	cmd := exec.Command("ssh",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		host, "echo", "test")
	if err := cmd.Run(); err != nil {
		result.Error = fmt.Errorf("SSH 连接失败：%v", err)
		return result
	}

	// 直接通过 SSH 执行各个硬件收集命令，不依赖远程 dbcheckperf
	info := &HardwareInfo{
		Host: host,
	}

	// 收集 CPU 信息
	info.CPUInfo = hc.collectRemoteCPUInfo(host)

	// 收集磁盘信息
	info.DiskInfos = hc.collectRemoteDiskInfo(host)

	// 收集 RAID 信息
	info.RAIDInfo = hc.collectRemoteRAIDInfo(host)

	// 收集网卡信息
	info.NICInfos = hc.collectRemoteNICInfo(host)

	// 收集内存信息
	info.MemoryInfo = hc.collectRemoteMemoryInfo(host)

	result.HardwareInfo = info
	return result
}

// runSSHCommand 通过 SSH 执行远程命令并返回输出（辅助函数）
// 使用统一的 SSH 选项，确保连接稳定性和安全性
func runSSHCommand(host string, command string) (string, error) {
	return runSSHCommandWithTimeout(host, command, 30*time.Second)
}

// runSSHCommandWithTimeout 通过 SSH 执行远程命令，带超时控制
func runSSHCommandWithTimeout(host string, command string, timeout time.Duration) (string, error) {
	cmd := exec.Command("ssh",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		host, command)

	// 设置命令执行超时
	if timeout > 0 {
		cmd.WaitDelay = timeout
	}

	output, err := cmd.Output()
	if err != nil {
		// 区分超时错误和其他错误
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "killed") {
			return "", fmt.Errorf("SSH 命令执行超时（%v）：%v", timeout, err)
		}
		return "", fmt.Errorf("SSH 命令执行失败：%v", err)
	}
	return string(output), nil
}

// runSSHCommandQuiet 通过 SSH 执行远程命令，失败时不返回错误（用于可选命令）
func runSSHCommandQuiet(host string, command string) string {
	output, _ := runSSHCommand(host, command)
	return string(output)
}

// collectRemoteCPUInfo 通过 SSH 收集远程主机 CPU 信息
func (hc *HardwareChecker) collectRemoteCPUInfo(host string) *CPUInfo {
	cpuInfo := &CPUInfo{}

	// 方法 1: 使用 lscpu 命令（最完整）
	output, err := runSSHCommand(host, "lscpu 2>/dev/null")
	if err == nil && output != "" {
		cpuInfo = hc.parseLscpuOutput(output)
	}

	// 方法 2: 如果 lscpu 失败或信息不完整，使用 /proc/cpuinfo
	if cpuInfo.Cores == 0 || cpuInfo.Model == "" {
		output, err = runSSHCommand(host, "cat /proc/cpuinfo")
		if err == nil {
			hc.parseProcCPUinfo(output, cpuInfo)
		}
	}

	// 方法 3: 使用 nproc 获取核心数（最后备用）
	if cpuInfo.Cores == 0 {
		output, err = runSSHCommand(host, "nproc")
		if err == nil {
			cores, _ := strconv.Atoi(strings.TrimSpace(output))
			if cores > 0 {
				cpuInfo.Cores = cores
			}
		}
	}

	// 设置默认值
	if cpuInfo.Sockets == 0 {
		cpuInfo.Sockets = 1
	}
	if cpuInfo.NUMANodes == 0 {
		cpuInfo.NUMANodes = 1
	}

	return cpuInfo
}

// parseProcCPUinfo 解析 /proc/cpuinfo 输出
func (hc *HardwareChecker) parseProcCPUinfo(output string, cpuInfo *CPUInfo) {
	lines := strings.Split(output, "\n")
	processorCount := 0
	physicalIds := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "model name", "Model name":
			if cpuInfo.Model == "" {
				cpuInfo.Model = value
			}
		case "cpu cores":
			if cpuInfo.Cores == 0 {
				cpuInfo.Cores, _ = strconv.Atoi(value)
			}
		case "physical id":
			physicalIds[value] = true
		case "processor":
			processorCount++
		}
	}

	// 如果没有解析到核心数，使用 processor 数量
	if cpuInfo.Cores == 0 && processorCount > 0 {
		cpuInfo.Cores = processorCount
	}

	// 使用 physical id 数量估算 sockets
	if cpuInfo.Sockets == 0 && len(physicalIds) > 0 {
		cpuInfo.Sockets = len(physicalIds)
	}
}

// collectRemoteDiskInfo 通过 SSH 收集远程主机磁盘信息
func (hc *HardwareChecker) collectRemoteDiskInfo(host string) []*DiskInfo {
	var diskInfos []*DiskInfo

	// 方法 1: 使用 lsblk 命令获取磁盘列表（首选）
	output, err := runSSHCommand(host, "lsblk -nd -o NAME,MODEL,SERIAL,TYPE,SIZE,ROTA --bytes 2>/dev/null")
	if err == nil && output != "" {
		diskInfos = hc.parseRemoteLsblkOutput(output)
		if len(diskInfos) > 0 {
			return diskInfos
		}
	}

	// 方法 2: 使用 fdisk -l 获取磁盘列表（备用）
	output, err = runSSHCommand(host, "fdisk -l 2>/dev/null")
	if err == nil && output != "" {
		diskInfos = hc.parseRemoteFdiskOutput(output)
		if len(diskInfos) > 0 {
			return diskInfos
		}
	}

	// 方法 3: 使用 smartctl 获取磁盘列表（最后备用）
	output, err = runSSHCommand(host, "smartctl --scan 2>/dev/null")
	if err == nil && output != "" {
		diskInfos = hc.collectRemoteDiskInfoSmartctl(host, output)
	}

	return diskInfos
}

// parseRemoteLsblkOutput 解析远程 lsblk 输出
func (hc *HardwareChecker) parseRemoteLsblkOutput(output string) []*DiskInfo {
	var diskInfos []*DiskInfo

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		diskInfo := &DiskInfo{
			Name: fields[0],
		}

		// 解析型号（可能包含空格，需要特殊处理）
		if len(fields) >= 2 && fields[1] != "-" {
			diskInfo.Model = fields[1]
			diskInfo.Vendor = hc.extractVendor(fields[1])
		}

		// 解析类型
		if len(fields) >= 4 {
			diskType := strings.ToLower(fields[3])
			if diskType == "disk" {
				// 判断是否为旋转磁盘
				if len(fields) >= 6 && fields[5] == "0" {
					diskInfo.Rotational = false
					diskInfo.Type = "SSD"
				} else if len(fields) >= 6 && fields[5] == "1" {
					diskInfo.Rotational = true
					diskInfo.Type = "HDD"
				} else {
					diskInfo.Type = "SSD"
				}
				// NVMe 特殊处理
				if strings.HasPrefix(fields[0], "nvme") {
					diskInfo.Type = "NVMe"
				}
			}
		}

		// 解析大小
		if len(fields) >= 5 {
			sizeStr := fields[4]
			// 解析带单位的大小（如 500G, 1T）
			size := hc.parseSizeString(sizeStr)
			diskInfo.Size = size
		}

		// 只添加有实际大小的磁盘
		if diskInfo.Size > 0 {
			diskInfos = append(diskInfos, diskInfo)
		}
	}

	return diskInfos
}

// parseRemoteFdiskOutput 解析远程 fdisk -l 输出
func (hc *HardwareChecker) parseRemoteFdiskOutput(output string) []*DiskInfo {
	var diskInfos []*DiskInfo

	// 解析 fdisk 输出中的磁盘信息
	// 示例：Disk /dev/sda: 500 GB, 500107862016 bytes
	re := regexp.MustCompile(`Disk\s+(/dev/\S+):\s*(\d+(?:\.\d+)?)\s*(GB|MB|TB|KB)`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			device := match[1]
			size := match[2]
			unit := match[3]

			// 提取磁盘名（如 sda）
			name := strings.TrimPrefix(device, "/dev/")

			diskInfo := &DiskInfo{
				Name: name,
				Type: "Unknown",
			}

			// 解析大小
			sizeVal, _ := strconv.ParseFloat(size, 64)
			var multiplier uint64 = 1
			switch unit {
			case "TB":
				multiplier = 1024 * 1024 * 1024 * 1024
			case "GB":
				multiplier = 1024 * 1024 * 1024
			case "MB":
				multiplier = 1024 * 1024
			case "KB":
				multiplier = 1024
			}
			diskInfo.Size = uint64(sizeVal * float64(multiplier))

			// 尝试从型号行提取型号
			modelRe := regexp.MustCompile(fmt.Sprintf(`%s\s+\S+:\s*(.+?)\s*\n`, device))
			modelMatch := modelRe.FindStringSubmatch(output)
			if len(modelMatch) >= 2 {
				diskInfo.Model = strings.TrimSpace(modelMatch[1])
				diskInfo.Vendor = hc.extractVendor(diskInfo.Model)
			}

			if diskInfo.Size > 0 {
				diskInfos = append(diskInfos, diskInfo)
			}
		}
	}

	return diskInfos
}

// collectRemoteDiskInfoSmartctl 通过 SSH 使用 smartctl 收集磁盘信息
func (hc *HardwareChecker) collectRemoteDiskInfoSmartctl(host string, scanOutput string) []*DiskInfo {
	var diskInfos []*DiskInfo

	// 解析 smartctl --scan 输出
	lines := strings.Split(scanOutput, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		device := fields[0]
		name := strings.TrimPrefix(device, "/dev/")

		// 使用 smartctl -i 获取详细信息
		output, err := runSSHCommand(host, fmt.Sprintf("smartctl -i %s 2>/dev/null", device))
		if err != nil {
			continue
		}

		diskInfo := &DiskInfo{
			Name: name,
			Type: "Unknown",
		}

		// 解析 smartctl 输出
		for _, smartLine := range strings.Split(output, "\n") {
			if strings.HasPrefix(smartLine, "Device Model:") || strings.HasPrefix(smartLine, "Model Number:") {
				parts := strings.SplitN(smartLine, ":", 2)
				if len(parts) == 2 {
					diskInfo.Model = strings.TrimSpace(parts[1])
					diskInfo.Vendor = hc.extractVendor(diskInfo.Model)
				}
			}
			if strings.HasPrefix(smartLine, "User Capacity:") {
				// 解析 "User Capacity: 500,107,862,016 bytes [500 GB]"
				if idx := strings.Index(smartLine, "bytes"); idx != -1 {
					sizeStr := strings.TrimSpace(smartLine[:idx])
					if idx2 := strings.LastIndex(sizeStr, " "); idx2 != -1 {
						sizeStr = sizeStr[idx2+1:]
						sizeStr = strings.ReplaceAll(sizeStr, ",", "")
						size, _ := strconv.ParseUint(sizeStr, 10, 64)
						diskInfo.Size = size
					}
				}
			}
			if strings.HasPrefix(smartLine, "Rotation Rate:") {
				if strings.Contains(smartLine, "Solid State Device") {
					diskInfo.Type = "SSD"
					diskInfo.Rotational = false
				} else if strings.Contains(smartLine, "rpm") {
					diskInfo.Type = "HDD"
					diskInfo.Rotational = true
				} else {
					diskInfo.Type = "SSD"
					diskInfo.Rotational = false
				}
			}
		}

		// NVMe 特殊处理
		if strings.HasPrefix(name, "nvme") {
			diskInfo.Type = "NVMe"
		}

		if diskInfo.Size > 0 {
			diskInfos = append(diskInfos, diskInfo)
		}
	}

	return diskInfos
}

// collectRemoteRAIDInfo 通过 SSH 收集远程主机 RAID 信息
func (hc *HardwareChecker) collectRemoteRAIDInfo(host string) *RAIDConfigInfo {
	info := &RAIDConfigInfo{
		StripeSize: 64, // 默认条带大小
	}

	// 方法 1: 使用 lspci 检测 RAID 卡
	output, err := runSSHCommand(host, "lspci 2>/dev/null")
	if err == nil {
		hc.parseLspciForRAID(output, info)
	}

	// 方法 2: 使用 megacli 获取 RAID 信息（LSI/Broadcom）
	if output := runSSHCommandQuiet(host, "which megacli"); strings.TrimSpace(output) != "" {
		hc.collectRemoteRAIDInfoMegacli(host, info)
	}

	// 方法 3: 使用 storcli 获取 RAID 信息（LSI/Broadcom）
	if output := runSSHCommandQuiet(host, "which storcli"); strings.TrimSpace(output) != "" {
		hc.collectRemoteRAIDInfoStorcli(host, info)
	}

	// 方法 4: 使用 perccli 获取 RAID 信息（Dell PERC）
	if output := runSSHCommandQuiet(host, "which perccli"); strings.TrimSpace(output) != "" {
		hc.collectRemoteRAIDInfoPerccli(host, info)
	}

	// 方法 5: 使用 ssacli/hpssacli 获取 RAID 信息（HP SmartArray）
	if output := runSSHCommandQuiet(host, "which ssacli"); strings.TrimSpace(output) != "" {
		hc.collectRemoteRAIDInfoSsacli(host, info, "ssacli")
	} else if output := runSSHCommandQuiet(host, "which hpssacli"); strings.TrimSpace(output) != "" {
		hc.collectRemoteRAIDInfoSsacli(host, info, "hpssacli")
	}

	// 方法 6: 检查软件 RAID
	output, err = runSSHCommand(host, "cat /proc/mdstat 2>/dev/null")
	if err == nil && strings.Contains(output, "active") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "Linux Software RAID (mdraid)"
		}
		info.StripeSize = 512
	}

	return info
}

// collectRemoteRAIDInfoMegacli 通过 SSH 使用 megacli 收集 RAID 信息
func (hc *HardwareChecker) collectRemoteRAIDInfoMegacli(host string, info *RAIDConfigInfo) {
	output, err := runSSHCommand(host, "megacli adpallinfo -aALL 2>/dev/null")
	if err != nil {
		return
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Product Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.RAIDModel = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "BBU") && strings.Contains(line, "Present") {
			info.BatteryBackup = true
		}
		if strings.Contains(line, "Stripe Size") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				size := strings.TrimSpace(parts[1])
				if strings.Contains(size, "KB") {
					size = strings.ReplaceAll(size, "KB", "")
					size = strings.TrimSpace(size)
					if val, err := strconv.Atoi(size); err == nil {
						info.StripeSize = val
					}
				}
			}
		}
	}
}

// collectRemoteRAIDInfoStorcli 通过 SSH 使用 storcli 收集 RAID 信息
func (hc *HardwareChecker) collectRemoteRAIDInfoStorcli(host string, info *RAIDConfigInfo) {
	output, err := runSSHCommand(host, "storcli /c0 show all 2>/dev/null")
	if err != nil {
		return
	}

	if strings.Contains(output, "RAID Level") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "Broadcom/LSI MegaRAID"
		}
		// 解析 RAID 级别
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "RAID Level") {
				if strings.Contains(line, "RAID0") {
					info.RAIDLevel = "RAID 0"
				} else if strings.Contains(line, "RAID1") {
					info.RAIDLevel = "RAID 1"
				} else if strings.Contains(line, "RAID5") {
					info.RAIDLevel = "RAID 5"
				} else if strings.Contains(line, "RAID6") {
					info.RAIDLevel = "RAID 6"
				} else if strings.Contains(line, "RAID10") {
					info.RAIDLevel = "RAID 10"
				}
			}
		}
	}
}

// collectRemoteRAIDInfoPerccli 通过 SSH 使用 perccli 收集 RAID 信息
func (hc *HardwareChecker) collectRemoteRAIDInfoPerccli(host string, info *RAIDConfigInfo) {
	output, err := runSSHCommand(host, "perccli /c0 show all 2>/dev/null")
	if err != nil {
		return
	}

	if strings.Contains(output, "RAID Level") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "Dell PERC"
		}
	}
}

// collectRemoteRAIDInfoSsacli 通过 SSH 使用 ssacli/hpssacli 收集 RAID 信息
func (hc *HardwareChecker) collectRemoteRAIDInfoSsacli(host string, info *RAIDConfigInfo, cmdName string) {
	output, err := runSSHCommand(host, fmt.Sprintf("%s controller all show config 2>/dev/null", cmdName))
	if err != nil {
		return
	}

	if strings.Contains(output, "RAID") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "HP SmartArray"
		}
	}
}

// collectRemoteNICInfo 通过 SSH 收集远程主机网卡信息
func (hc *HardwareChecker) collectRemoteNICInfo(host string) []*NICInfo {
	var nicInfos []*NICInfo

	// 使用 SSH 执行 ip 命令获取网卡列表
	output, err := runSSHCommand(host, "ip -o link show")
	if err != nil {
		return nicInfos
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// 提取网卡名称（去掉冒号）
		name := strings.TrimSuffix(fields[1], ":")

		// 跳过 lo 回环接口和虚拟接口
		if name == "lo" || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "tap") {
			continue
		}

		nicInfo := &NICInfo{
			Name: name,
			MTU:  1500, // 默认 MTU
		}

		// 获取 MAC 地址
		if len(fields) >= 5 {
			for i, field := range fields {
				if strings.Count(field, ":") == 5 && len(field) == 17 {
					nicInfo.MACAddress = strings.ToLower(field)
					// 下一个字段可能是 MTU
					if i+2 < len(fields) && strings.HasPrefix(fields[i+2], "mtu") {
						mtu, _ := strconv.Atoi(fields[i+2][4:])
						if mtu > 0 {
							nicInfo.MTU = mtu
						}
					}
					break
				}
			}
		}

		// 获取速率
		speedOutput, err := runSSHCommand(host, fmt.Sprintf("cat /sys/class/net/%s/speed", name))
		if err == nil {
			speed, _ := strconv.Atoi(strings.TrimSpace(string(speedOutput)))
			if speed > 0 {
				nicInfo.Speed = speed
			}
		}

		// 获取队列大小
		queueOutput, err := runSSHCommand(host, fmt.Sprintf("cat /sys/class/net/%s/tx_queue_len", name))
		if err == nil {
			queueSize, _ := strconv.Atoi(strings.TrimSpace(string(queueOutput)))
			if queueSize > 0 {
				nicInfo.QueueSize = queueSize
			}
		}
		if nicInfo.QueueSize == 0 {
			nicInfo.QueueSize = 1000
		}

		// 检测是否为 bond 接口
		if strings.HasPrefix(name, "bond") {
			nicInfo.IsBond = true
			// 获取绑定模式
			modeOutput, err := runSSHCommand(host, fmt.Sprintf("cat /sys/class/net/%s/bonding/mode", name))
			if err == nil {
				nicInfo.BondMode = strings.TrimSpace(string(modeOutput))
			}
			// 获取从网卡数量
			slavesOutput, err := runSSHCommand(host, fmt.Sprintf("cat /sys/class/net/%s/bonding/slaves", name))
			if err == nil {
				nicInfo.BondSlaves = len(strings.Fields(string(slavesOutput)))
			}
		}

		// 获取驱动信息
		driverOutput, err := runSSHCommand(host, fmt.Sprintf("ethtool -i %s", name))
		if err == nil {
			for _, line := range strings.Split(string(driverOutput), "\n") {
				if strings.HasPrefix(line, "driver:") {
					nicInfo.Driver = strings.TrimSpace(strings.TrimPrefix(line, "driver:"))
					break
				}
			}
		}
		if nicInfo.Driver == "" {
			nicInfo.Driver = "Unknown"
		}

		nicInfos = append(nicInfos, nicInfo)
	}

	return nicInfos
}

// collectRemoteMemoryInfo 通过 SSH 收集远程主机内存信息
func (hc *HardwareChecker) collectRemoteMemoryInfo(host string) *MemoryInfo {
	memInfo := &MemoryInfo{
		MemoryType:  "Unknown",
		MemorySpeed: 0,
		MemorySlots: 0,
	}

	// 方法 1: 使用 free 命令获取总内存
	output, err := runSSHCommand(host, "free -b 2>/dev/null")
	if err == nil {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Mem:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					total, _ := strconv.ParseUint(fields[1], 10, 64)
					memInfo.TotalMemory = total
				}
				break
			}
		}
	}

	// 方法 2: 尝试使用 dmidecode 获取内存详细信息（需要 root 权限）
	hc.collectRemoteMemoryInfoDmidecode(host, memInfo)

	// 方法 3: 尝试从 /sys/devices/system/node 获取 NUMA 节点数
	nodeOutput, err := runSSHCommand(host, "ls -d /sys/devices/system/node/node* 2>/dev/null")
	if err == nil {
		nodes := strings.Fields(nodeOutput)
		memInfo.NUMANodes = len(nodes)
		if memInfo.NUMANodes == 0 {
			memInfo.NUMANodes = 1
		}
	} else {
		memInfo.NUMANodes = 1
	}

	// 内存插槽数使用 NUMA 节点数估算（如果没有通过 dmidecode 获取到）
	if memInfo.MemorySlots == 0 {
		memInfo.MemorySlots = memInfo.NUMANodes
	}

	return memInfo
}

// collectRemoteMemoryInfoDmidecode 通过 SSH 使用 dmidecode 收集内存详细信息
func (hc *HardwareChecker) collectRemoteMemoryInfoDmidecode(host string, memInfo *MemoryInfo) {
	// 检查 dmidecode 是否存在
	output, err := runSSHCommand(host, "which dmidecode 2>/dev/null")
	if err != nil || strings.TrimSpace(output) == "" {
		return
	}

	// 执行 dmidecode 获取内存信息
	output, err = runSSHCommand(host, "dmidecode -t memory -q 2>/dev/null")
	if err != nil {
		return
	}

	hc.parseDmidecodeMemoryOutput(output, memInfo)
}

// parseSizeString 解析带单位的大小字符串（如 500G, 1T, 500M）
func (hc *HardwareChecker) parseSizeString(sizeStr string) uint64 {
	sizeStr = strings.ToUpper(sizeStr)
	var multiplier uint64 = 1

	if strings.HasSuffix(sizeStr, "T") {
		multiplier = 1024 * 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "T")
	} else if strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "G")
	} else if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	} else if strings.HasSuffix(sizeStr, "K") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "K")
	}

	size, _ := strconv.ParseUint(sizeStr, 10, 64)
	return size * multiplier
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

// collectCPUInfo 收集 CPU 信息
func (hc *HardwareChecker) collectCPUInfo() *CPUInfo {
	if hc.OSInfo.IsFreeBSD {
		return hc.collectCPUInfoFreeBSD()
	}
	return hc.collectCPUInfoLinux()
}

// collectCPUInfoLinux 收集 Linux CPU 信息
func (hc *HardwareChecker) collectCPUInfoLinux() *CPUInfo {
	cpuInfo := &CPUInfo{}

	// 使用 lscpu 一次性获取所有 CPU 信息
	if utils.CommandExists("lscpu") {
		cmd := exec.Command("lscpu")
		output, err := cmd.Output()
		if err == nil {
			cpuInfo = hc.parseLscpuOutput(string(output))
		}
	}

	// 如果 lscpu 失败或信息不完整，使用备用方法
	if cpuInfo.Cores == 0 {
		cpuInfo.Cores = hc.getCPUCoreFallback()
	}
	if cpuInfo.Sockets == 0 {
		cpuInfo.Sockets = 1
	}
	if cpuInfo.NUMANodes == 0 {
		cpuInfo.NUMANodes = 1
	}

	// ARM64 特殊处理
	if hc.OSInfo.Arch == "aarch64" && cpuInfo.Model == "" {
		cpuInfo.Model = hc.getARMCPUModel()
	}

	return cpuInfo
}

// getARMCPUModel 获取 ARM CPU 型号
func (hc *HardwareChecker) getARMCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "Unknown"
	}

	var cpuImplementer, cpuPart string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "CPU implementer") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				cpuImplementer = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "CPU part") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				cpuPart = strings.TrimSpace(parts[1])
			}
		}
	}

	// 解析 ARM CPU 型号
	if cpuImplementer == "0x41" { // ARM
		switch cpuPart {
		case "0xd03":
			return "Cortex-A53"
		case "0xd07":
			return "Cortex-A57"
		case "0xd08":
			return "Cortex-A72"
		case "0xd09":
			return "Cortex-A73"
		case "0xd0a":
			return "Cortex-A75"
		case "0xd40":
			return "Cortex-A76"
		case "0xd41":
			return "Cortex-A78"
		case "0xd42":
			return "Cortex-A710"
		case "0xd44":
			return "Cortex-X1"
		case "0xd47":
			return "Cortex-A715"
		case "0xd4e":
			return "Cortex-X2"
		}
	} else if cpuImplementer == "0x4e" { // NVIDIA
		return "NVIDIA Carmel"
	} else if cpuImplementer == "0x51" { // Qualcomm
		return "Qualcomm Kryo"
	}

	return "Unknown ARM CPU"
}

// collectCPUInfoFreeBSD 收集 FreeBSD CPU 信息
func (hc *HardwareChecker) collectCPUInfoFreeBSD() *CPUInfo {
	cpuInfo := &CPUInfo{
		Sockets:   1,
		NUMANodes: 1,
	}

	// 获取 CPU 型号
	cmd := exec.Command("sysctl", "hw.model")
	output, err := cmd.Output()
	if err == nil {
		line := strings.TrimSpace(string(output))
		if idx := strings.Index(line, ":"); idx != -1 {
			cpuInfo.Model = strings.TrimSpace(line[idx+1:])
		}
	}

	// 获取 CPU 核心数
	cmd = exec.Command("sysctl", "hw.ncpu")
	output, err = cmd.Output()
	if err == nil {
		line := strings.TrimSpace(string(output))
		if idx := strings.Index(line, ":"); idx != -1 {
			cores, _ := strconv.Atoi(strings.TrimSpace(line[idx+1:]))
			cpuInfo.Cores = cores
		}
	}

	// 获取物理核心数
	cmd = exec.Command("sysctl", "hw.ncpu_phys")
	output, err = cmd.Output()
	if err == nil {
		line := strings.TrimSpace(string(output))
		if idx := strings.Index(line, ":"); idx != -1 {
			phys, _ := strconv.Atoi(strings.TrimSpace(line[idx+1:]))
			if phys > 0 {
				cpuInfo.Cores = phys
			}
		}
	}

	return cpuInfo
}

// parseLscpuOutput 解析 lscpu 输出
func (hc *HardwareChecker) parseLscpuOutput(output string) *CPUInfo {
	cpuInfo := &CPUInfo{}

	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Model name":
			cpuInfo.Model = value
		case "CPU(s)":
			cpuInfo.Cores, _ = strconv.Atoi(value)
		case "Socket(s)":
			cpuInfo.Sockets, _ = strconv.Atoi(value)
		case "CPU base MHz":
			if freq, err := strconv.ParseFloat(value, 64); err == nil {
				cpuInfo.BaseFreq = int(freq)
			}
		case "CPU max MHz":
			if freq, err := strconv.ParseFloat(value, 64); err == nil {
				cpuInfo.TurboFreq = int(freq)
			}
		case "NUMA node(s)":
			cpuInfo.NUMANodes, _ = strconv.Atoi(value)
		}
	}

	// 如果 Model 为空，尝试从 /proc/cpuinfo 读取
	if cpuInfo.Model == "" {
		cpuInfo.Model = hc.getCPUModelFallback()
	}

	return cpuInfo
}

// getCPUCoreFallback 获取 CPU 核心数（备用方法）
func (hc *HardwareChecker) getCPUCoreFallback() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		count := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "processor") {
				count++
			}
		}
		if count > 0 {
			return count
		}
	}
	return 1
}

// getCPUModelFallback 获取 CPU 型号（备用方法）
func (hc *HardwareChecker) getCPUModelFallback() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return "Unknown"
}

// collectDiskInfo 收集磁盘信息
func (hc *HardwareChecker) collectDiskInfo() []*DiskInfo {
	if hc.OSInfo.IsFreeBSD {
		return hc.collectDiskInfoFreeBSD()
	}
	return hc.collectDiskInfoLinux()
}

// collectDiskInfoLinux 收集 Linux 磁盘信息
func (hc *HardwareChecker) collectDiskInfoLinux() []*DiskInfo {
	var diskInfos []*DiskInfo

	// 遍历 /sys/block 获取所有块设备
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return diskInfos
	}

	for _, entry := range entries {
		name := entry.Name()
		// 跳过 loop、ram 等设备
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}

		diskInfo := &DiskInfo{
			Name: name,
			Type: "Unknown",
		}

		// 获取磁盘信息
		hc.collectDiskBasicInfoLinux(name, diskInfo)

		// 只添加有实际大小的磁盘
		if diskInfo.Size > 0 {
			diskInfos = append(diskInfos, diskInfo)
		}
	}

	return diskInfos
}

// collectDiskBasicInfoLinux 收集 Linux 磁盘基本信息
func (hc *HardwareChecker) collectDiskBasicInfoLinux(name string, diskInfo *DiskInfo) {
	// 获取磁盘大小
	sizePath := fmt.Sprintf("/sys/block/%s/size", name)
	if data, err := os.ReadFile(sizePath); err == nil {
		sectors, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		diskInfo.Size = sectors * 512 // 每扇区 512 字节
	}

	// 获取是否旋转磁盘并确定类型
	rotPath := fmt.Sprintf("/sys/block/%s/queue/rotational", name)
	if data, err := os.ReadFile(rotPath); err == nil {
		diskInfo.Rotational = strings.TrimSpace(string(data)) == "1"
		if diskInfo.Rotational {
			diskInfo.Type = "HDD"
		} else {
			diskInfo.Type = "SSD"
		}
	}

	// 检测 NVMe
	if strings.HasPrefix(name, "nvme") {
		diskInfo.Type = "NVMe"
	}

	// 获取磁盘型号和厂家（多种方法）
	diskInfo.Model, diskInfo.Vendor = hc.getDiskModelLinux(name)
}

// getDiskModelLinux 获取 Linux 磁盘型号和厂家
func (hc *HardwareChecker) getDiskModelLinux(name string) (string, string) {
	// 方法 1: 尝试从 /sys/block 读取（最快）
	modelPath := fmt.Sprintf("/sys/block/%s/device/model", name)
	if data, err := os.ReadFile(modelPath); err == nil {
		model := strings.TrimSpace(string(data))
		if model != "" {
			return model, hc.extractVendor(model)
		}
	}

	// 方法 2: 尝试使用 smartctl（通用，支持 HDD/SSD/NVMe）
	if utils.CommandExists("smartctl") {
		if model, vendor := hc.getDiskModelSmartctl(name); model != "" {
			return model, vendor
		}
	}

	// 方法 3: 尝试使用 hdparm（仅支持 SATA/SAS）
	if utils.CommandExists("hdparm") {
		if model, vendor := hc.getDiskModelHdparm(name); model != "" {
			return model, vendor
		}
	}

	// 方法 4: 尝试使用 nvme-cli（仅支持 NVMe）
	if utils.CommandExists("nvme") && strings.HasPrefix(name, "nvme") {
		if model, vendor := hc.getDiskModelNvme(name); model != "" {
			return model, vendor
		}
	}

	return "Unknown", ""
}

// getDiskModelSmartctl 使用 smartctl 获取磁盘型号
func (hc *HardwareChecker) getDiskModelSmartctl(name string) (string, string) {
	device := "/dev/" + name
	// NVMe 设备需要特殊处理
	if strings.HasPrefix(name, "nvme") {
		device = "/dev/" + name
	}

	cmd := exec.Command("smartctl", "-i", device)
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	var model, vendor string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Model Number:") {
			model = strings.TrimSpace(strings.TrimPrefix(line, "Model Number:"))
		} else if strings.HasPrefix(line, "Device Model:") {
			model = strings.TrimSpace(strings.TrimPrefix(line, "Device Model:"))
		} else if strings.HasPrefix(line, "Vendor:") {
			vendor = strings.TrimSpace(strings.TrimPrefix(line, "Vendor:"))
		}
	}

	if model == "" {
		return "", ""
	}
	if vendor == "" {
		vendor = hc.extractVendor(model)
	}
	return model, vendor
}

// getDiskModelHdparm 使用 hdparm 获取磁盘型号
func (hc *HardwareChecker) getDiskModelHdparm(name string) (string, string) {
	cmd := exec.Command("hdparm", "-I", "/dev/"+name)
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Model Number:") {
			model := strings.TrimSpace(strings.TrimPrefix(line, "Model Number:"))
			return model, hc.extractVendor(model)
		}
	}
	return "", ""
}

// getDiskModelNvme 使用 nvme-cli 获取磁盘型号
func (hc *HardwareChecker) getDiskModelNvme(name string) (string, string) {
	cmd := exec.Command("nvme", "list")
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, name) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				model := fields[1]
				return model, hc.extractVendor(model)
			}
		}
	}
	return "", ""
}

// collectDiskInfoFreeBSD 收集 FreeBSD 磁盘信息
func (hc *HardwareChecker) collectDiskInfoFreeBSD() []*DiskInfo {
	var diskInfos []*DiskInfo

	// 使用 camcontrol 获取磁盘列表
	cmd := exec.Command("camcontrol", "devlist")
	output, err := cmd.Output()
	if err != nil {
		return diskInfos
	}

	// 解析 camcontrol 输出
	diskNames := hc.parseCamcontrolDevlist(string(output))

	for _, name := range diskNames {
		diskInfo := &DiskInfo{
			Name: name,
			Type: "Unknown",
		}

		// 获取磁盘详细信息
		hc.collectDiskBasicInfoFreeBSD(name, diskInfo)

		if diskInfo.Size > 0 {
			diskInfos = append(diskInfos, diskInfo)
		}
	}

	return diskInfos
}

// parseCamcontrolDevlist 解析 camcontrol devlist 输出
func (hc *HardwareChecker) parseCamcontrolDevlist(output string) []string {
	var disks []string
	for _, line := range strings.Split(output, "\n") {
		// 示例：<da0:pass0:0:0:0> ATA disk directly attached
		if strings.Contains(line, "disk") {
			parts := strings.Split(line, ":")
			if len(parts) > 0 {
				// 提取磁盘名（如 da0）
				namePart := strings.TrimSpace(parts[0])
				namePart = strings.Trim(namePart, "<")
				if strings.HasPrefix(namePart, "da") || strings.HasPrefix(namePart, "ada") || strings.HasPrefix(namePart, "nvd") {
					disks = append(disks, namePart)
				}
			}
		}
	}
	return disks
}

// collectDiskBasicInfoFreeBSD 收集 FreeBSD 磁盘基本信息
func (hc *HardwareChecker) collectDiskBasicInfoFreeBSD(name string, diskInfo *DiskInfo) {
	// 使用 smartctl 获取磁盘信息
	if utils.CommandExists("smartctl") {
		device := "/dev/" + name
		cmd := exec.Command("smartctl", "-i", device)
		output, err := cmd.Output()
		if err == nil {
			hc.parseSmartctlFreeBSDOutput(string(output), diskInfo)
		}
	}

	// 使用 camcontrol 获取磁盘信息
	cmd := exec.Command("camcontrol", "identify", name)
	output, err := cmd.Output()
	if err == nil {
		hc.parseCamcontrolIdentify(string(output), diskInfo)
	}
}

// parseSmartctlFreeBSDOutput 解析 FreeBSD smartctl 输出
func (hc *HardwareChecker) parseSmartctlFreeBSDOutput(output string, diskInfo *DiskInfo) {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Device Model:") || strings.HasPrefix(line, "Model Number:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				diskInfo.Model = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "User Capacity:") {
			// 解析 "User Capacity: 500,107,862,016 bytes [500 GB]"
			if idx := strings.Index(line, "bytes"); idx != -1 {
				sizeStr := strings.TrimSpace(line[:idx])
				if idx2 := strings.LastIndex(sizeStr, " "); idx2 != -1 {
					sizeStr = sizeStr[idx2+1:]
					sizeStr = strings.ReplaceAll(sizeStr, ",", "")
					size, _ := strconv.ParseUint(sizeStr, 10, 64)
					diskInfo.Size = size
				}
			}
		}
		if strings.HasPrefix(line, "Rotation Rate:") {
			if strings.Contains(line, "Solid State Device") {
				diskInfo.Type = "SSD"
				diskInfo.Rotational = false
			} else if strings.Contains(line, "rpm") {
				diskInfo.Type = "HDD"
				diskInfo.Rotational = true
			}
		}
	}
}

// parseCamcontrolIdentify 解析 camcontrol identify 输出
func (hc *HardwareChecker) parseCamcontrolIdentify(output string, diskInfo *DiskInfo) {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "model number") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				diskInfo.Model = strings.TrimSpace(parts[1])
			}
		}
	}
}

// extractVendor 从型号中提取厂家
func (hc *HardwareChecker) extractVendor(model string) string {
	model = strings.ToUpper(model)
	if strings.Contains(model, "SAMSUNG") {
		return "Samsung"
	}
	if strings.Contains(model, "INTEL") {
		return "Intel"
	}
	if strings.Contains(model, "WD ") || strings.Contains(model, "WESTERN DIGITAL") {
		return "Western Digital"
	}
	if strings.Contains(model, "SEAGATE") {
		return "Seagate"
	}
	if strings.Contains(model, "TOSHIBA") {
		return "Toshiba"
	}
	if strings.Contains(model, "HITACHI") {
		return "Hitachi"
	}
	if strings.Contains(model, "HGST") {
		return "HGST"
	}
	if strings.Contains(model, "CRUCIAL") {
		return "Crucial"
	}
	if strings.Contains(model, "KINGSTON") {
		return "Kingston"
	}
	return ""
}

// collectRAIDInfo 收集 RAID 信息
func (hc *HardwareChecker) collectRAIDInfo() *RAIDConfigInfo {
	if hc.OSInfo.IsFreeBSD {
		return hc.collectRAIDInfoFreeBSD()
	}
	return hc.collectRAIDInfoLinux()
}

// collectRAIDInfoLinux 收集 Linux RAID 信息
func (hc *HardwareChecker) collectRAIDInfoLinux() *RAIDConfigInfo {
	info := &RAIDConfigInfo{}

	// 使用 lspci 检测 RAID 卡
	if utils.CommandExists("lspci") {
		cmd := exec.Command("lspci")
		output, err := cmd.Output()
		if err == nil {
			hc.parseLspciForRAID(string(output), info)
		}
	}

	// 使用多种 RAID 工具获取详细信息
	hc.collectRAIDInfoMegacli(info)
	hc.collectRAIDInfoStorcli(info)
	hc.collectRAIDInfoPerccli(info)
	hc.collectRAIDInfoSsacli(info)

	// 检测软件 RAID
	data, err := os.ReadFile("/proc/mdstat")
	if err == nil && strings.Contains(string(data), "active") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "Linux Software RAID (mdraid)"
		}
		info.StripeSize = 512
	}

	// 默认条带大小
	if info.StripeSize == 0 {
		info.StripeSize = 64
	}

	return info
}

// parseLspciForRAID 解析 lspci 输出检测 RAID 卡
func (hc *HardwareChecker) parseLspciForRAID(output string, info *RAIDConfigInfo) {
	raidKeywords := []string{
		"raid", "sas controller", "storage controller", "perc",
		"smart array", "mega raid", "poweredge raid",
	}
	for _, line := range strings.Split(output, "\n") {
		lineLower := strings.ToLower(line)
		for _, keyword := range raidKeywords {
			if strings.Contains(lineLower, keyword) {
				info.HasRAID = true
				// 提取型号
				if idx := strings.Index(line, ":"); idx != -1 {
					info.RAIDModel = strings.TrimSpace(line[idx+1:])
				}
				break
			}
		}
		if info.HasRAID {
			break
		}
	}
}

// collectRAIDInfoMegacli 使用 megacli 获取 RAID 信息
func (hc *HardwareChecker) collectRAIDInfoMegacli(info *RAIDConfigInfo) {
	if !utils.CommandExists("megacli") {
		return
	}

	cmd := exec.Command("megacli", "adpallinfo", "-aALL")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Product Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.RAIDModel = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "BBU") && strings.Contains(line, "Present") {
			info.BatteryBackup = true
		}
		if strings.Contains(line, "Stripe Size") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				size := strings.TrimSpace(parts[1])
				if strings.Contains(size, "KB") {
					size = strings.ReplaceAll(size, "KB", "")
					size = strings.TrimSpace(size)
					if val, err := strconv.Atoi(size); err == nil {
						info.StripeSize = val
					}
				}
			}
		}
	}
}

// collectRAIDInfoStorcli 使用 storcli 获取 RAID 信息
func (hc *HardwareChecker) collectRAIDInfoStorcli(info *RAIDConfigInfo) {
	if !utils.CommandExists("storcli") {
		return
	}

	cmd := exec.Command("storcli", "/c0", "show", "all")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	content := string(output)
	if strings.Contains(content, "RAID Level") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "Broadcom/LSI MegaRAID"
		}
		// 解析 RAID 级别
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "RAID Level") {
				if strings.Contains(line, "RAID0") {
					info.RAIDLevel = "RAID 0"
				} else if strings.Contains(line, "RAID1") {
					info.RAIDLevel = "RAID 1"
				} else if strings.Contains(line, "RAID5") {
					info.RAIDLevel = "RAID 5"
				} else if strings.Contains(line, "RAID6") {
					info.RAIDLevel = "RAID 6"
				} else if strings.Contains(line, "RAID10") {
					info.RAIDLevel = "RAID 10"
				}
			}
		}
	}
}

// collectRAIDInfoPerccli 使用 perccli 获取 RAID 信息（Dell PERC）
func (hc *HardwareChecker) collectRAIDInfoPerccli(info *RAIDConfigInfo) {
	if !utils.CommandExists("perccli") {
		return
	}

	cmd := exec.Command("perccli", "/c0", "show", "all")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	content := string(output)
	if strings.Contains(content, "RAID Level") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "Dell PERC"
		}
	}
}

// collectRAIDInfoSsacli 使用 ssacli 获取 RAID 信息（HP SmartArray）
func (hc *HardwareChecker) collectRAIDInfoSsacli(info *RAIDConfigInfo) {
	if !utils.CommandExists("ssacli") && !utils.CommandExists("hpssacli") {
		return
	}

	cmdName := "ssacli"
	if !utils.CommandExists("ssacli") {
		cmdName = "hpssacli"
	}

	cmd := exec.Command(cmdName, "controller", "all", "show", "config")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	content := string(output)
	if strings.Contains(content, "RAID") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "HP SmartArray"
		}
	}
}

// collectRAIDInfoFreeBSD 收集 FreeBSD RAID 信息
func (hc *HardwareChecker) collectRAIDInfoFreeBSD() *RAIDConfigInfo {
	info := &RAIDConfigInfo{}

	// 使用 pciconf 检测 RAID 卡
	cmd := exec.Command("pciconf", "-l")
	output, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(strings.ToLower(line), "mass storage") {
				if strings.Contains(line, "raid") || strings.Contains(line, "sas") {
					info.HasRAID = true
					info.RAIDModel = "FreeBSD RAID Controller"
				}
			}
		}
	}

	// 检测 gmirror 软件 RAID
	cmd = exec.Command("gmirror", "status")
	output, err = cmd.Output()
	if err == nil && strings.Contains(string(output), "COMPLETE") {
		info.HasRAID = true
		if info.RAIDModel == "" {
			info.RAIDModel = "FreeBSD gmirror"
		}
		info.StripeSize = 64
	}

	return info
}

// collectNICInfo 收集网卡信息
func (hc *HardwareChecker) collectNICInfo() []*NICInfo {
	if hc.OSInfo.IsFreeBSD {
		return hc.collectNICInfoFreeBSD()
	}
	return hc.collectNICInfoLinux()
}

// collectNICInfoLinux 收集 Linux 网卡信息
func (hc *HardwareChecker) collectNICInfoLinux() []*NICInfo {
	var nicInfos []*NICInfo

	// 遍历 /sys/class/net 获取所有网卡
	netPath := "/sys/class/net"
	entries, err := os.ReadDir(netPath)
	if err != nil {
		return nicInfos
	}

	for _, entry := range entries {
		name := entry.Name()
		// 跳过 lo 回环接口和虚拟接口
		if name == "lo" || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "tap") {
			continue
		}

		nicInfo := &NICInfo{
			Name: name,
			MTU:  1500, // 默认 MTU
		}

		// 收集网卡信息
		hc.collectNICBasicInfoLinux(name, nicInfo)

		nicInfos = append(nicInfos, nicInfo)
	}

	return nicInfos
}

// collectNICBasicInfoLinux 收集 Linux 网卡基本信息
func (hc *HardwareChecker) collectNICBasicInfoLinux(name string, nicInfo *NICInfo) {
	netPath := "/sys/class/net"

	// 获取 MAC 地址
	if data, err := os.ReadFile(fmt.Sprintf("%s/%s/address", netPath, name)); err == nil {
		nicInfo.MACAddress = strings.TrimSpace(string(data))
	}

	// 获取 MTU
	if data, err := os.ReadFile(fmt.Sprintf("%s/%s/mtu", netPath, name)); err == nil {
		nicInfo.MTU, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}

	// 获取速率（处理 -1 表示未知的情况）
	if data, err := os.ReadFile(fmt.Sprintf("%s/%s/speed", netPath, name)); err == nil {
		speed, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if speed > 0 {
			nicInfo.Speed = speed
		}
	}

	// 获取队列大小
	if data, err := os.ReadFile(fmt.Sprintf("%s/%s/tx_queue_len", netPath, name)); err == nil {
		nicInfo.QueueSize, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	if nicInfo.QueueSize == 0 {
		nicInfo.QueueSize = 1000 // 默认队列大小
	}

	// 检测是否为 bond 接口
	if strings.HasPrefix(name, "bond") {
		nicInfo.IsBond = true
		// 获取绑定模式
		if data, err := os.ReadFile(fmt.Sprintf("%s/%s/bonding/mode", netPath, name)); err == nil {
			nicInfo.BondMode = strings.TrimSpace(string(data))
		}
		// 获取从网卡数量
		if data, err := os.ReadFile(fmt.Sprintf("%s/%s/bonding/slaves", netPath, name)); err == nil {
			nicInfo.BondSlaves = len(strings.Fields(string(data)))
		}
	}

	// 检测是否为 team 接口（RHEL7+）
	if strings.HasPrefix(name, "team") {
		nicInfo.IsBond = true
		nicInfo.BondMode = "team"
	}

	// 获取驱动信息
	nicInfo.Driver = hc.getNICDriverLinux(name)
}

// getNICDriverLinux 获取 Linux 网卡驱动
func (hc *HardwareChecker) getNICDriverLinux(name string) string {
	// 方法 1: 使用 ethtool
	if utils.CommandExists("ethtool") {
		cmd := exec.Command("ethtool", "-i", name)
		output, err := cmd.Output()
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "driver:") {
					return strings.TrimSpace(strings.TrimPrefix(line, "driver:"))
				}
			}
		}
	}

	// 方法 2: 从 /sys/class/net 读取
	driverPath := fmt.Sprintf("/sys/class/net/%s/device/driver", name)
	link, err := os.Readlink(driverPath)
	if err == nil {
		parts := strings.Split(link, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	return "Unknown"
}

// collectNICInfoFreeBSD 收集 FreeBSD 网卡信息
func (hc *HardwareChecker) collectNICInfoFreeBSD() []*NICInfo {
	var nicInfos []*NICInfo

	// 使用 ifconfig 获取网卡列表
	cmd := exec.Command("ifconfig", "-a")
	output, err := cmd.Output()
	if err != nil {
		return nicInfos
	}

	// 解析 ifconfig 输出
	interfaces := hc.parseIfconfigOutput(string(output))

	for _, iface := range interfaces {
		// 跳过 lo 回环接口和虚拟接口
		if iface.Name == "lo" || strings.HasPrefix(iface.Name, "epair") ||
			strings.HasPrefix(iface.Name, "vnet") || strings.HasPrefix(iface.Name, "tap") {
			continue
		}

		nicInfo := &NICInfo{
			Name: iface.Name,
			MTU:  iface.MTU,
		}

		// 收集 FreeBSD 网卡信息
		hc.collectNICBasicInfoFreeBSD(iface.Name, nicInfo)

		nicInfos = append(nicInfos, nicInfo)
	}

	return nicInfos
}

// InterfaceInfo 临时结构体用于存储 ifconfig 解析结果
type InterfaceInfo struct {
	Name string
	MTU  int
}

// parseIfconfigOutput 解析 ifconfig 输出
func (hc *HardwareChecker) parseIfconfigOutput(output string) []InterfaceInfo {
	var interfaces []InterfaceInfo
	var currentInterface string
	var currentMTU int

	for _, line := range strings.Split(output, "\n") {
		// 检测新的接口（行首没有空格且包含冒号）
		if len(line) > 0 && line[0] != ' ' && strings.Contains(line, ":") {
			// 保存之前的接口
			if currentInterface != "" {
				interfaces = append(interfaces, InterfaceInfo{
					Name: currentInterface,
					MTU:  currentMTU,
				})
			}
			// 提取新接口名
			parts := strings.Split(line, ":")
			currentInterface = strings.TrimSpace(parts[0])
			currentMTU = 1500 // 默认 MTU
		} else if strings.Contains(line, "mtu") {
			// 解析 MTU
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "mtu") {
					mtu, _ := strconv.Atoi(strings.TrimPrefix(field, "mtu"))
					if mtu > 0 {
						currentMTU = mtu
					}
				}
			}
		}
	}

	// 添加最后一个接口
	if currentInterface != "" {
		interfaces = append(interfaces, InterfaceInfo{
			Name: currentInterface,
			MTU:  currentMTU,
		})
	}

	return interfaces
}

// collectNICBasicInfoFreeBSD 收集 FreeBSD 网卡基本信息
func (hc *HardwareChecker) collectNICBasicInfoFreeBSD(name string, nicInfo *NICInfo) {
	// 使用 ifconfig 获取详细信息
	cmd := exec.Command("ifconfig", name)
	output, err := cmd.Output()
	if err == nil {
		hc.parseIfconfigDetailOutput(string(output), nicInfo)
	}

	// 使用 sysctl 获取网卡驱动
	nicInfo.Driver = hc.getNICDriverFreeBSD(name)

	// 检测是否为 lagg 接口（FreeBSD 的 bond）
	if strings.HasPrefix(name, "lagg") {
		nicInfo.IsBond = true
		nicInfo.BondMode = "lagg"
		// 获取从网卡数量
		cmd = exec.Command("ifconfig", name)
		output, err = cmd.Output()
		if err == nil {
			nicInfo.BondSlaves = hc.countLaggSlaves(string(output))
		}
	}
}

// parseIfconfigDetailOutput 解析 ifconfig 详细输出
func (hc *HardwareChecker) parseIfconfigDetailOutput(output string, nicInfo *NICInfo) {
	for _, line := range strings.Split(output, "\n") {
		// 获取 MAC 地址
		if strings.Contains(line, "ether") {
			for _, field := range strings.Fields(line) {
				if strings.Count(field, ":") == 5 {
					nicInfo.MACAddress = strings.ToLower(field)
				}
			}
		}
		// 获取速率
		if strings.Contains(line, "media") && strings.Contains(line, "baseT") {
			for _, field := range strings.Fields(line) {
				if strings.HasSuffix(field, "baseT") {
					speed := strings.TrimSuffix(field, "baseT")
					if val, err := strconv.Atoi(speed); err == nil {
						nicInfo.Speed = val
					}
				}
			}
		}
	}
}

// getNICDriverFreeBSD 获取 FreeBSD 网卡驱动
func (hc *HardwareChecker) getNICDriverFreeBSD(name string) string {
	// 使用 sysctl 获取驱动信息
	cmd := exec.Command("sysctl", "-b", "dev."+name+".%driver")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	// 使用 pciconf
	cmd = exec.Command("pciconf", "-l", "-v")
	output, err = cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(line, name) && strings.Contains(line, "driver") {
				if idx := strings.Index(line, "driver="); idx != -1 {
					return line[idx+7:]
				}
			}
		}
	}

	return "Unknown"
}

// countLaggSlaves 计算 lagg 从网卡数量
func (hc *HardwareChecker) countLaggSlaves(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "laggproto") || strings.Contains(line, "laggport") {
			count++
		}
	}
	return count
}

// collectMemoryInfo 收集内存信息
func (hc *HardwareChecker) collectMemoryInfo() *MemoryInfo {
	if hc.OSInfo.IsFreeBSD {
		return hc.collectMemoryInfoFreeBSD()
	}
	return hc.collectMemoryInfoLinux()
}

// collectMemoryInfoLinux 收集 Linux 内存信息
func (hc *HardwareChecker) collectMemoryInfoLinux() *MemoryInfo {
	memInfo := &MemoryInfo{
		MemoryType:  "Unknown",
		MemorySpeed: 0,
		MemorySlots: 0,
	}

	// 获取总内存
	ram, err := utils.GetTotalRAM()
	if err == nil {
		memInfo.TotalMemory = ram
	}

	// ARM64 使用 Device Tree 获取内存信息
	if hc.OSInfo.Arch == "aarch64" {
		hc.collectMemoryFromDeviceTree(memInfo)
	}

	// 使用 dmidecode 获取内存详细信息（需要 root 权限）
	if utils.CommandExists("dmidecode") && utils.IsRoot() {
		output, err := exec.Command("dmidecode", "-t", "memory", "-q").Output()
		if err == nil {
			hc.parseDmidecodeMemoryOutput(string(output), memInfo)
		}
	}

	// 尝试从 /sys/devices/system/node 获取 NUMA 节点数
	hc.collectMemoryFromSysfs(memInfo)

	// 如果没有检测到内存插槽数，使用 NUMA 节点数估算
	if memInfo.MemorySlots == 0 {
		memInfo.MemorySlots = memInfo.NUMANodes
		if memInfo.MemorySlots == 0 {
			memInfo.MemorySlots = 1
		}
	}

	return memInfo
}

// collectMemoryFromDeviceTree 从 Device Tree 收集内存信息（ARM64）
func (hc *HardwareChecker) collectMemoryFromDeviceTree(memInfo *MemoryInfo) {
	// 尝试从 /proc/device-tree/memory 读取
	if data, err := os.ReadFile("/proc/device-tree/memory/device_type"); err == nil {
		if strings.Contains(string(data), "memory") {
			// 读取 reg 属性获取内存大小
			if regData, err := os.ReadFile("/proc/device-tree/memory/reg"); err == nil {
				// 解析 reg 属性（比较复杂，简化处理）
				_ = regData
			}
		}
	}

	// 尝试从 /proc/device-tree/cpus 读取 CPU 信息
	if data, err := os.ReadFile("/proc/device-tree/cpus/cpu@0/device_type"); err == nil {
		// ARM CPU 信息
		_ = data
	}

	// 从 /sys/devices/system/node 获取 NUMA 节点数
	entries, err := os.ReadDir("/sys/devices/system/node")
	if err == nil {
		count := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "node") {
				count++
			}
		}
		if count > 0 {
			memInfo.NUMANodes = count
		}
	}
}

// collectMemoryFromSysfs 从 sysfs 收集内存信息
func (hc *HardwareChecker) collectMemoryFromSysfs(memInfo *MemoryInfo) {
	// 尝试从 /sys/devices/system/node 获取 NUMA 节点数
	entries, err := os.ReadDir("/sys/devices/system/node")
	if err == nil {
		count := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "node") {
				count++
			}
		}
		if count > 0 {
			memInfo.NUMANodes = count
		}
	}

	// 尝试从 /sys/firmware/dmi/entries 读取内存类型
	if data, err := os.ReadFile("/sys/firmware/dmi/entries/17-0"); err == nil {
		content := string(data)
		if strings.Contains(content, "DDR4") {
			memInfo.MemoryType = "DDR4"
		} else if strings.Contains(content, "DDR3") {
			memInfo.MemoryType = "DDR3"
		} else if strings.Contains(content, "DDR5") {
			memInfo.MemoryType = "DDR5"
		} else if strings.Contains(content, "LPDDR4") {
			memInfo.MemoryType = "LPDDR4"
		} else if strings.Contains(content, "LPDDR5") {
			memInfo.MemoryType = "LPDDR5"
		}
	}
}

// parseDmidecodeMemoryOutput 解析 dmidecode 内存输出
func (hc *HardwareChecker) parseDmidecodeMemoryOutput(output string, memInfo *MemoryInfo) {
	lines := strings.Split(output, "\n")
	slotCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Type:") {
			memType := strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
			if memType != "" && memType != "Unknown" {
				// 处理 "DDR4", "LPDDR4", "LPDDR4X" 等
				if strings.Contains(memType, "DDR5") {
					memInfo.MemoryType = "DDR5"
				} else if strings.Contains(memType, "DDR4") {
					memInfo.MemoryType = "DDR4"
				} else if strings.Contains(memType, "DDR3") {
					memInfo.MemoryType = "DDR3"
				} else if strings.Contains(memType, "LPDDR5") {
					memInfo.MemoryType = "LPDDR5"
				} else if strings.Contains(memType, "LPDDR4") {
					memInfo.MemoryType = "LPDDR4"
				} else {
					memInfo.MemoryType = memType
				}
			}
		}
		if strings.HasPrefix(line, "Speed:") {
			if speed, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Speed:"))); err == nil {
				if speed > memInfo.MemorySpeed {
					memInfo.MemorySpeed = speed
				}
			}
		}
		if strings.Contains(line, "Memory Device") && !strings.Contains(line, "No Module Installed") {
			slotCount++
		}
	}
	memInfo.MemorySlots = slotCount
}

// collectMemoryInfoFreeBSD 收集 FreeBSD 内存信息
func (hc *HardwareChecker) collectMemoryInfoFreeBSD() *MemoryInfo {
	memInfo := &MemoryInfo{
		MemoryType:  "Unknown",
		MemorySpeed: 0,
		MemorySlots: 0,
	}

	// 获取总内存
	cmd := exec.Command("sysctl", "hw.physmem")
	output, err := cmd.Output()
	if err == nil {
		line := strings.TrimSpace(string(output))
		if idx := strings.Index(line, ":"); idx != -1 {
			physmem, _ := strconv.ParseUint(strings.TrimSpace(line[idx+1:]), 10, 64)
			memInfo.TotalMemory = physmem
		}
	}

	// 使用 dmidecode（如果可用）
	if utils.CommandExists("dmidecode") {
		cmd = exec.Command("dmidecode", "-t", "memory", "-q")
		output, err = cmd.Output()
		if err == nil {
			hc.parseDmidecodeMemoryOutput(string(output), memInfo)
		}
	}

	// 如果没有检测到内存插槽数，设置为 1
	if memInfo.MemorySlots == 0 {
		memInfo.MemorySlots = 1
	}

	return memInfo
}
