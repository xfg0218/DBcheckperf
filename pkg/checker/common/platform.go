// Package common 提供公共辅助函数
// 包含平台检测、服务器厂家识别等多架构兼容功能
package common

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ServerPlatformInfo 服务器平台信息
type ServerPlatformInfo struct {
	// OSType OS 类型（物理机/虚拟机/超融合）
	OSType string
	// ServerVendor 服务器厂家
	ServerVendor string
	// ServerModel 服务器型号
	ServerModel string
	// ServerSeries 服务器系列
	ServerSeries string
	// Architecture 架构（x86_64/aarch64）
	Architecture string
}

// GetServerPlatformInfo 获取服务器平台信息（多架构兼容）
func GetServerPlatformInfo() *ServerPlatformInfo {
	info := &ServerPlatformInfo{
		Architecture: getArchitecture(),
		OSType:       detectOSType(),
		ServerVendor: detectServerVendor(),
		ServerModel:  detectServerModel(),
	}

	return info
}

// getArchitecture 获取系统架构
func getArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "x86"
	case "arm":
		return "arm"
	default:
		return runtime.GOARCH
	}
}

// detectOSType 检测 OS 类型（物理机/虚拟机/超融合）
func detectOSType() string {
	// 方法 1: 使用 systemd-detect-virt
	cmd := exec.Command("systemd-detect-virt")
	output, err := cmd.Output()
	if err == nil {
		virtType := strings.TrimSpace(string(output))
		switch virtType {
		case "none":
			// 检测是否为超融合
			if detectHyperconverged() {
				return "超融合"
			}
			return "物理机"
		case "kvm", "qemu", "amazon", "microsoft", "oracle", "vmware", "xen", "bochs", "uml", "docker", "lxc", "openvz", "vzlinux":
			return "虚拟机"
		default:
			if virtType != "" {
				return "虚拟机"
			}
		}
	}

	// 方法 2: 检查 /sys/class/dmi/id/product_name
	data, err := os.ReadFile("/sys/class/dmi/id/product_name")
	if err == nil {
		product := strings.ToLower(strings.TrimSpace(string(data)))
		if isVirtualProduct(product) {
			return "虚拟机"
		}
	}

	// 方法 3: 检查 virtio 设备
	if _, err := os.Stat("/sys/bus/virtio/drivers"); err == nil {
		return "虚拟机"
	}

	// 检测是否为超融合
	if detectHyperconverged() {
		return "超融合"
	}

	return "物理机"
}

// detectHyperconverged 检测是否为超融合架构
func detectHyperconverged() bool {
	// 检查 Nutanix
	if data, err := os.ReadFile("/sys/class/dmi/id/sys_vendor"); err == nil {
		vendor := strings.ToLower(string(data))
		if strings.Contains(vendor, "nutanix") {
			return true
		}
	}

	// 检查 VMware vSAN
	cmd := exec.Command("esxcli", "storage", "vsan", "get")
	if err := cmd.Run(); err == nil {
		return true
	}

	// 检查 SmartX
	if data, err := os.ReadFile("/proc/cmdline"); err == nil {
		cmdline := string(data)
		if strings.Contains(cmdline, "smartx") || strings.Contains(cmdline, "smtx") {
			return true
		}
	}

	return false
}

// isVirtualProduct 检查产品名称是否为虚拟机
func isVirtualProduct(product string) bool {
	virtualKeywords := []string{
		"virtual machine", "virtualbox", "vmware", "qemu", "kvm",
		"xen", "hyperv", "hyper-v", "parallels", "openstack",
		"cloud", "ecs", "aws", "azure", "google cloud",
	}

	for _, keyword := range virtualKeywords {
		if strings.Contains(product, keyword) {
			return true
		}
	}

	return false
}

// detectServerVendor 检测服务器厂家（多架构兼容）
func detectServerVendor() string {
	var vendor string

	// 方法 1: 读取 /sys/class/dmi/id/sys_vendor（x86 和 ARM 通用）
	if data, err := os.ReadFile("/sys/class/dmi/id/sys_vendor"); err == nil {
		vendor = strings.TrimSpace(string(data))
	}

	// 方法 2: 如果方法 1 未获取到，尝试 /sys/class/dmi/id/product_name
	if vendor == "" {
		if data, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
			vendor = strings.TrimSpace(string(data))
		}
	}

	// 方法 3: ARM 设备树检测
	if vendor == "" {
		if data, err := os.ReadFile("/proc/device-tree/model"); err == nil {
			vendor = strings.TrimSpace(string(data))
		}
	}

	// 方法 4: 使用 dmidecode 命令（需要 root 权限）
	if vendor == "" {
		cmd := exec.Command("dmidecode", "-s", "system-manufacturer")
		output, err := cmd.Output()
		if err == nil {
			vendor = strings.TrimSpace(string(output))
		}
	}

	// 标准化厂家名称
	return normalizeVendorName(vendor)
}

