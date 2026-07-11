#include "../vmlinux.h"

#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// Per_cpu Hash Map: 避免写同步代码
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_HASH);
    __type(key, u32); // pid
    __type(value, u64); // count
    __uint(max_entries, 1024);
} openat_counts SEC(".maps");

// 事件结构体
struct event {
    u32 pid;
    u32 uid;
    char comm[16]; // command
    int ret; // syscall return value
};

// Ring Buffer
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} ringbuf SEC(".maps");

// openat 入口 拦截
SEC("tp/syscalls/sys_enter_openat")
int handle_onenat_enter(struct trace_event_raw_sys_enter *ctx)
{
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;
    u32 tid = (u32)pid_tgid;

    // 计数递增
    u64 *count = bpf_map_lookup_elem(&openat_counts, &pid);
    u64 new_count = 1;
    if(count) {
        new_count = *count + 1;
    }
    bpf_map_update_elem(&openat_counts, &pid, &new_count, BPF_ANY);

    // 在 ringbuf 中申请内存，用于 event 结构
    struct event *e = bpf_ringbuf_reserve(&ringbuf, sizeof(*e), 0);
    if(!e) {
        goto out;
    }
    e->pid = pid;
    e->uid = bpf_get_current_uid_gid() & 0xFFFFFFFF;
    bpf_get_current_comm(&e->comm, sizeof(e->comm));
    e->ret = 0;

    // 提交到缓冲区
    bpf_ringbuf_submit(e, 0);

out:
    bpf_printk("[MAPS] PID=%d TID=%d openat_cnt=%llu", pid, tid, new_count);
    return 0;
}

// 拦截 openat 退出
SEC("tp/syscalls/sys_exit_openat")
int handle_openat_exit(struct trace_event_raw_sys_exit *ctx)
{
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;
    int ret = ctx->ret;

    struct event *e = bpf_ringbuf_reserve(&ringbuf, sizeof(*e), 0);
    if(!e) {
        return 0;
    }

    e->pid = pid;
    e->uid = bpf_get_current_uid_gid() & 0xFFFFFFFF;
    bpf_get_current_comm(&e->comm, sizeof(e->comm));
    e->ret = ret;

    bpf_ringbuf_submit(e, 0);
    return 0;
} 

char LICENSE[] SEC("license") = "GPL";
