# dbcheckperf - Database Performance Check Tool

dbcheckperf is a database performance check tool written in Go, based on the Greenplum gpcheckperf utility. It is used to test and verify host hardware performance, including disk I/O, memory bandwidth, and network performance.

**[中文版本](README_CN.md)**

## Features

- **Disk I/O Testing**: Uses `dd` command to test sequential and random read/write performance, single iteration for fast completion, uses `oflag=direct` and `conv=fsync` for accurate results
- **Random I/O Testing**: Default 4K random read/write testing using `dd` with `seek/skip` parameters for random position access, 100 iterations for quick completion
- **Memory Bandwidth Testing**: Uses STREAM benchmark method (Go implementation), actually executes Copy/Scale/Add/Triad memory operations
- **Network Performance Testing**: Supports iperf3/netperf/curl/TCP stream multiple test methods, automatically selects the best tool
- **Hardware Information Collection**: Collects CPU model/cores/turbo frequency, disk model/vendor/size, RAID configuration, network card details
- **System Information Collection**: Automatically collects CPU cores, memory size, network card speed, etc.
- **Hardware Detection**: Detects VM/physical machine, RAID card model, stripe size, network bonding, queue size, etc.
- **Multi-Architecture Support**: Compatible with x86/x86_64 (Intel/AMD) and ARM/ARM64 architecture servers
- **Tabular Output**: Clearly displays test results and summary statistics for each host
- **Quick Testing**: Single iteration testing for fast performance checks

## Installation

### Requirements

- Go 1.21 or higher

### Build

```bash
# Clone or enter project directory
cd /path/to/dbcheckperf

# Build
go build -o dbcheckperf ./cmd/main.go

# Install to GOPATH
go install .
```

## Usage

### Basic Syntax

```bash
# Disk I/O and memory bandwidth testing
dbcheckperf -d <test_dir> [-d <test_dir> ...]
     {-f <hostfile> | -h <host> [-h <host> ...]}
     [-r ds] [-B <block_size>] [-S <file_size>] [-D] [-v|-V]

# Network testing
dbcheckperf -d <temp_dir>
     {-f <hostfile> | -h <host> [-h <host> ...]}
     [-r n|N|M [--duration <time>]] [-D] [-v|-V] [--buffer-size <KB>]
```

### Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-B <block_size>` | Disk I/O test block size (KB) | 32KB |
| `-d <dir>` | Test directory (can be specified multiple times) | Required |
| `-D` | Display detailed results per host | false |
| `-f <hostfile>` | File containing host list | - |
| `-h <host>` | Hostname (can be specified multiple times) | - |
| `-r <test_type>` | Test type: d=disk, s=memory stream, n/N/M=network, H=hardware | dsn |
| `-S <file_size>` | Disk I/O test file size | 2xRAM |
| `-v` | Verbose mode | false |
| `-V` | Very verbose mode | false |
| `--duration <time>` | Network test duration | 15s |
| `--netperf` | Use netperf for network testing | false |
| `--buffer-size <KB>` | Network test buffer size | 8KB |
| `--random-bs <KB>` | Random I/O block size (KB) | 4KB |
| `--version` | Display version number | - |
| `-?` | Display help information | - |

### Test Type Descriptions

| Code | Test Type | Description |
|------|-----------|-------------|
| `d` | Disk I/O | Test sequential read/write performance |
| `s` | Memory Stream | Test memory bandwidth |
| `n` | Network Serial | Test network host by host |
| `N` | Network Parallel | Test all hosts simultaneously (requires even number of hosts) |
| `M` | Network Full Matrix | Each host tests with all other hosts |
| `H` | Hardware Info | Collect and display hardware configuration information |

## Examples

### Example 1: Run Disk I/O and Memory Bandwidth Tests

```bash
dbcheckperf -f hosts.txt -d /data1 -d /data2 -r ds
```

### Example 2: Run Disk I/O Test Only, Display Per-Host Results

```bash
dbcheckperf -h host1 -h host2 -d /data1 -r d -D -v
```

### Example 3: Run Parallel Network Test

```bash
dbcheckperf -f hosts.txt -r N -d /tmp
```

