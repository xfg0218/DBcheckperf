# dbcheckperf 项目上下文

## 项目概述

dbcheckperf 是一个用 Go 语言编写的数据库性能检查工具，基于 Greenplum gpcheckperf 工具的功能实现。它用于测试和验证主机硬件性能，包括磁盘 I/O、内存带宽和网络性能。

## 项目结构

```
dbcheckperf/
├── cmd/
│   └── main.go             # 主程序入口，CLI 命令行解析
├── go.mod                  # Go 模块定义
├── README.md               # 英文项目文档
├── README_CN.md            # 中文项目文档
├── QWEN.md                 # 本文件，AI 上下文文档
├── config/
│   └── config.go           # 配置管理，定义 Config 结构体和测试类型
└── pkg/
    ├── checker/            # 性能检查器（模块化）
    │   ├── checker.go      # 主 API 入口，类型别名
    │   ├── checker_test.go # 测试文件
    │   ├── common/         # 公共工具函数（IP 解析、SSH 命令）
    │   ├── disk/           # 磁盘 I/O 测试（顺序/随机读写 + 磁盘信息）
    │   ├── network/        # 网络性能测试 + 网络质量检测
    │   ├── memory/         # 内存带宽测试（STREAM 基准）
    │   ├── system/         # 系统信息收集（CPU/内存/磁盘/网络）
    │   ├── latency/        # 磁盘延迟和 IOPS 测试
    │   ├── iostat/         # IO 统计监控（/proc/diskstats）
    │   ├── numa/           # NUMA 信息收集
    │   └── kernel/         # 内核参数收集（VM/IO/网络）
    ├── reporter/
    │   └── reporter.go     # 报告生成器，表格格式化输出
    └── utils/
        └── utils.go        # 工具函数（字节转换、文件操作等）
```

### 模块说明

| 模块 | 文件 | 行数 | 功能描述 |
|------|------|------|----------|
| **common** | `common/common.go` | ~160 | IP 地址解析、SSH 命令执行（支持免密和密码认证）、主机名获取 |
| **disk** | `disk/disk.go` | ~1450 | 磁盘顺序读写、随机读写、磁盘类型/型号/容量/文件系统检测（支持 SSH 密码认证） |
| **network** | `network/network.go` | 680 | 网络性能测试（串行/并行/全矩阵模式）+ 网络质量检测（延迟/丢包/重传） |
| **memory** | `memory/memory.go` | 203 | 内存带宽测试（STREAM 基准） |
| **system** | `system/system.go` | 555 | 系统信息收集（CPU、内存、磁盘、虚拟化、RAID、网卡绑定等） |
| **latency** | `latency/latency.go` | 530 | 磁盘读写延迟测试、IOPS 测试（支持 dd/fio） |
| **iostat** | `iostat/iostat.go` | 455 | IO 统计监控（r/s, w/s, await, %util, avgqu-sz 等） |
| **numa** | `numa/numa.go` | 480 | NUMA 节点信息、内存分布、CPU 亲和性 |
| **kernel** | `kernel/kernel.go` | 532 | 内核参数收集（VM/IO/网络参数，支持告警检查） |
| **checker.go** | `checker.go` | 420 | 主 API 入口、类型别名、向后兼容 |
| **utils** | `utils/utils.go` | ~340 | 工具函数（字节转换、文件操作、SSH 认证文件解析等） |

### 重构和增强历史

- **2026-03-20**: SSH 密码认证功能增强
  - 新增 `-F` 命令行选项支持 SSH 认证配置文件
  - 新增 `pkg/checker/common/common.go` SSH 密码认证函数
  - 新增 `pkg/checker/disk/disk.go` 密码认证远程测试方法
  - 新增 `pkg/utils/utils.go` SSH 认证文件解析功能
  - 添加 `golang.org/x/crypto` 依赖
  - 支持无 SSH 免密登录情况下的远程主机测试

- **2026-03-20**: 模块化重构 - 将 checker.go (4,433 行) 拆分为 6 个模块
  - 主文件减少到 380 行（减少 91.4%）
  - 总代码从 4,433 行减少到 2,167 行（减少 51.1%）
  - 保持向后兼容的 API
  - 所有测试通过（11 个测试用例 + 5 个基准测试）

