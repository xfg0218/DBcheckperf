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
	"syscall"
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
	// BlockSize 磁盘块大小（字节）
	BlockSize int64
	// Scheduler 磁盘调度算法
	Scheduler string
	// DiskType 磁盘类型 (HDD/SSD/NVMe)
	DiskType string
	// DiskModel 磁盘型号
	DiskModel string
	// DiskCapacity 磁盘容量（字节）
	DiskCapacity uint64
	// FileSystem 文件系统类型
	FileSystem string
	// MountOptions 挂载选项
	MountOptions string
	// FreeSpace 可用空间（字节）
	FreeSpace uint64
	// InodeUsage inode 使用率 (%)
	InodeUsage float64
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
	// UseDirectIO 是否使用直接 IO（oflag=direct）
	UseDirectIO bool
	// UseFsync 是否使用 fsync（conv=fsync）
	UseFsync bool
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
		Iterations:    1,           // 仅测试一次
		Verbose:       verbose,
		RandBlockSize: randBlockSizeKB,
		UseDirectIO:   true,        // 默认使用直接 IO
		UseFsync:      true,        // 默认使用 fsync
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

	// 收集磁盘信息（块大小和调度算法）
	device, blockSize, scheduler := CollectDiskInfo(dir)
	result.BlockSize = blockSize
	result.Scheduler = scheduler

	if dc.Verbose {
		fmt.Printf("磁盘设备：%s, 块大小：%d 字节，调度算法：%s\n", device, blockSize, scheduler)
	}

	// 计算块大小和计数
	ioBlockSize := dc.BlockSize * 1024 // 转换为字节
	count := int(testSize / uint64(ioBlockSize))
	if count == 0 {
		count = 1
	}

	// 生成测试文件名
	testFile := filepath.Join(dir, fmt.Sprintf("dd_test_%d", time.Now().UnixNano()))

	if dc.Verbose {
		fmt.Printf("磁盘测试：%s\n", testFile)
	}

	// 确保测试完成后清理文件
	defer func() {
		if dc.Verbose {
			fmt.Printf("DEBUG: 清理测试文件：%s\n", testFile)
		}
		if err := os.Remove(testFile); err != nil {
			// 如果文件已经不存在，忽略错误
			if !os.IsNotExist(err) {
				fmt.Printf("警告：清理测试文件失败：%v\n", err)
			}
		} else if dc.Verbose {
			fmt.Printf("DEBUG: 测试文件已清理：%s\n", testFile)
		}
	}()

	// 多次迭代测试，取平均值
	var writeTimes []float64
	var readTimes []float64
	var writeBytesList []uint64
	var readBytesList []uint64

	for iter := 0; iter < dc.Iterations; iter++ {
		if dc.Verbose && dc.Iterations > 1 {
			fmt.Printf("磁盘测试迭代 %d/%d\n", iter+1, dc.Iterations)
		}

		// 每次迭代重新创建测试文件
		currentTestFile := testFile
		if dc.Iterations > 1 {
			currentTestFile = fmt.Sprintf("%s_%d", testFile, iter)
		}

		// 执行写入测试
		writeBytes, writeTime, err := dc.runWriteTest(currentTestFile, ioBlockSize, count)
		if err == nil {
			writeTimes = append(writeTimes, writeTime)
			writeBytesList = append(writeBytesList, writeBytes)

			// 清空缓存，确保读取测试准确
			dc.dropCaches()

			// 执行读取测试
			readBytes, readTime, err := dc.runReadTest(currentTestFile, ioBlockSize)
			if err == nil {
				readTimes = append(readTimes, readTime)
				readBytesList = append(readBytesList, readBytes)
			}

			// 清理测试文件
			os.Remove(currentTestFile)
		}
	}

	// 计算平均值
	var avgWriteTime, avgReadTime float64
	var avgWriteBytes, avgReadBytes uint64

	if len(writeTimes) > 0 {
		avgWriteTime = utils.Average(writeTimes)
		avgWriteBytes = uint64(utils.AverageUint64(writeBytesList))
	}
	if len(readTimes) > 0 {
		avgReadTime = utils.Average(readTimes)
		avgReadBytes = uint64(utils.AverageUint64(readBytesList))
	}

	// 设置顺序读写结果（使用平均值）
	result.WriteTime = avgWriteTime
	result.WriteBytes = avgWriteBytes
	if avgWriteTime > 0 {
		result.WriteBandwidth = float64(avgWriteBytes) / avgWriteTime / (1024 * 1024)
	}

	if avgReadTime > 0 {
		result.ReadTime = avgReadTime
		result.ReadBytes = avgReadBytes
		result.ReadBandwidth = float64(avgReadBytes) / avgReadTime / (1024 * 1024)
	}

	// 执行随机读写测试
	var err error
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

	// 收集远程磁盘信息（块大小和调度算法）
	device, blockSize, scheduler := CollectRemoteDiskInfo(host, dir)
	result.BlockSize = blockSize
	result.Scheduler = scheduler

	if dc.Verbose {
		fmt.Printf("远程磁盘设备：%s, 块大小：%d 字节，调度算法：%s\n", device, blockSize, scheduler)
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
	ioBlockSize := dc.BlockSize * 1024 // 转换为字节
	count := int(testSize / uint64(ioBlockSize))
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
		if dc.Verbose {
			fmt.Printf("DEBUG: 清理远程测试文件：%s@%s\n", testFile, host)
		}
		// 使用更可靠的清理命令，验证清理结果
		cleanupCmd := fmt.Sprintf("rm -f %s && echo 'CLEANED' || echo 'FAILED'", testFile)
		cleanupOutput, _ := dc.runSSHCommand(host, cleanupCmd)
		if dc.Verbose {
			if strings.Contains(cleanupOutput, "CLEANED") {
				fmt.Printf("DEBUG: 远程测试文件已清理：%s\n", testFile)
			} else {
				fmt.Printf("DEBUG: 远程测试文件清理结果：%s\n", cleanupOutput)
			}
		}
	}()

	// 执行写入测试（使用 time 命令包装 dd，获取 dd 内部时间）
	writeCmd := fmt.Sprintf("dd if=/dev/zero of=%s bs=%d count=%d oflag=direct conv=fsync 2>&1",
		testFile, ioBlockSize, count)
	writeOutput, _, err := dc.runSSHCommandWithTime(host, writeCmd)
	var writeBytes uint64
	var writeTime float64
	if err == nil {
		writeBytes, writeTime = dc.parseDDOutputAndTime(writeOutput)
		if writeBytes == 0 {
			writeBytes = uint64(ioBlockSize * count)
		}
	}

	// 如果 dd 输出中没有解析到时间，使用 SSH 命令执行时间作为备用
	if writeTime <= 0 {
		_, writeTime, _ = dc.runSSHCommandWithTime(host, writeCmd)
	}

	// 清空远程缓存
	dc.dropRemoteCaches(host)

	// 执行读取测试（使用 time 命令包装 dd，获取 dd 内部时间）
	readCmd := fmt.Sprintf("dd if=%s of=/dev/null bs=%d iflag=direct 2>&1", testFile, ioBlockSize)
	readOutput, _, err := dc.runSSHCommandWithTime(host, readCmd)
	var readBytes uint64
	var readTime float64
	if err == nil {
		readBytes, readTime = dc.parseDDOutputAndTime(readOutput)
		if readBytes == 0 {
			readBytes = writeBytes
		}
	}

	// 如果 dd 输出中没有解析到时间，使用 SSH 命令执行时间作为备用
	if readTime <= 0 {
		_, readTime, _ = dc.runSSHCommandWithTime(host, readCmd)
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

// runSSHCommandWithAuth 通过 SSH 密码认证执行远程命令并返回输出
func (dc *DiskChecker) runSSHCommandWithAuth(hostname, username, password string, port int, command string) (string, error) {
	return common.RunSSHCommandWithAuth(hostname, username, password, port, command)
}

// runSSHCommandWithTime 通过 SSH 执行远程命令并返回输出和执行时间
func (dc *DiskChecker) runSSHCommandWithTime(host string, command string) (string, float64, error) {
	start := time.Now()
	output, err := dc.runSSHCommand(host, command)
	duration := time.Since(start).Seconds()
	return output, duration, err
}

// runSSHCommandWithAuthAndTime 通过 SSH 密码认证执行远程命令并返回输出和执行时间
func (dc *DiskChecker) runSSHCommandWithAuthAndTime(hostname, username, password string, port int, command string) (string, float64, error) {
	start := time.Now()
	output, err := dc.runSSHCommandWithAuth(hostname, username, password, port, command)
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

// dropRemoteCachesWithAuth 清空远程主机系统缓存（密码认证）
func (dc *DiskChecker) dropRemoteCachesWithAuth(hostname, username, password string, port int) {
	// 尝试清空页面缓存、dentries 和 inodes
	dc.runSSHCommandWithAuth(hostname, username, password, port, "sync; echo 3 > /proc/sys/vm/drop_caches 2>/dev/null || true")
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

// runRemoteRandomTestWithAuth 执行远程随机读写测试（密码认证）
func (dc *DiskChecker) runRemoteRandomTestWithAuth(hostname, username, password string, port int, testFile string) (uint64, float64, uint64, float64, error) {
	// 获取文件大小
	sizeOutput, err := dc.runSSHCommandWithAuth(hostname, username, password, port, fmt.Sprintf("stat -c %%s %s 2>/dev/null || stat -f %%z %s 2>/dev/null", testFile, testFile))
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("无法获取远程测试文件大小：%v", err)
	}
	fileSize, _ := strconv.ParseUint(strings.TrimSpace(sizeOutput), 10, 64)

	// 随机读写块大小
	randBlockSize := dc.RandBlockSize * 1024 // 转换为字节
	randCount := 100                         // 随机操作次数（优化为 100 次，快速完成）

	// 随机写入测试
	randWriteStart := time.Now()
	randWriteBytes, err := dc.runRemoteRandomWriteWithAuth(hostname, username, password, port, testFile, randBlockSize, randCount, fileSize)
	randWriteDuration := time.Since(randWriteStart).Seconds()

	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("远程随机写入测试失败：%v", err)
	}

	// 清空远程缓存
	dc.dropRemoteCachesWithAuth(hostname, username, password, port)

	// 随机读取测试
	randReadStart := time.Now()
	randReadBytes, err := dc.runRemoteRandomReadWithAuth(hostname, username, password, port, testFile, randBlockSize, randCount, fileSize)
	randReadDuration := time.Since(randReadStart).Seconds()

	if err != nil {
		return randWriteBytes, randWriteDuration, 0, 0, fmt.Errorf("远程随机读取测试失败：%v", err)
	}

	return randWriteBytes, randWriteDuration, randReadBytes, randReadDuration, nil
}

// runRemoteRandomWriteWithAuth 执行远程随机写入测试（密码认证）
func (dc *DiskChecker) runRemoteRandomWriteWithAuth(hostname, username, password string, port int, testFile string, blockSize, count int, fileSize uint64) (uint64, error) {
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
		_, err := dc.runSSHCommandWithAuth(hostname, username, password, port, cmd)
		if err == nil {
			totalBytes += uint64(blockSize)
		}
	}

	return totalBytes, nil
}

