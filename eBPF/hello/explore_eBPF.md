# eBPF 开发全流程指南 — CO-RE + libbpf

> 系统环境：Debian 13 (Trixie) · Linux 6.12.86 · clang 19.1.7 · libbpf 1.5.0

---

## 目录

1. [整体流程概览](#1-整体流程概览)
2. [编写 eBPF 程序](#2-编写-ebpf-程序)
3. [编译](#3-编译)
4. [验证与调试](#4-验证与调试)
5. [加载与挂载](#5-加载与挂载)
6. [完整示例：从零到运行](#6-完整示例从零到运行)
7. [常见问题与技巧](#7-常见问题与技巧)

---

## 1. 整体流程概览

```
┌────────────┐    ┌────────────┐    ┌──────────────┐    ┌──────────────┐
│  编写源码   │ -> │   编译     │ -> │  验证/调试    │ -> │  加载/挂载    │
│  .bpf.c     │    │  .bpf.o    │    │  BTF/skel    │    │  内核执行     │
│  vmlinux.h  │    │  ELF + BTF │    │  bpftool     │    │  bpftool/load │
│  libbpf     │    │            │    │  trace_pipe  │    │  或 libbpf    │
└────────────┘    └────────────┘    └──────────────┘    └──────────────┘
       ▲                                                  │
       └──────────────────────────────────────────────────┘
                      持续迭代
```

**核心思想**：eBPF 程序是运行在内核沙箱中的事件驱动代码。它不能像普通程序那样独立运行——必须挂载到内核事件上（如系统调用、网络包到达、函数入口/出口），在内核触发事件时执行。

### 两种开发范式

| 范式 | 典型方式 | 特点 |
|------|----------|------|
| **CO-RE + libbpf** | `.bpf.c` → `clang` → `.o` → `libbpf` API 加载 | ✅ 一次编译到处运行<br>✅ 无需内核头文件<br>✅ 本指南采用 |
| **BCC** | Python 脚本中内嵌 C | ❌ 依赖 LLVM 运行时<br>❌ 每次运行都需编译 |

---

## 2. 编写 eBPF 程序

### 2.1 文件结构

一个典型的 eBPF 项目包含两个部分：

```
project/
├── minimal.bpf.c       ← eBPF 内核侧代码（运行在内核中）
├── minimal.c           ← 用户侧加载程序（可选，用 libbpf 加载）
├── Makefile            ← 编译规则
└── vmlinux.h           ← 内核类型定义（从 BTF 生成）
```

### 2.2 eBPF 内核程序基本骨架

```c
// ===== minimal.bpf.c =====
#include "vmlinux.h"                 // 所有内核类型（无需手写结构体）
#include <bpf/bpf_helpers.h>         // BPF helper 函数声明
#include <bpf/bpf_tracing.h>         // 追踪辅助宏（TP/ksym 等）

// SEC() 宏定义程序类型和挂载点
// 这里的 "raw_tp/sys_enter" 表示：挂载到所有系统调用的入口
SEC("raw_tp/sys_enter")
int trace_sys_enter(void *ctx)
{
    bpf_printk("syscall entered!\n");
    return 0;
}

// 许可证声明——GPL 才能使用某些 helper 函数
char LICENSE[] SEC("license") = "GPL";
```

### 2.3 常用 SEC() 标签

| 标签 | 程序类型 | 用途 |
|------|----------|------|
| `SEC("raw_tp/sys_enter")` | raw_tracepoint | 追踪所有系统调用入口 |
| `SEC("raw_tp/sys_exit")` | raw_tracepoint | 追踪所有系统调用出口 |
| `SEC("kprobe/sys_openat")` | kprobe | 动态插桩到任意内核函数入口 |
| `SEC("kretprobe/sys_openat")` | kprobe | 动态插桩到内核函数返回 |
| `SEC("xdp")` | XDP | 网络数据包过滤（网卡驱动层） |
| `SEC("tc")` | TC（流量控制） | 网络数据包处理（协议栈层） |
| `SEC("fentry/sys_openat")` | fentry | 轻量级函数入口追踪（需 BTF）|
| `SEC("tracepoint/syscalls/sys_enter_openat")` | tracepoint | 稳定的追踪点（参数结构化）|

> **选择建议**：
> - **学习/调试** → `raw_tp`（最简单，参数 ctx 无需解析）
> - **生产追踪** → `tracepoint`（稳定的 ABI，结构化参数）
> - **需要函数内部数据** → `kprobe`/`kprobe`（最灵活，但随内核版本变化）
> - **网络处理** → `XDP`（高性能 Layer 3+）或 `TC`（更丰富的网络功能）

### 2.4 eBPF Maps —— 内核与用户态通信

```c
// ===== map 定义 =====

// 哈希表 map：key → value 映射
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, u32);
    __type(value, u64);
} my_hash_map SEC(".maps");

// 环形缓冲区 map：高效的事件推送（推荐替代 perf_buffer）
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

// 数组 map：按索引访问
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 10);
    __type(key, u32);
    __type(value, u64);
} my_array SEC(".maps");
```

### 2.5 Helper 函数一览

eBPF 程序只能调用内核提供的"helper 函数"，不能调用任意内核函数。

```c
// 最常用的 helper（所有都需要 GPL 许可证）
bpf_printk("fmt", ...)            // 打印到 trace_pipe（调试用，仅 3 个参数）
bpf_get_current_pid_tgid()        // 获取当前进程 PID/TGID
bpf_get_current_comm(void *buf, int size)  // 获取进程名
bpf_ktime_get_ns()                // 获取纳秒级时间戳
bpf_map_lookup_elem(&map, &key)   // 查找 map 元素
bpf_map_update_elem(&map, &key, &val, flags)  // 更新 map
bpf_get_current_task()            // 获取当前 task_struct 指针
bpf_probe_read_kernel(&dst, size, &src)  // 安全读取内核内存
bpf_probe_read_user(&dst, size, &src)    // 安全读取用户态内存
bpf_perf_event_output(ctx, &map, flags, data, size)  // 向 perf 环写数据
bpf_ringbuf_output(&map, data, size, flags)           // 向环形缓冲写数据
```

---

## 3. 编译

### 3.1 单文件编译命令

```bash
clang -g -O2 -target bpf -I. -c minimal.bpf.c -o minimal.bpf.o
```

| 参数 | 含义 |
|------|------|
| `-g` | 生成 BTF 调试信息（CO-RE 必需！） |
| `-O2` | 优化级别，eBPF 验证器需要 |
| `-target bpf` | 编译为 BPF 字节码 |
| `-I.` | 头文件搜索路径（找到 vmlinux.h） |
| `-c` | 只编译，不链接 |
| `-D__TARGET_ARCH_x86` | 指定目标架构（使用 TAIL_CALL 等宏时必需）|

### 3.2 Makefile 模板

```makefile
CLANG    ?= clang
BPFTOOL  ?= bpftool
CFLAGS   := -g -O2 -target bpf -I.

all: minimal.bpf.o

%.bpf.o: %.bpf.c vmlinux.h
	$(CLANG) $(CFLAGS) -c $< -o $@
	$(BPFTOOL) gen skeleton $@ name minimal 2>/dev/null  # 可选：生成 skel

vmlinux.h:
	$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > $@

clean:
	rm -f *.o *.skel.h

.PHONY: all clean
```

### 3.3 编译产物检查

```bash
# 查看 ELF section
llvm-objdump -h minimal.bpf.o

# 查看 BPF 指令
llvm-objdump -d minimal.bpf.o

# 查看 BTF 信息
bpftool btf dump file minimal.bpf.o format raw

# 生成 C 语言的 skeleton 头文件
bpftool gen skeleton minimal.bpf.o name minimal > minimal.skel.h
```

---

## 4. 验证与调试

### 4.1 BTF/CO-RE 检测（编译后）

```bash
# 检查是否有 .BTF、.BTF.ext 段
llvm-readelf -S minimal.bpf.o | grep -E 'BTF|debug'

# 检查 CO-RE 重定位信息
bpftool gen object minimal.bpf.o /dev/null 2>&1 && echo "✅ CO-RE 格式正确"

# 更详尽：查看所有重定位记录
bpftool gen object minimal.bpf.o /tmp/checked.o
```

### 4.2 使用 bpf_printk 调试（最简单）

```c
// 在内核程序中
SEC("raw_tp/sys_enter")
int hello(void *ctx)
{
    // bpf_printk 支持最多 3 个参数
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;
    u32 tid = (u32)pid_tgid;

    bpf_printk("PID %d (TID %d) entered a syscall", pid, tid);
    return 0;
}
```

查看输出：

```bash
# 方式 1：实时追踪
sudo cat /sys/kernel/debug/tracing/trace_pipe

# 方式 2：查看静态快照
sudo cat /sys/kernel/debug/tracing/trace
```

> **注意**：`bpf_printk` 仅适合开发调试。生产环境应使用 `ringbuf` 或 `perf_event`。

### 4.3 验证器问题调试

eBPF 程序在加载时必须通过内核验证器（verifier）的检查。常见错误及处理：

```bash
# 错误：R2 min value is negative, either use unsigned or 'var &= const'
# 原因：循环边界可能为负值
# 修复：使用 unsigned 循环变量

# 错误：invalid access to packet, off=... size=..., R... has unknown scalar
# 原因：XDP 数据包边界未检查
# 修复：检查 packet 指针不超出 data_end

# 错误：Unrecognized arg#type
# 原因：使用了禁止的指针算术
# 修复：使用 bpf_probe_read_kernel() 读取指针指向的值
```

> **验证器黄金法则**：
> 1. 所有循环必须有**确定的最大上界**
> 2. 指针只能与标量加减，且结果必须在合法范围内
> 3. 禁止指针间算术运算（一指针减另一指针除外）
> 4. 未初始化的栈变量不能作为 helper 参数传递

### 4.4 map 内容查看

```bash
# 查看已加载程序的 map
bpftool prog list
bpftool map list

# 查看具体 map 的键值
bpftool map dump id <map_id>

# 查看 map 的 pin path（固定到 bpf 文件系统的路径）
bpftool map show id <map_id>
```

---

## 5. 加载与挂载

### 5.1 方法一：bpftool 直接加载（快速验证）

适用于开发调试，无需编写用户态加载程序：

```bash
# 将 .o 文件加载到内核并 pin 到 bpf 文件系统
sudo bpftool prog load minimal.bpf.o /sys/fs/bpf/hello_prog

# 查看已加载的程序
sudo bpftool prog list

# 手动挂载到事件
sudo bpftool prog attach /sys/fs/bpf/hello_prog raw_tracepoint sys_enter

# 或者用 pin 名称挂载
sudo bpftool prog attach id $(bpftool prog show -p | jq '.[0].id') raw_tracepoint sys_enter

# 卸载：删除 pin 文件 + 删除程序
sudo rm /sys/fs/bpf/hello_prog
sudo bpftool prog detach /sys/fs/bpf/hello_prog raw_tracepoint sys_enter
```

> ⚠️ 局限性：`bpftool prog load` 不能处理所有程序类型。复杂场景推荐使用方法二。

### 5.2 方法二：libbpf + skeleton（生产推荐）

完整的加载流程，需要编写用户态 C 程序。

**步骤 1：生成 skeleton**

```bash
bpftool gen skeleton minimal.bpf.o name minimal > minimal.skel.h
```

**步骤 2：编写用户态加载器**

```c
// ===== minimal.c =====
#include <stdio.h>
#include <unistd.h>
#include <sys/resource.h>
#include <bpf/libbpf.h>
#include "minimal.skel.h"    // 自动生成的 skeleton

// libbpf 错误回调（可选，用于诊断）
static int libbpf_print_fn(enum libbpf_print_level level,
                           const char *format, va_list args)
{
    return vfprintf(stderr, format, args);
}

int main(void)
{
    struct minimal_bpf *skel;
    int err;

    // 设置调试输出
    libbpf_set_print(libbpf_print_fn);

    // 打开并加载 BPF 程序
    skel = minimal_bpf__open_and_load();
    if (!skel) {
        fprintf(stderr, "Failed to open and load BPF skeleton\n");
        return 1;
    }

    // 挂载到事件
    err = minimal_bpf__attach(skel);
    if (err) {
        fprintf(stderr, "Failed to attach BPF skeleton\n");
        goto cleanup;
    }

    printf("eBPF program loaded and attached! Press Ctrl+C to exit.\n");

    // 轮询读取 map 或 ringbuf（如果有）
    // 此处仅保持运行
    for (;;) {
        sleep(1);
    }

cleanup:
    minimal_bpf__destroy(skel);
    return err;
}
```

**步骤 3：编译链接用户态程序**

```bash
gcc -g -O2 -Wall minimal.c -o minimal -lbpf -lz -lelf
```

> 需要链接 `-lbpf`（libbpf）、`-lz`（zlib）、`-lelf`（libelf）。

**步骤 4：运行**

```bash
sudo ./minimal
```

### 5.3 bpftool 常用加载命令速查

| 命令 | 用途 |
|------|------|
| `bpftool prog load <file> <pin_path>` | 加载程序并 pin 到文件系统 |
| `bpftool prog loadall <file> <pin_path>` | 加载 SEC() 中所有程序 |
| `bpftool prog attach <pin> <type> <target>` | 挂载到事件 |
| `bpftool prog detach <pin> <type> <target>` | 卸载 |
| `bpftool prog run <id> data_in <file>` | 手动注入数据运行测试 |
| `bpftool prog list` | 列出已加载程序 |
| `bpftool prog show id <id> --pretty` | 查看程序详细信息 |
| `bpftool map update pinned <pin> key <hex> value <hex>` | 手动写入 map |

### 5.4 自动挂载（Program Auto-Attach）

libbpf skeleton 支持自动挂载——根据 `SEC()` 标签推断挂载目标：

```c
// skeleton 会自动解析 "raw_tp/sys_enter" 并挂载
err = minimal_bpf__attach(skel);
```

支持的自动挂载类型：

| SEC 标签 | 自动挂载行为 |
|----------|-------------|
| `raw_tp/<tracepoint>` | 自动 attach 到指定 tracepoint |
| `kprobe/<func>` | 自动 attach 到内核函数入口 |
| `kretprobe/<func>` | 自动 attach 到内核函数返回 |
| `fentry/<func>` | 自动 attach（需 BTF） |
| `fexit/<func>` | 自动 attach |
| `xdp` | 需要手动指定网卡 |
| `tc` | 需要手动指定网卡 |

---

## 6. 完整示例：从零到运行

下面是一个追踪 `openat` 系统调用的完整例子，包含 CO-RE + libbpf 全流程。

### 6.1 创建项目

```bash
cd /home/kktori/learn/eBPF
mkdir -p trace_openat && cd trace_openat

# 确保 vmlinux.h 在工作目录
ln -s ../vmlinux.h .
```

### 6.2 编写内核程序

```c
// ===== trace_openat.bpf.c =====
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "GPL";

// 定义 map：记录每个 PID 的 openat 调用次数
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, u32);
    __type(value, u64);
} openat_count SEC(".maps");

// 定义 ringbuf 输出结构
struct event {
    u32 pid;
    u32 uid;
    char comm[16];
    char filename[256];
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} rb SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_openat")
int trace_openat_enter(struct trace_event_raw_sys_enter *ctx)
{
    // 解析系统调用参数（openat 的第二个参数是文件名）
    // ctx->args[0] = dfd, ctx->args[1] = filename, ctx->args[2] = flags
    const char *filename = (const char *)BPF_CORE_READ(ctx, args[1]);

    // 获取当前进程信息
    u64 id = bpf_get_current_pid_tgid();
    u32 pid = id >> 32;
    u32 tid = (u32)id;
    u32 uid = bpf_get_current_uid_gid();

    // 更新统计 map
    u64 *count = bpf_map_lookup_elem(&openat_count, &tid);
    u64 new_count = 1;
    if (count)
        new_count = *count + 1;
    bpf_map_update_elem(&openat_count, &tid, &new_count, BPF_ANY);

    // 向 ringbuf 发送事件（只有文件名为非空时才发送）
    if (filename) {
        struct event *e = bpf_ringbuf_reserve(&rb, sizeof(*e), 0);
        if (e) {
            e->pid = pid;
            e->uid = uid;
            bpf_get_current_comm(&e->comm, sizeof(e->comm));
            bpf_probe_read_user_str(&e->filename, sizeof(e->filename), filename);
            bpf_ringbuf_submit(e, 0);
        }
    }

    return 0;
}
```

### 6.3 编写用户态加载器

```c
// ===== trace_openat.c =====
#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <bpf/libbpf.h>
#include "trace_openat.skel.h"

static volatile bool exiting = false;

static void sig_handler(int sig)
{
    exiting = true;
}

// ringbuf 事件处理回调
static int handle_event(void *ctx, void *data, size_t data_sz)
{
    struct event *e = (struct event *)data;
    printf("PID %-6d (%-8s) UID %-5d openat: %s\n",
           e->pid, e->comm, e->uid, e->filename);
    return 0;
}

// libbpf 日志
static int libbpf_print(enum libbpf_print_level level,
                        const char *format, va_list args)
{
    return vfprintf(stderr, format, args);
}

int main(void)
{
    struct ring_buffer *rb = NULL;
    struct trace_openat_bpf *skel;
    int err;

    libbpf_set_print(libbpf_print);

    signal(SIGINT, sig_handler);
    signal(SIGTERM, sig_handler);

    // 打开并加载
    skel = trace_openat_bpf__open_and_load();
    if (!skel) {
        fprintf(stderr, "Failed to open BPF skeleton\n");
        return 1;
    }

    // 挂载
    err = trace_openat_bpf__attach(skel);
    if (err) {
        fprintf(stderr, "Failed to attach BPF skeleton\n");
        goto cleanup;
    }

    // 设置 ringbuf 消费者
    rb = ring_buffer__new(bpf_map__fd(skel->maps.rb),
                          handle_event, NULL, NULL);
    if (!rb) {
        fprintf(stderr, "Failed to create ring buffer\n");
        goto cleanup;
    }

    printf("✅ Tracing openat syscalls... Press Ctrl+C to stop.\n");
    printf("─── PID ──── COMM ────── UID ── FILENAME ───────────────\n");

    // 轮询 ringbuf
    while (!exiting) {
        err = ring_buffer__poll(rb, 100);  // 100ms timeout
        if (err == -EINTR)
            break;
        if (err < 0) {
            fprintf(stderr, "Error polling ring buffer: %d\n", err);
            break;
        }
    }

cleanup:
    ring_buffer__free(rb);
    trace_openat_bpf__destroy(skel);
    return 0;
}
```

### 6.4 编译与运行

```bash
# 生成 skeleton
bpftool gen skeleton trace_openat.bpf.o name trace_openat \
    > trace_openat.skel.h

# 编译用户态程序
gcc -g -O2 -Wall trace_openat.c -o trace_openat \
    -lbpf -lz -lelf

# 运行
sudo ./trace_openat
```

示例输出：

```
✅ Tracing openat syscalls... Press Ctrl+C to stop.
─── PID ──── COMM ────── UID ── FILENAME ───────────────
PID 1234   (tmux: server) UID 1000  openat: /etc/localtime
PID 5678   (vim)          UID 1000  openat: /home/user/.vimrc
PID 9012   (bash)         UID 1000  openat: /usr/share/bash-completion/...
...
```

---

## 7. 常见问题与技巧

### 7.1 `vmlinux.h` 缺失某些宏/结构体

某些情况下 `vmlinux.h` 的结构体缺少字段，可以手动补充：

```c
// 在 include "vmlinux.h" 之后补充
#ifndef MY_CUSTOM_STRUCT
struct my_custom_struct {
    u32 field1;
    u64 field2;
} __attribute__((preserve_access_index));
#endif
```

### 7.2 栈大小限制

```c
// ❌ 错误：大型栈变量
void bad_func(void *ctx) {
    char big_buffer[4096];  // 太大！eBPF 栈限制 512 字节
}

// ✅ 正确：使用 map 或 ringbuf
void good_func(void *ctx) {
    struct event *e = bpf_ringbuf_reserve(&rb, sizeof(*e), 0);
    if (e) {
        e->field = 42;
        bpf_ringbuf_submit(e, 0);
    }
}
```

### 7.3 指令数限制

```c
// 早期内核：4096 条指令限制（6.0 前）
// 当前内核（6.12+）：1000000 条（一百万）

// 如果仍然超限，可能需要：
// 1. 拆分程序（多个 SEC 段）
// 2. 使用 BPF 尾调用（BPF Tail Call）
// 3. 简化逻辑，去掉内联循环
```

### 7.4 BTF 相关问题

```bash
# 验证 vmlinux.h 是否正确
bpftool btf dump file /sys/kernel/btf/vmlinux format c | head -5

# 检查 .o 文件中的 BTF 信息
bpftool btf dump file minimal.bpf.o format raw | head -10

# CO-RE 重定位校验：如果成功，说明格式正确
bpftool gen object minimal.bpf.o /dev/null 2>&1
```

### 7.5 开发效率技巧

```bash
# 1. RAD 模式：直接用 bpftool 加载 .o，配合 trace_pipe 验证
#    无需编写用户态程序
clang -g -O2 -target bpf -I. -c test.bpf.c -o test.bpf.o
sudo bpftool prog load test.bpf.o /sys/fs/bpf/test_prog
sudo bpftool prog attach /sys/fs/bpf/test_prog raw_tracepoint sys_enter
sudo cat /sys/kernel/debug/tracing/trace_pipe

# 2. 一键编译+加载
make && sudo bpftool prog load *.bpf.o /sys/fs/bpf/ && echo "OK"

# 3. 用 bpftrace 先验证 hook 点是否可用
#    （bpftrace 是一种高层 DSL，可快速原型验证）
sudo bpftrace -e 'tracepoint:syscalls:sys_enter_openat { printf("openat: %s\n", str(args->filename)); }'
```

### 7.6 系统限制调整（生产部署时）

```bash
# 查看当前 eBPF 资源限制
ulimit -l           # RLIMIT_MEMLOCK（决定 eBPF map 可锁定的内存大小）
sudo bpftool info  # 查看系统级限制

# 临时放宽（调试用）
sudo bash -c 'ulimit -l unlimited && ./my_bpf_program'

# 永久配置（systemd 或 sysctl）
# /etc/security/limits.conf:  *  -  memlock   unlimited
# /etc/sysctl.conf:             kernel.bpf_stats_enabled=1
```

---

## 附录：快速参考卡片

```bash
# 一句话编译
clang -g -O2 -target bpf -I. -c prog.bpf.c -o prog.bpf.o

# 一句话生成 vmlinux.h
bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h

# 一句话加载并挂载到 tracepoint
bpftool prog load prog.bpf.o /sys/fs/bpf/prog && \
bpftool prog attach /sys/fs/bpf/prog raw_tracepoint sys_enter

# 一句话查看输出
sudo cat /sys/kernel/debug/tracing/trace_pipe

# 一句话生成 skeleton
bpftool gen skeleton prog.bpf.o name prog > prog.skel.h

# 一句话编译用户态
gcc -g -O2 -Wall loader.c -o loader -lbpf -lz -lelf
```

---

> **参考资源**
> - [libbpf 官方文档](https://github.com/libbpf/libbpf)
> - [BPF CO-RE 指南 (kernel.org)](https://docs.kernel.org/bpf/libbpf/libbpf_overview.html)
> - [bpftool 文档](https://manpages.debian.org/testing/bpftool/bpftool.8.en.html)