// detectServerModel 检测服务器型号
func detectServerModel() string {
	var model string

	// 方法 1: 读取 /sys/class/dmi/id/product_name
	if data, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
		model = strings.TrimSpace(string(data))
	}

	// 方法 2: ARM 设备树检测
	if model == "" {
		if data, err := os.ReadFile("/proc/device-tree/model"); err == nil {
			model = strings.TrimSpace(string(data))
		}
	}

	// 方法 3: 使用 dmidecode 命令
	if model == "" {
		cmd := exec.Command("dmidecode", "-s", "product-name")
		output, err := cmd.Output()
		if err == nil {
			model = strings.TrimSpace(string(output))
		}
	}

	// 方法 4: 使用 lscpu 命令
	if model == "" {
		cmd := exec.Command("lscpu")
		output, err := cmd.Output()
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "Model:") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						model = strings.TrimSpace(parts[1])
						break
					}
				}
			}
		}
	}

	if model == "" {
		model = "Unknown"
	}

	return model
}

// normalizeVendorName 标准化厂家名称
func normalizeVendorName(vendor string) string {
	if vendor == "" {
		return "未知"
	}

	vendorLower := strings.ToLower(vendor)

	// 服务器厂家关键词映射
	vendorMap := map[string][]string{
		"DELL":            {"dell", "poweredge"},
		"浪潮":             {"inspur", "浪潮", "inspur"},
		"联想":             {"lenovo", "thinksystem", "thinkcentre"},
		"华为":             {"huawei", "kunpeng", "taishan"},
		"新华三":           {"h3c", "new h3c"},
		"曙光":             {"sugon", "dawning", "中科曙光"},
		"超微":             {"supermicro", "super micro"},
		"HPE":             {"hp", "hpe", "proliant", "hp enterprise"},
		"Fujitsu":         {"fujitsu", "primergy", "primetower"},
		"Cisco":           {"cisco", "ucs"},
		"阿里巴巴":         {"alibaba", "aliyun", "alibaba cloud"},
		"腾讯":             {"tencent", "cloud", "tencent cloud"},
		"百度":             {"baidu", "cloud", "baidu cloud"},
		"Amazon":          {"amazon", "aws", "ec2"},
		"Microsoft":       {"microsoft", "azure"},
		"Google":          {"google", "cloud", "gcp"},
		"Oracle":          {"oracle", "cloud"},
		"Ampere":          {"ampere", "altra"},
		"Phytium":         {"phytium", "飞腾"},
		"Hygon":           {"hygon", "海光"},
		"Apple":           {"apple", "mac"},
		"Raspberry Pi":    {"raspberry", "pi"},
		"NVIDIA":          {"nvidia", "jetson"},
		"Qualcomm":        {"qualcomm", "snapdragon"},
	}

	for standardName, keywords := range vendorMap {
		for _, keyword := range keywords {
			if strings.Contains(vendorLower, keyword) {
				return standardName
			}
		}
	}

	// 如果没有匹配到，返回原始值（首字母大写）
	if len(vendor) > 0 {
		return strings.ToUpper(vendor[:1]) + vendor[1:]
	}

	return vendor
}

// GetOSInfo 获取操作系统信息（简化版）
func GetOSInfo() (name, version, arch string) {
	arch = getArchitecture()

	// 尝试读取 /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				pretty := strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				// 从 PRETTY_NAME 提取名称和版本
				parts := strings.SplitN(pretty, " ", 2)
				if len(parts) > 0 {
					name = parts[0]
				}
				if len(parts) > 1 {
					version = parts[1]
				}
				return
			}
		}
	}

	// 尝试读取 /etc/redhat-release
	data, err = os.ReadFile("/etc/redhat-release")
	if err == nil {
		line := strings.TrimSpace(string(data))
		name = "Linux"
		version = line
		return
	}

	// 使用 uname 命令
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err == nil {
		name = "Linux"
		version = strings.TrimSpace(string(output))
		return
	}

	name = "Linux"
	version = "Unknown"
	return
}
