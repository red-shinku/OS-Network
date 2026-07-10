#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

SEC("raw_tp/sys_enter")
int trace_system_enter(void *ctx)
{
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;
    u32 tid = (u32)pid_tgid;

    bpf_printk("[PID %d] [TID %d] entered syscall!", pid, tid);
    return 0;   
}

char LICENSE[] SEC("license") = "GPL";