- **2026-03-20**: MPP 数据库关键 IO 指标增强
  - 新增延迟测试模块（latency/）
  - 新增 IO 统计模块（iostat/）
  - 新增 NUMA 模块（numa/）
  - 新增内核参数模块（kernel/）
  - 扩展磁盘信息（类型/型号/容量/文件系统/可用空间/inode）
  - 扩展网络质量测试（延迟/丢包率/错包率/TCP 重传率/MTU）
  - 新增测试类型：l(延迟/IOPS), i(IO 统计), u(NUMA), k(内核参数), q(网络质量), I(磁盘信息)

## 核心功能

### 1. 磁盘 I/O 测试 (`-r d`)
- 使用 `dd` 命令测试顺序读写性能
- 支持自定义块大小 (`-B`) 和文件大小 (`-S`)
- **测试机制**:
  - 单次测试，快速完成
  - 使用 `oflag=direct` 和 `iflag=direct` 绕过系统缓存
  - 使用 `conv=fsync` 确保数据真正写入磁盘
  - 每次读取测试前清空系统缓存 (`/proc/sys/vm/drop_caches`)
  - 验证写入数据完整性（允许 1% 误差）
  - 测试完成后自动删除测试文件
- 报告写入/读取带宽 (MB/s)

### 1.1 随机读写测试 (`--random`)
- 使用 `dd` 命令配合 `seek/skip` 参数进行随机位置读写
- 默认 4KB 块大小 (`--random-bs`)
- 执行 1000 次随机读写操作
- 报告随机写入/读取带宽 (MB/s)

### 1.2 磁盘延迟和 IOPS 测试 (`-r l`)
- 使用 `dd` 或 `fio` 命令测试磁盘读写延迟
- **测试指标**:
  - 平均读/写延迟 (ms)
  - 最大/最小读/写延迟 (ms)
  - 读/写 IOPS (次/秒)
- **测试机制**:
  - 支持 dd 和 fio 两种测试方法（fio 优先）
  - 队列深度为 1，测量单次 IO 延迟
  - 可自定义块大小（默认 4KB）
  - 支持本地和远程测试

### 1.3 磁盘详细信息 (`-r I`)
- **磁盘硬件信息**:
  - 磁盘类型（HDD/SSD/NVMe）
  - 磁盘型号和厂家
  - 磁盘容量
  - 块大小和调度算法
- **文件系统信息**:
  - 文件系统类型（ext4/xfs 等）
  - 挂载选项（noatime/nodiratime 等）
  - 可用空间
  - inode 使用率

### 2. 内存带宽测试 (`-r s`)
- 使用 STREAM 基准测试方法（Go 语言实现）
- **实际执行四种内存操作**:
  - Copy: `a[i] = b[i]`
  - Scale: `a[i] = b[i] * 3`
  - Add: `a[i] = b[i] + c[i]`
  - Triad: `a[i] = b[i] + c[i] * 3`
- **测试机制**:
  - 单次测试，快速完成
  - 使用 80MB 大数组确保测试准确性
  - 计算每种操作的带宽 (MB/s)
- 报告总带宽和各项带宽 (MB/s)

### 3. 网络性能测试 (`-r n/N/M`)
- **串行模式 (`n`)**: 逐台主机测试
- **并行模式 (`N`)**: 同时向所有主机测试
- **全矩阵模式 (`M`)**: 每台主机与其他所有主机互测
- **多种测试方法** (自动选择):
  - iperf3 (优先): 专业的网络性能测试工具
  - netperf: 另一种网络测试工具
  - curl HTTP 下载测试
  - TCP 流测试 (备用)
- **测试机制**:
  - 单次测试，快速完成
  - 设置合理的超时时间
  - 验证实际传输字节数
- 支持 netperf (`--netperf`)

### 3.1 网络质量测试 (`-r q`)
- **测试指标**:
  - 网络延迟（平均/最大/最小，ms）
  - 丢包率 (%)
  - 错包率 (%)
  - TCP 重传率 (%)
  - MTU 大小
  - 网卡队列大小
- **测试方法**:
  - ping 命令测量延迟和丢包率
  - 解析 /proc/net/netstat 获取 TCP 重传率
  - ip -s link 获取错包率
  - ip link 获取 MTU