// runRemoteRandomReadWithAuth 执行远程随机读取测试（密码认证）
func (dc *DiskChecker) runRemoteRandomReadWithAuth(hostname, username, password string, port int, testFile string, blockSize, count int, fileSize uint64) (uint64, error) {
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
		_, err := dc.runSSHCommandWithAuth(hostname, username, password, port, cmd)
		if err == nil {
			totalBytes += uint64(blockSize)
		}
	}

	return totalBytes, nil
}

// RunRemoteWithAuth 通过 SSH 密码认证在远程主机执行磁盘 I/O 测试
func (dc *DiskChecker) RunRemoteWithAuth(hostname, username, password string, port int, dir string) (*DiskResult, error) {
	// 将主机名解析为 IP 地址
	resolvedHost := common.ResolveToIP(hostname)

	result := &DiskResult{
		Host: resolvedHost,
		Dir:  dir,
	}

	// 收集远程磁盘信息（块大小和调度算法）
	device, blockSize, scheduler := CollectRemoteDiskInfoWithAuth(hostname, username, password, port, dir)
	result.BlockSize = blockSize
	result.Scheduler = scheduler

	if dc.Verbose {
		fmt.Printf("远程磁盘设备：%s, 块大小：%d 字节，调度算法：%s\n", device, blockSize, scheduler)
	}

	// 计算测试文件大小
	testSize := dc.FileSize
	if testSize == 0 {
		// 默认使用 2 倍 RAM，远程主机需要通过 SSH 获取
		ramStr, err := dc.runSSHCommandWithAuth(hostname, username, password, port, "free -b | grep Mem | awk '{print $2}'")
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
	ioBlockSize := dc.BlockSize * 1024 // 转换为字节
	count := int(testSize / uint64(ioBlockSize))
	if count == 0 {
		count = 1
	}

	// 生成测试文件名
	testFile := path.Join(dir, fmt.Sprintf("dd_test_%d", time.Now().UnixNano()))

	if dc.Verbose {
		fmt.Printf("远程磁盘测试：%s@%s\n", testFile, hostname)
	}

	// 确保测试完成后清理文件
	defer func() {
		if dc.Verbose {
			fmt.Printf("DEBUG: 清理远程测试文件（密码认证）：%s@%s\n", testFile, hostname)
		}
		// 使用更可靠的清理命令，验证清理结果
		cleanupCmd := fmt.Sprintf("rm -f %s && echo 'CLEANED' || echo 'FAILED'", testFile)
		cleanupOutput, _ := dc.runSSHCommandWithAuth(hostname, username, password, port, cleanupCmd)
		if dc.Verbose {
			if strings.Contains(cleanupOutput, "CLEANED") {
				fmt.Printf("DEBUG: 远程测试文件已清理（密码认证）：%s\n", testFile)
			} else {
				fmt.Printf("DEBUG: 远程测试文件清理结果（密码认证）：%s\n", cleanupOutput)
			}
		}
	}()

	// 执行写入测试（使用 time 命令包装 dd，获取 dd 内部时间）
	writeCmd := fmt.Sprintf("dd if=/dev/zero of=%s bs=%d count=%d oflag=direct conv=fsync 2>&1",
		testFile, ioBlockSize, count)
	writeOutput, _, err := dc.runSSHCommandWithAuthAndTime(hostname, username, password, port, writeCmd)
	var writeBytes uint64
	var writeTime float64
	if err == nil {
		writeBytes, writeTime = dc.parseDDOutputAndTime(writeOutput)
		if writeBytes == 0 {
			writeBytes = uint64(ioBlockSize * count)
		}
	}

	// 如果 dd 输出中没有解析到时间，使用 SSH 命令执行时间作为备用
	if writeTime <= 0 {
		_, writeTime, _ = dc.runSSHCommandWithAuthAndTime(hostname, username, password, port, writeCmd)
	}

	// 清空远程缓存
	dc.dropRemoteCachesWithAuth(hostname, username, password, port)

	// 执行读取测试（使用 time 命令包装 dd，获取 dd 内部时间）
	readCmd := fmt.Sprintf("dd if=%s of=/dev/null bs=%d iflag=direct 2>&1", testFile, ioBlockSize)
	readOutput, _, err := dc.runSSHCommandWithAuthAndTime(hostname, username, password, port, readCmd)
	var readBytes uint64
	var readTime float64
	if err == nil {
		readBytes, readTime = dc.parseDDOutputAndTime(readOutput)
		if readBytes == 0 {
			readBytes = writeBytes
		}
	}

	// 如果 dd 输出中没有解析到时间，使用 SSH 命令执行时间作为备用
	if readTime <= 0 {
		_, readTime, _ = dc.runSSHCommandWithAuthAndTime(hostname, username, password, port, readCmd)
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
	randWriteBytes, randWriteTime, randReadBytes, randReadTime, err := dc.runRemoteRandomTestWithAuth(hostname, username, password, port, testFile)
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

// runWriteTest 执行单次写入测试
func (dc *DiskChecker) runWriteTest(testFile string, blockSize int, count int) (uint64, float64, error) {
	// 构建 dd 命令参数
	ddArgs := []string{
		"if=/dev/zero",
		fmt.Sprintf("of=%s", testFile),
		fmt.Sprintf("bs=%d", blockSize),
		fmt.Sprintf("count=%d", count),
	}
	
	// 根据配置添加可选参数
	if dc.UseDirectIO {
		ddArgs = append(ddArgs, "oflag=direct")
	}
	if dc.UseFsync {
		ddArgs = append(ddArgs, "conv=fsync")
	}

	writeCmd := exec.Command("dd", ddArgs...)
	var writeErr bytes.Buffer
	writeCmd.Stderr = &writeErr

	if err := writeCmd.Run(); err != nil {
		// 尝试降级参数（不使用 oflag=direct）
		ddArgsFallback := []string{
			"if=/dev/zero",
			fmt.Sprintf("of=%s", testFile),
			fmt.Sprintf("bs=%d", blockSize),
			fmt.Sprintf("count=%d", count),
		}
		if dc.UseFsync {
			ddArgsFallback = append(ddArgsFallback, "conv=fsync")
		}
		writeCmd = exec.Command("dd", ddArgsFallback...)
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

	// 解析 dd 输出，获取字节数和 dd 内部报告的时间
	writeBytes, writeDuration := dc.parseDDOutputAndTime(writeErr.String())

	// 如果 dd 输出中没有时间信息，使用 Go 测量的时间作为备用
	if writeDuration <= 0 {
		// 重新执行一次以测量时间（仅在 verbose 模式下）
		if dc.Verbose {
			fmt.Printf("DEBUG: dd 输出中未解析到时间，使用备用方法\n")
		}
		writeStart := time.Now()
		writeCmd.Run()
		writeDuration = time.Since(writeStart).Seconds()
	}

	// 验证写入的数据量
	expectedBytes := uint64(blockSize * count)

	// 如果无法解析 dd 输出，尝试从实际文件大小获取
	if writeBytes == 0 {
		if info, statErr := os.Stat(testFile); statErr == nil {
			writeBytes = uint64(info.Size())
			if dc.Verbose {
				fmt.Printf("DEBUG: dd 输出解析失败，从文件大小获取：%d 字节\n", writeBytes)
			}
		} else {
			writeBytes = expectedBytes // 如果文件大小也无法获取，使用预期值
			if dc.Verbose {
				fmt.Printf("DEBUG: 无法获取文件大小，使用预期值：%d 字节\n", writeBytes)
			}
		}
	}

	// 验证写入完整性（允许 1% 误差）
	if float64(writeBytes) < float64(expectedBytes)*0.99 {
		// 如果实际文件大小与预期差异较大，返回错误
		if info, statErr := os.Stat(testFile); statErr == nil {
			actualSize := uint64(info.Size())
			if float64(actualSize) < float64(expectedBytes)*0.99 {
				return 0, 0, fmt.Errorf("写入数据不完整：预期 %d 字节，实际 %d 字节", expectedBytes, actualSize)
			}
			// 如果文件大小正常，使用文件大小
			writeBytes = actualSize
		} else {
			return 0, 0, fmt.Errorf("写入数据不完整：预期 %d 字节，实际 %d 字节", expectedBytes, writeBytes)
		}
	}

	if dc.Verbose {
		fmt.Printf("DEBUG: 写入测试完成：%d 字节，耗时 %.2f 秒\n", writeBytes, writeDuration)
	}

	return writeBytes, writeDuration, nil
}

// runReadTest 执行单次读取测试
func (dc *DiskChecker) runReadTest(testFile string, blockSize int) (uint64, float64, error) {
	// 先确保文件存在
	if _, err := os.Stat(testFile); err != nil {
		return 0, 0, fmt.Errorf("测试文件不存在：%s", testFile)
	}

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

	// 解析 dd 输出，获取字节数和 dd 内部报告的时间
	readBytes, readDuration := dc.parseDDOutputAndTime(readErr.String())

	// 如果 dd 输出中没有时间信息，使用 Go 测量的时间作为备用
	if readDuration <= 0 {
		if dc.Verbose {
			fmt.Printf("DEBUG: dd 输出中未解析到时间，使用备用方法\n")
		}
		readStart := time.Now()
		readCmd.Run()
		readDuration = time.Since(readStart).Seconds()
	}

	if readBytes == 0 {
		// 如果无法解析，尝试从文件大小获取
		if info, statErr := os.Stat(testFile); statErr == nil {
			readBytes = uint64(info.Size())
			if dc.Verbose {
				fmt.Printf("DEBUG: dd 输出解析失败，从文件大小获取：%d 字节\n", readBytes)
			}
		} else {
			if dc.Verbose {
				fmt.Printf("DEBUG: 无法获取文件大小\n")
			}
		}
	}

	if dc.Verbose {
		fmt.Printf("DEBUG: 读取测试完成：%d 字节，耗时 %.2f 秒\n", readBytes, readDuration)
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
// 支持多种 dd 输出格式（不同 Linux 发行版可能格式不同）
func (dc *DiskChecker) parseDDOutput(output string) uint64 {
	bytes, _ := dc.parseDDOutputAndTime(output)
	return bytes
}

// parseDDOutputAndTime 解析 dd 命令输出，提取字节数和时间（秒）
// 返回：(字节数，时间（秒）)
// 支持多种 dd 输出格式（不同 Linux 发行版可能格式不同）
func (dc *DiskChecker) parseDDOutputAndTime(output string) (uint64, float64) {
	if dc.Verbose {
		fmt.Printf("DEBUG: dd 输出：%s\n", output)
	}

	var bytes uint64
	var seconds float64

	// 1. 匹配 "X bytes copied" 格式（最常见）
	re := regexp.MustCompile(`(\d+)\s*bytes`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		parsedBytes, err := strconv.ParseUint(matches[1], 10, 64)
		if err == nil {
			bytes = parsedBytes
			if dc.Verbose {
				fmt.Printf("DEBUG: 解析 dd 输出 [bytes 格式]: %d 字节\n", bytes)
			}
		}
	}

	// 2. 尝试匹配中文输出 "X 字节已复制"
	re = regexp.MustCompile(`(\d+)\s*字节`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		parsedBytes, err := strconv.ParseUint(matches[1], 10, 64)
		if err == nil {
			bytes = parsedBytes
			if dc.Verbose {
				fmt.Printf("DEBUG: 解析 dd 输出 [中文字节格式]: %d 字节\n", bytes)
			}
		}
	}

	// 3. 匹配 "X.X GB copied" 或 "X.X MB copied" 格式（某些系统使用科学计数法）
	re = regexp.MustCompile(`([\d.]+)\s*([KMGT]?B)\s+copied`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 3 {
		value, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			unit := strings.ToUpper(matches[2])
			var parsedBytes uint64
			switch unit {
			case "KB":
				parsedBytes = uint64(value * 1024)
			case "MB":
				parsedBytes = uint64(value * 1024 * 1024)
			case "GB":
				parsedBytes = uint64(value * 1024 * 1024 * 1024)
			case "TB":
				parsedBytes = uint64(value * 1024 * 1024 * 1024 * 1024)
			default:
				parsedBytes = uint64(value)
			}
			bytes = parsedBytes
			if dc.Verbose {
				fmt.Printf("DEBUG: 解析 dd 输出 [GB/MB 格式]: %d 字节 (%.2f %s)\n", bytes, value, unit)
			}
		}
	}

	// 4. 匹配 "X.X GiB copied" 格式（二进制单位）
	re = regexp.MustCompile(`([\d.]+)\s*([KMGT]?iB)\s+copied`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 3 {
		value, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			unit := strings.ToUpper(matches[2])
			var parsedBytes uint64
			switch unit {
			case "KIB":
				parsedBytes = uint64(value * 1024)
			case "MIB":
				parsedBytes = uint64(value * 1024 * 1024)
			case "GIB":
				parsedBytes = uint64(value * 1024 * 1024 * 1024)
			case "TIB":
				parsedBytes = uint64(value * 1024 * 1024 * 1024 * 1024)
			default:
				parsedBytes = uint64(value)
			}
			bytes = parsedBytes
			if dc.Verbose {
				fmt.Printf("DEBUG: 解析 dd 输出 [GiB/MiB 格式]: %d 字节 (%.2f %s)\n", bytes, value, unit)
			}
		}
	}

	// 5. 匹配 "(X.X GB, X GiB)" 格式中的字节数（括号内的详细格式）
	re = regexp.MustCompile(`\((\d+)\s*bytes`)
	matches = re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		parsedBytes, err := strconv.ParseUint(matches[1], 10, 64)
		if err == nil {
			bytes = parsedBytes
			if dc.Verbose {
				fmt.Printf("DEBUG: 解析 dd 输出 [括号 bytes 格式]: %d 字节\n", bytes)
			}
		}
	}

	// 6. 匹配时间格式："X.XX seconds", "X.XX 秒", "real XmX.XXs"
	// 6a. 匹配 "X.XX seconds" 或 "X.XX second"
	timeRe := regexp.MustCompile(`([\d.]+)\s*seconds?`)
	timeMatches := timeRe.FindStringSubmatch(output)
	if len(timeMatches) >= 2 {
		parsedTime, err := strconv.ParseFloat(timeMatches[1], 64)
		if err == nil {
			seconds = parsedTime
			if dc.Verbose {
				fmt.Printf("DEBUG: 解析 dd 输出时间 [seconds 格式]: %.2f 秒\n", seconds)
			}
		}
	}

	// 6b. 匹配中文时间 "X.XX 秒"
	timeRe = regexp.MustCompile(`([\d.]+)\s*秒`)
	timeMatches = timeRe.FindStringSubmatch(output)
	if len(timeMatches) >= 2 {
		parsedTime, err := strconv.ParseFloat(timeMatches[1], 64)
		if err == nil {
			seconds = parsedTime
			if dc.Verbose {
				fmt.Printf("DEBUG: 解析 dd 输出时间 [中文秒格式]: %.2f 秒\n", seconds)
			}
		}
	}

	// 6c. 匹配 "real XmX.XXs" 格式（time 命令输出）
	timeRe = regexp.MustCompile(`real\s+(\d+)m([\d.]+)s`)
	timeMatches = timeRe.FindStringSubmatch(output)
	if len(timeMatches) >= 3 {
		minutes, _ := strconv.ParseFloat(timeMatches[1], 64)
		secs, _ := strconv.ParseFloat(timeMatches[2], 64)
		seconds = minutes*60 + secs
		if dc.Verbose {
			fmt.Printf("DEBUG: 解析 dd 输出时间 [time 格式]: %.2f 秒\n", seconds)
		}
	}

	// 7. 匹配 "copied" 关键字，尝试从上下文中提取数字
	re = regexp.MustCompile(`copied`)
	if re.MatchString(output) {
		// 尝试提取第一个数字序列
		re = regexp.MustCompile(`(\d{6,})`)
		matches = re.FindStringSubmatch(output)
		if len(matches) >= 2 {
			parsedBytes, err := strconv.ParseUint(matches[1], 10, 64)
			if err == nil {
				bytes = parsedBytes
				if dc.Verbose {
					fmt.Printf("DEBUG: 解析 dd 输出 [copied 格式]: %d 字节\n", bytes)
				}
			}
		}
	}

	if dc.Verbose {
		fmt.Printf("DEBUG: 解析 dd 输出完成：%d 字节，%.2f 秒\n", bytes, seconds)
	}
	return bytes, seconds
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

		// 设置超时时间（5 秒），防止 dd 进程挂起
		cmd.WaitDelay = 5 * time.Second

		if err := cmd.Run(); err != nil {
			// 尝试降级参数
			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", testFile),
				fmt.Sprintf("bs=%d", blockSize), "count=1",
				fmt.Sprintf("seek=%d", offset/int64(blockSize)),
				"conv=notrunc,fsync")
			cmd.Stderr = &errBuf
			cmd.WaitDelay = 5 * time.Second
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

		// 设置超时时间（5 秒），防止 dd 进程挂起
		cmd.WaitDelay = 5 * time.Second

		if err := cmd.Run(); err != nil {
			// 尝试降级参数
			cmd = exec.Command("dd", fmt.Sprintf("if=%s", testFile), "of=/dev/null",
				fmt.Sprintf("bs=%d", blockSize), "count=1",
				fmt.Sprintf("skip=%d", offset/int64(blockSize)))
			cmd.Stderr = &errBuf
			cmd.WaitDelay = 5 * time.Second
			if err := cmd.Run(); err != nil {
				// 继续尝试，不中断测试
				continue
			}
		}
		totalBytes += blockSize
	}

	return uint64(totalBytes), nil
}

// GetDiskBlockSize 获取磁盘的物理块大小（以字节为单位）
// 通过读取 /sys/block/<device>/queue/hw_sector_size 获取
func GetDiskBlockSize(device string) int64 {
	// 尝试从 sysfs 获取硬件扇区大小
	content, err := os.ReadFile(fmt.Sprintf("/sys/block/%s/queue/hw_sector_size", device))
	if err == nil {
		size, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		if err == nil {
			return size
		}
	}

	// 尝试从 logical_block_size 获取
	content, err = os.ReadFile(fmt.Sprintf("/sys/block/%s/queue/logical_block_size", device))
	if err == nil {
		size, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		if err == nil {
			return size
		}
	}

	// 默认返回 512 字节
	return 512
}

// GetDiskScheduler 获取磁盘的 I/O 调度算法
// 返回当前使用的调度算法，如：none, mq-deadline, kyber, bfq, cfq 等
func GetDiskScheduler(device string) string {
	// 从 sysfs 读取调度算法
	content, err := os.ReadFile(fmt.Sprintf("/sys/block/%s/queue/scheduler", device))
	if err != nil {
		return "unknown"
	}

	scheduler := strings.TrimSpace(string(content))

	// 调度算法格式：[none] mq-deadline kyber bfq
	// 方括号包围的是当前使用的调度算法
	re := regexp.MustCompile(`\[([^\]]+)\]`)
	matches := re.FindStringSubmatch(scheduler)
	if len(matches) >= 2 {
		return matches[1]
	}

	// 如果没有方括号，返回第一个调度算法
	parts := strings.Fields(scheduler)
	if len(parts) > 0 {
		return parts[0]
	}

	return "unknown"
}

// GetDiskInfo 获取磁盘的详细信息
// 返回块大小和调度算法
func GetDiskInfo(device string) (int64, string) {
	blockSize := GetDiskBlockSize(device)
	scheduler := GetDiskScheduler(device)
	return blockSize, scheduler
}

// GetDiskDevice 根据测试目录获取磁盘设备名称
// 例如：/dev/sda, /dev/nvme0n1
func GetDiskDevice(path string) string {
	// 使用 df 命令获取设备
	cmd := exec.Command("df", path)
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "unknown"
	}

	// 解析 df 输出，获取设备名
	fields := strings.Fields(lines[1])
	if len(fields) < 1 {
		return "unknown"
	}

	device := fields[0]

	// 处理 /dev/mapper/ 设备
	if strings.HasPrefix(device, "/dev/mapper/") {
		// LVM 设备，返回 mapper 名称
		return device
	}

	// 处理分区设备，如 /dev/sda1 -> /dev/sda
	if strings.HasPrefix(device, "/dev/") {
		// 移除 /dev/ 前缀
		devName := strings.TrimPrefix(device, "/dev/")
		// 移除数字后缀（分区号）
		re := regexp.MustCompile(`^([a-zA-Z]+)\d+$`)
		matches := re.FindStringSubmatch(devName)
		if len(matches) >= 2 {
			return matches[1]
		}
		// 对于 nvme 设备，如 nvme0n1p1 -> nvme0n1
		re = regexp.MustCompile(`^(nvme\d+n\d+)p?\d*$`)
		matches = re.FindStringSubmatch(devName)
		if len(matches) >= 2 {
			return matches[1]
		}
		return devName
	}

	return "unknown"
}

// CollectDiskInfo 收集磁盘信息
// 自动检测测试目录所在的磁盘设备，并获取其块大小和调度算法
func CollectDiskInfo(dir string) (string, int64, string) {
	device := GetDiskDevice(dir)
	if device == "unknown" {
		return "unknown", 0, "unknown"
	}

	blockSize := GetDiskBlockSize(device)
	scheduler := GetDiskScheduler(device)

	return device, blockSize, scheduler
}

// CollectRemoteDiskInfo 收集远程主机的磁盘信息
func CollectRemoteDiskInfo(host, dir string) (string, int64, string) {
	// 获取磁盘设备
	deviceCmd := fmt.Sprintf("df %s | tail -1 | awk '{print $1}'", dir)
	deviceOutput, err := common.RunSSHCommand(host, deviceCmd)
	if err != nil {
		return "unknown", 0, "unknown"
	}

	device := strings.TrimSpace(deviceOutput)
	if device == "" || strings.HasPrefix(device, "/dev/mapper/") {
		// LVM 或其他特殊设备
		device = strings.TrimSpace(device)
		if device == "" {
			device = "unknown"
		}
	} else {
		// 处理分区设备
		devName := strings.TrimPrefix(device, "/dev/")
		re := regexp.MustCompile(`^([a-zA-Z]+)\d+$`)
		matches := re.FindStringSubmatch(devName)
		if len(matches) >= 2 {
			device = matches[1]
		}
		re = regexp.MustCompile(`^(nvme\d+n\d+)p?\d*$`)
		matches = re.FindStringSubmatch(devName)
		if len(matches) >= 2 {
			device = matches[1]
		}
	}

	// 获取块大小
	blockSizeCmd := fmt.Sprintf("cat /sys/block/%s/queue/hw_sector_size 2>/dev/null || cat /sys/block/%s/queue/logical_block_size 2>/dev/null || echo 512", device, device)
	blockSizeOutput, err := common.RunSSHCommand(host, blockSizeCmd)
	if err != nil {
		blockSizeOutput = "512"
	}
	blockSize, _ := strconv.ParseInt(strings.TrimSpace(blockSizeOutput), 10, 64)
	if blockSize == 0 {
		blockSize = 512
	}

	// 获取调度算法
	schedulerCmd := fmt.Sprintf("cat /sys/block/%s/queue/scheduler 2>/dev/null | tr -d '[]' | awk '{print $1}' || echo unknown", device)
	schedulerOutput, err := common.RunSSHCommand(host, schedulerCmd)
	if err != nil {
		schedulerOutput = "unknown"
	}
	scheduler := strings.TrimSpace(schedulerOutput)
	// 解析当前调度算法（方括号内的值）
	re := regexp.MustCompile(`\[([^\]]+)\]`)
	matches := re.FindStringSubmatch(schedulerOutput)
	if len(matches) >= 2 {
		scheduler = matches[1]
	} else {
		parts := strings.Fields(schedulerOutput)
		if len(parts) > 0 {
			scheduler = parts[0]
		}
	}

	return device, blockSize, scheduler
}

// CollectRemoteDiskInfoWithAuth 收集远程主机的磁盘信息（密码认证）
func CollectRemoteDiskInfoWithAuth(hostname, username, password string, port int, dir string) (string, int64, string) {
	// 获取磁盘设备
	deviceCmd := fmt.Sprintf("df %s | tail -1 | awk '{print $1}'", dir)
	deviceOutput, err := common.RunSSHCommandWithAuth(hostname, username, password, port, deviceCmd)
	if err != nil {
		return "unknown", 0, "unknown"
	}

	device := strings.TrimSpace(deviceOutput)
	if device == "" || strings.HasPrefix(device, "/dev/mapper/") {
		// LVM 或其他特殊设备
		device = strings.TrimSpace(device)
		if device == "" {
			device = "unknown"
		}
	} else {
		// 处理分区设备
		devName := strings.TrimPrefix(device, "/dev/")
		re := regexp.MustCompile(`^([a-zA-Z]+)\d+$`)
		matches := re.FindStringSubmatch(devName)
		if len(matches) >= 2 {
			device = matches[1]
		}
		re = regexp.MustCompile(`^(nvme\d+n\d+)p?\d*$`)
		matches = re.FindStringSubmatch(devName)
		if len(matches) >= 2 {
			device = matches[1]
		}
	}

	// 获取块大小
	blockSizeCmd := fmt.Sprintf("cat /sys/block/%s/queue/hw_sector_size 2>/dev/null || cat /sys/block/%s/queue/logical_block_size 2>/dev/null || echo 512", device, device)
	blockSizeOutput, err := common.RunSSHCommandWithAuth(hostname, username, password, port, blockSizeCmd)
	if err != nil {
		blockSizeOutput = "512"
	}
	blockSize, _ := strconv.ParseInt(strings.TrimSpace(blockSizeOutput), 10, 64)
	if blockSize == 0 {
		blockSize = 512
	}

	// 获取调度算法
	schedulerCmd := fmt.Sprintf("cat /sys/block/%s/queue/scheduler 2>/dev/null | tr -d '[]' | awk '{print $1}' || echo unknown", device)
	schedulerOutput, err := common.RunSSHCommandWithAuth(hostname, username, password, port, schedulerCmd)
	if err != nil {
		schedulerOutput = "unknown"
	}
	scheduler := strings.TrimSpace(schedulerOutput)
	// 解析当前调度算法（方括号内的值）
	re := regexp.MustCompile(`\[([^\]]+)\]`)
	matches := re.FindStringSubmatch(schedulerOutput)
	if len(matches) >= 2 {
		scheduler = matches[1]
	} else {
		parts := strings.Fields(schedulerOutput)
		if len(parts) > 0 {
			scheduler = parts[0]
		}
	}

	return device, blockSize, scheduler
}

// GetDiskType 获取磁盘类型 (HDD/SSD/NVMe)
func GetDiskType(device string) string {
	if device == "" || device == "unknown" {
		return "unknown"
	}

	// 检查是否为 NVMe 设备
	if strings.HasPrefix(device, "nvme") {
		return "NVMe"
	}

	// 检查 rotational 文件，0 表示 SSD，1 表示 HDD
	rotationalPath := fmt.Sprintf("/sys/block/%s/queue/rotational", device)
	data, err := os.ReadFile(rotationalPath)
	if err != nil {
		return "unknown"
	}

	rotational := strings.TrimSpace(string(data))
	if rotational == "0" {
		return "SSD"
	}
	return "HDD"
}

// GetDiskModel 获取磁盘型号
func GetDiskModel(device string) string {
	if device == "" || device == "unknown" {
		return "unknown"
	}

	// 尝试从 sysfs 读取型号
	modelPath := fmt.Sprintf("/sys/block/%s/device/model", device)
	data, err := os.ReadFile(modelPath)
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// 尝试使用 hdparm 获取型号
	cmd := exec.Command("hdparm", "-I", "/dev/"+device)
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Model Number:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) >= 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// 尝试使用 smartctl 获取型号
	cmd = exec.Command("smartctl", "-i", "/dev/"+device)
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Device Model:") || strings.Contains(line, "Model Number:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) >= 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	return "unknown"
}

// GetDiskCapacity 获取磁盘容量（字节）
func GetDiskCapacity(device string) uint64 {
	if device == "" || device == "unknown" {
		return 0
	}

	// 从 sysfs 读取大小（扇区数）
	sizePath := fmt.Sprintf("/sys/block/%s/size", device)
	data, err := os.ReadFile(sizePath)
	if err != nil {
		return 0
	}

	sectors, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}

	// 获取扇区大小（字节）
	sectorSizePath := fmt.Sprintf("/sys/block/%s/queue/hw_sector_size", device)
	data, err = os.ReadFile(sectorSizePath)
	if err != nil {
		sectorSizePath = fmt.Sprintf("/sys/block/%s/queue/logical_block_size", device)
		data, err = os.ReadFile(sectorSizePath)
	}
	if err != nil {
		return 0
	}

	sectorSize, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		sectorSize = 512 // 默认扇区大小
	}

	return sectors * sectorSize
}

