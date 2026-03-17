// dbcheckperf - 数据库性能检查工具
// 基于 Greenplum gpcheckperf 工具的功能实现
// 用于测试磁盘 I/O、内存带宽和网络性能
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"dbcheckperf/config"
	"dbcheckperf/pkg/checker"
	"dbcheckperf/pkg/reporter"
	"dbcheckperf/pkg/utils"
)

// 版本号
const Version = "1.0.0"

func main() {
	// 解析命令行参数
	cfg := parseFlags()

	// 验证配置
	if err := cfg.Validate(); err != nil {
		// fmt.Fprintf(os.Stderr, "配置错误：%v\n", err)
		printUsage()
		os.Exit(1)
	}

	// 创建报告生成器
	rep := reporter.NewReporter(cfg.DisplayPerHost, cfg.Verbose)
	// 设置块大小和随机读写块大小
	rep.SetBlockSize(cfg.BlockSize)
	rep.SetRandBlockSize(cfg.RandBlockSize)

	// 从文件读取主机列表（如果指定了文件）
	if cfg.HostFile != "" {
		hosts, err := utils.ReadHostFile(cfg.HostFile)
		if err != nil {
			rep.PrintError(fmt.Errorf("无法读取主机文件：%v", err))
			os.Exit(1)
		}
		cfg.Hosts = append(cfg.Hosts, hosts...)
	}

	// 打印开始信息
	if cfg.Verbose {
		fmt.Println("========================================")
		fmt.Println("dbcheckperf - 服务器性能测试工具")
		fmt.Println("========================================")
		fmt.Println()
	}

	// 检查是否仅运行硬件信息收集
	if hasTestType(cfg.TestTypes, config.TestHardware) && len(cfg.TestTypes) == 1 {
		runHardwareMode(cfg, rep)
		os.Exit(0)
	}

	// 收集系统信息
	rep.PrintProgress("正在收集系统信息...")
	systemChecker := checker.NewSystemChecker()
	systemInfo, err := systemChecker.Run()
	if err != nil && cfg.Verbose {
		rep.PrintVerbose("警告：无法获取完整系统信息：%v\n", err)
	}

	var allSystemInfos []*checker.SystemInfo
	if systemInfo != nil {
		allSystemInfos = append(allSystemInfos, systemInfo)
		rep.PrintSystemInfo(allSystemInfos)
	}

	// 运行磁盘测试
	var diskResults []*checker.DiskResult
	if hasTestType(cfg.TestTypes, config.TestDisk) {
		rep.PrintProgress("正在运行磁盘 I/O 测试...")

		// 解析文件大小
		var fileSize uint64
		if cfg.FileSize == "2xRAM" || cfg.FileSize == "" {
			ram, err := utils.GetTotalRAM()
			if err != nil {
				fileSize = 2 * 1024 * 1024 * 1024 // 默认 2GB
			} else {
				fileSize = ram
			}
		} else {
			fileSize, err = utils.ParseFileSize(cfg.FileSize)
			if err != nil && cfg.Verbose {
				rep.PrintVerbose("警告：无法解析文件大小，使用默认值：%v\n", err)
				fileSize = 2 * 1024 * 1024 * 1024
			}
		}

		// 检查是否指定了多台主机
		if len(cfg.Hosts) > 0 {
			// 多主机模式：通过 SSH 在每台主机上执行磁盘测试
			rep.PrintProgress(fmt.Sprintf("在 %d 台远程主机上执行磁盘测试...", len(cfg.Hosts)))

			// 获取本地主机名
			localHost, _ := os.Hostname()
			if localHost == "" {
				localHost = "localhost"
			}

			// 对每台主机执行测试
			for _, host := range cfg.Hosts {
				// 检查是否为本地主机
				isLocal := (host == localHost || host == "localhost" || host == "127.0.0.1")

				for _, dir := range cfg.TestDirs {
					if cfg.Verbose {
						fmt.Printf("测试主机 %s，目录 %s\n", host, dir)
					}

					var result *checker.DiskResult
					var err error

					if isLocal {
						// 本地主机，直接运行
						diskChecker := checker.NewDiskChecker(cfg.BlockSize, fileSize, cfg.Verbose, cfg.RandBlockSize)
						result, err = diskChecker.Run(dir)
						// 本地测试结果也使用 IP 地址
						result.Host = checker.ResolveToIP(result.Host)
					} else {
						// 远程主机，通过 SSH 运行
						diskChecker := checker.NewDiskChecker(cfg.BlockSize, fileSize, cfg.Verbose, cfg.RandBlockSize)
						result, err = diskChecker.RunRemote(host, dir)
					}

					if err != nil {
						rep.PrintVerbose("警告：主机 %s 目录 %s 测试失败：%v\n", host, dir, err)
						continue
					}
					diskResults = append(diskResults, result)
				}
			}
		} else {
			// 单机模式：仅在本机执行测试
			// 创建磁盘检查器
			diskChecker := checker.NewDiskChecker(cfg.BlockSize, fileSize, cfg.Verbose, cfg.RandBlockSize)

			// 对每个测试目录运行测试
			for _, dir := range cfg.TestDirs {
				rep.PrintProgress(fmt.Sprintf("测试目录：%s", dir))
				result, err := diskChecker.Run(dir)
				if err != nil {
					rep.PrintError(fmt.Errorf("目录 %s 测试失败：%v", dir, err))
					continue
				}
				diskResults = append(diskResults, result)
			}
		}

		if len(diskResults) > 0 {
			writeAgg := checker.AggregateDiskResults(diskResults, true)
			readAgg := checker.AggregateDiskResults(diskResults, false)
			rep.PrintDiskResults(diskResults, writeAgg, readAgg)
		}
	}

	// 运行内存带宽测试
	var streamResults []*checker.StreamResult
	if hasTestType(cfg.TestTypes, config.TestStream) {
		rep.PrintProgress("正在运行内存带宽测试...")

		streamChecker := checker.NewStreamChecker(cfg.Verbose)
		result, err := streamChecker.Run()
		if err != nil {
			rep.PrintError(fmt.Errorf("内存带宽测试失败：%v", err))
		} else {
			// 将主机名解析为 IP 地址
			result.Host = checker.ResolveToIP(result.Host)
			streamResults = append(streamResults, result)

			// 聚合结果
			agg := &checker.AggregateResults{
				MinValue:    result.TotalBandwidth,
				MaxValue:    result.TotalBandwidth,
				AvgValue:    result.TotalBandwidth,
				MedianValue: result.TotalBandwidth,
				MinHost:     result.Host,
				MaxHost:     result.Host,
				TotalValue:  result.TotalBandwidth,
				Count:       1,
			}

			rep.PrintStreamResults(streamResults, agg)
		}
	}

	// 运行网络测试
	var networkResults []checker.NetworkResult
	networkMode := getNetworkMode(cfg.TestTypes)
	if networkMode != "" {
		rep.PrintProgress(fmt.Sprintf("正在运行网络测试（模式：%s）...", networkMode))

		// 创建网络检查器
		networkChecker := checker.NewNetworkChecker(cfg.Duration, cfg.BufferSize, cfg.UseNetperf, cfg.Verbose)

		// 获取本地主机名
		localHost, _ := os.Hostname()
		if localHost == "" {
			localHost = "localhost"
		}

		// 根据模式运行测试
		switch networkMode {
		case "n":
			// 串行模式
			for _, remote := range cfg.Hosts {
				rep.PrintProgress(fmt.Sprintf("测试 %s -> %s", localHost, remote))
				results, err := networkChecker.RunParallel(localHost, []string{remote})
				if err != nil {
					rep.PrintVerbose("警告：%s 测试失败：%v\n", remote, err)
					continue
				}
				networkResults = append(networkResults, results...)
			}
		case "N":
			// 并行模式
			if len(cfg.Hosts)%2 != 0 {
				rep.PrintVerbose("警告：并行模式建议使用偶数台主机\n")
			}
			results, err := networkChecker.RunParallel(localHost, cfg.Hosts)
			if err != nil {
				rep.PrintError(fmt.Errorf("网络测试失败：%v", err))
			} else {
				networkResults = results
			}
		case "M":
			// 全矩阵模式
			for _, remote := range cfg.Hosts {
				if remote != localHost {
					results, err := networkChecker.RunParallel(localHost, []string{remote})
					if err != nil {
						rep.PrintVerbose("警告：%s 测试失败：%v\n", remote, err)
						continue
					}
					networkResults = append(networkResults, results...)
				}
			}
		}

		if len(networkResults) > 0 {
			agg := checker.AggregateNetworkResults(networkResults)
			rep.PrintNetworkResults(networkResults, agg, networkMode)
		}
	}

	// 打印汇总报告
	if len(diskResults) > 0 || len(streamResults) > 0 || len(networkResults) > 0 {
		rep.PrintSummary(diskResults, streamResults, networkResults)
	}

	fmt.Println("测试完成。")
}