### 4. 系统信息收集
- CPU 核心数和型号
- 内存大小
- 网卡速率
- 磁盘大小
- **虚拟机检测**: 识别 KVM、VMware、Xen、Hyper-V 等虚拟化类型
- **操作系统信息**: 操作系统名称和内核版本
- **RAID 信息**: RAID 卡型号、缓存大小、条带大小、RAID 级别
- **网络绑定**: 检测网卡绑定 (bond)、绑定模式、从网卡数量、队列大小
- **多架构支持**:
  - x86/x86_64: Intel/AMD 服务器 CPU
  - ARM/ARM64: Qualcomm/Ampere/Apple Silicon 等

### 4.1 IO 统计信息 (`-r i`)
- **实时 IO 监控指标** (从 /proc/diskstats):
  - r/s: 每秒读次数
  - w/s: 每秒写次数
  - rkB/s: 每秒读 KB 数
  - wkB/s: 每秒写 KB 数
  - await: 平均 IO 等待时间 (ms)
  - %util: 磁盘利用率
  - avgqu-sz: 平均队列长度
  - svctm: 平均服务时间 (ms)
- **告警提示**: 当 %util > 80% 时显示警告

### 4.2 NUMA 信息 (`-r u`)
- **NUMA 拓扑信息**:
  - NUMA 节点数
  - 每个节点的 CPU 核心数
  - 每个节点的内存大小和空闲内存
  - CPU 距离矩阵
- **NUMA 亲和性**:
  - 进程 CPU 亲和性
  - 中断亲和性配置
  - 设备（网卡/磁盘）NUMA 节点

### 4.3 内核参数 (`-r k`)
- **VM 参数**:
  - dirty_ratio / dirty_background_ratio
  - swappiness
  - overcommit_memory / overcommit_ratio
  - min_free_kbytes
  - transparent_hugepage
  - numa_balancing
- **IO 参数**:
  - read_ahead_kb
  - max_sectors_kb
  - nr_requests
  - scheduler
- **网络参数**:
  - somaxconn / netdev_max_backlog
  - tcp_max_syn_backlog / tcp_max_tw_buckets
  - tcp_tw_reuse / tcp_keepalive_time
- **告警检查**: 自动检测不合理的参数配置

### 4.4 SSH 密码认证支持 (`-F`)
- **功能说明**:
  - 支持在没有配置 SSH 免密登录的情况下测试远程主机
  - 通过 SSH 密码认证执行远程命令
  - 使用 golang.org/x/crypto/ssh 实现原生 SSH 客户端
- **SSH 认证文件格式**:
  ```
  # 格式：hostname username password [port]
  192.168.1.100 username password123 22
  192.168.1.101 username password456 22
  server3 root secret123 2222
  ```
- **使用示例**:
  ```bash
  # 使用 SSH 密码认证测试远程主机
  dbcheckperf -F ssh_auth.txt -d /data -r ds
  
  # 测试磁盘、内存和网络
  dbcheckperf -F ssh_auth.txt -d /tmp -r dsn -v
  ```
- **注意事项**:
  - 指定 `-F` 选项后，不需要再使用 `-f` 选项
  - 认证文件第一列为 hostname 或 IP 地址
  - 支持自定义 SSH 端口（默认为 22）
  - 密码以明文存储在认证文件中，请注意文件权限安全（建议 chmod 600）

### 5. 硬件信息收集 (`-r H`)
- **CPU 信息**:
  - CPU 型号
  - CPU 核心数
  - CPU 插槽数
  - 基准频率 (MHz)
  - 睿频 (MHz)
  - NUMA 节点数
- **磁盘信息**:
  - 设备名称 (如 sda, nvme0n1)
  - 磁盘型号
  - 磁盘厂家 (Samsung, Intel, Western Digital 等)
  - 磁盘类型 (HDD/SSD/NVMe)
  - 磁盘总大小
  - 是否旋转磁盘
- **RAID 信息**:
  - RAID 卡型号
  - RAID 缓存大小
  - 条带大小 (KB)
  - RAID 级别
  - 电池备份状态
  - 支持 megacli/storcli 工具检测
  - 支持软件 RAID (mdraid) 检测
- **网卡信息**:
  - 网卡名称
  - 网卡速率 (Mbps)
  - MTU 大小
  - 队列大小
  - 是否绑定 (bond)
  - 绑定模式
  - 绑定从网卡数量
  - 网卡驱动
  - MAC 地址
- **表格化输出**: 使用整齐的表格格式展示所有硬件信息

## 构建和运行