// GetFileSystemType 获取文件系统类型
func GetFileSystemType(path string) string {
	// 使用 stat -f 命令
	cmd := exec.Command("stat", "-f", "-c", "%T", path)
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	// 使用 df -T 命令
	cmd = exec.Command("df", "-T", path)
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}

	return "unknown"
}

// GetMountOptions 获取挂载选项
func GetMountOptions(path string) string {
	// 读取 /proc/mounts
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return "unknown"
	}

	// 查找匹配的挂载点
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "unknown"
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			mountPoint := fields[1]
			if mountPoint == "/" && absPath == "/" {
				return fields[3]
			}
			if strings.HasPrefix(absPath, mountPoint+"/") || absPath == mountPoint {
				return fields[3]
			}
		}
	}

	return "unknown"
}

// GetDiskFreeSpace 获取磁盘可用空间（字节）
func GetDiskFreeSpace(path string) uint64 {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0
	}
	return stat.Bavail * uint64(stat.Bsize)
}

// GetInodeUsage 获取 inode 使用率 (%)
func GetInodeUsage(path string) float64 {
	cmd := exec.Command("df", "-i", path)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0
	}

	fields := strings.Fields(lines[1])
	if len(fields) >= 5 {
		usageStr := strings.TrimSuffix(fields[4], "%")
		usage, err := strconv.ParseFloat(usageStr, 64)
		if err == nil {
			return usage
		}
	}

	return 0
}