// parseFlags 解析命令行参数
// 返回配置对象
func parseFlags() *config.Config {
	cfg := config.DefaultConfig()

	// 定义命令行参数
	var testTypesStr string
	var durationStr string

	flag.StringVar(&cfg.HostFile, "f", "", "包含主机列表的文件路径")
	flag.StringVar(&testTypesStr, "r", "dsn", "测试类型：d=磁盘，s=内存流，n/N/M=网络 (串行/并行/全矩阵)")
	flag.IntVar(&cfg.BlockSize, "B", 32, "磁盘 I/O 测试块大小 (KB)")
	flag.StringVar(&cfg.FileSize, "S", "2xRAM", "磁盘 I/O 测试文件大小 (KB/MB/GB)")
	flag.BoolVar(&cfg.DisplayPerHost, "D", false, "显示每台主机的详细结果")
	flag.BoolVar(&cfg.Verbose, "v", false, "详细模式")
	flag.BoolVar(&cfg.VeryVerbose, "V", false, "非常详细模式")
	flag.StringVar(&durationStr, "duration", "15s", "网络测试持续时间")
	flag.BoolVar(&cfg.UseNetperf, "netperf", false, "使用 netperf 进行网络测试")
	flag.IntVar(&cfg.BufferSize, "buffer-size", 8, "网络测试缓冲区大小 (KB)")
	flag.IntVar(&cfg.RandBlockSize, "random-bs", 0, "随机读写块大小 (KB)，默认使用 -B 参数的值")

	// 多值参数处理
	var hosts multiStringFlag
	var testDirs multiStringFlag
	flag.Var(&hosts, "h", "主机名（可多次指定）")
	flag.Var(&testDirs, "d", "测试目录（可多次指定）")

	// 版本和帮助
	showVersion := flag.Bool("version", false, "显示版本号")
	showHelp := flag.Bool("?", false, "显示帮助信息")

	flag.Parse()

	// 处理版本
	if *showVersion {
		fmt.Printf("dbcheckperf version %s\n", Version)
		os.Exit(0)
	}

	// 处理帮助
	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	// 设置主机和目录
	cfg.Hosts = hosts
	normalizedTestDirs := make([]string, 0, len(testDirs))
	for _, dir := range testDirs {
		normalizedDir := utils.NormalizeDirPath(dir)
		if normalizedDir != "" {
			normalizedTestDirs = append(normalizedTestDirs, normalizedDir)
		}
	}
	cfg.TestDirs = normalizedTestDirs

	// 解析测试类型
	cfg.TestTypes = parseTestTypes(testTypesStr)

	// 解析持续时间
	if d, err := time.ParseDuration(durationStr); err == nil {
		cfg.Duration = d
	}

	// 如果启用了非常详细模式，同时启用详细模式
	if cfg.VeryVerbose {
		cfg.Verbose = true
	}

	return cfg
}