```bash
# 编译
go build -o dbcheckperf ./cmd/main.go

# 运行帮助
./dbcheckperf -?

# 运行测试
./dbcheckperf -h localhost -d /tmp -r ds -v
```

## 主要命令行选项

| 选项 | 描述 | 默认值 |
|------|------|--------|
| `-d <目录>` | 测试目录（可多次指定） | 必需 |
| `-f <主机文件>` | 主机列表文件 | - |
| `-F <SSH 认证文件>` | SSH 认证配置文件（格式：hostname username password [port]） | - |
| `-h <主机名>` | 主机名（可多次指定） | - |
| `-r <类型>` | 测试类型：d=磁盘，s=内存流，n/N/M=网络，H=硬件，l=延迟/IOPS，i=IO 统计，u=NUMA，k=内核参数，q=网络质量，I=磁盘信息 | dsn |
| `-B <KB>` | 磁盘块大小 | 32 |
| `-S <大小>` | 磁盘测试文件大小 | 2xRAM |
| `-D` | 显示每台主机详情 | false |
| `-v` | 详细模式 | false |
| `--duration` | 网络测试时长 | 15s |
| `--random-bs <KB>` | 随机读写块大小 | 4 |
| `--latency-bs <KB>` | 延迟和 IOPS 测试块大小 | 4 |
| `--iostat-interval` | IO 统计采样间隔 | 1s |
| `--net-quality-target` | 网络质量测试目标主机 | - |
| `--netperf` | 使用 netperf 进行网络测试 | false |
| `--version` | 显示版本号 | - |
| `-?` | 显示帮助信息 | - |

## 输出格式

- **系统信息表格**: 主机名、CPU 核心数、内存大小、网卡速率
- **磁盘测试结果**: 写入/读取时间、数据量、带宽
- **内存带宽结果**: 复制、缩放、加法、三合一带宽
- **网络测试结果**: 源/目标主机、带宽
- **延迟测试结果**: 读/写延迟 (ms)、IOPS、最大/最小延迟
- **IO 统计结果**: r/s, w/s, rkB/s, wkB/s, await, %util, avgqu-sz, svctm
- **NUMA 信息**: 节点数、每节点 CPU/内存、CPU 距离矩阵
- **内核参数**: VM/IO/网络参数，附带告警提示
- **网络质量**: 延迟、丢包率、错包率、TCP 重传率、MTU
- **磁盘详细信息**: 磁盘类型、型号、容量、文件系统、可用空间、inode 使用率
- **汇总报告**: 各项测试的平均、最小、最大带宽

## 开发约定

1. **注释**: 所有函数使用中文注释
2. **包结构**:
   - `config`: 配置相关
   - `pkg/checker`: 性能检查逻辑（模块化）
     - `common`: 公共工具函数
     - `disk`: 磁盘 I/O 测试 + 磁盘信息检测
     - `network`: 网络性能测试 + 网络质量检测
     - `memory`: 内存带宽测试
     - `system`: 系统信息收集
     - `latency`: 磁盘延迟和 IOPS 测试
     - `iostat`: IO 统计监控
     - `numa`: NUMA 信息收集
     - `kernel`: 内核参数收集
   - `pkg/reporter`: 报告输出逻辑
   - `pkg/utils`: 通用工具函数
3. **错误处理**: 使用 error 返回值，重要错误打印到 stderr
4. **命名**: 使用 Go 标准命名约定
5. **导入路径**:
   ```go
   import (
       "dbcheckperf/pkg/checker/common"
       "dbcheckperf/pkg/checker/disk"
       "dbcheckperf/pkg/checker/network"
       "dbcheckperf/pkg/checker/memory"
       "dbcheckperf/pkg/checker/system"
       "dbcheckperf/pkg/checker/latency"
       "dbcheckperf/pkg/checker/iostat"
       "dbcheckperf/pkg/checker/numa"
       "dbcheckperf/pkg/checker/kernel"
   )
   ```
6. **向后兼容**: 使用类型别名和变量重定向保持 API 不变
   ```go
   type DiskResult = disk.DiskResult
   var NewDiskChecker = disk.NewDiskChecker
   type LatencyResult = latency.LatencyResult
   var NewLatencyChecker = latency.NewLatencyChecker
   ```

## 依赖

- Go 1.24+
- golang.org/x/crypto (SSH 密码认证支持)