### Example 4: Use Custom File Size and Block Size

```bash
dbcheckperf -h localhost -d /tmp -B 16k -S 5GB -r ds
```

### Example 5: Run Full Matrix Network Test

```bash
dbcheckperf -f hosts.txt -r M -d /tmp --duration 30s
```

### Example 6: Display Detailed Hardware Information

```bash
dbcheckperf -d /tmp -r ds -v
```

### Example 7: Collect Hardware Information

```bash
dbcheckperf -r H -v
```

### Example 8: Custom Random Block Size

```bash
dbcheckperf -h localhost -d /tmp -r d --random-bs 8 -v
```

### Example 9: Test with Large Block Size for SSD/NVMe

```bash
dbcheckperf -h localhost -d /data/nvme -B 256k -S 10GB -r d -v
```

This tests sequential and random I/O with 256KB block size, suitable for high-performance NVMe SSDs.

### Example 10: Multi-Host Disk Test with Detailed Output

```bash
dbcheckperf -h server1 -h server2 -h server3 -d /data -r d -D -v
```

This runs disk I/O tests on 3 hosts and displays detailed results for each host.

### Example 11: Network Test with Extended Duration

```bash
dbcheckperf -f hosts.txt -r n -d /tmp --duration 60s -v
```

This runs a 60-second serial network test for more accurate results.

### Example 12: Combined Disk, Memory, and Network Test

```bash
dbcheckperf -f hosts.txt -d /data -r dsN -D -v
```

This runs disk I/O, memory bandwidth, and parallel network tests on all hosts.

### Example 13: Hardware Inventory for All Hosts

```bash
dbcheckperf -f hosts.txt -r H -v
```

This collects detailed hardware information from all hosts including CPU, disk, RAID, NIC, and memory details.

This will display:
- **CPU Information**: Model, cores, sockets, base frequency, turbo frequency, NUMA nodes
- **Disk Information**: Device name, model, vendor, type (HDD/SSD/NVMe), size, rotational
- **RAID Information**: RAID card model, cache size, stripe size, RAID level, battery backup
- **Network Information**: Device name, speed, MTU, queue size, bond status, bond mode, driver, MAC address

## Output Format

### System Information Table

```
====================
==  System Information
====================

IP Address          Virtualization  CPU Model                    CPU Cores   Memory Size    OS                Kernel Version        Network Speed
----------------------------------------------------------------------------------------------------------------------------------
192.168.1.100       Physical        Intel Xeon E5-2680           16          64.00 GB       CentOS 7.9        3.10.0-1160           10000 Mbps
192.168.1.101       KVM             AMD EPYC 7K62                32          128.00 GB      Ubuntu 20.04      5.4.0-42              25000 Mbps
```

### Hardware Information Table

```
====================
==  Hardware Information
====================

--- CPU Information ---

IP Address            CPU Model                            Cores      Sockets    Base Freq    Turbo Freq   NUMA Nodes
--------------------------------------------------------------------------------------------------------------
192.168.1.100         Intel Xeon E5-2680 v3 @ 2.50GHz      12         1          2500 MHz     3300 MHz     1

--- Disk Information ---

Device       Model                               Vendor          Type         Size         Rotational
----------------------------------------------------------------------------------------------------
sda          Samsung SSD 860 PRO 1TB             Samsung        SSD          1.00 TB      No
sdb          WDC WD4003FFBX-68LU              Western Digital  HDD          4.00 TB      Yes
nvme0n1      INTEL SSDPE2KE016T8                 Intel          NVMe         1.60 TB      No

--- RAID Information ---

Status         RAID Model                                 Cache Size        Stripe Size    RAID Level        Battery Backup
-------------------------------------------------------------------------------------------------------------------------
Detected       LSI MegaRAID SAS 3508                      2.00 GB           256 KB         RAID10            Supported

--- Network Information ---

Device       Speed            MTU        Queue      Bond       Bond Mode       Driver          MAC Address
---------------------------------------------------------------------------------------------------------------
eth0         10000 Mbps       1500       1024       No         -               ixgbe           00:1a:2b:3c:4d:5e
eth1         10000 Mbps       1500       1024       No         -               ixgbe           00:1a:2b:3c:4d:5f
bond0        20000 Mbps       1500       1024       Yes        802.3ad         bonding         00:1a:2b:3c:4d:60
```

