// Package utils 提供通用工具函数
// 包含字节转换、时间格式化、文件操作等辅助功能
package utils

import (
	"dbcheckperf/config"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// FormatBytes 将字节数格式化为人类可读的字符串
// 例如：1024 -> "1 KB", 1048576 -> "1 MB"
func FormatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// ParseFileSize 解析文件大小字符串（支持 KB、MB、GB 后缀）
func ParseFileSize(sizeStr string) (uint64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// 处理 "2xRAM" 特殊格式
	if sizeStr == "2XRAM" {
		return GetTotalRAM()
	}

	// 使用正则表达式匹配数字和单位
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)(KB|MB|GB|TB)?$`)
	matches := re.FindStringSubmatch(sizeStr)

	if len(matches) < 2 {
		return 0, fmt.Errorf("无效的文件大小格式：%s", sizeStr)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	var multiplier uint64 = 1
	if len(matches) >= 3 && matches[2] != "" {
		switch matches[2] {
		case "KB":
			multiplier = 1024
		case "MB":
			multiplier = 1024 * 1024
		case "GB":
			multiplier = 1024 * 1024 * 1024
		case "TB":
			multiplier = 1024 * 1024 * 1024 * 1024
		}
	}

	return uint64(value * float64(multiplier)), nil
}

// GetTotalRAM 获取系统总内存（字节）
func GetTotalRAM() (uint64, error) {
	// 尝试从 /proc/meminfo 读取
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0, err
				}
				return kb * 1024, nil // 转换为字节
			}
		}
	}

	return 0, fmt.Errorf("无法获取系统内存信息")
}

// FormatBandwidth 格式化带宽值（MB/s）
func FormatBandwidth(mbPerSec float64) string {
	return fmt.Sprintf("%.2f MB/s", mbPerSec)
}

// FormatDuration 格式化持续时间
func FormatDuration(seconds float64) string {
	if seconds >= 3600 {
		return fmt.Sprintf("%.2f h", seconds/3600)
	}
	if seconds >= 60 {
		return fmt.Sprintf("%.2f m", seconds/60)
	}
	return fmt.Sprintf("%.2f s", seconds)
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists 检查目录是否存在
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// NormalizeDirPath 规范化目录路径，去除多余分隔符和末尾斜杠
func NormalizeDirPath(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	return filepath.Clean(dir)
}

// ReadHostFile 从主机文件读取主机列表
func ReadHostFile(filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var hosts []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		hosts = append(hosts, line)
	}

	return hosts, nil
}

// Min 返回两个 float64 中的较小值
func Min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个 float64 中的较大值
func Max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// Average 计算 float64 切片的平均值
func Average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// AverageUint64 计算 uint64 切片的平均值
func AverageUint64(values []uint64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum uint64
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

// Median 计算 float64 切片的中位数
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 创建排序副本
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// 简单排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// MinSlice 返回 float64 切片中的最小值
func MinSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// MaxSlice 返回 float64 切片中的最大值
func MaxSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// CreateTestFile 创建指定大小的测试文件
func CreateTestFile(filename string, size uint64) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// 使用 seek 方式快速创建大文件
	if size > 0 {
		_, err = file.Seek(int64(size-1), 0)
		if err != nil {
			return err
		}
		_, err = file.Write([]byte{0})
		if err != nil {
			return err
		}
	}

	return nil
}

// SSHAuthInfo SSH 认证信息
type SSHAuthInfo struct {
	Username string
	Password string
	Port     int
}

// ReadSSHAuthFile 从 SSH 认证文件读取主机认证信息
func ReadSSHAuthFile(filename string) (map[string]SSHAuthInfo, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	authMap := make(map[string]SSHAuthInfo)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue // 跳过格式不正确的行
		}

		hostname := fields[0]
		username := fields[1]
		password := fields[2]
		port := 22 // 默认 SSH 端口

		if len(fields) >= 4 {
			if p, err := strconv.Atoi(fields[3]); err == nil {
				port = p
			}
		}

		authMap[hostname] = SSHAuthInfo{
			Username: username,
			Password: password,
			Port:     port,
		}
	}

	return authMap, nil
}

// SSHAuthInfoToConfig 将 utils.SSHAuthInfo 转换为 config.SSHAuthInfo
func SSHAuthInfoToConfig(auth SSHAuthInfo) config.SSHAuthInfo {
	return config.SSHAuthInfo{
		Username: auth.Username,
		Password: auth.Password,
		Port:     auth.Port,
	}
}

// SSHAuthInfoMapToConfig 将 utils.SSHAuthInfo map 转换为 config.SSHAuthInfo map
func SSHAuthInfoMapToConfig(authMap map[string]SSHAuthInfo) map[string]config.SSHAuthInfo {
	result := make(map[string]config.SSHAuthInfo)
	for k, v := range authMap {
		result[k] = SSHAuthInfoToConfig(v)
	}
	return result
}
