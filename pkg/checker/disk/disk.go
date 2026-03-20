// Package disk 提供磁盘 I/O 测试功能
// 使用 dd 命令测试顺序读写和随机读写性能
package disk

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dbcheckperf/pkg/checker/common"
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
	// RandBlockSize 随机读写块大小（KB）
	RandBlockSize int
	// Host 远程主机名（用于远程测试）
	Host string
}

// NewDiskChecker 创建新的磁盘检查器
func NewDiskChecker(blockSizeKB int, fileSize uint64, verbose bool, randBlockSizeKB int) *DiskChecker {
	// 如果未指定随机块大小，使用顺序读写的块大小
	if randBlockSizeKB <= 0 {
		randBlockSizeKB = blockSizeKB
	}
	return &DiskChecker{
		BlockSize:     blockSizeKB,
		FileSize:      fileSize,
		Iterations:    1, // 仅测试一次
		Verbose:       verbose,
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
		Host: common.GetHostname(),
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
	testFile := filepath.Join(dir, fmt.Sprintf("dd_test_%d", time.Now().UnixNano()))

	if dc.Verbose {
		fmt.Printf("磁盘测试：%s\n", testFile)
	}

	// 确保测试完成后清理文件
	defer os.Remove(testFile)

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

	// 执行随机读写测试
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

	return result, nil
}

// RunRemote 通过 SSH 在远程主机执行磁盘 I/O 测试
func (dc *DiskChecker) RunRemote(host string, dir string) (*DiskResult, error) {
	// 将主机名解析为 IP 地址
	resolvedHost := common.ResolveToIP(host)

	result := &DiskResult{
		Host: resolvedHost,
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
	testFile := path.Join(dir, fmt.Sprintf("dd_test_%d", time.Now().UnixNano()))

	if dc.Verbose {
		fmt.Printf("远程磁盘测试：%s@%s\n", testFile, host)
	}

	// 确保测试完成后清理文件
	defer func() {
		dc.runSSHCommand(host, fmt.Sprintf("rm -f %s", testFile))
	}()

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

	// 执行随机读写测试
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

	return result, nil
}

// runSSHCommand 通过 SSH 执行远程命令并返回输出
func (dc *DiskChecker) runSSHCommand(host string, command string) (string, error) {
	return common.RunSSHCommand(host, command)
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
	randCount := 100                         // 随机操作次数（优化为 100 次，快速完成）

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
func (dc *DiskChecker) runRandomTest(testFile string) (uint64, float64, uint64, float64, error) {
	// 获取文件大小
	fileInfo, err := os.Stat(testFile)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("无法获取测试文件信息：%v", err)
	}
	fileSize := uint64(fileInfo.Size())

	// 随机读写块大小
	randBlockSize := dc.RandBlockSize * 1024 // 转换为字节
	randCount := 100                         // 随机操作次数（优化为 100 次，快速完成）

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