// CollectFullDiskInfo 收集完整的磁盘信息
func CollectFullDiskInfo(dir string) (device, diskType, diskModel string, diskCapacity, blockSize uint64, scheduler, fileSystem, mountOptions string, freeSpace uint64, inodeUsage float64) {
	device = GetDiskDevice(dir)
	if device == "" || device == "unknown" {
		return "unknown", "unknown", "unknown", 0, 0, "unknown", "unknown", "unknown", 0, 0
	}

	diskType = GetDiskType(device)
	diskModel = GetDiskModel(device)
	diskCapacity = GetDiskCapacity(device)
	blockSize = uint64(GetDiskBlockSize(device))
	scheduler = GetDiskScheduler(device)
	fileSystem = GetFileSystemType(dir)
	mountOptions = GetMountOptions(dir)
	freeSpace = GetDiskFreeSpace(dir)
	inodeUsage = GetInodeUsage(dir)

	return device, diskType, diskModel, diskCapacity, blockSize, scheduler, fileSystem, mountOptions, freeSpace, inodeUsage
}

// CollectRemoteFullDiskInfo 收集远程主机完整磁盘信息
func CollectRemoteFullDiskInfo(host, dir string) (device, diskType, diskModel string, diskCapacity, blockSize uint64, scheduler, fileSystem, mountOptions string, freeSpace uint64, inodeUsage float64) {
	// 获取磁盘设备
	deviceCmd := fmt.Sprintf("df %s | tail -1 | awk '{print $1}'", dir)
	deviceOutput, err := common.RunSSHCommand(host, deviceCmd)
	if err != nil {
		return "unknown", "unknown", "unknown", 0, 0, "unknown", "unknown", "unknown", 0, 0
	}

	device = strings.TrimSpace(deviceOutput)
	if device == "" {
		return "unknown", "unknown", "unknown", 0, 0, "unknown", "unknown", "unknown", 0, 0
	}

	// 处理分区设备
	devName := strings.TrimPrefix(device, "/dev/")
	re := regexp.MustCompile(`^([a-zA-Z]+)\d+$`)
	matches := re.FindStringSubmatch(devName)
	if len(matches) >= 2 {
		device = matches[1]
	}

	// 获取磁盘类型
	diskTypeCmd := fmt.Sprintf("cat /sys/block/%s/queue/rotational 2>/dev/null || echo 2", device)
	diskTypeOutput, _ := common.RunSSHCommand(host, diskTypeCmd)
	diskTypeOutput = strings.TrimSpace(diskTypeOutput)
	if strings.HasPrefix(device, "nvme") {
		diskType = "NVMe"
	} else if diskTypeOutput == "0" {
		diskType = "SSD"
	} else if diskTypeOutput == "1" {
		diskType = "HDD"
	} else {
		diskType = "unknown"
	}

	// 获取磁盘型号
	modelCmd := fmt.Sprintf("cat /sys/block/%s/device/model 2>/dev/null || hdparm -I /dev/%s 2>/dev/null | grep 'Model Number' | awk -F: '{print $2}' | xargs || echo unknown", device, device)
	diskModel, _ = common.RunSSHCommand(host, modelCmd)
	diskModel = strings.TrimSpace(diskModel)

	// 获取磁盘容量
	capacityCmd := fmt.Sprintf("cat /sys/block/%s/size 2>/dev/null", device)
	sectorsStr, _ := common.RunSSHCommand(host, capacityCmd)
	sectors, _ := strconv.ParseUint(strings.TrimSpace(sectorsStr), 10, 64)

	sectorSizeCmd := fmt.Sprintf("cat /sys/block/%s/queue/hw_sector_size 2>/dev/null || echo 512", device)
	sectorSizeStr2, _ := common.RunSSHCommand(host, sectorSizeCmd)
	sectorSize, _ := strconv.ParseUint(strings.TrimSpace(sectorSizeStr2), 10, 64)
	if sectorSize == 0 {
		sectorSize = 512
	}
	diskCapacity = sectors * sectorSize

	// 获取块大小
	blockSizeCmd := fmt.Sprintf("cat /sys/block/%s/queue/logical_block_size 2>/dev/null || echo 512", device)
	blockSizeStr, _ := common.RunSSHCommand(host, blockSizeCmd)
	blockSize, _ = strconv.ParseUint(strings.TrimSpace(blockSizeStr), 10, 64)
	if blockSize == 0 {
		blockSize = 512
	}

	// 获取调度算法
	schedulerCmd := fmt.Sprintf("cat /sys/block/%s/queue/scheduler 2>/dev/null | tr -d '[]' | awk '{print $1}' || echo unknown", device)
	schedulerOutput, _ := common.RunSSHCommand(host, schedulerCmd)
	scheduler = strings.TrimSpace(schedulerOutput)
	re = regexp.MustCompile(`\[([^\]]+)\]`)
	matches = re.FindStringSubmatch(schedulerOutput)
	if len(matches) >= 2 {
		scheduler = matches[1]
	}

	// 获取文件系统类型
	fsCmd := fmt.Sprintf("df -T %s | tail -1 | awk '{print $2}'", dir)
	fileSystem, _ = common.RunSSHCommand(host, fsCmd)
	fileSystem = strings.TrimSpace(fileSystem)

	// 获取挂载选项
	mountCmd := fmt.Sprintf("df %s | tail -1 | awk '{print $6}'", dir)
	mountOptions, _ = common.RunSSHCommand(host, mountCmd)
	mountOptions = strings.TrimSpace(mountOptions)

	// 获取可用空间
	freeCmd := fmt.Sprintf("df -B1 %s | tail -1 | awk '{print $4}'", dir)
	freeStr2, _ := common.RunSSHCommand(host, freeCmd)
	freeSpace, _ = strconv.ParseUint(strings.TrimSpace(freeStr2), 10, 64)

	// 获取 inode 使用率
	inodeCmd := fmt.Sprintf("df -i %s | tail -1 | awk '{print $5}' | tr -d '%%'", dir)
	inodeStr2, _ := common.RunSSHCommand(host, inodeCmd)
	inodeUsage, _ = strconv.ParseFloat(strings.TrimSpace(inodeStr2), 64)

	return device, diskType, diskModel, diskCapacity, blockSize, scheduler, fileSystem, mountOptions, freeSpace, inodeUsage
}
