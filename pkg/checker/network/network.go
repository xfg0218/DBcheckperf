// Package network 提供网络性能测试功能
// 支持多种网络测试方法：iperf3、netperf、curl、TCP 流
package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dbcheckperf/pkg/checker/common"
	"dbcheckperf/pkg/utils"
)

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

	// 将本地主机名解析为 IP
	resolvedLocal := common.ResolveToIP(localHost)

	// 为每个远程主机启动测试
	done := make(chan NetworkResult, len(remoteHosts))
	errChan := make(chan error, len(remoteHosts))

	for _, remote := range remoteHosts {
		go func(dest string) {
			// 将远程主机名解析为 IP
			resolvedDest := common.ResolveToIP(dest)
			result, err := nc.testSingle(resolvedLocal, resolvedDest)
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
// 尝试多种方法：iperf3 > curl > TCP 流 > ping 估算
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

	cmd := exec.Command("curl", "-o", "/dev/null", "-w", "%{speed_download}",
		"--connect-timeout", "5",
		"--max-time", fmt.Sprintf("%.0f", nc.Duration.Seconds()),
		testURL)
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