### Detailed Hardware Information Table

```
====================
==  Detailed Hardware Information
====================

--- RAID Information ---

IP Address            Status          RAID Model                            Cache Size        Stripe Size    RAID Level
-------------------------------------------------------------------------------------------------------------------
192.168.1.100         Detected        LSI MegaRAID SAS 3508                 2.00 GB           256 KB         RAID10
192.168.1.101         Detected        Broadcom MegaRAID SAS 9460-16i        4.00 GB           512 KB         RAID5

--- Network Bonding Information ---

IP Address            Bond Status     Bond Mode                             Slave Count      Queue Size
-----------------------------------------------------------------------------------------------
192.168.1.100         Bonded          802.3ad Dynamic link aggregation      2                1024
192.168.1.101         Unbonded        -                                     -                1000
```

### Disk Test Results Table

```
====================
==  Disk I/O Test Results
====================

--- Sequential Write Performance ---
IP Address            Time (sec)      Data Size      Bandwidth (MB/s)
-----------------------------------------------------------------
192.168.1.100         2.68s           7.7 GB         2872.06
192.168.1.101         2.52s           7.7 GB         3065.45

--- Sequential Read Performance ---
IP Address            Time (sec)      Data Size      Bandwidth (MB/s)
-----------------------------------------------------------------
192.168.1.100         2.26s           7.7 GB         3408.01
192.168.1.101         2.15s           7.7 GB         3586.12

--- Random Write Performance ---
IP Address            Time (sec)      Data Size      Bandwidth (MB/s)
-----------------------------------------------------------------
192.168.1.100         120.50s         4.00 GB        35.64
192.168.1.101         125.20s         4.00 GB        34.30

--- Random Read Performance ---
IP Address            Time (sec)      Data Size      Bandwidth (MB/s)
-----------------------------------------------------------------
192.168.1.100         85.30s          4.00 GB        50.35
192.168.1.101         88.70s          4.00 GB        48.42
```

### Network Test Results

```
====================
==  Network Performance Test Results
====================

Test Mode: Parallel Mode

--- Summary ---
Total Bandwidth: 450.25 MB/s
Average Bandwidth: 112.56 MB/s
Median Bandwidth: 110.88 MB/s
Minimum Bandwidth: 105.53 MB/s
Maximum Bandwidth: 120.45 MB/s
```

### Summary Report

```
╔══════════════════════════════════════════════════════════════╗
║                      T E S T   S U M M A R Y                 ║
╠══════════════════════════════════════════════════════════════╣
║  Sequential Write Bandwidth: 2872.06 MB/s [server1]          ║
║  Sequential Read Bandwidth: 3408.01 MB/s [server1]           ║
║  Random Write Bandwidth: 35.64 MB/s [server1]                ║
║  Random Read Bandwidth: 50.35 MB/s [server1]                 ║
║  Memory Bandwidth: 181397.02 MB/s [server1]                  ║
║  Network Bandwidth: 110.56 MB/s                              ║
╚══════════════════════════════════════════════════════════════╝
```

### Comprehensive Test Results Table

```
====================
==  Comprehensive Test Results
====================

Test Item              Average              Min                  Max                  Notes               
---------------------------------------------------------------------------------------------------
Sequential Write       2872.06 MB/s        2872.06 MB/s        2872.06 MB/s        1 test              
Sequential Read        3408.01 MB/s        3408.01 MB/s        3408.01 MB/s        1 test              
Random Write           35.64 MB/s          35.64 MB/s          35.64 MB/s          1 test
Random Read            50.35 MB/s          50.35 MB/s          50.35 MB/s          1 test
Memory Bandwidth       181397.02 MB/s      181397.02 MB/s      181397.02 MB/s      1 test              
Network Bandwidth      110.56 MB/s         110.56 MB/s         110.56 MB/s         1 test              
```

## Host File Format

Each line in the host file contains a hostname, with optional format:

```
# Basic format
hostname

# With username
username@hostname

# With SSH port
hostname:2222

# Full format
username@hostname:port
```

Example `hosts.txt`:

```
server1
server2
gpadmin@server3:2222
192.168.1.100
```

## Project Structure

```
dbcheckperf/
├── cmd/
│   └── main.go             # Main entry point
├── go.mod                  # Go module definition
├── README.md               # This document
├── config/
│   └── config.go           # Configuration management
└── pkg/
    ├── checker/
    │   └── checker.go      # Performance checker (disk, network, memory, system)
    ├── reporter/
    │   └── reporter.go     # Report generator (table output)
    └── utils/
        └── utils.go        # Utility functions
```

## Performance Threshold Reference

| Test Type | Minimum Expected Value | Description |
|-----------|------------------------|-------------|
| Network Bandwidth | 100 MB/s | Below this value, use `-r n` for serial testing |
| Disk Write | 100 MB/s | Depends on storage type (HDD/SSD) |
| Memory Bandwidth | 20000 MB/s | Depends on memory type and channel count |

## Notes

1. **Test Directory Permissions**: Users running the tool must have write permissions to all test directories
2. **File Size**: Disk test file size is recommended to be set to 2x system RAM to ensure disk I/O is tested rather than memory cache
3. **Network Subnet**: All hosts in the host file should be on the same subnet during network testing
4. **Parallel Mode**: Network parallel mode (`-r N`) recommends using an even number of hosts
5. **SSH Trust**: Multi-host testing requires SSH passwordless login configuration

## Test Rigor Description

### Disk I/O Testing
- **Direct I/O**: Uses `oflag=direct` and `iflag=direct` to bypass system cache
- **Data Sync**: Uses `conv=fsync` to ensure data is actually written to disk
- **Cache Clear**: Clears system cache before each read test
- **Data Validation**: Validates write data integrity with 1% tolerance
- **Quick Test**: Single iteration for fast completion
- **File Cleanup**: Automatically deletes test files after testing

### Random I/O Testing
- **4K Random**: Uses 4KB block size for random read/write testing by default
- **Random Position**: Uses `seek/skip` parameters for random read/write positioning
- **100 Iterations**: Executes 100 random read/write operations for quick completion
- **Direct I/O**: Uses `oflag=direct` and `iflag=direct` to bypass cache
- **Default Enabled**: Random I/O testing is automatically included with `-r d` disk tests

### Memory Bandwidth Testing
- **Actual Testing**: Uses Go implementation of STREAM benchmark, actually executes memory operations
- **Four Operations**: Copy, Scale, Add, Triad four memory operation tests
- **Large Array**: Uses 80MB test array to ensure accurate results
- **Quick Test**: Single iteration for fast completion

### Network Performance Testing
- **Multiple Methods**: Prefers iperf3, then netperf, fallback to TCP stream testing
- **Timeout Control**: Sets reasonable connection and transfer timeouts
- **Data Validation**: Validates actual bytes transferred and time
- **Quick Test**: Single iteration for fast completion

### System Information Detection (Multi-Architecture Support)
- **x86/x86_64**: Supports Intel and AMD server CPU detection
- **ARM/ARM64**: Supports ARM architecture servers (including Qualcomm, Ampere, Apple Silicon, etc.)
- **Virtualization Detection**: Supports KVM, VMware, Xen, Hyper-V and other virtualization types
- **RAID Detection**: Supports LSI/Broadcom MegaRAID and other RAID card detection

## Compatibility with gpcheckperf

This tool implements the main features of Greenplum gpcheckperf:

| Feature | gpcheckperf | dbcheckperf |
|---------|-------------|-------------|
| Disk I/O Testing | ✓ | ✓ |
| Memory Bandwidth Testing | ✓ | ✓ |
| Network Performance Testing | ✓ | ✓ |
| Host File Support | ✓ | ✓ |
| Multi-Directory Testing | ✓ | ✓ |
| Verbose Mode | ✓ | ✓ |
| Per-Host Results | ✓ | ✓ |
| netperf Support | ✓ | ✓ |

## License

This project is licensed under the MIT License.

## Contributing

Issues and feature requests are welcome!