## 相关文件

- `gpcheckperf.md`: Greenplum gpcheckperf 工具参考文档

## 代码修复与提交

- 当发现代码 BUG 时先进行提issue，然后根据问题进行分析和修复
- 代码修复后在自动提交到 GitHub 仓库
- 在提交时自动过滤临时文件，测试文件，PLAN 文件等
- 代码仅提交到github即可，不需要进行发版
- 手动在进行发版时，使用github上联系的版本号发版

## 文件更新

## 版本历史

| 版本 | 日期 | 类型 | 主要内容 |
|------|------|------|----------|
| v1.4.2 | 2026-03-28 | 文档更新 | 改进示例用户名（gpadmin → username） |
| v1.4.1 | 2026-03-28 | Bug 修复 | 修复 AMD 平台磁盘顺序读写测试结果为 0 的问题 |
| v1.4.0 | 2026-03-27 | 功能版 | CentOS 7.9 虚拟机支持、详细调试输出 |
| v1.3.1 | 2026-03-26 | Bug 修复 | 修复网络测试中的问题 |
| v1.3.0 | 2026-03-25 | 功能版 | 模块化重构、新增延迟/IOPS/IO 统计/NUMA/内核参数模块 |

### v1.4.2 详细说明 (2026-03-28)

**文档改进**:
- 将 README 中的示例用户名从 `gpadmin` 改为 `username`
- 避免用户直接复制示例导致混淆
- 修改位置：README.md、README_CN.md 中的 ssh_auth.txt 和 hosts.txt 示例

### v1.4.1 详细说明 (2026-03-28)

**问题**: AMD 平台磁盘顺序读写测试结果为 0.00 MB/s

**原因**: dd 命令输出格式多样性导致解析失败

**修复内容**:
- 增强 `parseDDOutput()` 函数 - 支持 6 种 dd 输出格式（bytes、中文、GB/MB、GiB/MiB、括号格式、copied 关键字）
- 改进 `runWriteTest()` 函数 - 添加文件大小备用方案，改进错误验证逻辑
- 改进 `runReadTest()` 函数 - 添加 DEBUG 调试输出
- 更新版本号为 1.4.1

**测试验证**:
```bash
# 编译
go build -o dbcheckperf ./cmd/main.go

# 测试
./dbcheckperf -h localhost -d /tmp -r ds -v
```

## 文件更新

- 每次代码修改后把相关功能更新 README.md 和 README_CN.md 和 QWEN.md

### 导入示例

```go
package main

import (
    "dbcheckperf/pkg/checker"
    "dbcheckperf/pkg/checker/common"
    "dbcheckperf/pkg/checker/disk"
    "dbcheckperf/pkg/checker/network"
    "dbcheckperf/pkg/checker/memory"
    "dbcheckperf/pkg/checker/system"
    "dbcheckperf/pkg/checker/latency"
    "dbcheckperf/pkg/checker/iostat"
    "dbcheckperf/pkg/checker/numa"
    "dbcheckperf/pkg/checker/kernel"
)

// 使用方式（保持向后兼容）
diskChecker := checker.NewDiskChecker(...)
networkChecker := checker.NewNetworkChecker(...)
streamChecker := checker.NewStreamChecker(...)
systemChecker := checker.NewSystemChecker()
hardwareChecker := checker.NewHardwareChecker()
latencyChecker := checker.NewLatencyChecker(...)
iostatChecker := checker.NewIOStatChecker(...)
numaChecker := checker.NewNUMAChecker(...)
kernelChecker := checker.NewKernelChecker(...)

// 直接使用子模块
common.ResolveToIP("host1")
latency.GetNetworkOffloadFeatures("eth0")
numa.GetNUMAInfo()
kernel.GetKernelParams()
```

### 编译和测试

```bash
# 编译整个项目
go build ./...

# 测试 checker 包
go test ./pkg/checker/... -v

# 测试覆盖率
go test ./pkg/checker/... -cover

# 单独测试子模块
go test ./pkg/checker/disk/...
go test ./pkg/checker/network/...
go test ./pkg/checker/latency/...
go test ./pkg/checker/iostat/...
go test ./pkg/checker/numa/...
go test ./pkg/checker/kernel/...
```

### 相关文件

- `pkg/checker/FINAL_REFACTOR_REPORT.md`: 模块化重构报告