// parseTestTypes 解析测试类型字符串
func parseTestTypes(s string) []config.TestType {
	var types []config.TestType

	for _, c := range s {
		switch c {
		case 'd':
			types = append(types, config.TestDisk)
		case 's':
			types = append(types, config.TestStream)
		case 'n':
			types = append(types, config.TestNetworkSerial)
		case 'N':
			types = append(types, config.TestNetworkParallel)
		case 'M':
			types = append(types, config.TestNetworkMatrix)
		case 'H':
			types = append(types, config.TestHardware)
		}
	}

	if len(types) == 0 {
		return []config.TestType{config.TestDisk, config.TestStream, config.TestNetworkSerial}
	}

	return types
}

// hasTestType 检查是否包含指定的测试类型
func hasTestType(types []config.TestType, target config.TestType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

// getNetworkMode 获取网络测试模式
func getNetworkMode(types []config.TestType) string {
	for _, t := range types {
		switch t {
		case config.TestNetworkSerial:
			return "n"
		case config.TestNetworkParallel:
			return "N"
		case config.TestNetworkMatrix:
			return "M"
		}
	}
	return ""
}

// multiStringFlag 支持多次指定的字符串标志
type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// printUsage 打印使用说明
func printUsage() {
	fmt.Println(`dbcheckperf - 数据库性能检查工具

用法:
  dbcheckperf -d <测试目录> [-d <测试目录> ...]
       {-f <主机文件> | -h <主机名> [-h <主机名> ...]}
       [-r ds] [-B <块大小>] [-S <文件大小>] [-D] [-v|-V]

  dbcheckperf -d <临时目录>
       {-f <主机文件> | -h <主机名> [-h <主机名> ...]}
       [-r n|N|M [--duration <时间>]] [-D] [-v|-V] [--buffer-size <KB>]

  dbcheckperf -r H  收集硬件信息

选项:
  -B <块大小>         磁盘 I/O 测试块大小 (KB)，默认 32KB，最大 1MB
  -d <目录>           测试目录（可多次指定）
  -D                  显示每台主机的详细结果
  -f <主机文件>       包含主机列表的文件（每行一个主机）
  -h <主机名>         主机名（可多次指定）
  -r <测试类型>       测试类型：d=磁盘，s=内存流，n/N/M=网络 (串行/并行/全矩阵)，H=硬件信息，默认 dsn
  -S <文件大小>       磁盘 I/O 测试文件大小 (KB/MB/GB)，默认 2 倍 RAM
  -v                  详细模式
  -V                  非常详细模式
  --duration <时间>   网络测试持续时间 (如 15s, 1m)，默认 15 秒
  --netperf           使用 netperf 进行网络测试
  --buffer-size <KB>  网络测试缓冲区大小 (KB)，默认 8KB
  --random-bs <KB>    随机读写块大小 (KB)，默认使用 -B 参数的值
  --version           显示版本号
  -?                  显示帮助信息

示例:
  # 运行磁盘 I/O 和内存带宽测试
  dbcheckperf -f hosts.txt -d /data1 -d /data2 -r ds

  # 仅运行磁盘 I/O 测试，显示每台主机结果
  dbcheckperf -h host1 -h host2 -d /data1 -r d -D -v

  # 运行并行网络测试
  dbcheckperf -f hosts.txt -r N -d /tmp

  # 使用自定义文件大小和块大小
  dbcheckperf -h localhost -d /tmp -B 16k -S 5GB -r ds

  # 自定义随机读写块大小（使用 4KB 进行随机测试）
  dbcheckperf -h localhost -d /tmp -B 32 -S 5GB --random-bs 4 -r d

  # 收集硬件信息
  dbcheckperf -r H -v

  # 使用大块大小测试高性能 NVMe SSD
  dbcheckperf -h localhost -d /data/nvme -B 256k -S 10GB -r d -v

  # 多主机磁盘测试并显示详细输出
  dbcheckperf -h server1 -h server2 -h server3 -d /data -r d -D -v

  # 延长网络测试时间（60 秒）
  dbcheckperf -f hosts.txt -r n -d /tmp --duration 60s -v

  # 组合磁盘、内存和网络并行测试
  dbcheckperf -f hosts.txt -d /data -r dsN -D -v`)
}

// runHardwareMode 运行硬件信息收集模式
func runHardwareMode(cfg *config.Config, rep *reporter.Reporter) {
	rep.PrintProgress("正在收集硬件信息...")

	hardwareChecker := checker.NewHardwareChecker(cfg.Verbose)

	// 如果没有指定主机，仅收集本地硬件信息
	if len(cfg.Hosts) == 0 {
		hardwareInfo, err := hardwareChecker.Run()
		if err != nil {
			rep.PrintError(fmt.Errorf("硬件信息收集失败：%v", err))
			return
		}
		if hardwareInfo != nil {
			remoteInfo := &checker.RemoteHardwareInfo{
				Host:         checker.ResolveToIP(hardwareInfo.Host),
				HardwareInfo: hardwareInfo,
			}
			rep.PrintHardwareResults([]*checker.RemoteHardwareInfo{remoteInfo})
		}
		return
	}

	// 多主机模式：对每台主机执行硬件信息收集
	var remoteHardwareInfos []*checker.RemoteHardwareInfo

	// 检查是否包含本地主机
	localHost, _ := os.Hostname()
	if localHost == "" {
		localHost = "localhost"
	}

	for _, host := range cfg.Hosts {
		if cfg.Verbose {
			fmt.Printf("正在收集主机 %s 的硬件信息...\n", host)
		}

		// 如果是本地主机，直接运行本地收集
		if host == localHost || host == "localhost" || host == "127.0.0.1" {
			hardwareInfo, err := hardwareChecker.Run()
			if err != nil {
				rep.PrintVerbose("警告：主机 %s 硬件信息收集失败：%v\n", host, err)
				continue
			}
			remoteHardwareInfos = append(remoteHardwareInfos, &checker.RemoteHardwareInfo{
				Host:         checker.ResolveToIP(host),
				HardwareInfo: hardwareInfo,
			})
		} else {
			// 远程主机，使用 SSH 连接
			remoteInfo := hardwareChecker.RunRemote(host)
			if remoteInfo.Error != nil {
				rep.PrintVerbose("警告：主机 %s 硬件信息收集失败：%v\n", host, remoteInfo.Error)
				continue
			}
			remoteHardwareInfos = append(remoteHardwareInfos, remoteInfo)
		}
	}

	if len(remoteHardwareInfos) > 0 {
		rep.PrintHardwareResults(remoteHardwareInfos)
	} else {
		rep.PrintError(fmt.Errorf("未能收集到任何主机的硬件信息"))
	}
}
